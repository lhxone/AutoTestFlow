# AutoTestFlow 前端设计文档

## 技术栈

| 组件 | 版本 | 说明 |
|------|------|------|
| Vue | 3.4 | Composition API + `<script setup>` |
| TypeScript | 5.4 | 全量类型覆盖 |
| Vite | 5.2 | 开发构建 |
| Ant Design Vue | 4.1 | UI 组件库 |
| Pinia | 2.1 | 状态管理 |
| Vue Router | 4.3 | 路由管理 |
| Axios | 1.6 | HTTP 请求 |
| Monaco Editor | 0.47 | 代码编辑/Diff 展示（预留） |

---

## 页面清单

| 路由 | 页面组件 | 说明 | 权限 |
|------|---------|------|------|
| `/login` | LoginPage.vue | 登录页 | 公开 (`meta.public: true`) |
| `/dashboard` | DashboardPage.vue | 工作台/概览 | 登录即可 |
| `/users` | UserList.vue | 用户管理 | `user:list` |
| `/projects` | ProjectList.vue | 项目管理 | `project:list` |
| `/issues` | IssueList.vue | 问题单列表 | `issue:list` |
| `/agents` | AgentList.vue | Agent 管理 | `agent:list` |
| `/reviews` | ReviewList.vue | Review 审核列表 | `review:list` |
| `/reviews/:id` | ReviewDetail.vue | Review 详情/审核操作 | `review:list` |
| `/test-tasks` | TestTaskList.vue | 测试任务列表 | `test:list` |
| `/executions` | ExecutionList.vue | 测试执行记录 | `test:list` |
| `/settings/zentao` | ZentaoSettings.vue | 禅道管理 | `role: admin` |
| `/settings/gitlab` | GitLabSettings.vue | GitLab 管理 | `role: admin` |

---

## 目录结构

```
frontend/src/
├── api/              # API 请求封装(8个模块)
│   ├── auth.ts       # 登录/登出/刷新
│   ├── user.ts       # 用户 CRUD
│   ├── project.ts    # 项目 CRUD
│   ├── issue.ts      # 问题单查询/同步
│   ├── agent.ts      # Agent/Skill/MCP
│   ├── review.ts     # Review 列表/详情/审核
│   ├── testTask.ts   # 测试任务/用例修改/执行记录
│   └── setting.ts    # 禅道/GitLab 系统设置
├── views/            # 页面组件(12个页面)
│   ├── login/        # LoginPage.vue
│   ├── dashboard/    # DashboardPage.vue
│   ├── user/         # UserList.vue
│   ├── project/      # ProjectList.vue
│   ├── issue/        # IssueList.vue
│   ├── agent/        # AgentList.vue
│   ├── review/       # ReviewList.vue + ReviewDetail.vue
│   ├── testTask/     # TestTaskList.vue + ExecutionList.vue
│   └── settings/     # ZentaoSettings.vue + GitLabSettings.vue
├── layouts/          # MainLayout 侧边栏布局
├── router/           # 路由 + 守卫
├── stores/           # Pinia (user store)
├── types/            # TypeScript 类型 + 枚举映射
├── utils/            # Axios 封装
├── App.vue
└── main.ts
```

---

## 路由守卫逻辑

```
用户访问页面
  │
  ├── 是公开页面(/login)？ → meta.public === true → 直接放行
  │
  ├── 有 access_token？(userStore.isLoggedIn)
  │     ├── 否 → 重定向到 /login
  │     └── 是 → userInfo 已加载？
  │              ├── 否 → 调用 userStore.fetchUserInfo() 拉取
  │              └── 是 → 检查页面权限(meta.permission)
  │                        ├── 通过 → 放行
  │                        └── 不通过 → 重定向到 /dashboard
```

注意: settings 路由使用 `meta.role: 'admin'` 标记，但当前路由守卫仅检查 `meta.permission`，role 级别的守卫需由菜单显隐 + 后端接口权限共同保障。

---

## 登录与权限控制方案

### Token 管理

- **access_token**: 存 `localStorage`，有效期 24h
- **refresh_token**: 存 `localStorage`，有效期 7d
- 每次请求自动在 Header 注入 `Authorization: Bearer {token}`
- 响应 401 时自动清除 Token 并跳转登录页

### 权限控制

- **路由级**: `meta.permission` 在路由守卫中校验
- **按钮级**: `v-if="userStore.hasPermission('xxx')"` 控制显隐
- **菜单级**: MainLayout 中根据权限动态显示菜单项
- **角色级**: 系统设置子菜单使用 `v-if="isAdmin"` 仅对 admin 角色展示
- admin 角色自动拥有全部权限

---

## 布局: MainLayout

MainLayout 采用 Ant Design Vue 的 `a-layout` 组件，包含:

- **可折叠侧边栏** (`a-layout-sider`): 宽度 220px，深色主题，折叠后显示 "ATF" 缩写
- **顶栏** (`a-layout-header`): 白色背景，左侧为折叠按钮 + 面包屑，右侧为用户头像下拉菜单(退出登录)
- **内容区** (`a-layout-content`): 白色卡片背景，16px 外边距，24px 内边距

### 侧边栏菜单项

| 菜单项 | 路由 | 图标 | 可见条件 |
|--------|------|------|----------|
| 工作台 | `/dashboard` | DashboardOutlined | 始终可见 |
| 项目管理 | `/projects` | ProjectOutlined | `project:list` |
| 问题单 | `/issues` | BugOutlined | `issue:list` |
| Agent 管理 | `/agents` | RobotOutlined | `agent:list` |
| Review 审核 | `/reviews` | AuditOutlined | `review:list` |
| 测试任务 | `/test-tasks` | ExperimentOutlined | `test:list` |
| 执行记录 | `/executions` | PlayCircleOutlined | `test:list` |
| 用户管理 | `/users` | TeamOutlined | `user:list` |
| 系统设置 (子菜单) | - | SettingOutlined | `isAdmin` |
| -- 禅道管理 | `/settings/zentao` | - | 同上 |
| -- GitLab 管理 | `/settings/gitlab` | - | 同上 |

---

## 各页面详细说明

### 1. LoginPage -- 登录页

- **路由**: `/login`
- **页面标题**: AutoTestFlow
- **权限**: 公开
- **API 调用**: `POST /api/auth/login` (通过 `login()`)，成功后调用 `GET /api/auth/me` (通过 `userStore.fetchUserInfo()`)
- **表单字段**:

| 字段 | 组件 | 必填 | 说明 |
|------|------|------|------|
| username | a-input | 是 | 用户名 |
| password | a-input-password | 是 | 密码 |

- **交互**: 
  - 点击「登 录」按钮提交表单，loading 状态防重复
  - 登录成功后存储 access_token / refresh_token，拉取用户信息，跳转 `/dashboard`
  - 页面底部提示默认账号: `admin / admin123`

---

### 2. DashboardPage -- 工作台

- **路由**: `/dashboard`
- **页面标题**: 工作台
- **权限**: 登录即可
- **API 调用**: 暂无 (TODO: 从 API 获取真实统计数据)
- **页面组成**:
  - **统计卡片行** (4列):
    - 项目总数
    - 待审核 Review (黄色)
    - 待人工介入 (红色)
    - 今日测试通过率 (绿色，百分比)
  - **快捷操作卡片**: 跳转按钮 -- 查看问题单、审核 Review、测试任务
  - **最近活动卡片**: 暂显示空状态 (`a-empty`)

---

### 3. UserList -- 用户管理

- **路由**: `/users`
- **页面标题**: 用户管理
- **权限**: `user:list`
- **API 调用**:
  - `GET /api/users` -- 用户列表 (通过 `getUserList()`)
  - `POST /api/users` -- 创建用户 (通过 `createUser()`)
  - `PUT /api/users/:id` -- 编辑用户 (通过 `updateUser()`)
  - `DELETE /api/users/:id` -- 删除用户 (通过 `deleteUser()`)
- **搜索栏**:
  - 关键词输入 (搜索用户名/姓名/邮箱，支持回车搜索)
  - 状态下拉 (启用/禁用)
  - 查询按钮 + 新增用户按钮
- **表格列**:

| 列名 | 字段 | 宽度 | 说明 |
|------|------|------|------|
| ID | id | 60px | |
| 用户名 | username | 自适应 | |
| 姓名 | real_name | 自适应 | |
| 邮箱 | email | 自适应 | |
| 角色 | roles | 自适应 | Tag 列表显示 |
| 状态 | status | 80px | 绿色=启用，红色=禁用 |
| 操作 | - | 140px | 编辑 / 删除(需确认) |

- **新增/编辑弹窗 (Modal)** 表单字段:

| 字段 | 组件 | 必填 | 说明 |
|------|------|------|------|
| username | a-input | 是 | 仅新增时显示 |
| password | a-input-password | 是 | 仅新增时显示 |
| real_name | a-input | 否 | 姓名 |
| email | a-input | 否 | 邮箱 |
| phone | a-input | 否 | 手机 |
| role_ids | a-select (multiple) | 是 | 角色多选: 管理员(1)/测试负责人(2)/测试工程师(3)/开发负责人(4)/查看者(5) |

- **交互**: 编辑时隐藏用户名和密码字段；删除前弹出确认气泡

---

### 4. ProjectList -- 项目管理

- **路由**: `/projects`
- **页面标题**: 项目管理
- **权限**: `project:list`
- **API 调用**:
  - `GET /api/projects` -- 项目列表 (通过 `getProjectList()`)
  - `POST /api/projects` -- 创建项目 (通过 `createProject()`)
  - `PUT /api/projects/:id` -- 编辑项目 (通过 `updateProject()`)
  - `DELETE /api/projects/:id` -- 删除项目 (通过 `deleteProject()`)
- **搜索栏**:
  - 关键词输入 (搜索项目名称)
  - 查询按钮 + 新增项目按钮
- **表格列**:

| 列名 | 字段 | 宽度 | 说明 |
|------|------|------|------|
| ID | id | 60px | |
| 项目名称 | name | 自适应 | |
| 禅道项目集 | zentao_project_name | 自适应 | 无则显示 "-" |
| 负责人 | owner.real_name | 100px | 无则显示 "-" |
| 状态 | status | 80px | 绿色=启用，红色=禁用 |
| 操作 | - | 140px | 编辑 / 删除(需确认) |

- **新增/编辑弹窗 (Modal, 640px宽)** 表单字段:

| 字段 | 组件 | 必填 | 说明 |
|------|------|------|------|
| name | a-input | 是 | 项目名称 |
| description | a-textarea (2行) | 否 | 项目描述 |
| func_doc_path | a-input | 否 | 功能文档路径，placeholder: docs/function.md |
| design_doc_path | a-input | 否 | 设计文档路径，placeholder: docs/design.md |
| db_doc_path | a-input | 否 | 数据库文档路径，placeholder: docs/database.md |
| test_doc_path | a-input | 否 | 测试文档路径，placeholder: docs/test.md |
| git_repo_url | a-input | 否 | Git 仓库地址 |
| git_branch | a-input | 否 | Git 分支，默认 main |
| zentao_project_id | a-input-number | 否 | 禅道项目集 ID |
| zentao_project_name | a-input | 否 | 禅道项目名称 |
| zentao_branch | a-input | 否 | 禅道分支 |

---

### 5. IssueList -- 问题单列表

- **路由**: `/issues`
- **页面标题**: 问题单列表
- **权限**: `issue:list`
- **API 调用**:
  - `GET /api/issues` -- 问题单列表 (通过 `getIssueList()`)
  - `POST /api/issues/sync` -- 同步禅道 (通过 `syncIssues()`)
  - `POST /api/test-tasks` -- 生成测试 (通过 `createTestTask()`)
- **搜索栏**:
  - 关键词输入 (搜索标题)
  - 禅道状态下拉 (active / resolved / closed)
  - 测试状态下拉 (使用 `TEST_STATUS_MAP` 枚举)
  - 查询按钮 + 同步禅道按钮
- **表格列** (横向滚动 1200px):

| 列名 | 字段 | 宽度 | 说明 |
|------|------|------|------|
| ID | zentao_id | 70px | 禅道编号 |
| 标题 | title | 自适应 | 超长省略 |
| 类型 | issue_type | 70px | |
| 严重程度 | severity | 90px | Tag: critical红/major橙/normal蓝/minor默认 |
| 禅道状态 | zentao_status | 100px | Tag 显示 |
| 测试状态 | test_status | 110px | 使用 TEST_STATUS_MAP 颜色映射 |
| 负责人 | assignee | 80px | |
| 提出人 | reporter | 80px | |
| 操作 | - | 100px | 固定右侧，「生成测试」按钮 |

- **生成测试按钮**: 仅当 `zentao_status === 'resolved'` 且 `test_status === 'pending'` 时可用
- **同步禅道弹窗 (Modal)** 表单字段:

| 字段 | 组件 | 必填 | 说明 |
|------|------|------|------|
| project_id | a-input-number | 是 | 项目 ID |
| full_sync | a-checkbox | 否 | 全量同步 |

---

### 6. AgentList -- Agent 管理

- **路由**: `/agents`
- **页面标题**: Agent 管理
- **权限**: `agent:list`
- **API 调用**:
  - `GET /api/agents` -- Agent 列表 (通过 `getAgentList()`)
  - `POST /api/agents` -- 创建 Agent (通过 `createAgent()`)
  - `PUT /api/agents/:id` -- 编辑 Agent (通过 `updateAgent()`)
  - `DELETE /api/agents/:id` -- 删除 Agent (通过 `deleteAgent()`)
- **搜索栏**:
  - 关键词输入 (搜索 Agent 名称)
  - 查询按钮 + 新增 Agent 按钮
- **表格列**:

| 列名 | 字段 | 宽度 | 说明 |
|------|------|------|------|
| ID | id | 60px | |
| 名称 | name | 自适应 | |
| 模型 | model_provider / model_name | 自适应 | 格式: "provider / name" |
| Skills | skills | 自适应 | Tag 列表，geekblue 色 |
| MCP Servers | mcp_servers | 自适应 | Tag 列表，purple 色 |
| 状态 | status | 80px | 绿色=启用，红色=停用 |
| 操作 | - | 140px | 编辑 / 删除(需确认) |

- **新增/编辑弹窗 (Modal, 600px宽)** 表单字段:

| 字段 | 组件 | 必填 | 说明 |
|------|------|------|------|
| name | a-input | 是 | 名称 |
| description | a-input | 否 | 描述 |
| model_provider | a-select | 是 | 模型提供商: Claude / OpenAI / 自定义 |
| model_name | a-input | 是 | 模型名称，placeholder: claude-sonnet-4-20250514 |
| api_key_ref | a-input | 否 | API Key 引用(配置名，非明文) |
| base_url | a-input | 否 | Base URL |
| max_tokens | a-input-number (min=1) | 否 | Max Tokens，默认 4096 |
| temperature | a-input-number (0~2, step=0.1) | 否 | Temperature，默认 0.3 |

---

### 7. ReviewList -- Review 审核列表

- **路由**: `/reviews`
- **页面标题**: Review 审核
- **权限**: `review:list`
- **API 调用**:
  - `GET /api/reviews` -- Review 列表 (通过 `getReviewList()`)
- **搜索栏**:
  - 审核状态下拉 (使用 `REVIEW_STATUS_MAP` 枚举)
  - 查询按钮
- **表格列**:

| 列名 | 字段 | 宽度 | 说明 |
|------|------|------|------|
| ID | id | 60px | |
| 标题 | title | 自适应 | 超长省略 |
| 关联问题单 | issue.title | 自适应 | 超长省略，无则显示 "-" |
| 状态 | status | 100px | 使用 REVIEW_STATUS_MAP 颜色映射 |
| 审核人 | reviewer.real_name | 100px | 无则显示「未分配」 |
| 创建时间 | created_at | 170px | |
| 操作 | - | 100px | pending 状态显示「去审核」，否则「查看」，跳转详情 |

---

### 8. ReviewDetail -- Review 详情/审核操作

- **路由**: `/reviews/:id`
- **页面标题**: Review 详情
- **权限**: `review:list`
- **API 调用**:
  - `GET /api/reviews/:id` -- Review 详情 (通过 `getReviewDetail()`)
  - `POST /api/reviews/:id/review` -- 执行审核 (通过 `doReview()`)
- **页面组成** (5个 Card 区块 + 审核操作区):

1. **关联问题单**: 卡片展示 `issue_title`
2. **测试用例**: Collapse 折叠面板，每个用例展示:
   - header: `[分类] 标题`
   - Descriptions 表格: 前置条件 / 测试步骤(pre 格式) / 预期结果 / 自测结果(Tag) / 来源(ai=蓝色, 其他=橙色)
3. **测试脚本**: 每个脚本展示文件路径、语言、代码内容(深色代码块 `#1e1e1e` 背景)
4. **测试文档**: 标题 + 预格式化内容 (灰色背景)
5. **审核记录**: Timeline 时间线，颜色: approve=绿 / reject=红 / 其他=蓝，显示审核人、动作描述、时间、评论
6. **审核操作** (仅 `status === 'pending'` 或 `'changes_requested'` 时显示):
   - 审核意见 (a-textarea, 3行)
   - 四个操作按钮: 通过(primary) / 驳回(danger) / 需修改 / 仅评论

---

### 9. TestTaskList -- 测试任务列表

- **路由**: `/test-tasks`
- **页面标题**: 测试任务
- **权限**: `test:list`
- **API 调用**:
  - `GET /api/test-tasks` -- 任务列表 (通过 `getTestTaskList()`)
  - `GET /api/test-tasks/:id/cases` -- 测试用例 (通过 `getTestCases()`)
  - `PUT /api/test-cases/:id` -- 修改用例 (通过 `updateTestCase()`)
- **搜索栏**:
  - 任务状态下拉 (待执行 / 执行中 / 已完成 / 失败)
  - 查询按钮
- **表格列**:

| 列名 | 字段 | 宽度 | 说明 |
|------|------|------|------|
| ID | id | 60px | |
| 关联问题单 | issue.title | 自适应 | 超长省略 |
| Skill | skill_name | 100px | |
| 状态 | status | 100px | Tag: pending默认/running蓝/completed绿/failed红 |
| 重试 | retry_count | 60px | |
| 创建时间 | created_at | 170px | |
| 操作 | - | 80px | 详情按钮 |

- **详情抽屉 (Drawer, 600px宽)**:
  - Descriptions 展示: 任务ID / Skill / 状态 / 开始时间 / 完成时间 / 重试次数 / 错误信息(红色)
  - **测试用例子表格**:

| 列名 | 字段 | 宽度 | 说明 |
|------|------|------|------|
| 标题 | title | 自适应 | 超长省略 |
| 分类 | category | 90px | |
| 自测 | self_test_result | 70px | Tag: pass=绿/其他=红 |
| 操作 | - | 70px | 修改按钮 |

- **修改测试用例弹窗 (Modal, 600px宽)** 表单字段:

| 字段 | 组件 | 必填 | 说明 |
|------|------|------|------|
| title | a-input | 否 | 标题 |
| precondition | a-textarea (2行) | 否 | 前置条件 |
| steps | a-textarea (4行) | 否 | 测试步骤 |
| expected | a-textarea (2行) | 否 | 预期结果 |
| change_note | a-input | 是 | 修改说明(必填，前端校验) |

---

### 10. ExecutionList -- 测试执行记录

- **路由**: `/executions`
- **页面标题**: 测试执行记录
- **权限**: `test:list`
- **API 调用**:
  - `GET /api/executions` -- 执行记录列表 (通过 `getExecutionList()`)
- **搜索栏**:
  - 执行状态下拉 (等待中 / 执行中 / 通过 / 失败 / 异常)
  - 查询按钮
- **表格列**:

| 列名 | 字段 | 宽度 | 说明 |
|------|------|------|------|
| ID | id | 60px | |
| 触发方式 | trigger_type | 90px | Tag 显示 |
| 分支 | branch | 120px | |
| 状态 | status | 90px | Tag: pending默认/running蓝/passed绿/failed红/error火山色 |
| 总用例 | total_cases | 80px | |
| 通过 | passed_cases | 60px | |
| 失败 | failed_cases | 60px | |
| 通过率 | pass_rate | 160px | a-progress 进度条，100%为 success 状态 |
| 耗时 | duration_sec | 80px | 格式: "Ns" |
| 执行时间 | created_at | 170px | |

---

### 11. ZentaoSettings -- 禅道管理

- **路由**: `/settings/zentao`
- **页面标题**: 禅道管理
- **权限**: `role: admin` (菜单级 `isAdmin` 控制)
- **API 调用**:
  - `GET /api/settings/zentao` -- 获取禅道配置 (通过 `getZentaoSettings()`)
  - `POST /api/settings/zentao` -- 保存禅道配置 (通过 `saveZentaoSettings()`)
  - `POST /api/settings/zentao/test` -- 测试连接 (通过 `testZentaoConnection()`)
- **页面组成**:

**连接配置卡片**:

| 字段 | 组件 | 必填 | 说明 |
|------|------|------|------|
| base_url | a-input | 是 | 禅道服务器地址，不含路径 |
| account | a-input | 是 | 登录账号 |
| password | a-input-password | 是 | 登录密码(未修改则保持原值) |
| token | a-input (disabled) | - | 自动获取，只读展示 |

- 「测试连接」按钮: 校验地址/账号/密码非空，调用 API 测试，成功时自动保存 Token
- 结果展示: `a-alert` 成功(success) 或 失败(error)

**同步策略卡片**:

| 字段 | 组件 | 必填 | 说明 |
|------|------|------|------|
| sync_enabled | a-switch | 否 | 自动同步开关 |
| sync_interval | a-input-number (5~1440) | 否 | 同步频率(分钟) |

- 底部「保存配置」按钮: 将 base_url / account / password(encrypted) / sync_interval / sync_enabled 批量提交

---

### 12. GitLabSettings -- GitLab 管理

- **路由**: `/settings/gitlab`
- **页面标题**: GitLab 管理
- **权限**: `role: admin` (菜单级 `isAdmin` 控制)
- **API 调用**:
  - `GET /api/settings/gitlab` -- 获取 GitLab 配置 (通过 `getGitLabSettings()`)
  - `POST /api/settings/gitlab` -- 保存 GitLab 配置 (通过 `saveGitLabSettings()`)
  - `POST /api/settings/gitlab/test` -- 测试连接 (通过 `testGitLabConnection()`)
- **页面组成**:

**连接配置卡片**:

| 字段 | 组件 | 必填 | 说明 |
|------|------|------|------|
| base_url | a-input | 是 | GitLab 服务器地址 |
| access_token | a-input-password | 是 | Personal Access Token (需 api + read_repository 权限) |
| api_version | a-select | 否 | API 版本: v4(推荐) / v3 |

- 「测试连接」按钮: 校验地址和 Token 非空，调用 API 测试
- 结果展示: `a-alert` 成功(success) 或 失败(error)
- 底部「保存配置」按钮: 将 base_url / access_token(encrypted) / api_version 批量提交

---

## 通用交互模式

### 列表页模式

所有列表页遵循统一模式:
1. **搜索栏**: `a-row` 布局，包含筛选条件 + 查询按钮 + 新增按钮(如有)
2. **数据表格**: `a-table`，size="middle"，分页 20 条/页，支持分页切换
3. **新增/编辑弹窗**: `a-modal`，`v-model:open` 控制，编辑时回填表单数据
4. **删除确认**: `a-popconfirm` 气泡确认

### 分页参数

- 默认 `page=1, page_size=20`
- 通过 `handleTableChange` 响应分页变化
- `pagination` 对象包含 `current / pageSize / total`

### 错误处理

- API 错误统一由 Axios 拦截器处理
- 成功操作通过 `message.success()` 提示
- 表单校验失败通过 `message.warning()` 提示

---

## 开发运行方式

```bash
cd frontend
npm install
npm run dev     # 启动开发服务 http://localhost:3000
npm run build   # 构建生产包到 dist/
```

开发模式下，`/api` 请求自动代理到 `http://localhost:8080`（后端服务）。
