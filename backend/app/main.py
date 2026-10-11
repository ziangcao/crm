import os
from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import FastAPI, Request
from fastapi.responses import FileResponse, JSONResponse
from fastapi.staticfiles import StaticFiles

from . import auth, dashboard, routes
from .db import initialize


@asynccontextmanager
async def lifespan(app):
    initialize()
    yield


app = FastAPI(title="Cordial CRM API", version="0.1.0", lifespan=lifespan,
              docs_url="/api/docs", openapi_url="/api/openapi.json", redoc_url=None)


@app.middleware("http")
async def request_protection(request: Request, call_next):
    if request.url.path.startswith("/api") and request.method not in {"GET", "HEAD", "OPTIONS"}:
        origin = request.headers.get("origin")
        allowed = {value.strip() for value in os.getenv("CRM_ALLOWED_ORIGINS", "").split(",") if value.strip()}
        expected = request.url.scheme + "://" + request.headers.get("host", "")
        if origin and origin != expected and origin not in allowed:
            return JSONResponse({"detail": "不允许跨站请求"}, status_code=403)
        if request.headers.get("sec-fetch-site") == "cross-site":
            return JSONResponse({"detail": "不允许跨站请求"}, status_code=403)
        try:
            if int(request.headers.get("content-length", "0")) > 2_000_000:
                return JSONResponse({"detail": "请求不能超过 2 MB"}, status_code=413)
        except ValueError:
            return JSONResponse({"detail": "请求长度无效"}, status_code=400)
    response = await call_next(request)
    response.headers["X-Content-Type-Options"] = "nosniff"
    response.headers["X-Frame-Options"] = "DENY"
    response.headers["Referrer-Policy"] = "same-origin"
    if request.url.path.startswith("/api"):
        response.headers["Cache-Control"] = "no-store"
    return response


@app.get("/api/health")
def health():
    return {"status": "ok"}


app.include_router(auth.router)
app.include_router(routes.router)
app.include_router(dashboard.router)

dist = Path(os.getenv("CRM_FRONTEND", str(Path(__file__).resolve().parents[2] / "frontend" / "dist")))
if dist.is_dir():
    app.mount("/assets", StaticFiles(directory=str(dist / "assets")), name="assets")

    @app.get("/")
    def index():
        return FileResponse(str(dist / "index.html"))
