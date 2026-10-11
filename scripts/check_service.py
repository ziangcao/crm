"""Read-only HTTP smoke test for the built, single-origin deployment."""
import argparse
import json
import re
from urllib.parse import urljoin, urlsplit
from urllib.request import urlopen

parser = argparse.ArgumentParser(description="检查 CRM 页面、静态资源与接口，不修改业务数据")
parser.add_argument("url", nargs="?", default="http://127.0.0.1:8000")
args = parser.parse_args()
base = args.url.rstrip("/") + "/"


def get(path):
    address = urljoin(base, path)
    if urlsplit(address).netloc != urlsplit(base).netloc:
        raise RuntimeError("页面引用了非同源资源：" + address)
    with urlopen(address, timeout=15) as response:
        assert response.status == 200, path
        content = response.read()
        assert content, path
        return content


assert json.loads(get("api/health"))["status"] == "ok"
assert isinstance(json.loads(get("api/setup"))["required"], bool)
assert "openapi" in json.loads(get("api/openapi.json"))
page = get("").decode("utf-8")
assert '<div id="app"></div>' in page
assets = re.findall(r'(?:src|href)="(/assets/[^"?]+)"', page)
assert any(asset.endswith(".js") for asset in assets)
assert any(asset.endswith(".css") for asset in assets)
for asset in assets:
    get(asset)
print("PASS: health, setup, OpenAPI, HTML and %d static assets" % len(assets))
