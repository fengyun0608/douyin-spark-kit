# 伙伴 SparkPal

> 让每一段火花 都不白攒。
>
> 抖音火花自动续期工具。客户端与服务端全部开源；官方托管只是「不用自己养服务器」。

本目录是 App 与服务端的完整实现，协议实现见仓库根目录的 `go/`、`src/` 与 `docs/`。

---

## 这是什么

```
扫码绑定抖音 → 勾选要保的好友 → 每天固定时间自动发一条 → 火花不断
```

## 两种用法

| | 官方托管 | 自建 |
|---|---|---|
| 地址 | `https://114.66.28.185:48285/api/spark` | 自己部署 |
| 费用 | 当前免费（内测需邀请码） | 免费 |
| 服务器 | 我们养 | 你养 |
| 功能 | 一样 | 一样 |

**客户端只有一份代码**，差异全部由服务端 `/api/spark/meta` 返回的能力决定：

```json
{ "official": true, "billing": false, "needInvite": true, "quota": {...} }
```

客户端只写这一行判断，没有任何「是不是官方」的硬编码：

```dart
if (meta.billing) showSubscriptionEntry();
```

---

## 目录

```
sparkpal/
├── app/       Flutter 客户端（Android）
├── server/    Go 服务端
├── deploy/    docker-compose / nginx / 一键安装
└── scripts/   构建与门禁脚本
```

---

## 服务端接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/spark/health` | 存活 |
| GET | `/api/spark/meta` | **能力声明（客户端分叉点）** |
| POST | `/api/spark/auth/invite/check` | 校验邀请码 |
| POST | `/api/spark/auth/sms/send` | 发验证码 |
| POST | `/api/spark/auth/sms/verify` | 校验 → token |
| POST | `/api/spark/auth/pair` | 自建配对码换 token |
| GET | `/api/spark/auth/me` | 当前身份与额度 |
| POST | `/api/spark/bind/qr/enqueue` | 抖音扫码入队出码 |
| GET | `/api/spark/bind/qr/status` | 轮询；confirmed 后落库 |
| GET | `/api/spark/accounts` | 已绑定的抖音号 |

## 环境变量

```bash
PORT=3210
DB_DSN='user:pass@tcp(127.0.0.1:3306)/sparkpal?parseTime=true&charset=utf8mb4'
JWT_SECRET='...'
MODE='cloud'          # cloud | self
OFFICIAL='true'
BILLING='false'       # true 才出现订阅入口
NEED_INVITE='true'    # 邀请码独立开关
SERVER_NAME='伙伴官方'
QR_BASE='http://127.0.0.1:3200'   # Playwright 扫码服务，留空则关闭绑定
SMS_WEBHOOK=''        # 留空则验证码打到日志（仅开发）
SMS_KEY=''
```

> `.env` 里的值**必须加引号**，`tcp(...)` 里的括号会让 bash 解析失败。

---

## 构建

```powershell
# 发版：混淆 + 分离符号表
.\scripts\build-release.ps1

# 不混淆（调试）
.\scripts\build-release.ps1 -NoObfuscate
```

**为什么脚本要换目录构建**：Flutter 的 AOT 编译器（gen_snapshot）在 Windows 上读不了含中文的路径，项目放在 `D:\项目\开源\` 下时 `--release` 必然失败。脚本先镜像到 `D:\dev\sparkpal_build` 再构建，最后把产物拷回 `dist/`。

**符号表必须保留**：混淆后线上崩溃要用它还原堆栈。

## 质量门禁

```
本地   scripts/hooks/pre-commit     提交前跑 analyze / go vet / test
远端   .github/workflows/ci.yml     客户端 + 服务端 + 密钥检查三道门
```

安装本地钩子：

```bash
cp sparkpal/scripts/hooks/pre-commit .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

---

## 当前进度

```
[x] 服务端：meta / 邀请码 / 手机号登录 / 配对码 / JWT
[x] 抖音绑定：出码 / 轮询 / 解析身份 / 好友入库
[x] cookie AES-256 加密存储
[x] Flutter 客户端：选服务器 / 邀请码 / 登录 / 配对 / 首页
[x] 混淆发版 + 符号表分离
[x] 本地钩子 + GitHub Actions 门禁
[ ] 好友勾选与定时任务
[ ] 订阅与支付
```

## 许可证

MIT
