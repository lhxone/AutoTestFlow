# AutoTestFlow - AI 驱动自动化测试平台

从禅道问题单到测试报告的全自动闭环：**同步问题单 → AI 生成测试 → 人工 Review → Git 推送 → CI 执行 → 报告通知**。

---

## 技术栈

| 层 | 技术 | 版本 |
|----|------|------|
| 前端 | Vue 3 + TypeScript + Vite + Ant Design Vue | Vue 3.4, Vite 5.2, AntDV 4.1 |
| 后端 | Go + Gin + GORM | Go 1.22+, Gin 1.9, GORM 1.25 |
| 数据库 | MySQL | 8.0 |
| AI | Claude API / OpenAI API | claude-sonnet-4-20250514 |
| CI | GitLab CI | pytest + pytest-json-report |
| 容器 | Docker + Docker Compose | Docker 20+ |

---

## 快速开始（开发环境）

### 前置依赖

| 工具 | 最低版本 | 用途 |
|------|---------|------|
| Docker & Docker Compose | 20+ | 运行 MySQL |
| Go | 1.22+ | 编译后端 |
| Node.js & npm | 18+ / 8+ | 编译前端 |
| Git | 2.30+ | 代码管理 |

### 一键启动（PowerShell）

```powershell
.\scripts\start-dev.ps1
```

此脚本会自动：启动 MySQL 容器 → 初始化数据库 → 启动后端 → 安装前端依赖 → 启动前端。

### 手动启动

```powershell
# 1. 启动 MySQL 容器
docker-compose up -d mysql

# 2. 等待 MySQL 就绪(约10秒)，初始化数据库
.\scripts\init-db.ps1
# 或手动执行:
# Get-Content backend\migrations\001_init.sql | docker exec -i atf-mysql mysql -uroot -proot
# Get-Content backend\migrations\002_seed_skill.sql | docker exec -i atf-mysql mysql -uroot -proot
# Get-Content backend\migrations\003_system_setting.sql | docker exec -i atf-mysql mysql -uroot -proot

# 3. 编译并启动后端
cd backend
go mod tidy
go build -o bin/server.exe cmd/server/main.go
.\bin\server.exe -config configs/config.yaml

# 4. 启动前端(新终端窗口)
cd frontend
npm install
npm run dev
```

### 访问地址

| 服务 | 地址 | 说明 |
|------|------|------|
| 前端 | http://localhost:3000 | Vue 开发服务器，自动代理 /api 到后端 |
| 后端 | http://localhost:8080 | Go Gin HTTP 服务 |
| MySQL | localhost:3306 | root / root，数据库 auto_test_flow |

### 默认登录

- 账号：`admin`
- 密码：`admin123`
- 角色：管理员（全部权限）

---

## 生产环境部署

### 方式一：Docker Compose 全容器部署（推荐）

仓库已内置可直接运行的全容器文件：

- `docker-compose.prod.yml`
- `backend/Dockerfile`
- `backend/configs/config.docker.yaml`
- `frontend/Dockerfile`
- `frontend/nginx.conf`

#### 1) 启动前检查

```powershell
docker --version
docker compose version
```

#### 2) （可选）覆盖默认配置

默认端口和数据库配置：

- Frontend: `80`
- Backend: `8080`
- MySQL: `3306`
- MySQL root 密码: `root`

如需覆盖，可在命令前设置环境变量：

```powershell
$env:MYSQL_ROOT_PASSWORD='YourStrongPassword'
$env:MYSQL_DATABASE='auto_test_flow'
$env:FRONTEND_PORT='80'
$env:BACKEND_PORT='8080'
$env:MYSQL_PORT='3306'
$env:ATF_MYSQL_DATA_DIR='/data/AutoTestFlow/mysql'
$env:ATF_GIT_WORK_DIR='/data/AutoTestFlow/workspace/repos'
$env:ATF_CLI_WORKSPACE_DIR='/data/AutoTestFlow/workspace/cli-runtime'
$env:ATF_LOG_DIR='/data/AutoTestFlow/logs'
```

#### 3) 构建并启动

```powershell
docker compose -f docker-compose.prod.yml up -d --build
```

默认持久化目录布局：

- `/data/AutoTestFlow/app`：部署到服务器上的项目工作区
- `/data/AutoTestFlow/mysql`：MySQL 数据目录
- `/data/AutoTestFlow/workspace/repos`：Git 工作目录
- `/data/AutoTestFlow/workspace/cli-runtime`：CLI / Eino 运行时工作区
- `/data/AutoTestFlow/logs`：后端日志目录

#### 4) 查看状态与日志

```powershell
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs -f backend
docker compose -f docker-compose.prod.yml logs -f frontend
```

#### 5) 访问地址

- 前端：`http://localhost`（或你设置的 `FRONTEND_PORT`）
- 后端健康检查：`http://localhost:8080/health`（或你设置的 `BACKEND_PORT`）
- MySQL：`localhost:3306`（或你设置的 `MYSQL_PORT`）

#### 6) 停止与清理

```powershell
docker compose -f docker-compose.prod.yml down
```

如需连同数据库数据卷一起删除（危险操作）：

```powershell
docker compose -f docker-compose.prod.yml down -v
```

### 方式二：裸机部署

1. **MySQL**：安装 MySQL 8.0，执行 `backend/migrations/` 下所有 SQL 文件
2. **后端**：编译 `go build -o server cmd/server/main.go`，配置 `config.yaml`，用 systemd 托管
3. **前端**：`npm run build`，将 `dist/` 部署到 Nginx，配置反向代理

### GitLab CI/CD 自动部署

仓库根目录 `.gitlab-ci.yml` 已调整为 `test -> build -> deploy` 三阶段：

- `backend-test`：执行 `go test ./...`
- `frontend-check`：执行 `npm ci && npm run build`
- `build-backend-image` / `build-frontend-image`：验证前后端 Docker 镜像可构建
- `deploy-production`：通过 SSH 把仓库同步到 `root@192.168.53.106:/data/AutoTestFlow/app`，随后在远端执行 `docker compose up -d --build`

需要在 GitLab CI/CD Variables 中至少配置以下变量：

- `DEPLOY_PASSWORD`：用于登录 `root@192.168.53.106` 的 SSH 密码
- `MYSQL_ROOT_PASSWORD`：生产环境 MySQL root 密码

建议同时配置以下变量：

- `ATF_JWT_SECRET`
- `ATF_ZENTAO_BASE_URL`
- `ATF_ZENTAO_API_TOKEN`
- `ATF_MAIL_HOST`
- `ATF_MAIL_PORT`
- `ATF_MAIL_USERNAME`
- `ATF_MAIL_PASSWORD`
- `ATF_MAIL_FROM`

如果你想固定服务器指纹而不是在流水线里动态扫描，可额外配置：

- `DEPLOY_SSH_KNOWN_HOSTS`

### 部署后配置

部署后通过管理后台完成以下配置（系统设置菜单）：

| 步骤 | 页面 | 说明 |
|------|------|------|
| 1 | 系统设置 → 禅道管理 | 配置禅道地址、账号密码，点击「测试连接」 |
| 2 | 系统设置 → GitLab 管理 | 配置 GitLab 地址和 Access Token |
| 3 | 项目管理 → 新建项目 | 创建项目，关联禅道项目和 Git 仓库 |
| 4 | config.yaml → ai 段 | 配置 AI API Key（Claude 或 OpenAI） |
| 5 | config.yaml → mail 段 | 配置 SMTP 邮件服务器（可选） |

---

## 项目结构

```
AutoTestFlow/
├── backend/                          # Go 后端 (60+ 文件)
│   ├── cmd/server/main.go            # 入口
│   ├── internal/
│   │   ├── config/                   # 配置加载 (Viper)
│   │   ├── middleware/               # JWT 认证 + RBAC 权限 + CORS + 日志
│   │   ├── model/                    # 数据模型 (30 张表)
│   │   ├── handler/                  # HTTP 控制器 (10 个)
│   │   ├── service/                  # 业务逻辑 (11 个)
│   │   ├── repository/              # 数据访问层 (9 个)
│   │   ├── dto/                      # 请求/响应结构 (8 个)
│   │   ├── router/                   # 路由注册 + 权限绑定
│   │   ├── cron/                     # 定时任务 (禅道同步 + AI 自动生成)
│   │   └── pkg/                      # 工具 (响应/错误码/JWT/AI客户端)
│   ├── migrations/                   # 数据库 SQL (3 个)
│   ├── configs/config.yaml           # 配置文件
│   └── bin/                          # 编译产物
├── frontend/                         # Vue 3 前端 (25+ 文件)
│   └── src/
│       ├── api/                      # API 封装 (8 个模块)
│       ├── views/                    # 页面组件 (12 个页面)
│       │   ├── login/                # 登录页
│       │   ├── dashboard/            # 工作台
│       │   ├── user/                 # 用户管理
│       │   ├── project/              # 项目管理
│       │   ├── issue/                # 问题单列表
│       │   ├── agent/                # Agent 管理
│       │   ├── review/               # Review 审核 (列表 + 详情)
│       │   ├── testTask/             # 测试任务 + 执行记录
│       │   └── settings/             # 系统设置 (禅道 + GitLab)
│       ├── layouts/                  # 侧边栏布局
│       ├── router/                   # 路由 + 守卫
│       ├── stores/                   # Pinia 状态管理
│       ├── types/                    # TypeScript 类型定义
│       └── utils/                    # Axios 封装
├── docs/                             # 项目文档 (5 个)
│   ├── api-reference.md              # API 接口完整清单
│   ├── frontend-design.md            # 前端设计文档
│   ├── mock-dependencies.md          # Mock 依赖清单
│   └── gen-test-skill-design.md      # gen-test Skill 设计
├── test-repo-template/               # 测试仓库模板
│   ├── tests/                        # pytest 示例
│   ├── pytest.ini                    # pytest 配置
│   ├── requirements.txt              # Python 依赖
│   └── .gitlab-ci.yml                # 测试仓库 CI
├── scripts/                          # 运维脚本
│   ├── start-dev.ps1                 # 一键启动开发环境
│   └── init-db.ps1                   # 数据库初始化
├── docker-compose.yml                # 开发环境容器编排
├── .gitlab-ci.yml                    # 项目 CI/CD
├── Makefile                          # 常用命令
└── AGENT.md                          # AI 开发指南
```

---

## 核心业务流程

```
项目配置完成 ──→ 系统设置中配置禅道/GitLab
    │
    ▼
定时同步禅道问题单 (可配置频率，默认30分钟)
    │
    ▼
筛选"已解决"问题单 ──→ 自动或手动触发
    │
    ▼
AI 生成测试用例/脚本/文档 (gen-test Skill)
    │
    ▼
进入"待审核"状态 ──→ 自动创建 Review 任务
    │
    ▼
人工 Review (通过 / 驳回 / 需修改)
    │
    ▼
审核通过 ──→ 推送到 Git 测试仓库
    │
    ▼
CI 执行测试 (每日定时 / 手动触发)
    │
    ├── 通过 → 生成报告 → 邮件通知负责人
    └── 失败 → 人工介入修改用例/脚本 → 重新执行
```

---

## 配置说明

### config.yaml（后端静态配置）

| 段 | 配置项 | 说明 | 必须 |
|----|--------|------|------|
| server | port, mode | 服务端口和运行模式 | 是 |
| database | host, port, user, password, dbname | MySQL 连接 | 是 |
| jwt | secret, expire_hours | JWT 密钥和有效期 | 是 |
| ai | provider, api_key, model | AI 模型配置 | 否(有Mock) |
| git | work_dir, commit_author, commit_email | Git 工作目录 | 否 |
| mail | host, port, username, password | SMTP 配置 | 否(有Mock) |

### 系统设置（管理后台动态配置）

| 页面 | 配置项 | 说明 |
|------|--------|------|
| 禅道管理 | base_url, account, password, sync_interval | 禅道连接和同步频率 |
| GitLab 管理 | base_url, access_token, api_version | GitLab 连接 |

> 详见 [Mock 依赖清单](docs/mock-dependencies.md) 了解哪些功能有 Mock 实现

---

## 角色与权限

| 角色 | 编码 | 项目 | 用户 | 问题单 | Agent | Review | 测试 | 报告 | 系统设置 |
|------|------|------|------|--------|-------|--------|------|------|---------|
| 管理员 | admin | 全部 | 全部 | 全部 | 全部 | 全部 | 全部 | 全部 | 全部 |
| 测试负责人 | test_lead | 查看/编辑 | 查看 | 全部 | 管理 | 审核 | 全部 | 全部 | - |
| 测试工程师 | tester | 查看 | - | 查看 | 查看 | 提交 | 执行/介入 | 查看 | - |
| 开发负责人 | dev_lead | 查看 | - | 查看 | - | 查看 | 查看 | 查看 | - |
| 查看者 | viewer | 查看 | - | 查看 | - | 查看 | 查看 | 查看 | - |

---

## 文档索引

| 文档 | 说明 |
|------|------|
| [API 接口清单](docs/api-reference.md) | 全部 REST API 定义、参数、响应格式 |
| [前端设计文档](docs/frontend-design.md) | 12 个页面的字段、交互、路由设计 |
| [Mock 依赖清单](docs/mock-dependencies.md) | 所有 Mock 实现位置和替换方法 |
| [gen-test Skill 设计](docs/gen-test-skill-design.md) | AI 生成测试的 Prompt、输入输出、容错机制 |
| [AGENT.md](AGENT.md) | AI 辅助开发指南，架构约定和开发规范 |

---

## 常用命令

```powershell
# 数据库
docker-compose up -d mysql          # 启动 MySQL
docker-compose down -v              # 停止并删除数据卷(慎用)
.\scripts\init-db.ps1               # 初始化数据库

# 后端
cd backend
go mod tidy                          # 补全依赖
go build -o bin/server.exe cmd/server/main.go  # 编译
.\bin\server.exe -config configs/config.yaml   # 运行

# 前端
cd frontend
npm install                          # 安装依赖
npm run dev                          # 开发模式
npm run build                        # 生产构建
```
