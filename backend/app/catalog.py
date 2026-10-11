"""Module definitions shared with the TypeScript UI through /api/catalog."""


def field(key, label, kind="text", required=False, **kwargs):
    return dict(key=key, label=label, type=kind, required=required, **kwargs)


def choice(key, label, options, required=True):
    return field(key, label, "select", required, options=options)


def ref(key, label, module, required=True):
    return field(key, label, "reference", required, module=module)


NAME = field("name", "名称", required=True)
NOTE = field("notes", "备注", "textarea")
SOURCE = choice("source", "来源", ["官网咨询", "市场活动", "客户转介绍", "主动开发", "合作伙伴", "其他"])
INDUSTRY = choice("industry", "行业", ["互联网", "制造业", "金融", "零售", "教育", "医疗", "企业服务", "其他"])
CUSTOMER = ref("customer_id", "客户", "customers")
AMOUNT = field("amount", "金额（元）", "money", True)
STAGES = ["初步沟通", "需求确认", "方案报价", "商务谈判", "赢单", "输单"]
APPROVABLE = {"quotes", "contracts", "orders", "invoices", "payments"}

MODULES = {
    "leads": dict(label="线索", group="客户经营", description="发现潜在需求，让每条线索都有下一步", fields=[
        NAME, field("company", "公司", required=True), field("contact", "联系人", required=True),
        field("phone", "联系电话"), field("email", "邮箱", "email"), SOURCE, INDUSTRY,
        choice("status", "状态", ["待跟进", "跟进中", "无效"]), NOTE]),
    "customers": dict(label="客户", group="客户经营", description="连接客户，沉淀长期价值", fields=[
        NAME, INDUSTRY, SOURCE, choice("level", "客户级别", ["A · 重点", "B · 潜力", "C · 普通"]),
        choice("status", "状态", ["潜在客户", "合作客户", "流失客户"]), field("phone", "电话"),
        field("website", "网站"), field("address", "地址"), NOTE]),
    "contacts": dict(label="联系人", group="客户经营", description="掌握关键联系人，建立更好的客户关系", fields=[
        NAME, CUSTOMER, field("title", "职位"), field("phone", "手机"),
        field("email", "邮箱", "email"), choice("role", "决策角色", ["决策者", "使用者", "影响者", "其他"]), NOTE]),
    "opportunities": dict(label="商机", group="销售协作", description="从第一次沟通，到下一次赢单", fields=[
        NAME, CUSTOMER, AMOUNT, choice("stage", "销售阶段", STAGES),
        field("probability", "赢单概率（%）", "number", True, min=0, max=100),
        field("expected_date", "预计成交日期", "date", True), NOTE]),
    "products": dict(label="产品", group="销售协作", description="统一产品目录与标准价格", shared=True, fields=[
        NAME, field("code", "产品编码", required=True, unique=True), field("category", "分类", required=True),
        field("price", "单价（元）", "money", True), field("unit", "单位", required=True),
        choice("status", "状态", ["在售", "停售"]), NOTE]),
    "quotes": dict(label="报价", group="销售协作", description="清晰报价，把价值传递给客户", fields=[
        NAME, CUSTOMER, ref("opportunity_id", "关联商机", "opportunities", False),
        field("items", "产品明细", "items", True), field("valid_until", "有效期至", "date", True), NOTE]),
    "contracts": dict(label="合同", group="交易履约", description="从约定到交付，每一份承诺有迹可循", fields=[
        NAME, field("code", "合同编号", required=True, unique=True), CUSTOMER,
        ref("quote_id", "关联报价", "quotes", False), AMOUNT, field("signed_date", "签约日期", "date", True),
        field("end_date", "到期日期", "date", True), NOTE]),
    "orders": dict(label="订单", group="交易履约", description="跟踪每笔订单的履约进度", fields=[
        NAME, field("code", "订单编号", required=True, unique=True), CUSTOMER,
        ref("contract_id", "关联合同", "contracts"), field("items", "产品明细", "items", True),
        field("delivery_date", "交付日期", "date", True),
        choice("delivery_status", "履约状态", ["待交付", "交付中", "已完成"]), NOTE]),
    "companies": dict(label="工商抬头", group="交易履约", description="管理客户开票信息", fields=[
        NAME, CUSTOMER, field("tax_number", "纳税人识别号", required=True, unique=True),
        field("bank", "开户银行"), field("bank_account", "银行账号"), field("address", "注册地址"), NOTE]),
    "invoices": dict(label="发票", group="交易履约", description="关联客户与合同，跟踪开票记录", fields=[
        NAME, field("code", "发票号码", required=True, unique=True), CUSTOMER,
        ref("contract_id", "关联合同", "contracts"), ref("company_id", "开票抬头", "companies"),
        AMOUNT, field("issued_date", "开票日期", "date", True),
        choice("type", "发票类型", ["增值税专用发票", "增值税普通发票"]), NOTE]),
    "payment_plans": dict(label="回款计划", group="交易履约", description="让应收有计划，让回款有节奏", fields=[
        NAME, CUSTOMER, ref("contract_id", "关联合同", "contracts"), AMOUNT,
        field("due_date", "计划回款日期", "date", True), NOTE]),
    "payments": dict(label="回款", group="交易履约", description="记录实际到账，掌握资金进度", fields=[
        NAME, CUSTOMER, ref("contract_id", "关联合同", "contracts"),
        ref("plan_id", "关联回款计划", "payment_plans", False), AMOUNT,
        field("received_date", "到账日期", "date", True), choice("method", "收款方式", ["银行转账", "现金", "其他"]), NOTE]),
    "tenders": dict(label="标讯", group="销售协作", description="汇集招投标信息，及时捕捉新机会", fields=[
        NAME, field("company", "招标单位", required=True), field("amount", "预算（元）", "money"),
        field("deadline", "截止日期", "date", True), field("url", "公告链接"),
        choice("status", "状态", ["关注中", "准备投标", "已投标", "中标", "未中标"]), NOTE]),
    "tasks": dict(label="任务", group="工作空间", description="把重要的下一步，放进今天的计划", fields=[
        NAME, ref("customer_id", "关联客户", "customers", False),
        field("due_date", "截止日期", "date", True), choice("priority", "优先级", ["高", "中", "低"]),
        choice("status", "状态", ["待处理", "进行中", "已完成"]), NOTE]),
}

for key, module in MODULES.items():
    module["key"] = key
    module["approvable"] = key in APPROVABLE
