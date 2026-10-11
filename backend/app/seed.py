"""Optional, clearly fictional demo data; never creates a default password."""
from datetime import date, timedelta

from .db import insert_record, now, uid
from .service import create


def seed(db, user):
    today = date.today()

    def day(delta):
        return str(today + timedelta(days=delta))

    companies = ["星河科技有限公司", "远山智能制造", "云帆数字科技", "青禾教育集团", "澄明企业服务", "北辰医疗科技"]
    industries = ["互联网", "制造业", "互联网", "教育", "企业服务", "医疗"]
    customers = []
    for i, name in enumerate(companies):
        customer = create(db, "customers", dict(name=name, industry=industries[i], source="客户转介绍" if i % 2 else "官网咨询",
            level=["A · 重点", "B · 潜力", "C · 普通"][i % 3], status="合作客户" if i < 2 else "潜在客户",
            address=["上海市徐汇区", "杭州市滨江区", "北京市海淀区"][i % 3], notes="虚构演示数据，可删除或编辑"), user)
        customers.append(customer["id"])
        create(db, "contacts", dict(name=["陈晓", "李明", "王颖", "周宁", "赵悦", "林远"][i],
            customer_id=customer["id"], title="采购负责人", email="contact%d@example.com" % i, role="决策者"), user)
        create(db, "opportunities", dict(name=name.replace("有限公司", "") + " · 数字化升级", customer_id=customer["id"],
            amount=str([128000, 86000, 240000, 58000, 96000, 180000][i]),
            stage=["商务谈判", "方案报价", "需求确认", "初步沟通", "赢单", "需求确认"][i],
            probability=[80, 60, 40, 20, 100, 40][i], expected_date=day(10 + i * 6)), user)
        db.execute("INSERT INTO followups VALUES (?,?,?,?,?,?,?)", (uid(), customer["id"], user["id"],
            ["已完成产品演示，客户希望进一步了解实施周期。", "确认了采购预算，下周安排方案沟通。", "客户对数据管理模块感兴趣，已发送产品资料。"][i % 3],
            ["会议", "电话", "微信"][i % 3], day(i + 1), now()))
    for i, name in enumerate(["企业版年度订阅", "专业实施服务", "数据迁移服务"]):
        create(db, "products", dict(name=name, code="PRD-00%d" % (i + 1), category="软件服务", price=str([48000, 28000, 12000][i]), unit="套", status="在售"), user)
    for i, name in enumerate(["新域科技 · CRM 咨询", "森屿零售 · 系统升级", "初见品牌 · 销售协作"]):
        create(db, "leads", dict(name=name, company=name.split(" · ")[0], contact=["刘先生", "张女士", "吴先生"][i],
            source="官网咨询", industry="零售" if i == 1 else "互联网", status="待跟进"), user)
    for i, name in enumerate(["与星河科技确认实施排期", "准备远山智能产品方案", "回访云帆数字采购负责人", "整理本周销售进展"]):
        create(db, "tasks", dict(name=name, customer_id=customers[i], due_date=day(i - 1), priority="高" if i < 2 else "中", status="待处理"), user)
    for i in range(2):
        contract_id = insert_record(db, "contracts", dict(name=companies[i] + "年度服务合同", code="DEMO-HT-00%d" % i,
            customer_id=customers[i], amount=str([128000, 86000][i]), signed_date=day(-20 - i * 35), end_date=day(345),
            approval_status="approved", notes="演示已审批合同", quote_id=""), user["id"])
        db.execute("INSERT INTO approvals VALUES (?,?,?,?,?,?,?,?)", (uid(), contract_id, user["id"], "approved", user["id"], "演示审批记录", now(), now()))
        plan = create(db, "payment_plans", dict(name="首期回款 · " + companies[i], customer_id=customers[i],
            contract_id=contract_id, amount="40000", due_date=day(-3 if i else 7)), user)
        payment_id = insert_record(db, "payments", dict(name=companies[i] + "首期到账", customer_id=customers[i],
            contract_id=contract_id, plan_id=plan["id"], amount="30000", received_date=day(-5 - i * 30), method="银行转账", approval_status="approved"), user["id"])
        db.execute("INSERT INTO approvals VALUES (?,?,?,?,?,?,?,?)", (uid(), payment_id, user["id"], "approved", user["id"], "演示审批记录", now(), now()))
    create(db, "tenders", dict(name="智慧园区客户管理平台采购", company="示例园区管理中心", amount="300000",
        deadline=day(21), status="关注中", notes="虚构演示标讯"), user)
