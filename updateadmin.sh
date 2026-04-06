#!/usr/bin/env bash
# updateadmin.sh — 重置 GoBooks 系统管理员密码
#
# 用法：
#   bash updateadmin.sh
#   bash updateadmin.sh admin@example.com newpassword
#
# 依赖：psql、python3（用于 bcrypt hash 生成）
# 运行环境：项目根目录（与 .env 同级）

set -euo pipefail

# ── 1. 加载 .env ──────────────────────────────────────────────────────────────
ENV_FILE="$(dirname "$0")/.env"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "错误：找不到 .env 文件（期望路径：$ENV_FILE）"
  exit 1
fi

# 只提取 DB_* 变量，忽略注释和空行
while IFS='=' read -r key value; do
  [[ "$key" =~ ^#.*$ || -z "$key" ]] && continue
  key="$(echo "$key" | xargs)"
  value="$(echo "$value" | xargs)"
  case "$key" in
    DB_HOST)     DB_HOST="$value" ;;
    DB_PORT)     DB_PORT="$value" ;;
    DB_USER)     DB_USER="$value" ;;
    DB_PASSWORD) DB_PASSWORD="$value" ;;
    DB_NAME)     DB_NAME="$value" ;;
    DB_SSLMODE)  DB_SSLMODE="$value" ;;
  esac
done < "$ENV_FILE"

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-gobooks}"
DB_PASSWORD="${DB_PASSWORD:-gobooks}"
DB_NAME="${DB_NAME:-gobooks}"
DB_SSLMODE="${DB_SSLMODE:-disable}"

# ── 2. 读取参数或交互输入 ─────────────────────────────────────────────────────
if [[ $# -ge 2 ]]; then
  ADMIN_EMAIL="$1"
  NEW_PASSWORD="$2"
else
  read -rp "管理员 Email: " ADMIN_EMAIL
  read -rsp "新密码（至少8位）: " NEW_PASSWORD
  echo
  read -rsp "确认新密码: " NEW_PASSWORD_CONFIRM
  echo
  if [[ "$NEW_PASSWORD" != "$NEW_PASSWORD_CONFIRM" ]]; then
    echo "错误：两次输入的密码不一致"
    exit 1
  fi
fi

if [[ ${#NEW_PASSWORD} -lt 8 ]]; then
  echo "错误：密码至少需要8位"
  exit 1
fi

# ── 3. 生成 bcrypt hash ───────────────────────────────────────────────────────
# 优先用 python3 + bcrypt，与 Go bcrypt.DefaultCost(10) 一致
if ! python3 -c "import bcrypt" 2>/dev/null; then
  echo "正在安装 python3-bcrypt..."
  pip3 install bcrypt --quiet
fi

HASH=$(python3 - <<PYEOF
import bcrypt, sys
pw = sys.stdin.buffer.read()
h = bcrypt.hashpw(pw, bcrypt.gensalt(rounds=10))
print(h.decode())
PYEOF
<<< "$NEW_PASSWORD")

if [[ -z "$HASH" ]]; then
  echo "错误：bcrypt hash 生成失败"
  exit 1
fi

# ── 4. 更新数据库 ─────────────────────────────────────────────────────────────
export PGPASSWORD="$DB_PASSWORD"

# 检查用户是否存在
USER_COUNT=$(psql \
  -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
  --no-password -tAc \
  "SELECT COUNT(*) FROM sysadmin_users WHERE email = '$ADMIN_EMAIL';" 2>&1)

if [[ "$USER_COUNT" == "0" ]]; then
  echo "警告：找不到 email 为 '$ADMIN_EMAIL' 的管理员账号"
  echo ""
  echo "当前已有管理员账号："
  psql \
    -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
    --no-password -c \
    "SELECT id, email, is_active, created_at FROM sysadmin_users ORDER BY id;"
  exit 1
fi

# 执行更新
psql \
  -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
  --no-password -c \
  "UPDATE sysadmin_users
   SET password_hash = '$HASH',
       updated_at    = NOW()
   WHERE email = '$ADMIN_EMAIL';"

echo ""
echo "✓ 管理员密码已更新"
echo "  Email: $ADMIN_EMAIL"
echo "  请使用新密码登录 /admin/login"
