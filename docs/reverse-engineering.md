# 逆向记录：抖音网页登录与私信

> 全部结论都来自**自己的账号**上抓包 + 复现。
> 抖音会改，字段可能失效，看到不对就按文中的方法自己抓一遍。

---

## 一、二维码登录

### 1.1 接口

```
GET https://sso.douyin.com/get_qrcode/
POST/GET https://www.douyin.com/passport/web/get_qrcode/?...
```

**纯 HTTP 直接请求拿不到。** 实测：

```
HTTP 200   content-length: 0
bd-ticket-guard-result: 1101
```

或者返回一段 JS 挑战 HTML：

```html
<script>
  var e="10006", t="31827";
  document.cookie = "gfkadpd=" + ...
</script>
```

这是字节的 acrawler / ticket-guard 体系。需要：

- `a_bogus`：由混淆的 JS 生成
- `bd-ticket`：登录过程写进浏览器存储（不在 cookie 里）
- `msToken`：服务端下发的短期令牌

**只补 msToken 不够**（试过，仍是 1101）。

**可行做法：用真实浏览器**（Playwright，`channel: 'msedge'` 更稳）。

### 1.2 响应字段

```json
{
  "data": {
    "error_code": 0,
    "expire_time": 1795531229,
    "qrcode": "iVBORw0KGgoAAAANSUhEUg...",          // PNG base64
    "qrcode_index_url": "https://api.amemv.com/ucenter_web/...token=xxx",
    "token": "f0b43a3549cefda981e679bdd24cae06_hl",
    "is_frontier": true
  }
}
```

**关键理解：**

| 字段 | 是什么 | 能不能用来拉起抖音 |
|---|---|---|
| `qrcode` | 二维码 PNG 的 base64 | 显示用 |
| `qrcode_index_url` | 网页中间页 | ❌ 不能 |
| `token` | 轮询用 | 轮询 `check_qrconnect` |
| 二维码图里的内容 | `https://v.douyin.com/xxx/?hide_nav_bar=1` | ✅ **只能用它** |

实测对照（同一次会话）：

```
qrcode_index_url  https://api.amemv.com/ucenter_web/app/aweme/scan_login/index/...  371 字符
二维码真实内容     https://v.douyin.com/B0ISL29W7DM/?hide_nav_bar=1                   48 字符
```

### 1.3 拉起抖音 App

```js
const uri = `snssdk1128://webview/?url=${encodeURIComponent(shortUrl)}`;
```

Android：

```kotlin
val intent = Intent(Intent.ACTION_VIEW, Uri.parse(uri))
intent.setPackage("com.ss.android.ugc.aweme")   // 极速版 com.ss.android.ugc.aweme.lite
intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
startActivity(intent)
```

Android 11+ 必须声明包可见性，否则 `startActivity` 抛 `ActivityNotFoundException`：

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

**踩过的错**：直接

```kotlin
Intent(ACTION_VIEW, Uri.parse("https://api.amemv.com/...")).setPackage("com.ss.android.ugc.aweme")
```

→ 抛异常，系统回退到浏览器。因为抖音没给 `api.amemv.com` 注册 App Links。

### 1.4 登录态

扫完并在手机上点「确认登录」后，浏览器 context 里会出现：

```
sessionid=...        ← 判据
ttwid=...
passport_csrf_token=...
```

`sessionid` 是 HttpOnly，网页 JS 读不到，但原生 `CookieManager` 和 Playwright 的 `ctx.cookies()` 都能拿到。

---

## 二、为什么 `following/list` 拿不到全量

### 2.1 现象

```
GET https://www.douyin.com/aweme/v1/web/user/following/list/?...
HTTP 200
content-length: 0
bd-ticket-guard-result: 1101
```

即使带上完整 cookie（59 条）也是空 body。**不是 cookie 失效，是缺票据。**

### 2.2 试过的路

| 尝试 | 结果 |
|---|---|
| 带完整 cookie | 空 body |
| 加 `msToken`（回吐服务端下发的） | 仍 1101 |
| 换 `count` / `offset` / `max_time` 分页参数 | 无关，先被拦 |
| 页面内 `fetch`（同源、credentials: include） | 空 body |
| 浏览器里点「关注」标签，让页面自己发 | 会发请求，但仍是空 |

**结论**：这个接口只有在**登录握手刚完成的那个浏览器上下文**里才通行，因为票据在上下文存储中。

### 2.3 由此得出的架构

```
扫码登录 → 不要关浏览器
         → 用同一个 context 打开 /user/self
         → 点「关注」标签让页面自己请求
         → 旁路拦 following/list 响应
         → 收集到 has_more=false
```

**注意**：这一步很慢（滚动加载），**绝不能塞进登录确认那一步**，否则轮询接口会超时。

---

## 三、私信发送（不走浏览器）

见 [`im-protocol.md`](im-protocol.md)。

---

## 四、一份踩坑清单

| # | 坑 | 真相 |
|---|---|---|
| 1 | 手机 WebView 二维码永远转圈 | 抖音识别 WebView/自动化，得用桌面 UA + `useWideViewPort` 或服务端浏览器 |
| 2 | 改 `innerWidth` 伪造桌面 | ❌ 会把 SPA 布局搞崩，页面卡在 loading |
| 3 | 拿 `qrcode_index_url` 拉起抖音 | ❌ 必须用二维码图里的 `v.douyin.com` 短链 |
| 4 | `intent://` 交给 `url_launcher` | ❌ 它不解析，系统会开浏览器；要用原生 `Intent.parseUri` 或直接构造 scheme |
| 5 | 跨会话发送返回 7100 | 每个会话有独立 salt |
| 6 | 改 protobuf 的 `f3` 当时间戳 | ❌ 那是固定 salt `7479746146479407679`，改了会 7905 |
| 7 | 自己拼 `get_message_by_init` 的 body | ❌ `req.body is nil`，要用抓到的模板，只 patch 序号 |
| 8 | JS 用 `Math.pow(2,63)` 拼 varint | ❌ 精度丢失，时间戳错乱；必须 `BigInt` |
| 9 | `sec_uid` 判断用了 `s[:10]` 比 12 字符前缀 | ❌ 永远 false；用 `strings.HasPrefix` |
| 10 | 会话列表解析出 0 条 | 少剥一层 `f2043` 包装 |
