package usecase

type Field struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Module      string   `json:"module"`
	Required    bool     `json:"required"`
	Unique      bool     `json:"unique"`
	Shared      bool     `json:"shared"`
	ReadOnly    bool     `json:"read_only"`
	Custom      bool     `json:"custom,omitempty"`
	Standard    bool     `json:"standard,omitempty"`
	Disableable bool     `json:"disableable,omitempty"`
	Group       string   `json:"group,omitempty"`
	Sources     []string `json:"sources,omitempty"`
	Columns     []Field  `json:"columns,omitempty"`
	Options     []string `json:"options,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
}
type Module struct {
	Key              string  `json:"key"`
	Label            string  `json:"label"`
	Group            string  `json:"group"`
	Description      string  `json:"description"`
	Shared           bool    `json:"shared"`
	Approvable       bool    `json:"approvable"`
	Fields           []Field `json:"fields"`
	DocumentTemplate string  `json:"document_template,omitempty"`
}

func f(key, label, kind string, required bool) Field {
	return Field{Key: key, Label: label, Type: kind, Required: required}
}
func choice(key, label string, options ...string) Field {
	return Field{Key: key, Label: label, Type: "select", Required: true, Options: options}
}
func ref(key, label, module string, required bool) Field {
	return Field{Key: key, Label: label, Type: "reference", Module: module, Required: required}
}

var Modules = map[string]Module{
	"leads":         {Key: "leads", Label: "线索", Group: "客户经营", Description: "发现潜在需求", Shared: false, Approvable: false, Fields: []Field{f("name", "名称", "text", true), f("company", "公司", "text", true), f("contact", "联系人", "text", true), f("phone", "联系电话", "text", false), f("email", "邮箱", "email", false), choice("source", "来源", "官网咨询", "市场活动", "客户转介绍", "主动开发", "合作伙伴", "其他"), choice("industry", "行业", "互联网", "制造业", "金融", "零售", "教育", "医疗", "企业服务", "其他"), choice("status", "状态", "待跟进", "跟进中", "无效"), f("notes", "备注", "textarea", false)}},
	"customers":     {Key: "customers", Label: "客户", Group: "客户经营", Description: "连接客户", Shared: false, Approvable: false, Fields: []Field{f("name", "名称", "text", true), choice("industry", "行业", "互联网", "制造业", "金融", "零售", "教育", "医疗", "企业服务", "其他"), choice("source", "来源", "官网咨询", "市场活动", "客户转介绍", "主动开发", "合作伙伴", "其他"), choice("level", "客户级别", "A · 重点", "B · 潜力", "C · 普通"), choice("status", "状态", "潜在客户", "合作客户", "流失客户"), f("phone", "电话", "text", false), f("website", "网站", "text", false), f("address", "地址", "text", false), f("notes", "备注", "textarea", false)}},
	"contacts":      {Key: "contacts", Label: "联系人", Group: "客户经营", Description: "掌握关键联系人", Shared: false, Approvable: false, Fields: []Field{f("name", "名称", "text", true), ref("customer_id", "客户", "customers", true), f("title", "职位", "text", false), f("phone", "手机", "text", false), f("email", "邮箱", "email", false), choice("role", "决策角色", "决策者", "使用者", "影响者", "其他"), f("notes", "备注", "textarea", false)}},
	"opportunities": {Key: "opportunities", Label: "商机", Group: "销售协作", Description: "跟进商机", Shared: false, Approvable: false, Fields: []Field{f("name", "名称", "text", true), ref("customer_id", "客户", "customers", true), f("amount", "金额（元）", "money", true), choice("stage", "销售阶段", "初步沟通", "需求确认", "方案报价", "商务谈判", "赢单", "输单"), f("probability", "赢单概率（%）", "number", true), f("expected_date", "预计成交日期", "date", true), f("notes", "备注", "textarea", false)}},
	"products":      {Key: "products", Label: "产品", Group: "销售协作", Description: "统一产品目录", Shared: true, Approvable: false, Fields: []Field{f("name", "名称", "text", true), f("code", "产品编码", "text", true), f("category", "分类", "text", true), f("price", "单价（元）", "money", true), f("unit", "单位", "text", true), choice("status", "状态", "在售", "停售"), f("notes", "备注", "textarea", false)}},
	"prices":        {Key: "prices", Label: "价格表", Group: "销售协作", Description: "维护产品价格策略", Shared: false, Approvable: false, Fields: []Field{f("name", "价格表名称", "text", true), choice("status", "状态", "启用", "停用"), f("items", "产品价格明细", "items", true), f("notes", "备注", "textarea", false)}},
	"quotes":        {Key: "quotes", Label: "报价", Group: "销售协作", Description: "清晰报价", Shared: false, Approvable: true, Fields: []Field{f("name", "名称", "text", true), ref("customer_id", "客户", "customers", true), ref("opportunity_id", "关联商机", "opportunities", false), f("items", "产品明细", "items", true), f("valid_until", "有效期至", "date", true), f("notes", "备注", "textarea", false)}},
	"contracts":     {Key: "contracts", Label: "合同", Group: "交易履约", Description: "管理合同", Shared: false, Approvable: true, Fields: []Field{f("name", "名称", "text", true), f("code", "合同编号", "text", true), ref("customer_id", "客户", "customers", true), ref("quote_id", "关联报价", "quotes", false), f("amount", "金额（元）", "money", true), choice("stage", "合同阶段", "待签署", "已签署", "合同变更", "履行中", "履行完毕", "作废", "归档"), f("signed_date", "签约日期", "date", true), f("end_date", "到期日期", "date", true), f("notes", "备注", "textarea", false)}},
	"orders":        {Key: "orders", Label: "订单", Group: "交易履约", Description: "跟踪履约", Shared: false, Approvable: true, Fields: []Field{f("name", "名称", "text", true), f("code", "订单编号", "text", true), ref("customer_id", "客户", "customers", true), ref("contract_id", "关联合同", "contracts", true), f("items", "产品明细", "items", true), f("delivery_date", "交付日期", "date", true), choice("delivery_status", "履约状态", "待交付", "交付中", "已完成"), f("notes", "备注", "textarea", false)}},
	"companies":     {Key: "companies", Label: "工商抬头", Group: "交易履约", Description: "管理开票信息", Shared: false, Approvable: false, Fields: []Field{f("name", "名称", "text", true), ref("customer_id", "客户", "customers", true), f("tax_number", "纳税人识别号", "text", true), f("bank", "开户银行", "text", false), f("bank_account", "银行账号", "text", false), f("address", "注册地址", "text", false), f("notes", "备注", "textarea", false)}},
	"invoices":      {Key: "invoices", Label: "发票", Group: "交易履约", Description: "跟踪开票", Shared: false, Approvable: true, Fields: []Field{f("name", "名称", "text", true), f("code", "发票号码", "text", true), ref("customer_id", "客户", "customers", true), ref("contract_id", "关联合同", "contracts", true), ref("company_id", "开票抬头", "companies", true), f("amount", "金额（元）", "money", true), f("issued_date", "开票日期", "date", true), choice("type", "发票类型", "增值税专用发票", "增值税普通发票"), f("notes", "备注", "textarea", false)}},
	"payment_plans": {Key: "payment_plans", Label: "回款计划", Group: "交易履约", Description: "管理应收", Shared: false, Approvable: false, Fields: []Field{f("name", "名称", "text", true), ref("customer_id", "客户", "customers", true), ref("contract_id", "关联合同", "contracts", true), f("amount", "金额（元）", "money", true), f("due_date", "计划回款日期", "date", true), f("notes", "备注", "textarea", false)}},
	"payments":      {Key: "payments", Label: "回款", Group: "交易履约", Description: "记录实际到账", Shared: false, Approvable: true, Fields: []Field{f("name", "名称", "text", true), ref("customer_id", "客户", "customers", true), ref("contract_id", "关联合同", "contracts", true), ref("plan_id", "关联回款计划", "payment_plans", false), f("amount", "金额（元）", "money", true), f("received_date", "到账日期", "date", true), choice("method", "收款方式", "银行转账", "现金", "其他"), f("notes", "备注", "textarea", false)}},
	"tenders":       {Key: "tenders", Label: "标讯", Group: "销售协作", Description: "收集标讯", Shared: false, Approvable: false, Fields: []Field{f("name", "名称", "text", true), f("company", "招标单位", "text", true), f("amount", "预算（元）", "money", false), f("deadline", "截止日期", "date", true), f("url", "公告链接", "text", false), choice("status", "状态", "关注中", "准备投标", "已投标", "中标", "未中标"), f("notes", "备注", "textarea", false)}},
	"tasks":         {Key: "tasks", Label: "任务", Group: "工作空间", Description: "管理下一步", Shared: false, Approvable: false, Fields: []Field{f("name", "名称", "text", true), ref("customer_id", "关联客户", "customers", false), f("due_date", "截止日期", "date", true), choice("priority", "优先级", "高", "中", "低"), choice("status", "状态", "待处理", "进行中", "已完成"), f("notes", "备注", "textarea", false)}},
}

func init() {
	product := Modules["products"]
	product.Fields = append(product.Fields[:len(product.Fields)-1], Field{Key: "stock", Label: "可用库存", Type: "number", Required: true, Min: numberPointer(0), Max: numberPointer(999999999)}, Field{Key: "reserved_stock", Label: "预占库存", Type: "number", ReadOnly: true, Min: numberPointer(0), Max: numberPointer(999999999)}, Field{Key: "specifications", Label: "多级产品规格", Type: "subtable", Columns: []Field{{Key: "level_one", Label: "一级规格", Type: "text", Required: true}, {Key: "level_two", Label: "二级规格", Type: "text"}, {Key: "sku", Label: "SKU", Type: "text", Required: true}, {Key: "stock", Label: "库存", Type: "number", Required: true, Min: numberPointer(0), Max: numberPointer(999999999)}, {Key: "price", Label: "规格单价", Type: "money", Required: true}}}, product.Fields[len(product.Fields)-1])
	Modules["products"] = product
}

func numberPointer(value float64) *float64 { return &value }
