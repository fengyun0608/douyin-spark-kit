# 扫码服务（Playwright）

给 App 出抖音登录二维码。单并发排队，一个会话占用一个浏览器。

```bash
npm i
node server.js        # 默认 127.0.0.1:3200
```

## 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/enqueue` | 入队，返回 `ticket` 与排位 |
| GET | `/status?ticket=` | `queued` / `running` / `ready` / `expired` / `error` |
| POST | `/cancel?ticket=` | 取消排队或关闭会话 |
| GET | `/health` | 会话数、队列长度、存活浏览器数 |
| POST | `/gc` | **强制清场**：清掉所有会话与浏览器 |

## 浏览器生命周期（踩过的三个坑）

这三个坑都真实发生过，直接把服务器内存吃光、让所有人「总失效」：

### ① 异常路径漏关浏览器

```js
page.goto() 抛异常时，browser 没有任何人关
→ 每失败一次漏一个 Chromium，一个约占 350MB
```

解决：所有启动的浏览器都登记进 `liveBrowsers`，由后台定时器兜底回收。

### ② 一个不扫，全员排队

`READY_TTL` 原来是 5.5 分钟：一个人拿到码不扫，后面所有人排在队里干等。
现在收到 **2.5 分钟**，并新增 `QUEUE_TTL`（排队 3 分钟丢弃）、`RUNNING_TTL`（生成卡 4 分钟强制释放）。

### ③ 回收器误杀正在生成的浏览器

`s.browser` 只在 `launchAndGrabQR()` 返回后才赋值，中间那段空窗里
watchdog 会把它当成孤儿关掉，worker 于是报「二维码没出来（可能被风控）」。

解决：新启动的浏览器打 `__inflight` 标记，且 20 秒内不动：

```js
function browserInUse(b) {
    if (b.__inflight) return true;                              // 正在生成
    if (Date.now() - (b.__bornAt || 0) < 20 * 1000) return true; // 太年轻
    for (const s of sessions.values()) if (s.browser === b) return true;
    return false;
}
```

## 运维

```bash
curl -s http://127.0.0.1:3200/health     # 看 sessions / browsers
curl -s -X POST http://127.0.0.1:3200/gc # 卡住了就清场
pkill -9 -f chrome-headless              # 最狠的兜底
```

## 资源

```
单个浏览器  约 300–400 MB（含 7 个渲染进程）
1 核 2G   同时只能跑 1 个，必须排队
```

**注意**：这个服务同时被星语和伙伴使用，共用一个队列。用户量上来后要给伙伴单独起一个实例，或者把 `READY_TTL` 再压短。
