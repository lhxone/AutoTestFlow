# AGENT.md — AutoTestFlow AI 开发指南

本文档面向使用 AI（Claude Code 等）辅助开发本项目的开发者。阅读本文档可以快速理解项目架构、开发约定和常见任务的处理方式。

---

## 项目概述

AutoTestFlow 是一个 AI 驱动的自动化测试平台，核心闭环为：

```
禅道同步问题单 → AI 生成测试 → 人工 Review → Git 推送 → CI 执行 → 报告通知
```

- **后端**: Go 1.22+ / Gin / GORM / MySQL 8.0
- **前端**: Vue 3 / TypeScript / Vite / Ant Design Vue 4
- **AI**: Claude API / OpenAI API（通过 HTTP 调用）
- **外部集成**: 禅道 v4.12 RESTful API、GitLab API v4

---

## 架构与分层

### 后端分层

```
cmd/server/main.go          ← 入口：配置→DB→权限缓存→路由→定时任务→HTTP
    │
    ├── router/              ← 路由注册 + 中间件绑定
    ├── middleware/           ← JWT认证 / RBAC权限 / CORS / 请求日志
    ├── handler/             ← HTTP 控制器（参数校验→调用 service→返回响应）
    ├── service/             ← 业务逻辑（核心在这里）
    ├── repository/          ← 数据访问（GORM 操作）
    ├── model/               ← 数据库模型（GORM struct）
    ├── dto/                 ← 请求/响应结构体
    ├── cron/                ← 定时任务调度
    └── pkg/                 ← 公共工具（响应封装/错误码/JWT/AI工具）
```

### 前端分层

```
src/
├── api/          ← Axios 请求封装（按模块拆分）
├── views/        ← 页面组件（每个功能一个目录）
├── layouts/      ← 全局布局（侧边栏 + 顶栏）
├── router/       ← 路由定义 + JWT 守卫 + 权限校验
├── stores/       ← Pinia 状态管理（当前只有 user store）
├── types/        ← TypeScript 接口和枚举
└── utils/        ← Axios 实例（拦截器 + 统一错误处理）
```

---

## 开发约定

### 新增一个 CRUD 模块的标准流程

以新增"XX管理"为例：

1. **数据库**: 在 `migrations/` 下新建 SQL 文件，建表并初始化数据
2. **Model**: 在 `internal/model/` 新建 `xx.go`，定义 GORM struct
3. **DTO**: 在 `internal/dto/` 新建 `xx_dto.go`，定义请求/响应结构
4. **Repository**: 在 `internal/repository/` 新建 `xx_repo.go`，实现 CRUD
5. **Service**: 在 `internal/service/` 新建 `xx_service.go`，实现业务逻辑
6. **Handler**: 在 `internal/handler/` 新建 `xx_handler.go`，实现 HTTP 入口
7. **Router**: 在 `internal/router/router.go` 注册路由，绑定权限中间件
8. **前端 API**: 在 `frontend/src/api/` 新建 `xx.ts`
9. **前端类型**: 在 `frontend/src/types/index.ts` 添加 TypeScript 接口
10. **前端页面**: 在 `frontend/src/views/xx/` 新建 Vue 组件
11. **前端路由**: 在 `frontend/src/router/index.ts` 添加路由
12. **侧边栏**: 在 `frontend/src/layouts/MainLayout.vue` 添加菜单项

### 命名约定

| 层 | Go 命名 | 文件命名 | 示例 |
|----|---------|---------|------|
| Model | PascalCase struct | `xx.go` | `model/project.go` → `type Project struct` |
| DTO | PascalCase + Request/Response | `xx_dto.go` | `dto/project_dto.go` → `CreateProjectRequest` |
| Repository | PascalCase + Repo | `xx_repo.go` | `repository/project_repo.go` → `ProjectRepo` |
| Service | PascalCase + Service | `xx_service.go` | `service/project_service.go` → `ProjectService` |
| Handler | PascalCase + Handler | `xx_handler.go` | `handler/project_handler.go` → `ProjectHandler` |

| 层 | 前端命名 | 文件命名 |
|----|---------|---------|
| API | camelCase 函数 | `xx.ts` |
| 页面 | PascalCase Vue 组件 | `XxList.vue`, `XxDetail.vue` |
| 类型 | PascalCase interface | `types/index.ts` 中集中定义 |

### API 响应约定

所有 API 返回统一结构：

```json
{
  "code": 0,          // 0=成功, >0=业务错误
  "message": "success",
  "data": {}           // 业务数据
}
```

分页：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "list": [],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

错误码定义在 `internal/pkg/errors.go`。

### 权限控制

- 后端：`middleware.RequirePermission("module:action")` 或 `middleware.RequireRoles("admin")`
- 前端路由：`meta: { permission: 'module:action' }` 在路由守卫中校验
- 前端按钮：`v-if="userStore.hasPermission('module:action')"`
- admin 角色自动拥有全部权限

### 数据库规范

- 主键：`BIGINT UNSIGNED AUTO_INCREMENT`
- 时间：`DATETIME` + `DEFAULT CURRENT_TIMESTAMP`
- 软删除：使用 GORM 的 `gorm.DeletedAt`（仅需要软删除的表）
- 唯一索引命名：`uk_字段名`
- 普通索引命名：`idx_字段名`
- SQL 文件开头必须加 `SET NAMES utf8mb4;`（防止中文乱码）

---

## 关键模块说明

### 禅道集成

- **API 版本**: 禅道 v4.12 RESTful API v1
- **认证流程**: `POST /api.php/v1/tokens` 获取 Token → 后续请求 Header 加 `Token: {token}`
- **获取 Bug**: `GET /api.php/v1/products/{id}/bugs`
- **配置位置**: 管理后台「系统设置 → 禅道管理」，存储在 `system_setting` 表
- **Token 自动刷新**: `setting_service.go` 的 `GetZentaoToken()` 方法，过期时自动重新获取
- **代码位置**: `service/zentao_service.go`、`service/setting_service.go`

### AI 生成（gen-test）

- **代码位置**: `service/gentest_service.go`
- **Prompt 模板**: 在 `buildPrompt()` 方法中
- **输出格式**: 严格要求 JSON，通过 `pkg.ExtractJSON()` 做容错解析
- **Mock 触发条件**: `config.yaml` 中 `ai.api_key` 为空
- **支持的 Provider**: Claude（`/v1/messages`）、OpenAI（`/v1/chat/completions`）

### Review 审核流程

- AI 生成完成后自动创建 `review_task`
- 审核操作：approve / reject / request_changes / comment
- approve 后触发 Git 推送
- 代码位置：`service/review_service.go`、`handler/review_handler.go`

### 人工介入

- 测试失败后，可通过前端修改测试用例或脚本
- 每次修改自动保存版本（`test_case_version` / `test_script_version`）
- 记录介入日志（`manual_intervention` 表）
- 代码位置：`service/intervention_service.go`

### 系统设置

- 存储在 `system_setting` 表，category + key 的键值对模式
- 加密字段（`encrypted=1`）在 API 返回时显示 `******`
- 前端传 `******` 时后端跳过更新（不覆盖原值）
- 代码位置：`service/setting_service.go`、`repository/setting_repo.go`

---

## Mock 实现速查

| 模块 | Mock 方法 | 触发条件 | 代码位置 |
|------|----------|---------|---------|
| 禅道同步 | `syncWithMockData()` | `zentao.base_url` 或 `zentao.api_token` 为空 | `service/zentao_service.go` |
| AI 生成 | `mockAIOutput()` | `ai.api_key` 为空 | `service/gentest_service.go` |
| 邮件通知 | `SendTestReport()` 中跳过 | `mail.host` 为空 | `service/notification_service.go` |
| CI 执行 | `mockExecution()` | 项目 `git_repo_url` 为空 | `service/ci_service.go` |

所有 Mock 方法在代码中用 `[MOCK]` 注释标注。

---

## 常见开发任务

### 新增一个系统设置页面

1. 在 `003_system_setting.sql` 中 INSERT 默认配置行
2. 在 `setting_handler.go` 中添加 Get/Save/Test 三个方法
3. 在 `router.go` 的 settings group 中注册路由
4. 在 `frontend/src/api/setting.ts` 中添加 API 函数
5. 在 `frontend/src/views/settings/` 中新建页面
6. 在 `router/index.ts` 和 `MainLayout.vue` 中添加路由和菜单

### 接入新的 AI Provider

1. 在 `gentest_service.go` 的 `callAI()` 方法中添加新的 case
2. 实现 `callXxxAPI()` 方法
3. 在 `config.go` 的 `AIConfig` 中确认配置字段足够
4. 在 `config.yaml` 中添加示例配置

### 新增一个定时任务

1. 在 `cron/scheduler.go` 的 `Start()` 中用 `s.cron.AddFunc()` 注册
2. 实现任务函数
3. cron 表达式使用 6 段格式（秒 分 时 日 月 周）

### 新增一个权限

1. 在 `001_init.sql` 的 `permission` 表中 INSERT
2. 在对应角色的 `role_permission` INSERT 中添加
3. 在 `router.go` 中用 `middleware.RequirePermission("xxx")` 绑定
4. 前端用 `v-if="userStore.hasPermission('xxx')"` 控制显隐

---

## 编译和测试

```powershell
# 后端编译
$env:Path = "D:\Program Files\Go\bin;" + $env:Path
$env:GOPROXY = "https://goproxy.cn,direct"
cd backend
go build -o bin/server.exe cmd/server/main.go

# 前端构建
cd frontend
npm run build

# 数据库 migration
Get-Content backend\migrations\0xx_xxx.sql | docker exec -i atf-mysql mysql -uroot -proot
```

编译后必须重启后端进程（杀掉旧进程再启动新 binary）。前端 Vite 开发模式有热更新，不需要重启。

---

## 外部系统 API 参考

### 禅道 v4.12 RESTful API

| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 获取 Token | POST | `/api.php/v1/tokens` | Body: `{"account":"xx","password":"xx"}` |
| 产品 Bug 列表 | GET | `/api.php/v1/products/{id}/bugs` | Header: `Token: {token}` |
| 项目列表 | GET | `/api.php/v1/projects` | Header: `Token: {token}` |
| 产品列表 | GET | `/api.php/v1/products` | Header: `Token: {token}` |

### GitLab API v4

| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 当前用户 | GET | `/api/v4/user` | Header: `PRIVATE-TOKEN: {token}` |
| 项目列表 | GET | `/api/v4/projects` | 支持分页和搜索 |
| 触发 Pipeline | POST | `/api/v4/projects/{id}/trigger/pipeline` | 需要 Trigger Token |

---

## 注意事项

1. **循环依赖**: Go 包之间不能循环 import。`pkg` 不能 import `middleware`，`middleware` 不能 import `service`
2. **SQL 文件编码**: 所有 SQL 文件开头必须有 `SET NAMES utf8mb4;`
3. **Go 代理**: 国内环境需要设置 `GOPROXY=https://goproxy.cn,direct`
4. **HTTPS 证书**: 内网禅道/GitLab 可能用自签证书，HTTP 客户端已配置 `InsecureSkipVerify: true`
5. **密码字段**: 加密字段在 API 返回时显示 `******`，前端回传 `******` 时后端跳过更新
6. **前端代理**: 开发模式下 `/api` 请求由 Vite 代理到 `localhost:8080`
