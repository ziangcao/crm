import csv
import io
import json
import re
from datetime import date
from decimal import Decimal, InvalidOperation

from fastapi import HTTPException

from .catalog import APPROVABLE, MODULES
from .db import audit, encode, insert_record, now


def fail(message, status=422):
    raise HTTPException(status, message)


def definition(db, module):
    if module not in MODULES:
        fail("模块不存在", 404)
    result = dict(MODULES[module])
    result["fields"] = list(result["fields"]) + [json.loads(r[0]) for r in db.execute(
        "SELECT definition FROM custom_fields WHERE module=? ORDER BY key", (module,))]
    return result


def serialize(db, row):
    result = dict(row)
    result["data"] = json.loads(row["data"])
    user = db.execute("SELECT name FROM users WHERE id=?", (row["owner_id"],)).fetchone()
    result["owner_name"] = user[0] if user else "公海"
    result["references"] = {}
    for f in definition(db, row["module"])["fields"]:
        if f["type"] == "reference" and result["data"].get(f["key"]):
            ref = db.execute("SELECT data FROM records WHERE id=?", (result["data"][f["key"]],)).fetchone()
            result["references"][f["key"]] = json.loads(ref[0])["name"] if ref else "已删除"
    return result


def get_record(db, module, record_id, user, write=False):
    definition(db, module)
    row = db.execute("SELECT * FROM records WHERE id=? AND module=?", (record_id, module)).fetchone()
    if not row:
        fail("记录不存在", 404)
    shared = MODULES[module].get("shared", False)
    pool = module in {"customers", "leads"} and row["owner_id"] is None
    if user["role"] == "sales":
        if write and (shared or row["owner_id"] != user["id"]):
            fail("没有修改权限，请先领取公海记录", 403)
        if not write and not shared and not pool and row["owner_id"] != user["id"]:
            fail("记录不存在", 404)
    return row


def visible_records(db, module, user, scope="all"):
    definition(db, module)
    rows = db.execute("SELECT * FROM records WHERE module=? ORDER BY updated_at DESC, id", (module,)).fetchall()
    for row in rows:
        if scope == "pool":
            if module not in {"customers", "leads"} or row["owner_id"] is not None:
                continue
        elif scope == "mine":
            if row["owner_id"] != user["id"]:
                continue
        elif user["role"] == "sales" and not MODULES[module].get("shared"):
            if row["owner_id"] != user["id"]:
                continue
        yield row


def check_version(row, version):
    if row["version"] != version:
        fail("记录已被其他人更新，请刷新后重试", 409)


def money(value, label="金额", positive=False):
    try:
        number = Decimal(str(value))
        if not number.is_finite() or number < 0 or number > Decimal("999999999999.99"):
            raise InvalidOperation
        if number != number.quantize(Decimal("0.01")) or (positive and number <= 0):
            raise InvalidOperation
        return format(number, ".2f")
    except (InvalidOperation, ValueError):
        fail(label + "必须是有效的非负金额，最多两位小数" + ("且大于零" if positive else ""))


def validate(db, module, data, user, record_id=None):
    fields = definition(db, module)["fields"]
    allowed = {f["key"] for f in fields}
    if set(data) - allowed:
        fail("包含未知或只读字段：" + ", ".join(sorted(set(data) - allowed)))
    clean = {}
    for f in fields:
        key, kind, value = f["key"], f["type"], data.get(f["key"])
        if value is None or value == "":
            if f["required"]:
                fail(f["label"] + "不能为空")
            clean[key] = ""
            continue
        if kind == "items":
            if not isinstance(value, list) or not 1 <= len(value) <= 100:
                fail("请添加 1–100 条产品明细")
            items, total = [], Decimal(0)
            for item in value:
                if not isinstance(item, dict):
                    fail("产品明细格式错误")
                product = get_record(db, "products", str(item.get("product_id", "")), user)
                product_data = json.loads(product["data"])
                if product_data["status"] != "在售":
                    fail("产品已停售：" + product_data["name"])
                try:
                    qty = Decimal(str(item.get("quantity", 0)))
                    if not qty.is_finite() or not 0 < qty <= 1000000 or qty != qty.quantize(Decimal("0.001")):
                        raise InvalidOperation
                except (InvalidOperation, ValueError):
                    fail("数量须大于零，最多三位小数")
                price = money(item.get("unit_price", product_data["price"]), "单价")
                subtotal = (qty * Decimal(price)).quantize(Decimal("0.01"))
                total += subtotal
                items.append(dict(product_id=product["id"], name=product_data["name"],
                                  quantity=str(qty), unit_price=price, subtotal=str(subtotal)))
            clean[key] = items
            clean["amount"] = money(total, positive=True)
        elif kind == "money":
            clean[key] = money(value, f["label"], module in APPROVABLE or module == "payment_plans")
        elif kind == "number":
            try:
                number = float(value)
                if not f.get("min", -1e12) <= number <= f.get("max", 1e12):
                    raise ValueError
            except (ValueError, TypeError):
                fail(f["label"] + "超出允许范围")
            clean[key] = number
        elif kind == "reference":
            target = get_record(db, f["module"], str(value), user)
            if target["owner_id"] is None and not MODULES[f["module"]].get("shared"):
                fail("请先领取关联的公海记录")
            clean[key] = target["id"]
        else:
            if not isinstance(value, str):
                fail(f["label"] + "格式错误")
            value = value.strip()
            if f["required"] and not value:
                fail(f["label"] + "不能为空")
            if len(value) > (5000 if kind == "textarea" else 300):
                fail(f["label"] + "内容过长")
            if kind == "select" and value not in f["options"]:
                fail(f["label"] + "选项无效")
            if kind == "date":
                try:
                    date.fromisoformat(value)
                except ValueError:
                    fail(f["label"] + "日期格式应为 YYYY-MM-DD")
            if kind == "email" and value and not re.fullmatch(r"[^\s@]+@[^\s@]+\.[^\s@]+", value):
                fail("邮箱格式无效")
            clean[key] = value
        if f.get("unique"):
            for row in db.execute("SELECT id,data FROM records WHERE module=?", (module,)):
                if row["id"] != record_id and json.loads(row["data"]).get(key) == clean[key]:
                    fail(f["label"] + "已存在", 409)

    customer = clean.get("customer_id")
    for f in fields:
        if f["type"] != "reference" or not clean.get(f["key"]) or f["module"] == "customers":
            continue
        related = json.loads(db.execute("SELECT data FROM records WHERE id=?", (clean[f["key"]],)).fetchone()[0])
        if customer and related.get("customer_id") and customer != related["customer_id"]:
            fail(f["label"] + "与所选客户不一致")
        if f["module"] in {"contracts", "quotes"} and related.get("approval_status") != "approved":
            fail(f["label"] + "必须先通过审批")
        if f["module"] == "payment_plans" and related.get("contract_id") != clean.get("contract_id"):
            fail("回款计划与合同不一致")
    if module == "contracts" and clean["end_date"] < clean["signed_date"]:
        fail("合同到期日期不能早于签约日期")
    if module in {"orders", "invoices", "payments", "payment_plans"}:
        contract = json.loads(db.execute("SELECT data FROM records WHERE id=?", (clean["contract_id"],)).fetchone()[0])
        total = Decimal(clean["amount"])
        for r in db.execute("SELECT id,data FROM records WHERE module=?", (module,)):
            other = json.loads(r["data"])
            if r["id"] != record_id and other.get("contract_id") == clean["contract_id"] and other.get("approval_status") != "rejected":
                total += Decimal(other["amount"])
        if total > Decimal(contract["amount"]):
            fail(MODULES[module]["label"] + "累计金额不能超过合同金额")
    if module == "payments" and clean.get("plan_id"):
        plan = json.loads(db.execute("SELECT data FROM records WHERE id=?", (clean["plan_id"],)).fetchone()[0])
        total = Decimal(clean["amount"])
        for r in db.execute("SELECT id,data FROM records WHERE module='payments'"):
            other = json.loads(r["data"])
            if r["id"] != record_id and other.get("plan_id") == clean["plan_id"] and other.get("approval_status") != "rejected":
                total += Decimal(other["amount"])
        if total > Decimal(plan["amount"]):
            fail("累计回款不能超过关联回款计划金额")
    return clean


def editable(row):
    data = json.loads(row["data"])
    if data.get("approval_status") in {"pending", "approved"}:
        fail("审批中或已通过的单据不能修改或删除", 409)
    if data.get("converted_customer_id"):
        fail("已转化的线索不能修改或删除", 409)


def update_data(db, row, data):
    db.execute("UPDATE records SET data=?,version=version+1,updated_at=? WHERE id=?",
               (encode(data), now(), row["id"]))


def references_to(db, record_id):
    for row in db.execute("SELECT * FROM records WHERE id!=?", (record_id,)):
        data = json.loads(row["data"])
        if any(value == record_id for key, value in data.items() if key.endswith("_id")):
            yield row
        elif any(item.get("product_id") == record_id for item in data.get("items", [])):
            yield row


def transfer_customer(db, record, owner):
    ids = {record["id"]}
    if record["module"] == "customers":
        ids.update(row["id"] for row in references_to(db, record["id"]))
    for record_id in ids:
        db.execute("UPDATE records SET owner_id=?,version=version+1,updated_at=? WHERE id=?", (owner, now(), record_id))


def create(db, module, data, user):
    if MODULES[module].get("shared") and user["role"] == "sales":
        fail("只有管理员和经理可以维护产品目录", 403)
    clean = validate(db, module, data, user)
    if module in APPROVABLE:
        clean["approval_status"] = "draft"
    owner = user["id"]
    if clean.get("customer_id"):
        owner = db.execute("SELECT owner_id FROM records WHERE id=?", (clean["customer_id"],)).fetchone()[0]
    record_id = insert_record(db, module, clean, owner)
    audit(db, user, "创建", module, record_id, clean["name"])
    return serialize(db, get_record(db, module, record_id, user))


def export_csv(db, module, rows):
    output = io.StringIO()
    writer = csv.writer(output)
    fields = [f for f in definition(db, module)["fields"] if f["type"] != "items"]
    writer.writerow([f["label"] for f in fields] + ["负责人", "创建时间"])

    def safe(value):
        text = str(value if value is not None else "")
        return "'" + text if text.lstrip().startswith(("=", "+", "-", "@", "\t", "\r")) else text

    for row in rows:
        record = serialize(db, row)
        writer.writerow([safe(record["references"].get(f["key"], record["data"].get(f["key"], ""))) for f in fields]
                        + [safe(record["owner_name"]), record["created_at"]])
    return "\ufeff" + output.getvalue()
