import hashlib
import os
import re
import secrets
import time
from typing import Optional

from fastapi import APIRouter, Depends, Request, Response
from pydantic import BaseModel, ConfigDict, Field

from .db import audit, connection, now, password_hash, uid, verify_password
from .service import fail

router = APIRouter(prefix="/api")


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid")


class Login(StrictModel):
    username: str = Field(min_length=1, max_length=80)
    password: str = Field(min_length=1, max_length=200)


class Setup(Login):
    name: str = Field(min_length=1, max_length=80)
    demo: bool = False


class UserCreate(Setup):
    role: str = "sales"
    department: str = Field(default="", max_length=80)


class UserUpdate(StrictModel):
    name: str = Field(min_length=1, max_length=80)
    role: str
    department: str = Field(default="", max_length=80)
    active: bool = True
    password: Optional[str] = Field(default=None, max_length=200)


class PasswordChange(StrictModel):
    current_password: str = Field(max_length=200)
    new_password: str = Field(min_length=10, max_length=200)


def public_user(user):
    return {key: user[key] for key in ("id", "username", "name", "role", "department", "active")}


def current_user(request: Request):
    token = request.cookies.get("crm_session", "")
    with connection() as db:
        user = db.execute("""SELECT users.* FROM sessions JOIN users ON users.id=sessions.user_id
                             WHERE token=? AND expires>? AND active=1""",
                          (hashlib.sha256(token.encode()).hexdigest(), time.time())).fetchone()
        if not user:
            fail("请先登录", 401)
        return public_user(user)


def administrator(user=Depends(current_user)):
    if user["role"] != "admin":
        fail("需要管理员权限", 403)
    return user


def manager(user=Depends(current_user)):
    if user["role"] not in {"admin", "manager"}:
        fail("需要管理员或经理权限", 403)
    return user


def validate_password(password):
    if len(password) < 10 or not re.search(r"[A-Za-z]", password) or not re.search(r"\d", password):
        fail("密码至少 10 位，包含字母和数字")


def new_user(db, payload, role):
    validate_password(payload.password)
    if not re.fullmatch(r"[a-zA-Z0-9_.-]{3,40}", payload.username):
        fail("用户名须为 3–40 位字母、数字、点、横线或下划线")
    if not payload.name.strip():
        fail("姓名不能为空")
    if db.execute("SELECT 1 FROM users WHERE username=?", (payload.username,)).fetchone():
        fail("用户名已存在", 409)
    user_id = uid()
    db.execute("INSERT INTO users VALUES (?,?,?,?,?,?,1,?)", (
        user_id, payload.username, payload.name.strip(), password_hash(payload.password), role,
        getattr(payload, "department", ""), now()))
    return public_user(db.execute("SELECT * FROM users WHERE id=?", (user_id,)).fetchone())


def start_session(db, response, user_id):
    token = secrets.token_urlsafe(48)
    db.execute("DELETE FROM sessions WHERE expires<?", (time.time(),))
    db.execute("INSERT INTO sessions VALUES (?,?,?)", (hashlib.sha256(token.encode()).hexdigest(), user_id, time.time() + 43200))
    response.set_cookie("crm_session", token, max_age=43200, httponly=True,
                        secure=os.getenv("CRM_COOKIE_SECURE") == "1", samesite="strict", path="/api")


@router.get("/setup")
def setup_status():
    with connection() as db:
        return {"required": db.execute("SELECT count(*) FROM users").fetchone()[0] == 0}


@router.post("/setup", status_code=201)
def setup(payload: Setup, response: Response):
    with connection(write=True) as db:
        if db.execute("SELECT 1 FROM users LIMIT 1").fetchone():
            fail("系统已初始化", 409)
        user = new_user(db, payload, "admin")
        if payload.demo:
            from .seed import seed
            seed(db, user)
        audit(db, user, "初始化", "system")
        start_session(db, response, user["id"])
        return user


@router.post("/auth/login")
def login(payload: Login, response: Response):
    # Persist failed attempts even when authentication fails.
    error = False
    key = hashlib.sha256(payload.username.encode()).hexdigest()
    with connection(write=True) as db:
        attempt = db.execute("SELECT * FROM login_attempts WHERE key=?", (key,)).fetchone()
        if attempt and attempt["blocked_until"] > time.time():
            fail("尝试次数过多，请 15 分钟后再试", 429)
        user = db.execute("SELECT * FROM users WHERE username=? AND active=1", (payload.username,)).fetchone()
        if not user or not verify_password(payload.password, user["password"]):
            if not user:
                password_hash(payload.password, "authentication-timing-padding")
            failures = (attempt["failures"] if attempt and attempt["blocked_until"] == 0 else 0) + 1
            blocked = time.time() + 900 if failures >= 5 else 0
            db.execute("INSERT OR REPLACE INTO login_attempts VALUES (?,?,?)", (key, failures, blocked))
            error = True
        else:
            db.execute("DELETE FROM login_attempts WHERE key=?", (key,))
            start_session(db, response, user["id"])
            audit(db, user, "登录", "system")
            result = public_user(user)
    if error:
        fail("用户名或密码错误", 401)
    return result


@router.get("/auth/me")
def me(user=Depends(current_user)):
    return user


@router.post("/auth/logout")
def logout(request: Request, response: Response, user=Depends(current_user)):
    with connection(write=True) as db:
        db.execute("DELETE FROM sessions WHERE token=?", (hashlib.sha256(request.cookies.get("crm_session", "").encode()).hexdigest(),))
    response.delete_cookie("crm_session", path="/api")
    return {"ok": True}


@router.post("/auth/password")
def change_password(payload: PasswordChange, response: Response, user=Depends(current_user)):
    validate_password(payload.new_password)
    with connection(write=True) as db:
        old = db.execute("SELECT password FROM users WHERE id=?", (user["id"],)).fetchone()[0]
        if not verify_password(payload.current_password, old):
            fail("原密码错误", 400)
        db.execute("UPDATE users SET password=? WHERE id=?", (password_hash(payload.new_password), user["id"]))
        db.execute("DELETE FROM sessions WHERE user_id=?", (user["id"],))
        start_session(db, response, user["id"])
        audit(db, user, "修改密码", "system")
    return {"ok": True}


@router.get("/users")
def users(user=Depends(current_user)):
    with connection() as db:
        if user["role"] == "sales":
            return [user]
        return [public_user(row) for row in db.execute("SELECT * FROM users ORDER BY created_at")]


@router.post("/users", status_code=201)
def create_user(payload: UserCreate, user=Depends(administrator)):
    if payload.role not in {"admin", "manager", "sales"}:
        fail("角色无效")
    with connection(write=True) as db:
        created = new_user(db, payload, payload.role)
        audit(db, user, "创建用户", "system", created["id"], created["name"])
        return created


@router.put("/users/{user_id}")
def update_user(user_id: str, payload: UserUpdate, user=Depends(administrator)):
    if payload.role not in {"admin", "manager", "sales"} or not payload.name.strip():
        fail("姓名或角色无效")
    if user_id == user["id"] and (not payload.active or payload.role != "admin"):
        fail("不能停用自己或移除自己的管理员权限")
    with connection(write=True) as db:
        old = db.execute("SELECT * FROM users WHERE id=?", (user_id,)).fetchone()
        if not old:
            fail("用户不存在", 404)
        db.execute("UPDATE users SET name=?,role=?,department=?,active=? WHERE id=?", (
            payload.name.strip(), payload.role, payload.department, payload.active, user_id))
        if payload.password:
            validate_password(payload.password)
            db.execute("UPDATE users SET password=? WHERE id=?", (password_hash(payload.password), user_id))
        if payload.password or not payload.active or payload.role != old["role"]:
            db.execute("DELETE FROM sessions WHERE user_id=?", (user_id,))
        audit(db, user, "更新用户", "system", user_id, payload.name)
        return public_user(db.execute("SELECT * FROM users WHERE id=?", (user_id,)).fetchone())
