import os
import tempfile
import unittest

from fastapi.testclient import TestClient

from backend.app.main import app


class CRMTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.old_db = os.environ.get("CRM_DB")
        os.environ["CRM_DB"] = self.temp.name + "/test.sqlite3"
        self.client = TestClient(app)
        self.client.__enter__()
        result = self.client.post("/api/setup", json={"username": "admin", "password": "TestPassword2026", "name": "管理员"})
        self.assertEqual(result.status_code, 201, result.text)

    def tearDown(self):
        self.client.__exit__(None, None, None)
        if self.old_db is None:
            os.environ.pop("CRM_DB", None)
        else:
            os.environ["CRM_DB"] = self.old_db
        self.temp.cleanup()

    def create(self, module, data):
        r = self.client.post("/api/records/" + module, json={"data": data})
        self.assertEqual(r.status_code, 201, r.text)
        return r.json()

    def customer(self, name="测试客户"):
        return self.create("customers", dict(name=name, industry="互联网", source="官网咨询", level="A · 重点", status="潜在客户"))

    def approve(self, module, record):
        r = self.client.post("/api/records/%s/%s/actions/submit" % (module, record["id"]), json={"version": record["version"]})
        self.assertEqual(r.status_code, 200, r.text)
        approval = next(a for a in self.client.get("/api/approvals").json() if a["record_id"] == record["id"] and a["status"] == "pending")
        result = self.client.post("/api/approvals/" + approval["id"] + "/decision", json={"decision": "approved"})
        self.assertEqual(result.status_code, 200, result.text)
        return self.client.get("/api/records/%s/%s" % (module, record["id"])).json()

    def test_sales_lifecycle_and_financial_limits(self):
        lead = self.create("leads", dict(name="试用咨询", company="新客户", contact="张先生", source="官网咨询", industry="互联网", status="待跟进"))
        path = "/api/records/leads/" + lead["id"]
        self.assertEqual(self.client.post(path + "/followups", json={"content": "已经沟通需求"}).status_code, 201)
        converted = self.client.post(path + "/actions/convert", json={"version": 1})
        self.assertEqual(converted.status_code, 200, converted.text)
        customer_id = converted.json()["data"]["converted_customer_id"]
        self.assertEqual(self.client.post(path + "/actions/convert", json={"version": 2}).status_code, 409)
        self.assertEqual(len(self.client.get("/api/records/customers/" + customer_id).json()["followups"]), 1)
        product = self.create("products", dict(name="订阅", code="P001", category="软件", price="100.10", unit="套", status="在售"))
        quote = self.create("quotes", dict(name="报价单", customer_id=customer_id, valid_until="2027-01-01",
            items=[dict(product_id=product["id"], quantity="3", unit_price="100.10")]))
        self.assertEqual(quote["data"]["amount"], "300.30")
        self.approve("quotes", quote)
        contract = self.create("contracts", dict(name="合同", code="HT001", customer_id=customer_id, quote_id=quote["id"], amount="300.30", signed_date="2026-01-01", end_date="2027-01-01"))
        approved = self.approve("contracts", contract)
        self.assertEqual(self.client.put("/api/records/contracts/" + contract["id"], json={"data": {}, "version": approved["version"]}).status_code, 409)
        payment = dict(name="首款", customer_id=customer_id, contract_id=contract["id"], amount="200.10", received_date="2026-02-01", method="银行转账")
        self.approve("payments", self.create("payments", payment))
        payment["amount"] = "100.21"
        self.assertEqual(self.client.post("/api/records/payments", json={"data": payment}).status_code, 422)
        payment["amount"] = "100.20"
        self.approve("payments", self.create("payments", payment))
        dashboard = self.client.get("/api/dashboard").json()
        self.assertEqual(dashboard["received_amount"], "300.30")
        self.assertEqual(dashboard["receivable"], "0.00")
        order = self.create("orders", dict(name="订单", code="DD001", customer_id=customer_id, contract_id=contract["id"],
            items=[dict(product_id=product["id"], quantity="3", unit_price="100.10")], delivery_date="2026-10-01", delivery_status="待交付"))
        order = self.approve("orders", order)
        self.assertEqual(self.client.post("/api/records/orders/" + order["id"] + "/actions/deliver", json={"version": order["version"]}).status_code, 200)

    def test_access_control_pool_and_conflict(self):
        customer = self.customer()
        other = self.create("contacts", dict(name="联系人", customer_id=customer["id"], role="决策者"))
        result = self.client.post("/api/users", json=dict(username="seller", password="SalesPassword2026", name="销售", role="sales"))
        self.assertEqual(result.status_code, 201, result.text)
        sales = TestClient(app)
        self.assertEqual(sales.post("/api/auth/login", json=dict(username="seller", password="SalesPassword2026")).status_code, 200)
        path = "/api/records/customers/" + customer["id"]
        self.assertEqual(sales.get(path).status_code, 404)
        self.assertEqual(sales.get("/api/records/customers").json()["total"], 0)
        self.assertEqual(sales.get("/api/audit").status_code, 403)
        self.assertEqual(sales.post("/api/records/products", json={"data": {}}).status_code, 403)
        self.assertEqual(self.client.post(path + "/actions/release", json={"version": 1}).status_code, 200)
        self.assertEqual(sales.get("/api/records/customers?scope=pool").json()["total"], 1)
        claimed = sales.post(path + "/actions/claim", json={"version": 2})
        self.assertEqual(claimed.status_code, 200, claimed.text)
        self.assertEqual(sales.get("/api/records/contacts/" + other["id"]).status_code, 200)
        self.assertEqual(self.client.post(path + "/actions/claim", json={"version": 2}).status_code, 409)
        stale = sales.put(path, json={"version": 1, "data": customer["data"]})
        self.assertEqual(stale.status_code, 409)

    def test_validation_import_transaction_and_csv(self):
        customer = self.customer("=malicious()")
        self.assertIn("'=malicious()", self.client.get("/api/records/customers/export").text)
        self.assertEqual(self.client.post("/api/records/customers", json={"data": {**customer["data"], "approval_status": "approved"}}).status_code, 422)
        result = self.client.post("/api/records/customers/import", json={"records": [customer["data"], {"name": "invalid"}]})
        self.assertEqual(result.status_code, 422)
        self.assertEqual(self.client.get("/api/records/customers").json()["total"], 1)
        self.assertEqual(self.client.post("/api/catalog/customers/fields", json=dict(key="custom_region", label="区域", type="select", options=["华东", "华南"])).status_code, 201)
        result = self.client.put("/api/records/customers/" + customer["id"], json={"version": 1, "data": {**customer["data"], "custom_region": "华东"}})
        self.assertEqual(result.status_code, 200, result.text)
        self.assertEqual(self.client.post("/api/records/unknown", json={"data": {}}).status_code, 404)

    def test_auth_setup_csrf_and_lockout(self):
        self.assertEqual(self.client.post("/api/setup", json=dict(username="other", password="TestPassword2026", name="其他")).status_code, 409)
        self.assertEqual(self.client.post("/api/auth/logout", headers={"Origin": "https://evil.example"}).status_code, 403)
        for _ in range(5):
            self.assertEqual(self.client.post("/api/auth/login", json=dict(username="admin", password="wrong")).status_code, 401)
        self.assertEqual(self.client.post("/api/auth/login", json=dict(username="admin", password="TestPassword2026")).status_code, 429)
        self.assertEqual(self.client.post("/api/auth/logout").status_code, 200)
        self.assertEqual(self.client.get("/api/dashboard").status_code, 401)

    def test_reject_resubmit_and_relation_validation(self):
        a, b = self.customer("A"), self.customer("B")
        contract = self.create("contracts", dict(name="合同", code="HT002", customer_id=a["id"], amount="100", signed_date="2026-01-01", end_date="2027-01-01"))
        self.client.post("/api/records/contracts/" + contract["id"] + "/actions/submit", json={"version": 1})
        approval = self.client.get("/api/approvals").json()[0]
        path = "/api/approvals/" + approval["id"] + "/decision"
        self.assertEqual(self.client.post(path, json={"decision": "rejected"}).status_code, 422)
        self.assertEqual(self.client.post(path, json={"decision": "rejected", "comment": "补充条款"}).status_code, 200)
        self.assertEqual(self.client.post(path, json={"decision": "approved"}).status_code, 409)
        contract["version"] = 3
        self.approve("contracts", contract)
        wrong = self.client.post("/api/records/payment_plans", json={"data": dict(name="回款计划", customer_id=b["id"], contract_id=contract["id"], amount="50", due_date="2026-10-01")})
        self.assertEqual(wrong.status_code, 422)
        self.assertEqual(self.client.delete("/api/records/customers/" + a["id"] + "?version=1").status_code, 409)

    def test_demo_seed(self):
        from backend.app.db import connection
        from backend.app.seed import seed
        with connection(write=True) as db:
            user = dict(db.execute("SELECT * FROM users LIMIT 1").fetchone())
            seed(db, user)
        result = self.client.get("/api/dashboard")
        self.assertEqual(result.status_code, 200, result.text)
        self.assertEqual(result.json()["customers"], 6)
        self.assertEqual(result.json()["received_amount"], "60000")

    def test_invoice_plan_and_payment_integrity(self):
        customer = self.customer()
        contract = self.create("contracts", dict(name="服务合同", code="FIN-001", customer_id=customer["id"],
            amount="1000.00", signed_date="2026-01-01", end_date="2027-01-01"))
        self.approve("contracts", contract)
        company = self.create("companies", dict(name="测试开票抬头", customer_id=customer["id"], tax_number="TEST-TAX-001"))
        invoice = dict(name="首期发票", code="FP001", customer_id=customer["id"], contract_id=contract["id"],
            company_id=company["id"], amount="600", issued_date="2026-03-01", type="增值税普通发票")
        self.approve("invoices", self.create("invoices", invoice))
        invoice.update(code="FP002", amount="400.01")
        self.assertEqual(self.client.post("/api/records/invoices", json={"data": invoice}).status_code, 422)
        plan = self.create("payment_plans", dict(name="首期计划", customer_id=customer["id"], contract_id=contract["id"], amount="300", due_date="2026-04-01"))
        payment = dict(name="首期回款", customer_id=customer["id"], contract_id=contract["id"], plan_id=plan["id"], amount="301", received_date="2026-04-01", method="银行转账")
        self.assertEqual(self.client.post("/api/records/payments", json={"data": payment}).status_code, 422)
        payment["amount"] = "300"
        self.create("payments", payment)
        changed_plan = {**plan["data"], "amount": "100"}
        self.assertEqual(self.client.put("/api/records/payment_plans/" + plan["id"], json={"data": changed_plan, "version": 1}).status_code, 422)

    def test_concurrent_pool_claim_has_one_winner(self):
        from concurrent.futures import ThreadPoolExecutor
        customer = self.customer()
        path = "/api/records/customers/" + customer["id"]
        self.client.post(path + "/actions/release", json={"version": 1})
        clients = []
        for index in range(2):
            username = "claimant" + str(index)
            self.client.post("/api/users", json=dict(username=username, password="TestPassword2026", name=username, role="sales"))
            client = TestClient(app)
            client.post("/api/auth/login", json=dict(username=username, password="TestPassword2026"))
            clients.append(client)
        with ThreadPoolExecutor(max_workers=2) as pool:
            futures = [pool.submit(c.post, path + "/actions/claim", json={"version": 2}) for c in clients]
            statuses = sorted(f.result().status_code for f in futures)
        # A losing request may fail its ownership check before its version check.
        self.assertEqual(statuses[0], 200)
        self.assertIn(statuses[1], [404, 409])
        self.assertEqual(sum(c.get(path).status_code == 200 for c in clients), 1)

    def test_disabling_account_revokes_sessions(self):
        person = self.client.post("/api/users", json=dict(username="disabled", password="TestPassword2026", name="待停用", role="sales")).json()
        sales = TestClient(app)
        self.assertEqual(sales.post("/api/auth/login", json=dict(username="disabled", password="TestPassword2026")).status_code, 200)
        result = self.client.put("/api/users/" + person["id"], json=dict(name="待停用", role="sales", active=False))
        self.assertEqual(result.status_code, 200)
        self.assertEqual(sales.get("/api/auth/me").status_code, 401)
        self.assertEqual(sales.post("/api/auth/login", json=dict(username="disabled", password="TestPassword2026")).status_code, 401)


if __name__ == "__main__":
    unittest.main()
