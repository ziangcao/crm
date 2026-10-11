import json
import re
from datetime import date
from typing import Any, Dict, List, Optional

from fastapi import APIRouter, Depends, Query, Response
from pydantic import Field

from .auth import StrictModel, administrator, current_user, manager
from .catalog import APPROVABLE, MODULES
from .db import audit, connection, encode, insert_record, now, uid
from .service import (check_version, create, definition, editable, export_csv, fail,
                      get_record, references_to, serialize, transfer_customer, update_data,
                      validate, visible_records)

router = APIRouter(prefix="/api")


class RecordInput(StrictModel):
    data: Dict[str, Any]
    version: Optional[int] = None


class ActionInput(StrictModel):
    version: int = Field(ge=1)
    owner_id: Optional[str] = None


class FollowupInput(StrictModel):
    content: str = Field(min_length=1, max_length=5000)
    method: str = "电话"
    next_date: Optional[date] = None


class DecisionInput(StrictModel):
    decision: str
    comment: str = Field(default="", max_length=2000)


class CustomFieldInput(StrictModel):
    key: str = Field(pattern=r"^custom_[a-z][a-z0-9_]{0,35}$")
    label: str = Field(min_length=1, max_length=40)
    type: str = "text"
    options: List[str] = Field(default_factory=list, max_length=30)


class ImportInput(StrictModel):
    records: List[Dict[str, Any]] = Field(min_length=1, max_length=500)


def business_data(db, module, data):
    return {f["key"]: data.get(f["key"]) for f in definition(db, module)["fields"]}


def filtered(db, module, user, q, scope, status):
    for row in visible_records(db, module, user, scope):
        data = json.loads(row["data"])
        if q and q.casefold() not in " ".join(str(v) for v in data.values()).casefold():
            continue
        if status and status not in {data.get("status"), data.get("stage"), data.get("approval_status")}:
            continue
        yield row


@router.get("/catalog")
def catalog(user=Depends(current_user)):
    with connection() as db:
        return [definition(db, key) for key in MODULES]


@router.post("/catalog/{module}/fields", status_code=201)
def add_field(module: str, payload: CustomFieldInput, user=Depends(administrator)):
    if payload.type not in {"text", "textarea", "number", "date", "select", "email"}:
        fail("不支持的字段类型")
    if not payload.label.strip():
        fail("字段名称不能为空")
    if payload.type == "select" and (not payload.options or any(not v.strip() or len(v) > 100 for v in payload.options)):
        fail("请提供有效的下拉选项")
    with connection(write=True) as db:
        existing = definition(db, module)["fields"]
        if any(f["key"] == payload.key for f in existing):
            fail("字段标识已存在", 409)
        if len(existing) >= 60:
            fail("每个模块最多 60 个字段")
        value = dict(payload.model_dump(), required=False)
        db.execute("INSERT INTO custom_fields VALUES (?,?,?)", (module, payload.key, encode(value)))
        audit(db, user, "新增自定义字段", module, detail=payload.label)
        return value


@router.get("/records/{module}")
def list_records(module: str, q: str = Query(default="", max_length=200),
                 scope: str = Query(default="all", pattern="^(all|mine|pool)$"),
                 status: str = "", page: int = Query(default=1, ge=1),
                 size: int = Query(default=20, ge=1, le=100), user=Depends(current_user)):
    with connection() as db:
        rows = list(filtered(db, module, user, q, scope, status))
        return {"items": [serialize(db, r) for r in rows[(page - 1) * size:page * size]],
                "total": len(rows), "page": page, "size": size}


@router.get("/records/{module}/export")
def export(module: str, q: str = "", scope: str = "all", status: str = "", user=Depends(current_user)):
    with connection() as db:
        rows = list(filtered(db, module, user, q, scope, status))
        if len(rows) > 10000:
            fail("单次导出最多 10000 条，请缩小筛选范围")
        audit(db, user, "导出", module, detail=str(len(rows)) + " 条")
        return Response(export_csv(db, module, rows), media_type="text/csv; charset=utf-8",
                        headers={"Content-Disposition": 'attachment; filename="' + module + '.csv"'})


@router.post("/records/{module}/import", status_code=201)
def import_records(module: str, payload: ImportInput, user=Depends(current_user)):
    with connection(write=True) as db:
        definition(db, module)
        items = [create(db, module, data, user) for data in payload.records]
        audit(db, user, "批量导入", module, detail=str(len(items)) + " 条")
        return {"count": len(items)}


@router.post("/records/{module}", status_code=201)
def create_record(module: str, payload: RecordInput, user=Depends(current_user)):
    with connection(write=True) as db:
        definition(db, module)
        return create(db, module, payload.data, user)


@router.get("/records/{module}/{record_id}")
def read_record(module: str, record_id: str, user=Depends(current_user)):
    with connection() as db:
        row = get_record(db, module, record_id, user)
        result = serialize(db, row)
        result["followups"] = [dict(r) for r in db.execute("""SELECT f.*,u.name AS user_name FROM followups f
          JOIN users u ON u.id=f.user_id WHERE record_id=? ORDER BY f.created_at DESC, f.id""", (record_id,))]
        result["approvals"] = [dict(r) for r in db.execute("""SELECT a.*,u.name AS requester_name,v.name AS reviewer_name
          FROM approvals a JOIN users u ON u.id=a.requested_by LEFT JOIN users v ON v.id=a.reviewed_by
          WHERE record_id=? ORDER BY a.created_at DESC""", (record_id,))]
        related = []
        for other_module in MODULES:
            for other in visible_records(db, other_module, user):
                data = json.loads(other["data"])
                if any(v == record_id for k, v in data.items() if k.endswith("_id")):
                    related.append(serialize(db, other))
        result["related"] = related
        return result


@router.put("/records/{module}/{record_id}")
def edit_record(module: str, record_id: str, payload: RecordInput, user=Depends(current_user)):
    with connection(write=True) as db:
        row = get_record(db, module, record_id, user, write=True)
        check_version(row, payload.version)
        editable(row)
        old = json.loads(row["data"])
        clean = validate(db, module, payload.data, user, record_id)
        if old.get("customer_id") and clean.get("customer_id") != old["customer_id"]:
            fail("已创建记录的所属客户不能更改")
        if list(references_to(db, record_id)):
            for key in ("amount", "contract_id", "items"):
                if old.get(key) != clean.get(key):
                    fail("记录已有下游关联，不能修改金额、合同或产品明细")
        if module in APPROVABLE:
            clean["approval_status"] = "draft"
        if module == "opportunities" and clean["stage"] in {"赢单", "输单"}:
            clean["probability"] = 100 if clean["stage"] == "赢单" else 0
        update_data(db, row, clean)
        audit(db, user, "更新", module, record_id, clean["name"])
        return serialize(db, get_record(db, module, record_id, user))


@router.delete("/records/{module}/{record_id}")
def delete_record(module: str, record_id: str, version: int, user=Depends(current_user)):
    with connection(write=True) as db:
        row = get_record(db, module, record_id, user, write=True)
        check_version(row, version)
        editable(row)
        if list(references_to(db, record_id)):
            fail("记录存在关联数据，不能删除", 409)
        if db.execute("SELECT 1 FROM approvals WHERE record_id=?", (record_id,)).fetchone():
            fail("记录已有审批历史，需保留追溯，不能删除", 409)
        audit(db, user, "删除", module, record_id, json.loads(row["data"])["name"])
        db.execute("DELETE FROM records WHERE id=?", (record_id,))
        return {"ok": True}


@router.post("/records/{module}/{record_id}/followups", status_code=201)
def add_followup(module: str, record_id: str, payload: FollowupInput, user=Depends(current_user)):
    if payload.method not in {"电话", "微信", "邮件", "拜访", "会议", "其他"} or not payload.content.strip():
        fail("跟进方式或内容无效")
    with connection(write=True) as db:
        get_record(db, module, record_id, user, write=True)
        db.execute("INSERT INTO followups VALUES (?,?,?,?,?,?,?)", (uid(), record_id, user["id"],
                   payload.content.strip(), payload.method, str(payload.next_date) if payload.next_date else None, now()))
        audit(db, user, "添加跟进", module, record_id)
        return {"ok": True}


@router.post("/records/{module}/{record_id}/actions/{action}")
def record_action(module: str, record_id: str, action: str, payload: ActionInput, user=Depends(current_user)):
    with connection(write=True) as db:
        row = get_record(db, module, record_id, user, write=action != "claim")
        check_version(row, payload.version)
        data = json.loads(row["data"])
        if action in {"claim", "release", "assign"}:
            if module not in {"customers", "leads", "tasks"} or (module == "tasks" and action != "assign"):
                fail("该模块不支持此操作")
            if data.get("converted_customer_id"):
                fail("已转化线索请通过客户进行分配")
            if action == "claim":
                if row["owner_id"] is not None:
                    fail("该记录已被领取", 409)
                owner = user["id"]
            elif action == "release":
                if row["owner_id"] is None:
                    fail("记录已在公海", 409)
                owner = None
            else:
                if user["role"] == "sales":
                    fail("需要经理或管理员权限", 403)
                owner = payload.owner_id
                if not db.execute("SELECT 1 FROM users WHERE id=? AND active=1", (owner,)).fetchone():
                    fail("请选择有效负责人")
            transfer_customer(db, row, owner)
        elif action == "convert":
            if module != "leads":
                fail("仅线索支持转化")
            editable(row)
            if data["status"] == "无效" or row["owner_id"] is None:
                fail("无效或未领取的线索不能转化")
            owner = row["owner_id"]
            customer = create(db, "customers", dict(name=data["company"], industry=data["industry"],
                source=data["source"], level="B · 潜力", status="潜在客户", phone=data.get("phone", ""),
                notes=data.get("notes", "")), user)
            db.execute("UPDATE records SET owner_id=? WHERE id=?", (owner, customer["id"]))
            create(db, "contacts", dict(name=data["contact"], customer_id=customer["id"],
                phone=data.get("phone", ""), email=data.get("email", ""), role="其他"), user)
            data["converted_customer_id"] = customer["id"]
            data["status"] = "已转化"
            update_data(db, row, data)
            for f in db.execute("SELECT * FROM followups WHERE record_id=?", (record_id,)).fetchall():
                db.execute("INSERT INTO followups VALUES (?,?,?,?,?,?,?)", (uid(), customer["id"], f["user_id"],
                    f["content"], f["method"], f["next_date"], f["created_at"]))
        elif action == "submit":
            if module not in APPROVABLE:
                fail("该模块无需审批")
            editable(row)
            if row["owner_id"] is None:
                fail("请先领取所属客户")
            validate(db, module, business_data(db, module, data), user, record_id)
            data["approval_status"] = "pending"
            update_data(db, row, data)
            db.execute("INSERT INTO approvals VALUES (?,?,?,'pending',NULL,'',?,NULL)",
                       (uid(), record_id, user["id"], now()))
        elif action == "deliver":
            if module != "orders" or data.get("approval_status") != "approved":
                fail("只有已通过审批的订单可以交付")
            if data["delivery_status"] == "已完成":
                fail("订单已交付", 409)
            data["delivery_status"] = "已完成"
            update_data(db, row, data)
        else:
            fail("操作不存在", 404)
        audit(db, user, {"claim": "领取", "release": "放入公海", "assign": "分配", "convert": "转为客户",
                        "submit": "提交审批", "deliver": "完成交付"}[action], module, record_id, data["name"])
        # A released record is still readable from the pool.
        return serialize(db, db.execute("SELECT * FROM records WHERE id=?", (record_id,)).fetchone())


@router.get("/approvals")
def list_approvals(status: str = "", user=Depends(current_user)):
    with connection() as db:
        rows = db.execute("""SELECT a.*,r.module,r.data,r.owner_id,u.name AS requester_name,v.name AS reviewer_name
          FROM approvals a JOIN records r ON r.id=a.record_id JOIN users u ON u.id=a.requested_by
          LEFT JOIN users v ON v.id=a.reviewed_by ORDER BY a.created_at DESC""")
        result = []
        for r in rows:
            if user["role"] == "sales" and r["owner_id"] != user["id"]:
                continue
            if status and r["status"] != status:
                continue
            item = dict(r)
            item["name"] = json.loads(item.pop("data"))["name"]
            result.append(item)
        return result


@router.post("/approvals/{approval_id}/decision")
def decide(approval_id: str, payload: DecisionInput, user=Depends(manager)):
    if payload.decision not in {"approved", "rejected"}:
        fail("审批结果无效")
    if payload.decision == "rejected" and not payload.comment.strip():
        fail("驳回时请填写原因")
    with connection(write=True) as db:
        approval = db.execute("SELECT * FROM approvals WHERE id=?", (approval_id,)).fetchone()
        if not approval:
            fail("审批不存在", 404)
        if approval["status"] != "pending":
            fail("该审批已处理", 409)
        row = db.execute("SELECT * FROM records WHERE id=?", (approval["record_id"],)).fetchone()
        data = json.loads(row["data"])
        if payload.decision == "approved":
            validate(db, row["module"], business_data(db, row["module"], data), user, row["id"])
        data["approval_status"] = payload.decision
        update_data(db, row, data)
        db.execute("UPDATE approvals SET status=?,reviewed_by=?,comment=?,reviewed_at=? WHERE id=?",
                   (payload.decision, user["id"], payload.comment.strip(), now(), approval_id))
        audit(db, user, "审批通过" if payload.decision == "approved" else "审批驳回", row["module"], row["id"], payload.comment)
        return {"ok": True}


@router.get("/audit")
def audit_log(page: int = Query(default=1, ge=1), user=Depends(administrator)):
    with connection() as db:
        total = db.execute("SELECT count(*) FROM audit").fetchone()[0]
        items = [dict(r) for r in db.execute("""SELECT a.*,u.name AS user_name FROM audit a
          LEFT JOIN users u ON u.id=a.user_id ORDER BY a.created_at DESC,a.rowid DESC LIMIT 30 OFFSET ?""", ((page - 1) * 30,))]
        return {"items": items, "total": total}
