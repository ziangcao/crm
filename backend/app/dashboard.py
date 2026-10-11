import json
from datetime import date
from decimal import Decimal

from fastapi import APIRouter, Depends

from .auth import current_user
from .catalog import MODULES, STAGES
from .db import connection
from .service import serialize, visible_records

router = APIRouter(prefix="/api")


@router.get("/dashboard")
def dashboard(user=Depends(current_user)):
    with connection() as db:
        rows = {key: list(visible_records(db, key, user)) for key in MODULES}
        data = {key: [json.loads(r["data"]) for r in values] for key, values in rows.items()}
        today = date.today().isoformat()
        active_opps = [v for v in data["opportunities"] if v["stage"] not in {"赢单", "输单"}]
        contracts = [v for v in data["contracts"] if v["approval_status"] == "approved"]
        payments = [v for v in data["payments"] if v["approval_status"] == "approved"]

        def total(items):
            return sum((Decimal(v["amount"]) for v in items), Decimal(0))

        contract_total, payment_total = total(contracts), total(payments)
        funnel = [dict(stage=stage, count=sum(v["stage"] == stage for v in data["opportunities"]),
                       amount=str(total(v for v in data["opportunities"] if v["stage"] == stage))) for stage in STAGES]
        months = []
        current = date.today().year * 12 + date.today().month - 1
        for value in range(current - 5, current + 1):
            year, month = divmod(value, 12)
            prefix = "%04d-%02d" % (year, month + 1)
            months.append(dict(month=prefix, contracts=str(total(v for v in contracts if v["signed_date"].startswith(prefix))),
                               payments=str(total(v for v in payments if v["received_date"].startswith(prefix)))))
        activities = []
        visible_ids = {r["id"] for values in rows.values() for r in values}
        for item in db.execute("""SELECT f.*,r.module,r.data,u.name AS user_name FROM followups f
          JOIN records r ON r.id=f.record_id JOIN users u ON u.id=f.user_id ORDER BY f.created_at DESC,f.rowid DESC"""):
            if item["record_id"] in visible_ids:
                activities.append(dict(id=item["id"], record_id=item["record_id"], module=item["module"],
                    name=json.loads(item["data"])["name"], content=item["content"], method=item["method"],
                    user_name=item["user_name"], created_at=item["created_at"]))
            if len(activities) >= 8:
                break
        tasks = sorted([r for r in rows["tasks"] if json.loads(r["data"])["status"] != "已完成"],
                       key=lambda r: json.loads(r["data"])["due_date"])
        overdue = []
        for row in rows["payment_plans"]:
            plan = json.loads(row["data"])
            received = total(v for v in payments if v.get("plan_id") == row["id"])
            if plan["due_date"] < today and received < Decimal(plan["amount"]):
                overdue.append(dict(id=row["id"], name=plan["name"], remaining=str(Decimal(plan["amount"]) - received), due_date=plan["due_date"]))
        return dict(customers=len(data["customers"]), leads=len(data["leads"]), opportunities=len(active_opps),
                    pipeline=str(total(active_opps)), weighted_pipeline=str(sum((Decimal(v["amount"]) * Decimal(str(v["probability"])) / 100 for v in active_opps), Decimal(0))),
                    contract_amount=str(contract_total), received_amount=str(payment_total),
                    receivable=str(contract_total - payment_total),
                    conversion_rate=round(100 * sum(bool(v.get("converted_customer_id")) for v in data["leads"]) / max(1, len(data["leads"])), 1),
                    funnel=funnel, months=months, activities=activities,
                    tasks=[serialize(db, r) for r in tasks[:6]], overdue_plans=overdue,
                    counts={key: len(values) for key, values in rows.items()})
