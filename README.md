# SmartEstate 智慧社区物业管理系统

> 面向业主、物业人员和管理员的数字化社区服务平台：在线报修、模拟缴费、公告发布、个人房产资料和物业工作台一体化管理。

## Docker 一键启动（推荐）

```bash
docker compose up -d
```

启动完成后访问：

- 前端：<http://localhost:18412>
- 后端健康检查：<http://localhost:19412/healthz>
- 后端 API：<http://localhost:19412/api/v1>

演示帐号密码均为 `password123`：业主 `13800000001`、物业 `13800000002`、管理员 `13800000003`。

## 主要功能

- **物业工作台**：汇总待办报修、待审访客申请、本月已收费用和近期公告。
- **报修管理**：业主创建水电/家具/公共设施等报修；物业筛选、分配和更新进度。
- **费用缴纳**：按业主展示账单，通过支付宝沙箱模拟完成支付和记录查询。
- **访客通行（闭环）**：登记车牌、到访时段、楼栋房号生成待审申请；物业批准时同一车牌在重叠时段只能存在一张有效通行证，冲突则整次拒绝并保持待审（失败原因回写页面，刷新后仍可读）；到访开始前可撤销，撤销或过期后释放时段；重复审批与并发审批只能成功一次。
- **社区公告**：置顶、发布、详情查看与阅读计数。
- **个人中心**：更新昵称、头像 URL，并绑定楼栋、单元和房间。
- **安全与治理**：JWT 登录态、RBAC、操作日志、敏感接口内存限流、统一 JSON 响应。

## 技术栈

| 层级 | 技术 |
| --- | --- |
| 前端 | Vue 3 + TypeScript + Vite + Element Plus + ECharts |
| 后端 | **Go 1.22 + Gin + GORM** |
| 数据库 | MySQL 8.0（本地开发未设置 DSN 时回退 SQLite） |
| 认证 | JWT + RBAC |
| 部署 | Docker Compose + Nginx 反向代理 |

## 本地开发（备选）

前端：

```bash
cd frontend
npm install
npm run dev
```

后端：

```bash
cd backend && go mod tidy && go run ./cmd/server
```

构建后端：

```bash
cd backend && go build ./...
```

默认本地后端采用 SQLite 文件 `backend/smartestate.db`。若要连接 MySQL，请设置 `DB_DRIVER=mysql` 和 `DB_DSN`。

## 常用 API 清单

所有业务接口均以 `/api/v1` 开头，并使用 `{ "code": 0, "message": "ok", "data": ... }` 响应包裹。除登录与健康检查外均需 `Authorization: Bearer <token>`。

| 方法 | 接口 | 用途 / 权限 |
| --- | --- | --- |
| POST | `/auth/login` | 登录（限流） |
| GET/PUT | `/users/me` | 获取或更新个人资料 |
| GET | `/users/staff` | 获取处理人员，`repair:manage` |
| GET/POST | `/repairs` | 工单列表 / 创建工单 |
| PATCH | `/repairs/:id/assign` | 分配处理人，`repair:manage` |
| PATCH | `/repairs/:id/status` | 更新进度，`repair:manage` |
| GET/POST | `/payments` | 账单列表 / 生成账单 |
| POST | `/payments/:id/pay` | 模拟支付（限流） |
| GET/POST | `/announcements` | 公告列表 / 发布，发布需 `announcement:publish` |
| GET | `/announcements/:id` | 公告详情并记录阅读 |
| GET/POST | `/visitor-passes` | 访客通行证列表（业主仅本人，物业全部，可 `?status=` 过滤）/ 登记生成待审申请 |
| POST | `/visitor-passes/:id/approve` | 物业批准，需 `visitor:approve`；重叠冲突整次拒绝并保持待审（HTTP 409 + 冲突原因）；重复/并发审批仅一次成功 |
| POST | `/visitor-passes/:id/reject` | 物业拒绝待审申请并填写原因，需 `visitor:approve` |
| POST | `/visitor-passes/:id/revoke` | 到访开始前撤销（业主本人或物业），撤销后释放时段 |
| GET | `/dashboard/summary` | 工作台汇总（含待审访客申请数 `pending_visitor_passes`） |
| GET | `/operation-logs` | 操作日志，`log:read` |

OpenAPI 摘要位于 `backend/api/openapi.yaml`。

## 项目结构

```text
.
├── frontend/
│   ├── src/api/                # user、repair、payment、announcement、visitor 请求
│   ├── src/stores/             # authStore、userStore、repairStore、paymentStore、visitorStore
│   ├── src/types/              # 共享实体和 permission 类型
│   ├── src/components/common/  # StatCard、RepairStatusBadge、RepairCard、VisitorPassCard 等
│   ├── src/hooks/              # useAuth、useRepairStats、usePermission
│   ├── src/pages/              # Dashboard、Repairs、Payments、Announcements、Visitors、Profile
│   ├── src/router/             # 路由及 guards
│   ├── src/utils/              # request、roleText、feeCalculator
│   └── src/constants/          # repair、user、visitor、errorCodes
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/{config,model,repository,service,handler,router,middleware,dto,constants,util}
│   ├── migrations/
│   ├── api/openapi.yaml
│   └── Dockerfile
├── database/init.sql
├── docker-compose.yml
└── .env.example
```

## 贯穿全栈的实体与分层

`User` 依次存在于数据库/GORM 模型、`model/user.go`、`repository/user_repository.go`、`service/user_service.go`、`handler/user_handler.go`、`router/users.go`、前端 `api/user.ts`、`stores/userStore.ts` 与共享类型。`Repair`、`Payment`、`Announcement` 均按模型→仓储→服务→处理器→路由→前端 API→页面/组件分层，CRUD 没有合并在单一文件。

**严禁合并职责到单一文件。** 当前实现按 `handler → service → repository → model` 单向依赖拆分。为了兼容原始练习的“牵一发动全身”约束，日志模板、权限码、枚举、格式化与提示文案也分散在指定常量/工具/路由/前端文件中；这是题目规定的高耦合演示设计，不应作为新生产系统的推荐模式。

## 枚举出现位置清单

### RepairStatus

- 后端定义：`backend/internal/constants/repair.go`；数据库 `Repair.status`；模型 `backend/internal/model/repair.go`。
- 后端使用：`backend/internal/service/repair_service.go` 状态机、`backend/internal/handler/repair_handler.go` DTO 校验、`backend/internal/constants/log_templates.go`、`backend/internal/util/formatter.go`。
- 前端定义：`frontend/src/constants/repair.ts`、`frontend/src/types/index.ts`。
- 前端使用：`frontend/src/components/common/RepairStatusBadge.vue`、`RepairCard.vue`、`frontend/src/pages/Repairs.vue` 的筛选器、`frontend/src/api/repair.ts`、`frontend/src/hooks/useRepairStats.ts`。

### UserRole

- 后端定义：`backend/internal/constants/user.go`；数据库 `User.role`；模型 `backend/internal/model/user.go`。
- 后端使用：`backend/internal/service/permission_service.go`、`backend/internal/middleware/auth.go`、`middleware/rbac.go`、路由权限与 `backend/internal/util/formatter.go`。
- 前端定义：`frontend/src/constants/user.ts`、`frontend/src/types/index.ts`。
- 前端使用：`frontend/src/stores/authStore.ts`、`frontend/src/hooks/useAuth.ts`、`usePermission.ts`、`frontend/src/router/index.ts` 的 meta、`router/guards.ts`、`components/common/PermissionButton.ts`、`utils/roleText.ts` 与 `App.vue`。

### VisitorPassStatus / 有效态

访客通行证状态：`pending`（待审）、`approved`（已批准）、`rejected`（已拒绝）、`revoked`（已撤销）；已批准通行证还会按当前时间派生非持久化有效态 `upcoming`（未生效）/`active`（通行中）/`expired`（已过期）。

- 后端定义：`backend/internal/constants/visitor.go`；数据库 `visitor_passes.status`；模型 `backend/internal/model/visitor_pass.go`（`effective_state` 为 `gorm:"-"` 计算字段）。
- 后端使用：`backend/internal/service/visitor_pass_service.go`（创建、批准冲突检查、撤销窗口、状态装饰）、`backend/internal/repository/visitor_pass_repository.go`（重叠查询与条件更新）、`backend/internal/handler/visitor_pass_handler.go`、`backend/internal/util/formatter.go`、`constants/messages.go`、`constants/log_templates.go`、`constants/permissions.go`（`visitor:approve`）、`router/visitors.go`。
- 前端定义：`frontend/src/constants/visitor.ts`、`frontend/src/types/index.ts`。
- 前端使用：`frontend/src/components/common/VisitorPassCard.vue`、`frontend/src/pages/Visitors.vue`、`frontend/src/api/visitor.ts`、`frontend/src/stores/visitorStore.ts`、`hooks/usePermission.ts`、`types/permission.ts`、`constants/errorCodes.ts`（40901）。

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `COMPOSE_PROJECT_NAME` | `smartestate` | Compose 项目及容器名前缀 |
| `DB_NAME` / `DB_USER` / `DB_PASSWORD` | `smartestate_db` / `smartestate_user` / `smartestate_pwd` | MySQL 应用数据库与账号 |
| `DB_ROOT_PASSWORD` | `smartestate_root` | MySQL root 密码 |
| `JWT_SECRET` | `change_me_to_a_long_random_string` | JWT 签名密钥，生产必须替换 |
| `FRONTEND_PORT` / `BACKEND_PORT` / `DB_PORT` | `18412` / `19412` / `3306` | 对外端口 |

## Docker 部署说明

Nginx 提供 SPA 静态资源并将 `/api/` 代理至 Docker 内部的 `backend:8080`；浏览器前端只请求同源 `/api`。MySQL 使用命名卷 `smartestate_mysql_data` 持久化。数据库健康后才启动后端，后端通过 `/healthz` 健康后才启动前端。

常见问题：

1. 端口被占用时，修改 `.env` 中的 `FRONTEND_PORT`、`BACKEND_PORT` 或 `DB_PORT` 后重新执行 `docker compose up -d`。
2. 需重置演示数据时运行 `docker compose down -v`，这会删除 MySQL 持久化数据。
3. 中文路径可正常使用：Compose 的 build context 采用相对路径，未将宿主绝对路径传入容器。

## License

MIT
