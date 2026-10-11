#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ -z "${CRM_DB_DSN:-}" ]]; then
  echo "请配置 CRM_DB_DRIVER 和 CRM_DB_DSN，或使用 docker compose up --build 启动。"
  exit 1
fi

npm run build --prefix frontend
go run ./backend/cmd/migrate
exec go run ./backend/cmd/server
