import hashlib
import hmac
import json
import os
import secrets
import sqlite3
from contextlib import contextmanager
from datetime import datetime, timezone
from pathlib import Path


def now():
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def uid():
    return secrets.token_hex(12)


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"))


def password_hash(password, salt=None):
    salt = salt or secrets.token_hex(16)
    digest = hashlib.pbkdf2_hmac("sha256", password.encode(), salt.encode(), 600000).hex()
    return salt + ":" + digest


def verify_password(password, saved):
    return hmac.compare_digest(password_hash(password, saved.split(":")[0]), saved)


@contextmanager
def connection(write=False):
    path = Path(os.getenv("CRM_DB", str(Path(__file__).parent.parent / "data" / "crm.sqlite3")))
    path.parent.mkdir(parents=True, exist_ok=True)
    db = sqlite3.connect(str(path), timeout=15)
    db.row_factory = sqlite3.Row
    db.execute("PRAGMA foreign_keys=ON")
    try:
        if write:
            db.execute("BEGIN IMMEDIATE")
        yield db
        db.commit()
    except Exception:
        db.rollback()
        raise
    finally:
        db.close()


def initialize():
    with connection() as db:
        db.execute("PRAGMA journal_mode=WAL")
        db.executescript("""
        CREATE TABLE IF NOT EXISTS users (
          id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
          password TEXT NOT NULL, role TEXT NOT NULL, department TEXT NOT NULL DEFAULT '',
          active INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL
        );
        CREATE TABLE IF NOT EXISTS sessions (
          token TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id), expires REAL NOT NULL
        );
        CREATE TABLE IF NOT EXISTS login_attempts (
          key TEXT PRIMARY KEY, failures INTEGER NOT NULL, blocked_until REAL NOT NULL
        );
        CREATE TABLE IF NOT EXISTS records (
          id TEXT PRIMARY KEY, module TEXT NOT NULL, data TEXT NOT NULL,
          owner_id TEXT REFERENCES users(id), version INTEGER NOT NULL DEFAULT 1,
          created_at TEXT NOT NULL, updated_at TEXT NOT NULL
        );
        CREATE INDEX IF NOT EXISTS records_module_owner ON records(module, owner_id);
        CREATE TABLE IF NOT EXISTS followups (
          id TEXT PRIMARY KEY, record_id TEXT NOT NULL REFERENCES records(id) ON DELETE CASCADE,
          user_id TEXT NOT NULL REFERENCES users(id), content TEXT NOT NULL, method TEXT NOT NULL,
          next_date TEXT, created_at TEXT NOT NULL
        );
        CREATE TABLE IF NOT EXISTS approvals (
          id TEXT PRIMARY KEY, record_id TEXT NOT NULL REFERENCES records(id),
          requested_by TEXT NOT NULL REFERENCES users(id), status TEXT NOT NULL,
          reviewed_by TEXT REFERENCES users(id), comment TEXT NOT NULL DEFAULT '',
          created_at TEXT NOT NULL, reviewed_at TEXT
        );
        CREATE UNIQUE INDEX IF NOT EXISTS approvals_one_pending
          ON approvals(record_id) WHERE status='pending';
        CREATE TABLE IF NOT EXISTS audit (
          id TEXT PRIMARY KEY, user_id TEXT REFERENCES users(id), action TEXT NOT NULL,
          module TEXT NOT NULL, record_id TEXT, detail TEXT NOT NULL, created_at TEXT NOT NULL
        );
        CREATE TABLE IF NOT EXISTS custom_fields (
          module TEXT NOT NULL, key TEXT NOT NULL, definition TEXT NOT NULL,
          PRIMARY KEY(module, key)
        );
        PRAGMA user_version=1;
        """)


def audit(db, user, action, module, record_id=None, detail=""):
    db.execute("INSERT INTO audit VALUES (?,?,?,?,?,?,?)",
               (uid(), user["id"], action, module, record_id, detail, now()))


def insert_record(db, module, data, owner):
    record_id = uid()
    timestamp = now()
    db.execute("INSERT INTO records VALUES (?,?,?,?,1,?,?)",
               (record_id, module, encode(data), owner, timestamp, timestamp))
    return record_id
