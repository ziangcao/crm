"""Create a consistent Docker Compose backup of CRM data and uploads."""
import argparse
import json
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

parser = argparse.ArgumentParser(description="备份 Docker Compose 部署的 CRM 数据库和附件")
parser.add_argument("destination", type=Path, help="新的备份目录路径")
parser.add_argument("--stop-crm", action="store_true", help="确认暂停 CRM 服务，以取得数据库和附件的一致快照")
args = parser.parse_args()

if not args.stop_crm:
    parser.error("为保证数据库和附件一致，必须显式添加 --stop-crm")
if args.destination.exists():
    parser.error("目标备份目录已存在，请使用新的目录")

destination = args.destination.resolve()
temporary = destination.with_name(destination.name + ".partial")
if temporary.exists():
    parser.error("存在未完成的临时备份目录，请先检查后删除：" + str(temporary))


def compose(*command, stdout=None):
    subprocess.run(["docker", "compose", *command], check=True, stdout=stdout)


temporary.mkdir(parents=True)
stopped = False
try:
    compose("stop", "crm")
    stopped = True
    with (temporary / "database.dump").open("wb") as database:
        compose("exec", "-T", "postgres", "pg_dump", "--username=crm", "--dbname=crm", "--format=custom", "--no-owner", "--no-privileges", stdout=database)
    uploads = temporary / "uploads"
    uploads.mkdir()
    compose("cp", "crm:/app/uploads/.", str(uploads))
    (temporary / "manifest.json").write_text(json.dumps({
        "created_at": datetime.now(timezone.utc).isoformat(),
        "database": "database.dump",
        "uploads": "uploads",
    }, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    temporary.rename(destination)
    print("备份完成：" + str(destination))
except (OSError, subprocess.CalledProcessError) as error:
    shutil.rmtree(temporary, ignore_errors=True)
    print("备份失败：" + str(error), file=sys.stderr)
    sys.exit(1)
finally:
    if stopped:
        try:
            compose("start", "crm")
        except subprocess.CalledProcessError as error:
            print("CRM 服务未能自动启动，请执行 docker compose start crm：" + str(error), file=sys.stderr)
