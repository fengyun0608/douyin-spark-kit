# douyin-spark-kit · 续火工具箱

> 抖音「自动续火花」背后的技术拆解：扫码登录、二维码短链、拉起 App、私信发送协议。
>
> **只服务于你自己的账号。** 不包含任何爬取他人数据的功能。

---

## 这个仓库解决什么问题

想让好友之间的「火花」不断，需要每天发一条消息。手动发很烦，所以要做自动化。

真正难的不是定时，是下面这四件事：

| 难点 | 现象 | 本仓库的答案 |
|---|---|---|
| 拿登录态 | 手机 WebView 里二维码永远转圈 | 用 Playwright + 反检测，服务端生成 |
| 二维码拿不到有效期 | 图片有了但不知道内容 | `get_qrcode` 返回的 `qrcode` 就是 PNG，解码得到 `v.douyin.com` 短链 |
| 点一下就跳抖音 | 直接开 https 网址只会跳浏览器 | 必须包成 `snssdk1128://webview/?url=<encode(短链)>` |
| 浏览器之外发消息 | 纯 HTTP 调接口返回空 | IM 接口走 protobuf，且**每个会话有独立 salt** |

---

## 快速开始

### 1. 起二维码服务

```bash
npm install
PLAYWRIGHT_DOWNLOAD_HOST=https://cdn.npmmirror.com/binaries/playwright \
  npx playwright install chromium

node src/qr-login.js
# → http://127.0.0.1:3200
```

```bash
# 拿二维码
curl -X POST http://127.0.0.1:3200/start
# {
#   "ticket": "abc123",
#   "qr": "iVBORw0KGgoAAAANSUhEUg...",        ← PNG base64，直接显示
#   "shortUrl": "https://v.douyin.com/xxxx/?hide_nav_bar=1",
#   "expireAt": 1795531229000
# }

# 轮询
curl 'http://127.0.0.1:3200/status?ticket=abc123'
# {"status":"ready"}          ← 等人扫
# {"status":"confirmed","cookie":"ttwid=...; sessionid=...; ..."}
```

### 2. 拉起抖音

```js
const uri = `snssdk1128://webview/?url=${encodeURIComponent(shortUrl)}`;
// Android: Intent(ACTION_VIEW, uri).setPackage("com.ss.android.ugc.aweme")
```

Android 11+ 还要在 `AndroidManifest.xml` 声明包可见性：

```xml
<queries>
  <package android:name="com.ss.android.ugc.aweme" />
  <package android:name="com.ss.android.ugc.aweme.lite" />
  <intent>
    <action android:name="android.intent.action.VIEW" />
    <data android:scheme="snssdk1128" />
  </intent>
</queries>
```

### 3. 发私信

见 [`docs/im-protocol.md`](docs/im-protocol.md) 和 [`go/im/`](go/im/)。

---

## 目录

```
src/
  qr-login.js          二维码登录服务（Playwright）
  decode-qr.js         从二维码图里解出短链
go/im/
  proto.go             手写 protobuf 编解码
  client.go            IM 客户端 + 发送
docs/
  reverse-engineering.md   完整逆向过程（最值得看的一篇）
  im-protocol.md           私信协议字段布局
  open-source-scope.md     开源范围说明
```

---

## 三个坑，值不值钱自己判断

### 坑 1：二维码里不是 `qrcode_index_url`

```
qrcode_index_url   https://api.amemv.com/ucenter_web/...token=xxx   371 字符
二维码真实内容      https://v.douyin.com/B0ISL29W7DM/?hide_nav_bar=1  48 字符
```

**两个完全不同的东西。** 用前者拉起抖音必然失败。

### 坑 2：`bd-ticket-guard-result: 1101`

纯 HTTP 请求 `following/list` 会返回 **200 + 空 body**，响应头带 `bd-ticket-guard-result: 1101`。

原因是缺 `bd-ticket` 票据 —— 这个票据是**登录过程写进浏览器存储的**，不在 cookie 里。

**唯一可靠解法**：用刚完成扫码登录的那个浏览器上下文去请求，让抖音自己的 JS 生成签名。

### 坑 3：跨会话发送 7100

同一个 salt 发不同的人 → `status=7100`。

**每个会话有独立的 salt**，等于该会话最后一条消息的 `f5` 字段。必须按会话存。

---

## 免责声明

- 本项目仅用于**学习浏览器自动化与私有协议**，以及自动化**自己的**账号
- 请遵守抖音用户协议；因使用本项目导致的账号风险由使用者自行承担
- 不提供批量骚扰、爬取他人隐私、绕过风控牟利等能力，也不要往这个方向改

## License

MIT

---

## 姊妹项目：伙伴 SparkPal

基于本仓库的协议实现，做了一个面向普通用户的 App：

`
App（Android）   扫码绑定 → 勾选好友 → 每天自动发 → 火花不断
服务端            自建一条命令部署，或直接用官方托管
`

- 客户端与服务端**全部开源**（MIT）
- 官方托管的差别只有一处：**不用自己养服务器**
- UI 预览：ssets/ui-preview.png
- 方案：docs/sparkpal-monorepo-plan.md

