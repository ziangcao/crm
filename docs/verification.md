# 验证记录

验证日期：2026-09-28。

## 已通过

| 检查 | 结果 |
| --- | --- |
| `npm ci --include=optional` | 按锁文件重新安装成功，修复中断后缺失的 Vue / Rollup 文件 |
| `npm run build --prefix frontend` | Vue / TypeScript 类型检查与 Vite 生产构建通过 |
| 审批字段条件分支 | Go 审批用例与前端生产构建通过；覆盖字段白名单、非法条件拒绝及流转跳级 |
| 报价/合同基础打印导出 | Vue / TypeScript 类型检查与 Vite 生产构建通过；实际打印窗口需在浏览器中验收 |
| CSV 导入模板下载 | Vue / TypeScript 类型检查与 Vite 生产构建通过；模板使用服务端 CSV 导入支持的字段标识表头 |
| XLSX 导入模板下载 | Go 单测验证字段标识工作表与只读权限拒绝；控制器编译、Vue / TypeScript 类型检查与 Vite 生产构建通过 |
| 本阶段全量回归 | `gofmt -d` 无差异；`go test ./...`、`go vet ./...` 与 `npm run build --prefix frontend` 全部通过 |
| 部门数据范围 | 自定义角色的本人、本部门、下级部门、指定部门、全部范围记录可见性用例通过；非法或空指定部门、跨组织成员部门均被拒绝；`go test ./...` 与 `npm run build --prefix frontend` 通过 |
| 字段级权限 | 隐藏字段不会出现在角色目录、记录输出或搜索数据中；只读字段在目录中标记并由记录表单禁用，服务端拒绝隐藏和只读字段写入；非法字段规则被拒绝；`go test ./backend/...`、`go vet ./backend/...` 与 `npm run build --prefix frontend` 通过。 |
| PWA、企业微信通知与动态表单布局 | PWA 清单、Service Worker、静态资源路由及前端生产构建通过；企业微信配置为可选且缺失时拒绝不完整配置，站内通知持久化后才尝试外部投递；字段分组和排序目录用例通过；`go test ./backend/...`、`go vet ./backend/...` 与 `npm run build --prefix frontend` 通过。 |
| 公式、子表与文档模板 | 服务端公式、逐行子表校验、模板字段白名单、长度和非法占位符测试通过；模板安全转义后打印。 |
| 审批画布与销售流程 | 前端生产构建通过；审批画布复用版本化步骤模型，商机阶段持久化与边界测试通过。 |
| 产品定价与库存 | 折扣税率 Decimal 计算、多级规格、订单预占/释放/交付核销和拆单编译测试通过。 |
| 公告、审计与 OpenAPI | 组织公告目标成员测试、字段前后差异审计、扩展 OpenAPI 与前端构建通过。 |
| 企业微信与集成适配 | OAuth、通讯录和 token 协议使用无网络 HTTP Transport 模拟通过；六类连接器模拟/真实端点测试通过且状态不暴露 Token。 |
| 自助 BI | 分组汇总、非法字段拒绝及前端生产构建通过。 |
| `python -m unittest discover -s backend/tests -v` | 9 组测试全部通过 |
| 首次初始化并载入演示数据 | HTTP 201；工作台和 OpenAPI 返回 200 |
| `bash -n scripts/setup.sh scripts/start.sh` | Shell 语法检查通过 |
| `docker compose config --quiet` | Compose 配置检查通过 |
| `python3 scripts/backup.py --help` | Docker Compose PostgreSQL 备份脚本参数检查通过；未连接 Docker 或操作业务数据 |
| `scripts/check_service.py http://127.0.0.1:8000` | 实际运行服务的健康接口、初始化状态、OpenAPI、首页 HTML、JS 与 CSS 均返回成功 |

业务测试覆盖：

- 销售数据隔离、公海领取、关联联系人移交、过期版本冲突。
- 两位销售同时领取同一客户，只允许一位成功。
- 线索转化不可重复，跟进历史复制到客户。
- 产品报价金额精确计算，合同审批、订单审批及交付。
- 回款累计上限、发票累计上限、回款计划额度与关联客户一致性。
- 驳回、重提和旧审批重复处理保护。
- 导入中任一记录失败则整批回滚，CSV 公式前缀转义。
- 自定义字段、未知字段拒绝、未知模块处理。
- 初始化幂等防护、跨站写入拒绝、登录失败限流、退出登录及账号停用后会话失效。
- 演示数据和统计金额。

测试均使用独立临时数据库；正式本地数据库未初始化账号，用户可在首次打开时自行设置管理员。

## 尚未通过环境验收的项目

- **浏览器视觉及点击验收未完成**：浏览器工具访问本地服务返回 `net::ERR_BLOCKED_BY_CLIENT`；不能将构建成功等同于浏览器交互验收通过。已完成真实 HTTP 静态资源和接口检查。
- **Docker 镜像构建与容器启动未验证**：本机 Docker daemon 未运行；仅验证 Compose 文件语法。
- **恢复演练**：2026-09-27 从运行中服务创建只读 PostgreSQL 转储和附件副本，在独立临时 PostgreSQL 17 容器执行 `pg_restore --clean --if-exists --no-owner`。恢复后校验记录 `25`、成员 `3`、附件元数据 `1`；临时容器已删除，未修改生产数据库、卷或业务记录。
- 尚未进行大规模数据量、压力、长时间运行和全量安全评审。
- 功能覆盖范围与上游差异详见 [功能对照](feature-parity.md)，没有完成 CordysCRM 全量功能对齐验收。

## 本地复查

```bash
bash scripts/start.sh
```

另一个终端：

```bash
.venv/bin/python scripts/check_service.py http://127.0.0.1:8000
.venv/bin/python -m unittest discover -s backend/tests -v
```

浏览器访问 http://127.0.0.1:8000 ，创建管理员后，可按 README 的业务流程验证新建客户、线索转化、商机看板、跟进和审批操作。默认无预设密码。
