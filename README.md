# Cordys CRM Go

Cordys CRM 的 Go 衍生实现。后端采用 Gin、GORM 和 PostgreSQL/MySQL，前端为 Vue 3，支持私有部署。

本项目遵循 [FIT2CLOUD Open Source License](LICENSE)，包括 GPLv3 义务、Cordys CRM Logo、商标和版权信息保留要求。功能对齐以 `/Users/caojunfeng/codeWorkSpace/CordysCRM` 为基线。

## 本地启动

建议 Go 1.24、Node.js 22，并准备 PostgreSQL 或 MySQL。

```bash
bash scripts/setup.sh
export CRM_DB_DRIVER=postgres
export CRM_DB_DSN='postgres://crm:password@127.0.0.1:5432/crm?sslmode=disable'
bash scripts/start.sh
```

打开 http://127.0.0.1:8000 。第一次进入创建管理员账号，没有预设密码。可选择载入虚构演示数据；正式业务可取消勾选以创建空系统。已有依赖时直接运行 `bash scripts/start.sh`。

健康检查：http://127.0.0.1:8000/api/health 。请通过 PostgreSQL 或 MySQL 配置连接数据库；停止服务按 Ctrl+C。

## 开发模式

两个终端分别运行：

```bash
go run ./backend/cmd/migrate
go run ./backend/cmd/server
```

```bash
cd frontend
npm run dev
```

打开 Vite 输出的本地地址。前端 `/api` 由 Vite 代理到后端；生产模式由 Go 服务同源提供构建后的前端。页面使用 hash 路由，刷新子页面有效。

## 移动端 PWA

生产部署后，可在手机浏览器打开 CRM 并使用浏览器的“添加到主屏幕”或“安装应用”功能安装 PWA。安装后以独立窗口启动，并缓存应用壳以便临时离线时打开；业务数据、登录和写入操作仍需要网络连接。

## Docker

```bash
docker compose up -d --build
```

访问 http://127.0.0.1:8000 。命名卷 `crm-postgres` 保存数据库。默认仅本机可访问；在对外开放服务前完成管理员初始化。HTTPS 反向代理部署时设置 `CRM_COOKIE_SECURE=1`，必要时将精确的前端 Origin 配置到 `CRM_ALLOWED_ORIGINS`（逗号分隔）。

如需在保留现有数据的前提下补充可联调的演示数据，执行：

```bash
docker compose exec crm ./crm-seed
```

首次执行会创建演示管理员 `demo`，密码为 `DemoPass123`，并写入客户、线索、商机、产品、报价、合同、订单、发票、回款及任务等关联记录。重复执行只补齐缺失的演示记录，不会清空或覆盖已有业务数据。

### GitHub 构建与服务器更新

推送 `main` 分支后，`.github/workflows/deploy.yml` 只构建 `linux/amd64` 和 `linux/arm64` 镜像，并发布到 `ghcr.io/<owner>/<repository>:latest` 与 `sha-<完整提交 SHA>`。在 GitHub Actions 的 `Publish CRM Image` 中确认 `publish` 成功后，再到服务器手动更新。数据库与附件保存在服务器 Docker 命名卷中，不会写入镜像或上传 GitHub。

服务器首次部署前安装 Docker Engine 与 Compose 插件。从本机项目目录将部署文件传到服务器，不需要服务器访问 GitHub：

```bash
ssh user@server 'mkdir -p /opt/crm/deploy /opt/crm/scripts'
scp deploy/docker-compose.yml deploy/.env.example user@server:/opt/crm/deploy/
scp scripts/update-self-hosted.sh user@server:/opt/crm/scripts/
```

在服务器创建仅保存在服务器上的配置文件：

```bash
cd /opt/crm
cp deploy/.env.example .env
chmod 600 .env
```

编辑 `/opt/crm/.env`，将 `CRM_IMAGE` 设为自己的 `ghcr.io/<owner>/<repository>:latest`，并设置 `CRM_DB_PASSWORD`、`CRM_CONFIG_ENCRYPTION_KEY` 和线上域名。两个密钥可分别使用 `openssl rand -hex 32` 生成。私有 GHCR 镜像还需用具有 `read:packages` 权限的凭据在服务器执行 `docker login ghcr.io`。

首次启动以及后续镜像更新都在服务器执行：

```bash
cd /opt/crm
bash scripts/update-self-hosted.sh
```

脚本使用 `deploy/docker-compose.yml`、服务器上的 `.env` 和固定的 Compose 项目名 `crm`，依次拉取镜像、启动服务并检查 `/api/health`。如需使用其他配置文件，可传入绝对路径 `ENV_FILE` 或 `COMPOSE_FILE`。部署文件有修改时，先从本机重新执行上面的 `scp` 命令。更新已有服务前先备份数据库和附件；不要执行 `docker compose down -v`。

需要回滚时，将 `/opt/crm/.env` 中的 `CRM_IMAGE` 改为历史 `sha-<完整提交 SHA>`，再执行 `bash scripts/update-self-hosted.sh`。

## 已实现的流程

1. 创建线索、记录跟进，转化为客户和联系人；跟进记录复制到客户，原线索保留转化关联。
2. 维护客户与联系人，创建商机并推进阶段；商机可切换列表与看板。
3. 维护产品、多级规格与可用库存，报价明细支持规格、折扣和税率，服务器使用 Decimal 计算含税金额。
4. 报价审批通过后关联创建合同；合同审批通过后创建订单、工商抬头、发票、回款计划、回款。
5. 报价、合同、订单、发票、回款支持按角色或指定管理员/经理逐级审批，可按金额门槛或字段条件跳过不适用级别；字段条件支持等于、不等于、包含和为空，且同级条件需全部满足。当前审批人可转签或加签给同组织启用的管理员或经理，可驳回、撤回、修改并再次提交。已通过与审批中的单据锁定业务字段。
6. 同一业务记录重复上传同名附件会保留为递增版本，历史版本仍可下载或按权限删除。
7. 报价与合同详情可打印为基础对外交付单据；报价包含产品明细和合计，浏览器可直接保存为 PDF。
8. 订单创建和修改会原子预占库存，删除草稿释放库存，完成交付核销预占；草稿订单可按产品明细拆单。回款与开票等累计金额不得超过合同金额，关联回款计划的到账金额不得超过计划金额。
9. 管理员可配置商机销售阶段、审批画布、表单布局和纯文本文档模板；模板使用 `{{字段标识}}`，浏览器打印可保存 PDF，模板不会执行 HTML。
10. 管理员可发布组织公告；管理员和经理可在“自助 BI”按模块、维度和指标生成权限内报表。
7. 公海支持领取、释放和分配；客户移交会同步移交关联业务。并发领取及过期版本提交返回冲突错误。

统计页面展示客户数、商机金额、加权预计成交额、已审批合同及回款、六个月趋势、销售漏斗、任务和逾期回款计划。统计基于当前用户可见数据。

## 角色

| 角色 | 范围 |
| --- | --- |
| 管理员 | 全部业务、审批、成员管理、自定义字段、操作日志 |
| 销售经理 | 全部业务、审批与分配；不包含系统配置 |
| 销售成员 | 自己的业务、只读产品目录、公海浏览与领取 |

管理员可在“组织与角色”为自定义角色配置各模块的查看/维护权限、仅本人/本部门/本部门及下级/指定部门/全部数据五种范围，以及非必填字段的隐藏或只读规则，再在“成员与权限”把销售基础角色的成员绑定到该组织、角色和部门。绑定成员的访问同时受模块权限、记录所有权、组织隔离、角色数据范围和字段规则限制；字段规则由服务端在目录、记录、搜索、导出、导入及 API Key/Agent 请求中执行。维护权限包含查看权限。Cookie 与用户 API Key 每次请求读取当前角色权限，修改角色后无需重新发放密钥。自定义角色不授予管理员或经理能力。

旧成员未绑定组织/自定义角色时保留原有权限。旧记录无组织归属，绑定自定义角色的成员无法查看，需先规划数据迁移；已有组织记录在启用部门范围前也应补齐负责人部门快照，否则部门范围会保守拒绝访问。部署前运行现有数据库迁移命令以增加成员和记录的组织、角色及负责人部门字段。经理和管理员均可处理待审批记录，包括自己提交的记录；尚无审批可视化设计器或分离职责规则。

单组织部署在完成数据库备份后，可用以下命令为尚未归属组织的旧成员和记录补齐归属。默认只输出预览；仅在确认数量正确后添加 `--apply`。已归属其他组织的数据不会被改写。

```bash
go run ./backend/cmd/assign-organization --organization-id <组织ID>
go run ./backend/cmd/assign-organization --organization-id <组织ID> --apply
```

## 数据导入导出

- 列表支持按当前筛选导出 CSV，最多 10000 条，包含 UTF-8 BOM，并处理常见表格公式注入前缀。
- 导入接受 JSON 数组、CSV 或 XLSX 文件；页面可下载字段标识表头的 XLSX、带 UTF-8 BOM 的 CSV 模板，CSV 也可直接回导列表导出的文件，XLSX 读取首个工作表，表头也可使用字段标识。页面会先在回滚事务中预览全部校验结果，确认后才写入；一次最多 500 条、文件最大 5 MB，整批提交失败全部回滚。
- 关联字段使用内部记录 ID，可从详情 API 或列表 API 获取。不是上游数据一键迁移，也不是 XLSX 导入。
- 管理员可给模块添加选填的文本、多行文本、数字、日期、邮箱、下拉、公式求和和子表字段，表单与详情自动显示。公式字段仅允许引用同模块数值或金额字段，结果由服务端计算且客户端不能写入；子表支持文本、数字、金额和日期列，每个字段最多 100 行。
- 管理员可在“业务模块”配置字段分组和排序，布局只改变表单与详情展示，不改变字段校验、权限或导入导出规则。
- 管理员可为商机配置 2–20 个销售阶段；已有记录保留原值，新建和编辑使用最新阶段。

## 验证

```bash
go test ./backend/...
npm run build --prefix frontend
```

Go 编译测试不修改业务数据库。启动服务后可使用现有前端进行接口联调。

## 配置与备份

| 环境变量 | 用途 |
| --- | --- |
| `CRM_DB_DRIVER` | 数据库类型：`postgres` 或 `mysql` |
| `CRM_DB_DSN` | 数据库连接串 |
| `CRM_HOST` / `CRM_PORT` | `scripts/start.sh` 的监听地址与端口，默认 `127.0.0.1:8000` |
| `CRM_CUSTOMER_RECYCLE_DAYS` | 客户闲置自动回收到公海的天数；`0`（默认）表示关闭，设置正整数后每小时检查一次 |
| `CRM_COOKIE_SECURE` | 设置 `1` 后只通过 HTTPS 发送会话 Cookie |
| `CRM_ALLOWED_ORIGINS` | 额外允许的写请求 Origin，需精确指定 |
| `CRM_FRONTEND` | 后端托管的前端 dist 目录 |
| `CRM_WECOM_CORP_ID` | 企业微信企业 ID；配置企业微信通知时必填 |
| `CRM_WECOM_AGENT_ID` | 企业微信自建应用 AgentId；配置企业微信通知时必填 |
| `CRM_WECOM_SECRET` | 企业微信自建应用 Secret；配置企业微信通知时必填且不得提交到仓库 |
| `CRM_WECOM_OAUTH_REDIRECT_URL` | 企业微信 OAuth 回调地址，例如 `https://crm.example.com/api/auth/wecom/callback` |
| `CRM_INTEGRATION_MOCK` | 设置 `1` 后飞书、钉钉、SSO、税务、银行、标讯连接器使用模拟联调 |
| `CRM_FEISHU_WEBHOOK_URL` / `CRM_DINGTALK_WEBHOOK_URL` | 办公平台适配端点 |
| `CRM_SSO_DISCOVERY_URL` | SSO/OIDC 发现或适配端点 |
| `CRM_TAX_ADAPTER_URL` / `CRM_BANK_ADAPTER_URL` / `CRM_TENDER_ADAPTER_URL` | 税务、银行、标讯适配端点 |

企业微信为可选能力，可由管理员在“系统管理 → 业务模块 → 企业微信”中配置，也可通过上述环境变量提供默认配置。企业 ID、AgentId 和 Secret 必须同时配置；后台保存后即时生效，删除后台配置会恢复环境变量配置。CRM 用户名映射企业微信 `userid`；支持审批消息、OAuth 登录和管理员通讯录同步。启用 OAuth 时还需填写回调地址，并在企业微信后台设置可信域名、应用可见范围和可信 IP。保存后先执行“连接测试”，再到“成员与权限”同步通讯录。外部投递失败不会影响审批或站内通知。

“业务模块”的集成中心可查看飞书、钉钉、SSO、税务开票、银行对账和自动标讯连接状态并执行模拟或真实端点测试。Token 只从服务端环境变量读取，不通过接口返回。真实业务处理仍取决于对应平台账号、授权范围、回调域名和受监管接口资质。

本地脚本读取进程环境变量，也会读取 `.env`。Docker Compose 可读取 `.env`。密码以 bcrypt 哈希保存，会话使用 HttpOnly / SameSite Cookie，默认有效 12 小时；失败登录有次数限制，改密及停用会撤销相关会话。

Docker Compose 的 PostgreSQL 部署可使用下列脚本生成一致快照。脚本只在显式传入 `--stop-crm` 时暂停 CRM 服务，并在完成后自动启动；备份目录包含 `database.dump`、`uploads/` 和不含密钥的 `manifest.json`，不会覆盖已有目录。

```bash
python3 scripts/backup.py backups/$(date +%F-%H%M%S) --stop-crm
```

恢复只能先在隔离的演练环境进行，不能直接覆盖生产卷：停止演练环境的 `crm` 服务后，将 `database.dump` 通过 `pg_restore --clean --if-exists --no-owner` 恢复到空的演练数据库，再用 `docker compose cp <备份目录>/uploads/. crm:/app/uploads` 还原附件，启动服务并执行健康检查、登录、附件下载及关键记录抽查。恢复成功后才可按同一版本、同一流程安排生产变更；MySQL 或非 Compose 部署请使用对应数据库的原生一致性备份与恢复工具。

## 实现边界

JSON 业务记录适用于当前业务模型与后续迭代。筛选与统计目前在应用层处理，未进行大数据量或多实例压力测试。会计、开票、收款均是业务登记，不会向银行或税务平台实际发起交易。

参考项目核对范围：上游 README、技术栈和前端模块目录。当前已实现 PWA、审批画布、动态布局、公式、子表、文档模板、自助 BI、MCP、企业微信及外部连接器适配层。原生 App、真实第三方账号联调、受监管接口生产交易和 CordysCRM 数据库直连迁移仍需要外部工程、凭据或明确的上游版本/schema。
