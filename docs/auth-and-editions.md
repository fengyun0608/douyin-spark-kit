# 伙伴 SparkPal · 登录体系与双版本

> 补充 `huoban-blueprint.md`，定稿登录架构与开源/官方版边界。

---

## 一、核心原则

**收费不放在客户端，放在服务端。**

```
同一份代码，两种部署形态，靠「连哪台服务器」决定体验差异
```

| | 开源版（自建） | 官方版（托管） |
|---|---|---|
| 代码 | 完全一样 | 完全一样 |
| 连的服务器 | 你自己的 | 我们的 |
| 登录方式 | 填服务器地址 + 扫码配对 | 手机号 / 邮箱 |
| 订阅 | 无（自建服务器返回无限制） | 有，按绑定账号额度 |
| 谁养服务器 | 你 | 我们 |
| 抖音改版谁跟 | 你 | 我们 |
| 数据 | 在你机器上 | 在我们这，多端同步 |

**为什么这样设计**：
```
· 代码开源，客户端藏不住付费开关
· 但订阅校验在服务端，开源版改了客户端也没用
· 官方版的价值不是「解锁功能」，是「替你运维」
```

---

## 二、构建开关

同一仓库，两个构建目标：

```bash
# 官方版
flutter build apk --dart-define=SERVER_MODE=cloud

# 开源版（默认）
flutter build apk --dart-define=SERVER_MODE=self
```

### 差异只在这几处

```dart
class DeployMode {
  static const bool isCloud =
      String.fromEnvironment('SERVER_MODE', defaultValue: 'self') == 'cloud';
}
```

| 首屏 | cloud | self |
|---|---|---|
| 欢迎页 | 一句话价值 + 「手机号登录」 | 一句话价值 + 「连接我的服务器」 |
| 登录页 | 手机号 + 验证码 | 服务器地址 + 配对码 |
| 设置页 | 订阅、订单、恢复购买 | 服务器地址、导出数据 |
| 额度 | 由服务端 plan 决定 | 服务端固定返回 unlimited |

**其余 100% 相同的代码。**

---

## 三、官方版登录体系

### 3.1 主路径：手机号 + 验证码

```
输入手机号 → 发验证码（6 位，5 分钟有效）
→ 校验 → 首次注册自动建账号
→ 返回 access_token（JWT，30 天）+ refresh_token（90 天）
```

**为什么是手机号**：
```
国内用户最熟悉，转化最高
不需要记密码
天然防批量注册（一个号一条短信）
```

**成本**：短信约 ¥0.04/条，注册一次 1 条 + 找回 1 条，可接受。

### 3.2 备用路径：邮箱 + 密码

```
给海外用户 / 不想留手机号的用户
邮箱验证码注册 → 设置密码 → 之后密码登录
```

### 3.3 设备管理

```
一个账号最多 3 台设备（Pro）
新设备登录 → 老设备列表里能看到，可踢
超过设备数 → 提示踢掉一台
```

### 3.4 账号与抖音号的解耦（关键）

```
HuoUser（伙伴账号） 1 ──── N HuoDouyin（绑定的抖音号）
```

```
为什么不能直接用抖音扫码当账号：
· Pro 要绑 3 个抖音号，需要一个父账号
· 用户换抖音号 → 订阅不能跟着丢
· 多设备登录 → 需要一个稳定身份
```

### 3.5 账号能做的事，只有三件

```
① 订阅（订阅挂在账号上，不挂在抖音号上）
② 多设备（换手机不丢好友勾选和历史）
③ 数据归属（注销即删，可导出）
```

**不发消息、不读抖音数据、不做社交。** 账号的功能边界必须窄。

---

## 四、自建版登录体系

### 4.1 不需要账号

```
首次打开
  → 填服务器地址（如 http://192.168.1.10:3200）
  → 服务器生成 6 位配对码，显示在服务器终端 / 网页
  → App 输入配对码
  → 服务器返回长期 token，存本地
```

**配对码的作用**：防止别人扫到你的服务器地址就能用。

### 4.2 服务器端

```go
// 自建模式下，所有额度检查直接放行
func (s *Server) checkQuota(userID string) Quota {
    if s.Mode == ModeSelfHosted {
        return Quota{Plan: "unlimited", MaxAccounts: -1, MaxFriends: -1, MaxDaily: -1}
    }
    return s.loadQuota(userID)
}
```

**一行判断，没有隐藏逻辑。** 开源版用户看得见，也改得动——但那台服务器是他自己的。

---

## 五、OpenAPI 与 Token

```
POST /api/spark/auth/sms/send      发验证码（cloud）
POST /api/spark/auth/sms/verify    校验 → token（cloud）
POST /api/spark/auth/email/login   邮箱密码登录（cloud）
POST /api/spark/auth/pair          配对码换 token（self）
GET  /api/spark/auth/me            当前身份 + 额度
POST /api/spark/auth/logout

Authorization: Bearer <token>
```

**token 内容**：
```json
{"uid":"...", "plan":"pro", "exp":..., "mode":"cloud"}
```

---

## 六、额度模型

```go
type Quota struct {
    Plan         string // free | pro | team | unlimited
    MaxAccounts  int    // -1 = 不限
    MaxFriends   int
    MaxDaily     int
    MaxDevices   int
    Features     map[string]bool
}
```

```
free      1 账号 / 10 好友 / 5 条每日 / 1 设备
pro       3 账号 / 全部好友 / 50 条 / 3 设备
team      20 账号 / 全部 / 500 条 / 不限设备
unlimited 全 -1（自建）
```

**额度检查点只有三处**：
```
① 绑抖音号时  检查 MaxAccounts
② 拉好友时    检查 MaxFriends
③ 发送时      检查 MaxDaily
```

---

## 七、订阅与收费入口

### 7.1 入口位置（仅 cloud 构建出现）

```
设置页 → 「升级专业版」
首页倒计时卡下方（火花最危险时出现）→ 「守护更多火花」
绑定第 2 个抖音号时 → 「专业版可绑 3 个」
```

### 7.2 支付

```
Android  微信 / 支付宝（自有码支付，星语已跑通）
iOS      暂不做
```

### 7.3 订阅状态

```
服务端为准，客户端只展示
每次启动拉一次 /api/spark/auth/me
到期前 3 天 / 1 天各推一次提醒
```

---

## 八、安全

```
· 手机号验证码：同一号码 60 秒 1 条，每天 10 条上限
· 配对码：6 位，5 分钟有效，一次性
· token：JWT，HS256，密钥走环境变量
· cookie：AES-256 加密入库
· 接口永不返回完整 cookie
· 注销：物理删除账号 + 抖音号 + 好友 + 日志
```

---

## 九、最终定稿

| 项 | 决定 |
|---|---|
| 中文名 | **伙伴** |
| 英文名 | SparkPal |
| 范围 | 只做「火花」相关，不做其他平台、不做社交 |
| 客户端 | 开源 MIT |
| 服务端 | 开源 MIT |
| 收费 | 只收托管服务，按绑定抖音号数量 |
| 客户端差异 | 只有一个构建开关，决定首屏与订阅入口 |
| 官方登录 | 手机号验证码（主）+ 邮箱密码（备） |
| 自建登录 | 服务器地址 + 配对码，无账号 |
| 账号职责 | 只管订阅 / 多设备 / 数据归属 |
