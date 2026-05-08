# AutoTestFlow API 接口文档

> Base URL: `http://localhost:8080/api`
> 认证方式: Bearer Token (JWT)，在请求头中添加 `Authorization: Bearer <access_token>`
> 三方集成接口认证方式: Header Token，在请求头中添加 `X-ATF-Token: <integration_api_token>`

---

## 1. 认证模块 `/auth`

### 1.1 POST /auth/login — 登录

- **权限**: 公开
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | 用户名，2~64 字符 |
| password | string | 是 | 密码，6~128 字符 |

- **响应 data**:

| 字段 | 类型 | 说明 |
|------|------|------|
| access_token | string | 访问令牌 |
| refresh_token | string | 刷新令牌 |
| expires_in | int | 过期时间(秒) |
| user | UserInfo | 用户信息对象 |

**UserInfo 结构**:

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint64 | 用户 ID |
| username | string | 用户名 |
| real_name | string | 真实姓名 |
| email | string | 邮箱 |
| phone | string | 手机号 |
| avatar | string | 头像 URL |
| status | int8 | 状态 |
| roles | string[] | 角色编码列表 |
| permissions | string[] | 权限编码列表 |

### 1.2 POST /auth/refresh — 刷新令牌

- **权限**: 公开
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| refresh_token | string | 是 | 刷新令牌 |

- **响应 data**: 同登录响应

### 1.3 GET /auth/me — 获取当前用户信息

- **权限**: 登录即可
- **请求体**: 无
- **响应 data**: UserInfo 对象

### 1.4 PUT /auth/password — 修改密码

- **权限**: 登录即可
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| old_password | string | 是 | 旧密码，最少 6 字符 |
| new_password | string | 是 | 新密码，6~128 字符 |

- **响应 data**: `{}`

### 1.5 POST /auth/logout — 登出

- **权限**: 登录即可
- **请求体**: 无
- **响应 data**: `{}`

---

## 2. 用户管理 `/users`

### 2.1 GET /users — 用户列表

- **权限**: `user:list`
- **Query 参数**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，最小 1 |
| page_size | int | 否 | 每页条数，1~100 |
| keyword | string | 否 | 搜索关键词 |
| status | int8 | 否 | 状态筛选 |
| role_code | string | 否 | 角色编码筛选 |

- **响应 data**: 分页列表

### 2.2 POST /users — 创建用户

- **权限**: `user:create`
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | 用户名，2~64 字符 |
| password | string | 是 | 密码，6~128 字符 |
| real_name | string | 否 | 真实姓名，最长 64 字符 |
| email | string | 否 | 邮箱，最长 128 字符 |
| phone | string | 否 | 手机号，最长 20 字符 |
| role_ids | uint64[] | 是 | 角色 ID 列表，至少 1 个 |

- **响应 data**: 创建的用户对象

### 2.3 GET /users/:id — 用户详情

- **权限**: `user:list`
- **路径参数**: `id` — 用户 ID
- **响应 data**: 用户对象

### 2.4 PUT /users/:id — 编辑用户

- **权限**: `user:update`
- **路径参数**: `id` — 用户 ID
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| real_name | string | 否 | 真实姓名，最长 64 字符 |
| email | string | 否 | 邮箱，最长 128 字符 |
| phone | string | 否 | 手机号，最长 20 字符 |
| status | int8 | 否 | 状态，0 或 1 |
| role_ids | uint64[] | 否 | 角色 ID 列表 |

- **响应 data**: `{}`

### 2.5 DELETE /users/:id — 删除用户

- **权限**: `user:delete`
- **路径参数**: `id` — 用户 ID
- **响应 data**: `{}`

---

## 3. 项目管理 `/projects`

### 3.1 GET /projects — 项目列表

- **权限**: `project:list`
- **Query 参数**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，最小 1 |
| page_size | int | 否 | 每页条数，1~100 |
| keyword | string | 否 | 搜索关键词 |
| status | int8 | 否 | 状态筛选 |

- **响应 data**: 分页列表

### 3.2 POST /projects — 创建项目

- **权限**: `project:create`
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 项目名称，最长 128 字符 |
| description | string | 否 | 项目描述 |
| func_doc_path | string | 否 | 功能文档路径 |
| design_doc_path | string | 否 | 设计文档路径 |
| db_doc_path | string | 否 | 数据库文档路径 |
| test_doc_path | string | 否 | 测试文档路径 |
| extra_files_path | string | 否 | 额外文件路径 |
| git_repo_url | string | 否 | Git 仓库 URL |
| git_branch | string | 否 | Git 分支 |
| zentao_project_id | int | 否 | 禅道项目 ID |
| zentao_project_name | string | 否 | 禅道项目名称 |
| zentao_branch | string | 否 | 禅道分支 |
| owner_id | uint64 | 否 | 负责人 ID |

- **响应 data**: 创建的项目对象

### 3.3 GET /projects/:id — 项目详情

- **权限**: `project:list`
- **路径参数**: `id` — 项目 ID
- **响应 data**: 项目对象

### 3.4 PUT /projects/:id — 编辑项目

- **权限**: `project:update`
- **路径参数**: `id` — 项目 ID
- **请求体**: 同创建字段(均可选)，额外支持:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| status | int8 | 否 | 项目状态 |

- **响应 data**: `{}`

### 3.5 DELETE /projects/:id — 删除项目

- **权限**: `project:delete`
- **路径参数**: `id` — 项目 ID
- **响应 data**: `{}`

---

## 4. 问题单管理 `/issues`

### 4.1 GET /issues — 问题单列表

- **权限**: `issue:list`
- **Query 参数**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，最小 1 |
| page_size | int | 否 | 每页条数，1~100 |
| project_id | uint64 | 否 | 项目 ID 筛选 |
| zentao_status | string | 否 | 禅道状态筛选 |
| test_status | string | 否 | 测试状态筛选 |
| keyword | string | 否 | 搜索关键词 |
| assignee | string | 否 | 指派人筛选 |

- **响应 data**: 分页列表

### 4.2 GET /issues/:id — 问题单详情

- **权限**: `issue:list`
- **路径参数**: `id` — 问题单 ID
- **响应 data**: 问题单对象

### 4.3 POST /issues/sync — 手动触发同步

- **权限**: `issue:sync`
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| project_id | uint64 | 是 | 项目 ID |
| full_sync | bool | 否 | true=全量同步，false=增量同步 |

- **响应 data**: `{}`

### 4.4 PUT /issues/:id/test-status — 更新测试状态

- **权限**: `issue:update`
- **路径参数**: `id` — 问题单 ID
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| test_status | string | 是 | 目标测试状态(见枚举表) |
| remark | string | 否 | 备注 |

- **响应 data**: `{}`

### 4.5 GET /issues/:id/interventions — 人工介入记录

- **权限**: `test:list`
- **路径参数**: `id` — 问题单 ID
- **响应 data**: 介入记录列表

---

## 5. Agent 管理 `/agents`

### 5.1 GET /agents — Agent 列表

- **权限**: `agent:list`
- **Query 参数**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，最小 1 |
| page_size | int | 否 | 每页条数，1~100 |
| keyword | string | 否 | 搜索关键词 |
| status | int8 | 否 | 状态筛选 |

- **响应 data**: 分页列表

### 5.2 POST /agents — 创建 Agent

- **权限**: `agent:manage`
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | Agent 名称，最长 64 字符 |
| description | string | 否 | 描述 |
| model_provider | string | 是 | 模型提供商，可选值: `claude`, `openai`, `custom` |
| model_name | string | 是 | 模型名称 |
| api_key_ref | string | 否 | API Key 引用 |
| base_url | string | 否 | 模型服务地址 |
| max_tokens | int | 否 | 最大 Token 数 |
| temperature | float64 | 否 | 温度参数 |
| config_json | object | 否 | 自定义配置 JSON |
| skill_ids | uint64[] | 否 | 关联的 Skill ID 列表 |
| mcp_server_ids | uint64[] | 否 | 关联的 MCP Server ID 列表 |

- **响应 data**: 创建的 Agent 对象

### 5.3 GET /agents/:id — Agent 详情

- **权限**: `agent:list`
- **路径参数**: `id` — Agent ID
- **响应 data**: Agent 对象

### 5.4 PUT /agents/:id — 编辑 Agent

- **权限**: `agent:manage`
- **路径参数**: `id` — Agent ID
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 否 | Agent 名称，最长 64 字符 |
| description | string | 否 | 描述 |
| model_provider | string | 否 | 模型提供商，可选值: `claude`, `openai`, `custom` |
| model_name | string | 否 | 模型名称 |
| api_key_ref | string | 否 | API Key 引用 |
| base_url | string | 否 | 模型服务地址 |
| max_tokens | int | 否 | 最大 Token 数 |
| temperature | float64 | 否 | 温度参数 |
| status | int8 | 否 | 状态 |
| config_json | object | 否 | 自定义配置 JSON |
| skill_ids | uint64[] | 否 | 关联的 Skill ID 列表 |
| mcp_server_ids | uint64[] | 否 | 关联的 MCP Server ID 列表 |

- **响应 data**: `{}`

### 5.5 POST /agents/test — 测试 Agent 连接

- **权限**: `agent:manage`
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model_provider | string | 是 | 模型提供商，可选值: `claude`, `openai`, `zhipu`, `custom` |
| model_name | string | 是 | 模型名称 |
| api_key_ref | string | 否 | API Key 引用 |
| test_api_key | string | 否 | 临时测试用 API Key，不会保存 |
| base_url | string | 否 | Base URL，不传则使用提供商默认地址 |
| max_tokens | int | 否 | 最大 Token 数 |
| temperature | float64 | 否 | 温度参数 |

- **响应 data**:

| 字段 | 类型 | 说明 |
|------|------|------|
| success | bool | 是否连接成功 |
| message | string | 结果消息 |
| provider | string | 实际测试的提供商 |
| model | string | 实际测试的模型 |
| base_url | string | 实际测试的 Base URL |
| latency_ms | int64 | 接口耗时（毫秒） |
| sample_output | string | 模型返回的示例内容 |

### 5.6 DELETE /agents/:id — 删除 Agent

- **权限**: `agent:manage`
- **路径参数**: `id` — Agent ID
- **响应 data**: `{}`

---

## 6. Skill 管理 `/skills`

### 6.1 GET /skills — Skill 列表

- **权限**: `agent:list`
- **响应 data**: Skill 列表

### 6.2 POST /skills — 创建 Skill

- **权限**: `agent:manage`
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | Skill 名称，最长 64 字符 |
| description | string | 否 | 描述 |
| skill_type | string | 否 | 类型，可选值: `builtin`, `custom` |
| prompt_template | string | 否 | 提示词模板 |
| input_schema | object | 否 | 输入 Schema (JSON) |
| output_schema | object | 否 | 输出 Schema (JSON) |
| config_json | object | 否 | 自定义配置 JSON |

- **响应 data**: 创建的 Skill 对象

---

## 7. MCP Server 管理 `/mcp-servers`

### 7.1 GET /mcp-servers — MCP Server 列表

- **权限**: `agent:list`
- **响应 data**: MCP Server 列表

### 7.2 POST /mcp-servers — 创建 MCP Server

- **权限**: `agent:manage`
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 名称，最长 64 字符 |
| description | string | 否 | 描述 |
| server_type | string | 是 | 服务类型，可选值: `stdio`, `sse`, `streamable_http` |
| command | string | 否 | 启动命令 (stdio 模式) |
| args | object | 否 | 启动参数 (JSON) |
| url | string | 否 | 服务地址 (sse/streamable_http 模式) |
| env_vars | object | 否 | 环境变量 (JSON) |

- **响应 data**: 创建的 MCP Server 对象

---

## 8. Review 审核 `/reviews`

### 8.1 GET /reviews — Review 列表

- **权限**: `review:list`
- **Query 参数**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，最小 1 |
| page_size | int | 否 | 每页条数，1~100 |
| project_id | uint64 | 否 | 项目 ID 筛选 |
| status | string | 否 | 审核状态筛选(见枚举表) |
| reviewer_id | uint64 | 否 | 审核人 ID 筛选 |

- **响应 data**: 分页列表

### 8.2 GET /reviews/:id — Review 详情

- **权限**: `review:list`
- **路径参数**: `id` — Review ID
- **响应 data**:

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint64 | Review ID |
| title | string | 标题 |
| status | string | 审核状态 |
| issue_title | string | 关联问题单标题 |
| test_cases | TestCaseVO[] | 测试用例列表 |
| test_scripts | TestScriptVO[] | 测试脚本列表 |
| test_docs | TestDocVO[] | 测试文档列表 |
| records | ReviewRecordVO[] | 审核记录列表 |

**TestCaseVO 结构**:

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint64 | 用例 ID |
| title | string | 用例标题 |
| category | string | 分类 |
| precondition | string | 前置条件 |
| steps | string | 步骤 |
| expected | string | 预期结果 |
| self_test_result | string | 自测结果 |
| source | string | 来源 |

**TestScriptVO 结构**:

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint64 | 脚本 ID |
| file_path | string | 文件路径 |
| file_content | string | 文件内容 |
| language | string | 编程语言 |
| source | string | 来源 |

**TestDocVO 结构**:

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint64 | 文档 ID |
| title | string | 文档标题 |
| content | string | 文档内容 |
| doc_type | string | 文档类型 |
| source | string | 来源 |

**ReviewRecordVO 结构**:

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint64 | 记录 ID |
| reviewer_name | string | 审核人姓名 |
| action | string | 操作类型 |
| comment | string | 评论内容 |
| created_at | string | 创建时间 |

### 8.3 POST /reviews/:id/review — 执行审核操作

- **权限**: `review:approve`
- **路径参数**: `id` — Review ID
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| action | string | 是 | 操作类型，可选值: `approve`, `reject`, `request_changes`, `comment` |
| comment | string | 否 | 审核评论 |

- **响应 data**: `{}`

---

## 9. 测试任务 `/test-tasks`

### 9.1 GET /test-tasks — 任务列表

- **权限**: `test:list`
- **Query 参数**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，最小 1 |
| page_size | int | 否 | 每页条数，1~100 |
| project_id | uint64 | 否 | 项目 ID 筛选 |
| issue_id | uint64 | 否 | 问题单 ID 筛选 |
| status | string | 否 | 任务状态筛选 |

- **响应 data**: 分页列表

### 9.2 GET /test-tasks/:id — 任务详情

- **权限**: `test:list`
- **路径参数**: `id` — 任务 ID
- **响应 data**: 任务对象

### 9.3 POST /test-tasks — 创建任务(触发 AI 生成)

- **权限**: `test:trigger`
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| issue_id | uint64 | 是 | 问题单 ID |
| agent_id | uint64 | 否 | 指定 Agent ID，不传则使用默认 |

- **响应 data**: 创建的任务对象

### 9.4 GET /test-tasks/:id/cases — 获取任务的测试用例

- **权限**: `test:list`
- **路径参数**: `id` — 任务 ID
- **响应 data**: 测试用例列表

---

## 10. 人工修改接口

### 10.1 PUT /test-cases/:id — 修改测试用例

- **权限**: `test:intervene`
- **路径参数**: `id` — 测试用例 ID
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| title | string | 否 | 用例标题 |
| precondition | string | 否 | 前置条件 |
| steps | string | 否 | 步骤 |
| expected | string | 否 | 预期结果 |
| change_note | string | 是 | 修改说明 |

- **响应 data**: `{}`

### 10.2 PUT /test-scripts/:id — 修改测试脚本

- **权限**: `test:intervene`
- **路径参数**: `id` — 测试脚本 ID
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_content | string | 是 | 脚本文件内容 |
| change_note | string | 是 | 修改说明 |

- **响应 data**: `{}`

---

## 11. 测试执行 `/executions`

### 11.1 GET /executions — 执行记录列表

- **权限**: `test:list`
- **Query 参数**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，最小 1 |
| page_size | int | 否 | 每页条数，1~100 |
| project_id | uint64 | 否 | 项目 ID 筛选 |
| status | string | 否 | 执行状态筛选 |

- **响应 data**: 分页列表

### 11.2 POST /executions/trigger — 手动触发测试执行

- **权限**: `test:trigger`
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| project_id | uint64 | 是 | 项目 ID |
| branch | string | 否 | Git 分支，不传使用项目默认分支 |

- **响应 data**: 执行记录对象

---

## 12. CI 回调 `/ci`

### 12.1 POST /ci/callback — CI 测试结果回调

- **权限**: 公开(由 CI 系统调用，通过 CI Token 验证)
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| execution_id | uint64 | 否 | 执行记录 ID |
| summary | object | 是 | 测试汇总 |
| summary.total | int | 是 | 总用例数 |
| summary.passed | int | 是 | 通过数 |
| summary.failed | int | 是 | 失败数 |
| summary.skipped | int | 是 | 跳过数 |
| duration | float64 | 否 | 执行耗时(秒) |
| tests | array | 否 | 测试明细列表 |
| tests[].nodeid | string | 是 | pytest node ID |
| tests[].outcome | string | 是 | 结果: `passed`, `failed`, `skipped` |
| tests[].message | string | 否 | 失败消息 |

- **响应 data**: `{}`

> 此接口由 GitLab CI Pipeline 在测试完成后通过 `curl` 调用，将 pytest-json-report 的结果回传给平台。

---

## 13. 系统设置 `/settings`

> 所有系统设置接口仅 **admin** 角色可访问。

### 13.1 GET /settings/zentao — 获取禅道配置

- **权限**: `admin` 角色
- **请求体**: 无
- **响应 data**: `SettingVO[]`

| 字段 | 类型 | 说明 |
|------|------|------|
| key | string | 配置键名 |
| value | string | 配置值(加密字段显示为掩码) |
| encrypted | int8 | 是否加密(1=加密) |
| description | string | 配置说明 |
| updated_at | string | 更新时间 |

### 13.2 PUT /settings/zentao — 保存禅道配置

- **权限**: `admin` 角色
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| settings | SettingItem[] | 是 | 设置列表，至少 1 项 |

**SettingItem 结构**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| key | string | 是 | 配置键名 |
| value | string | 否 | 配置值 |
| encrypted | int8 | 否 | 是否加密 |
| description | string | 否 | 配置说明 |

- **响应 data**: `{}`

### 13.3 POST /settings/zentao/test — 测试禅道连接

- **权限**: `admin` 角色
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| base_url | string | 是 | 禅道服务地址 |
| account | string | 是 | 禅道账号 |
| password | string | 是 | 禅道密码 |

- **响应 data**:

| 字段 | 类型 | 说明 |
|------|------|------|
| success | bool | 是否连接成功 |
| token | string | 获取到的 Token(成功时返回) |
| message | string | 结果消息 |

### 13.4 GET /settings/gitlab — 获取 GitLab 配置

- **权限**: `admin` 角色
- **请求体**: 无
- **响应 data**: `SettingVO[]`（结构同 13.1）

### 13.5 PUT /settings/gitlab — 保存 GitLab 配置

- **权限**: `admin` 角色
- **请求体**: 同 13.2

### 13.6 POST /settings/gitlab/test — 测试 GitLab 连接

- **权限**: `admin` 角色
- **请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| base_url | string | 是 | GitLab 服务地址 |
| access_token | string | 是 | GitLab Access Token |

- **响应 data**:

| 字段 | 类型 | 说明 |
|------|------|------|
| success | bool | 是否连接成功 |
| message | string | 结果消息 |
| username | string | GitLab 用户名(成功时返回) |

---

## 14. 三方集成接口 `/integration`

### 14.1 GET /integration/project-metrics — 项目维度指标

- **权限**: 三方 Header Token
- **独立文档**: [项目维度指标 API](api-project-metrics.md)

### 14.2 POST /integration/devflow-submit — DevFlow 提交通知

- **权限**: 三方 Header Token
- **说明**: 请求与响应历史会记录到 `api_exchange_log`，`api_name = devflow_submit`

### 14.3 POST /integration/cicd-deploy — CI/CD 部署通知

- **权限**: 三方 Header Token
- **说明**: 请求与响应历史会记录到 `api_exchange_log`，`api_name = cicd_deploy`

---

## 15. 健康检查

### 15.1 GET /health — 服务健康检查

- **权限**: 公开
- **请求体**: 无
- **响应**: `{ "status": "ok" }`

> 注意: 此接口路径为 `/health`，不在 `/api` 前缀下。

---

## 枚举值参考

### 测试状态 `test_status`

| 值 | 含义 |
|----|------|
| pending | 未开始 |
| generating | 生成中 |
| review_pending | 待审核 |
| review_approved | 审核通过 |
| review_rejected | 审核驳回 |
| testing | 测试中 |
| passed | 通过 |
| partial_passed | 部分通过 |
| all_failed | 全部失败 |
| intervention_needed | 待人工介入 |
| intervention_in_progress | 人工修复中 |
| error | 异常 |

### Review 审核状态 `review_status`

| 值 | 含义 |
|----|------|
| pending | 待审核 |
| approved | 审核通过 |
| rejected | 审核驳回 |
| changes_requested | 需修改 |

### Review 操作 `review_action`

| 值 | 含义 |
|----|------|
| approve | 审核通过 |
| reject | 审核驳回 |
| request_changes | 需修改 |
| comment | 仅评论 |

### 模型提供商 `model_provider`

| 值 | 含义 |
|----|------|
| claude | Anthropic Claude |
| openai | OpenAI |
| custom | 自定义模型 |

### Skill 类型 `skill_type`

| 值 | 含义 |
|----|------|
| builtin | 内置 |
| custom | 自定义 |

### MCP Server 类型 `server_type`

| 值 | 含义 |
|----|------|
| stdio | 标准输入输出 |
| sse | Server-Sent Events |
| streamable_http | 可流式 HTTP |

---

## 统一响应格式

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

### 分页响应

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

### 业务错误码

| 错误码 | 含义 |
|--------|------|
| 0 | 成功 |
| 10001 | 参数错误 |
| 10002 | 未授权 |
| 10003 | 无权限 |
| 10004 | 资源不存在 |
| 10005 | 资源已存在 |
| 10006 | 内部错误 |
| 20001 | 禅道接口错误 |
| 20002 | AI 服务错误 |
| 20003 | Git 操作错误 |
| 20004 | 邮件发送错误 |
| 30001 | Review 任务不存在 |
| 30002 | Review 任务非待审核状态 |
