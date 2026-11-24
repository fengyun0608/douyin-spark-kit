# 私信协议（不走浏览器）

> 全部字段来自**自己账号**的抓包与复现。
> 结论：不需要 `a_bogus`、不需要 WebView，只要 cookie + protobuf。

---

## 一、基本信息

```
POST https://imapi.douyin.com/v1/message/send
Header: Cookie: <登录后的完整 cookie>
Header: User-Agent: <桌面 Edge/Chrome UA>

Body: protobuf（自定义，非标准 IM 协议）
```

`v1/message/send` 是 cmd `100`（放在 protobuf field 1）。

**不需要签名。** 这一点很重要 —— 网上很多说法要求 `a_bogus`，实际 IM 接口不要求。

---

## 二、protobuf 结构

手写编解码即可，不需要 `.proto` 文件。见 [`go/im/proto.go`](../go/im/proto.go)。

顶层字段：

| field | wire | 含义 |
|---|---|---|
| 1 | varint | cmd，发送是 `100` |
| 2 | varint | 序号，**严格递增**，重复会 `req.body is nil` |
| 3 | varint | salt（见下节，最容易踩坑的地方） |
| 5 | bytes  | 键值对列表 |

`field 5` 里是若干 `kvPair`，每个 `kvPair` 由 `key(bytes)` + `value(bytes)` 组成，值是 JSON 字符串。

---

## 三、salt：每个会话不一样

### 现象

| 场景 | 结果 |
|---|---|
| 用固定值 `7479746146479407679` 发互关好友 | ✅ 成功 |
| 用同一个值发另一个会话的人 | ❌ `status=7100` |
| 把 `f3` 改成当前时间戳 | ❌ `status=7905`（风控） |

### 真相

- `f3` **不是时间戳**，是 salt
- 互关（`follow_status=2`）的人可以用**账号级固定 salt**：`7479746146479407679`
- 单向关注 + 有聊天记录 → 必须用**该会话自己的 salt**
- 会话 salt 来自该会话最后一条消息的 `f5` 字段，需要按会话单独存

```
DouyinFriend.convSalt   ← 每个会话存一份
```

### 判定逻辑

```
if 互关:        salt = 7479746146479407679
elif 有聊天记录: salt = 该会话的 convSalt
else:           发不出去（7100，需要先和 TA 聊一句）
```

---

## 四、错误码

| 码 | 含义 | 处理 |
|---|---|---|
| `0` | 成功 | — |
| `7100` | 需要先和 TA 聊一句 / salt 不匹配 | 换该会话的 salt，或先手动发一条 |
| `7905` | 风控 | salt 或序号错了，检查 `f3` 是否被改动 |
| `-14` | 登录过期 | 重新扫码 |

---

## 五、会话列表

```
POST https://imapi.douyin.com/v1/message/get_message_by_init
field 1 = 2043
```

### 关键坑：不能自己拼 body

自己构造的 protobuf 会被拒：`req.body is nil`。

**正确做法**：抓一份真实请求的 body 存成模板（约 713 字节），之后每次只 patch：

- `field 2`（序号，必须递增）
- 需要翻页时 patch 游标字段

同理，发送也要保存一份约 1417 字节的 `message/send` 模板。

### 响应解析要多剥一层

```
响应 f6 → 里面还有一层 f2043 包装 → 才是会话数组
```

少剥这一层，解析结果永远是 0 条。

### 会话 ID 格式

```
0:1:<自己的uid>:<对方uid>
```

可以从中反推自己的数字 uid，不用额外接口。

### sec_uid 判断

`sec_uid` 形如 `MS4wLjABAAAA...`（12 字符前缀）。

```go
// ❌ 错：比 10 个字符
if s[:10] == "MS4wLjABAAAA" { }

// ✅ 对
if strings.HasPrefix(s, "MS4wLjABAAAA") { }
```

---

## 六、JS 里拼 varint 的坑

时间戳是 64 位，超过 `2^53` 后 JS 的 `number` 会丢精度：

```js
// ❌ 尾数被截断，抖音收到的时间戳是错的
const v = Math.floor(Date.now() / 1000);
// ✅
const v = BigInt(Date.now()) / 1000n;
```

---

## 七、最小可用流程

```
1. 扫码或上传 cookie 拿到 sessionid
2. get_message_by_init → 会话列表（含 convId / secUid / 昵称 / 头像 / salt）
3. 选定目标 → 按上面的规则决定用哪个 salt
4. message/send 发送
5. 需要撤回 → v1/message/recall（cmd 候选 201~210，需逐个探测）
```

---

## 八、免责

仅供学习协议与自有账号自动化。**不要**用于群发骚扰、爬取他人隐私。
