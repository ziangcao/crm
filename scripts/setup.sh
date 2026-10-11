#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

npm ci --include=optional --prefix frontend
go mod download
echo "依赖安装完成。配置 CRM_DB_DRIVER、CRM_DB_DSN 后执行：bash scripts/start.sh"
