#!/usr/bin/env bash
# 伙伴 SparkPal 一键安装（自建）
#
#   curl -fsSL https://raw.githubusercontent.com/fengyun0608/sparkpal/main/deploy/install.sh | bash
#
# 做四件事：检查 docker → 拉代码 → 生成密钥 → 启动并打印地址与配对码

set -euo pipefail

REPO="${SPARKPAL_REPO:-https://github.com/fengyun0608/sparkpal.git}"
DIR="${SPARKPAL_DIR:-$HOME/sparkpal}"
PORT="${PORT:-3210}"

say() { printf '\033[1;36m%s\033[0m\n' "$*"; }
die() { printf '\033[1;31m%s\033[0m\n' "$*" >&2; exit 1; }

say "① 检查环境"
command -v docker >/dev/null 2>&1 || die "没装 docker。先跑：curl -fsSL https://get.docker.com | sh"
docker compose version >/dev/null 2>&1 || die "docker compose 不可用，请升级 docker"
if command -v free >/dev/null 2>&1; then
  MEM_MB=$(free -m | awk '/^Mem:/{print $2}')
  [ "$MEM_MB" -lt 900 ] && say "   注意：内存只有 ${MEM_MB}MB，扫码会排队很慢"
fi

say "② 取代码"
if [ -d "$DIR/.git" ]; then
  git -C "$DIR" pull --ff-only
else
  git clone --depth 1 "$REPO" "$DIR"
fi

cd "$DIR/deploy"

say "③ 生成配置"
if [ ! -f .env ]; then
  SECRET=$(head -c 48 /dev/urandom | base64 | tr -d '=+/' | head -c 48)
  PAIR=$(head -c 4 /dev/urandom | od -An -tu4 | tr -d ' \n' | tail -c 6)
  DBPASS=$(head -c 18 /dev/urandom | base64 | tr -d '=+/')
  cat > .env <<EOF
PORT=$PORT
JWT_SECRET=$SECRET
DB_PASSWORD=$DBPASS
PAIR_CODE=$PAIR
SERVER_NAME=我的伙伴服务器
EOF
  chmod 600 .env
  say "   已生成 .env（含随机密钥与配对码）"
else
  say "   .env 已存在，跳过"
fi

# shellcheck disable=SC1091
set -a; . ./.env; set +a

say "④ 启动"
mkdir -p ../bin
mkdir -p data/mysql

# 服务端二进制：优先用仓库里预编译的，没有就提示
if [ ! -x ../bin/spark-api ]; then
  cat >&2 <<'MSG'
   缺少 bin/spark-api。
   两种办法：
     a) 用官方发布页下载对应架构的二进制放到 deploy/../bin/spark-api
     b) 本机装了 Go 的话：cd ../server && go build -o ../bin/spark-api ./cmd/spark-api
MSG
  die "  继续之前先补上这个文件"
fi

docker compose up -d

say "⑤ 等待启动"
for i in $(seq 1 40); do
  if curl -fsS "http://127.0.0.1:${PORT}/api/spark/health" >/dev/null 2>&1; then break; fi
  sleep 1
done

IP=$(hostname -I 2>/dev/null | awk '{print $1}')
[ -z "${IP:-}" ] && IP="127.0.0.1"

cat <<EOF

┌────────────────────────────────────────────┐
│ 伙伴服务器已启动                            │
│                                            │
│   地址    http://${IP}:${PORT}              │
│   配对码  ${PAIR_CODE}                      │
│                                            │
│ 把这行地址填进 App 即可                     │
└────────────────────────────────────────────┘

管理命令：
  cd ${DIR}/deploy
  docker compose logs -f api     看日志
  docker compose restart api     重启
  docker compose down            停止
EOF
