#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="${ENV_FILE:-$ROOT_DIR/.env}"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT_DIR/deploy/docker-compose.yml}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker 未安装，无法更新 CRM。" >&2
  exit 1
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "未找到环境变量文件: $ENV_FILE" >&2
  exit 1
fi

if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "未找到部署编排文件: $COMPOSE_FILE" >&2
  exit 1
fi

export CRM_ENV_FILE="$ENV_FILE"
compose=(docker compose -p crm --env-file "$ENV_FILE" -f "$COMPOSE_FILE")

"${compose[@]}" pull crm
"${compose[@]}" up -d --remove-orphans

for attempt in $(seq 1 30); do
  if "${compose[@]}" exec -T crm wget -q -O /dev/null http://127.0.0.1:8000/api/health; then
    "${compose[@]}" ps
    exit 0
  fi
  sleep 2
done

"${compose[@]}" logs --tail=100 crm
echo "CRM 健康检查失败。" >&2
exit 1
