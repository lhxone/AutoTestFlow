# AutoTestFlow Mock 依赖清单

> 本文档记录当前系统中所有 Mock 实现位置，用于追踪外部依赖的接入状态。
> Mock 仅在开发/测试阶段使用，上线前必须替换为真实实现。
>
> 最后更新: 2026-04-10

---

## 总览

| # | 模块 | Mock 方法 | 触发条件 | 外部依赖 |
|---|------|-----------|----------|----------|
| 1 | 禅道同步 | `syncWithMockData()` | `zentao.base_url` 或 `zentao.api_token` 为空 | 禅道 API |
| 2 | AI 生成测试 | `mockAIOutput()` | `ai.api_key` 为空 | Claude / OpenAI API |
| 3 | 文档读取 | `readDocFile()` | 始终为 Mock（函数未实现真实读取） | Git 工作区 / Git API |
| 4 | 邮件通知 | `SendTestReport()` 内联 | `mail.host` 为空 | SMTP 邮件服务器 |
| 5 | CI 执行 | `mockExecution()` | 项目 `git_repo_url` 为空 | GitLab CI Pipeline |
| 6 | GitLab Pipeline 触发 | `triggerGitLabPipeline()` 占位 | 代码占位，未完成真实调用 | GitLab Trigger API |
| 7 | Git 推送 | `PushReviewedContent()` | 无 Mock，但依赖环境完备 | Git CLI + 远程仓库 |

---

## 1. 禅道同步 Mock

| 项 | 说明 |
|----|------|
| **文件** | `backend/internal/service/zentao_service.go` |
| **Mock 方法** | `syncWithMockData()` (第 142 行) |
| **入口方法** | `SyncProjectIssues()` (第 34 行) |
| **触发条件** | `config.yaml` 中 `zentao.base_url` 或 `zentao.api_token` 为空时走 Mock 分支 |
| **Mock 行为** | 每次同步固定插入 3 条 Mock 问题单，详情如下: |
| | - ID 1001: `[Mock] 用户登录后token未正确刷新`，severity=major，status=resolved |
| | - ID 1002: `[Mock] 项目列表分页参数异常时返回500`，severity=normal，status=resolved |
| | - ID 1003: `[Mock] 导出报告按钮在移动端不显示`，severity=minor，status=active |
| | 所有 Mock 数据 reporter/assignee 使用固定假名 (zhangsan/lisi/wangwu/zhaoliu/sunqi) |
| **真实实现** | `syncFromZentao()` (第 55 行)，已完整实现，调用 `GET /api.php/v1/projects/{id}/bugs` |
| **替换方式** | 配置真实禅道地址和 API Token 后自动走真实接口，无需改代码 |
| **涉及 API** | `GET {base_url}/api.php/v1/projects/{id}/bugs?limit=500` |
| **认证方式** | HTTP Header `Token: {api_token}` |
| **需要确认** | 禅道版本（开源版/企业版）、API 认证方式（Token/Session）、字段映射是否匹配 |
| **标记注释** | `// [MOCK] 此方法为Mock实现，真实环境需替换为 syncFromZentao` (第 141 行) |

### 配置项

```yaml
zentao:
  base_url: ""      # 禅道服务器地址，为空时触发 Mock
  api_token: ""     # 禅道 API Token，为空时触发 Mock
  sync_interval: "" # 同步周期
```

---

## 2. AI 生成测试 Mock

| 项 | 说明 |
|----|------|
| **文件** | `backend/internal/service/gentest_service.go` |
| **Mock 方法** | `mockAIOutput()` (第 479 行) |
| **入口方法** | `callAI()` (第 264 行) -> 判断 `cfg.APIKey` |
| **触发条件** | `config.yaml` 中 `ai.api_key` 为空 |
| **Mock 行为** | 返回固定结构的 `GenTestOutput`，包含: |
| | - 4 个测试用例: 主流程(main_flow)、异常流程(exception)、边界条件(boundary)、回归验证(regression) |
| | - 用例标题基于 issue title 动态拼接，但步骤/预期结果为模板占位文本 |
| | - 所有用例 self_test_result 均为 `pass` |
| | - 1 个 Python 测试脚本模板 (`tests/test_issue_mock.py`)，内含 4 个 `assert True` 的空测试方法 |
| | - 1 篇 Markdown 测试文档模板 |
| | - 1 条 summary 文本 |
| **真实实现** | `callClaudeAPI()` (第 353 行) 或 `callOpenAIAPI()` (第 414 行)，根据 `ai.provider` 分发 |
| **替换方式** | 配置 `ai.api_key` 后，根据 `ai.provider` 自动走真实 API |
| **涉及 API** | Claude: `POST {base_url}/v1/messages`; OpenAI: `POST {base_url}/v1/chat/completions` |
| **认证方式** | Claude: Header `x-api-key` + `anthropic-version: 2023-06-01`; OpenAI: Header `Authorization: Bearer {key}` |
| **HTTP 超时** | 120 秒 (`httpClient` 初始化于第 37 行) |
| **需要确认** | API Key 额度、模型选择、Token 上限、响应 JSON 解析稳定性 |
| **标记注释** | `// [MOCK] AI未配置时使用Mock输出` (第 268 行), `// [MOCK] 此方法为Mock实现` (第 478 行) |

### AI 真实调用注意事项

- Prompt 在 `buildPrompt()` 方法（第 288 行）中定义，要求 AI 严格返回 JSON 格式
- 响应文本通过 `pkg.ExtractJSON()` 提取 JSON（支持 AI 返回被 ` ```json ``` ` 包裹的情况）
- 当前超时设为 120 秒，复杂项目可能需要调大
- 建议后续增加 JSON Schema 校验，确保 `GenTestOutput` 结构完整

### 配置项

```yaml
ai:
  provider: ""     # "claude" 或 "openai"
  api_key: ""      # API Key，为空时触发 Mock
  base_url: ""     # API 基础地址(OpenAI 可选，默认 https://api.openai.com)
  model: ""        # 模型名称
  max_tokens: 0    # 最大 Token 数
  temperature: 0.0 # 温度参数(仅 OpenAI 使用)
```

---

## 3. 文档读取 Mock

| 项 | 说明 |
|----|------|
| **文件** | `backend/internal/service/gentest_service.go` |
| **Mock 方法** | `readDocFile()` (第 579 行) |
| **调用位置** | `buildInput()` (第 232 行) 中读取项目的 4 种文档路径 |
| **触发条件** | **始终触发** -- 函数体为硬编码占位，未实现真实文件读取 |
| **Mock 行为** | 返回固定字符串 `[待从 {path} 读取文档内容]` |
| **影响范围** | 项目的 `func_doc_path`、`design_doc_path`、`db_doc_path`、`test_doc_path` 四个文档字段 |
| **替换方式** | 需要编码实现，两种方案: |
| | 方案1: 从本地 Git 工作区读取 -- `os.ReadFile(filepath.Join(gitWorkDir, path))` |
| | 方案2: 通过 Git API 远程读取文件内容 |
| **文档截断** | `buildInput()` 中每个文档内容最大 10000 字符 (`pkg.TruncateString`) |
| **标记注释** | `// [MOCK] 当前返回占位内容，后续需要从 Git 工作区或远程读取` (第 578 行) |
| **TODO** | `// TODO: 实现真实文件读取` (第 580 行) |

> **注意**: 这是唯一一个没有配置开关、始终处于 Mock 状态的实现。即使 AI 配置了真实 API Key，传给 AI 的文档上下文仍然是占位文本，会严重影响 AI 生成质量。**建议优先级提升为 P0**。

---

## 4. 邮件通知 Mock

| 项 | 说明 |
|----|------|
| **文件** | `backend/internal/service/notification_service.go` |
| **Mock 位置** | `SendTestReport()` 方法内联 (第 29-45 行)，无独立 Mock 方法 |
| **触发条件** | `config.yaml` 中 `mail.host` 为空 |
| **Mock 行为** | 跳过实际邮件发送，为每个收件人在 `notification_log` 表创建一条记录: |
| | - channel = `email` |
| | - status = `skipped` |
| | - subject = `[AutoTestFlow] 测试报告 - {title}` |
| | - content = report.Summary |
| **真实实现** | `sendEmail()` (第 52 行)，使用 `gomail` 库发送 HTML 格式邮件 |
| **替换方式** | 配置 SMTP 服务器信息后自动启用真实发送 |
| **邮件格式** | HTML，包含用例统计、通过率、是否人工介入、报告链接 |
| **TLS** | 当 `use_ssl: true` 时启用（当前使用 `InsecureSkipVerify: true`） |
| **需要确认** | SMTP 服务器地址、端口、SSL/TLS 配置、发件人邮箱授权码 |
| **标记注释** | `// [MOCK] 邮件未配置，仅记录日志` (第 32 行) |

### 配置项

```yaml
mail:
  host: ""       # SMTP 服务器地址，为空时触发 Mock
  port: 465      # SMTP 端口
  username: ""   # SMTP 用户名
  password: ""   # SMTP 密码/授权码
  from: ""       # 发件人地址
  use_ssl: false # 是否使用 SSL
```

---

## 5. CI 执行 Mock

| 项 | 说明 |
|----|------|
| **文件** | `backend/internal/service/ci_service.go` |
| **Mock 方法** | `mockExecution()` (第 96 行) |
| **入口方法** | `triggerGitLabPipeline()` (第 72 行) -> 判断 `project.GitRepoURL` |
| **触发条件** | 项目的 `git_repo_url` 字段为空 |
| **Mock 行为** | `time.Sleep(3秒)` 模拟执行延迟后，更新执行记录: |
| | - status = `passed` |
| | - total_cases = 4, passed = 3, failed = 1 |
| | - pass_rate = 75.0% |
| | - duration = 12 秒 |
| **真实流程** | 调用 GitLab Trigger API `POST /api/v4/projects/:id/trigger/pipeline`，Pipeline 执行完毕后通过 `POST /api/ci/callback` 回调 |
| **回调接口** | `backend/internal/handler/ci_callback_handler.go` 已实现 |
| **需要确认** | GitLab 项目 ID、CI Trigger Token、回调地址 (`ATF_CALLBACK_URL` 环境变量) |
| **标记注释** | `// [MOCK] 当 GitLab CI 未配置时，模拟执行` (第 71 行), `// [MOCK]` (第 95 行) |

---

## 6. GitLab Pipeline 触发（代码占位，未完成）

| 项 | 说明 |
|----|------|
| **文件** | `backend/internal/service/ci_service.go` |
| **方法** | `triggerGitLabPipeline()` (第 72 行) |
| **当前状态** | 非 Mock 分支（`git_repo_url` 非空时）进入此段，但只更新状态为 `running`，**未实际调用 GitLab API** |
| **已有代码** | `triggerGitLabAPI()` (第 113 行) 已实现完整的 HTTP 调用逻辑，但未被 `triggerGitLabPipeline()` 调用 |
| **差距** | 需要从项目配置中获取 GitLab project ID 和 trigger token，然后调用 `triggerGitLabAPI()` |
| **标记注释** | `// TODO: 从项目配置中获取 GitLab project ID 和 trigger token` (第 81 行) |
| **涉及 API** | `POST {gitlab_url}/api/v4/projects/{id}/trigger/pipeline` |

---

## 7. Git 推送（非 Mock，环境依赖）

| 项 | 说明 |
|----|------|
| **文件** | `backend/internal/service/gitops_service.go` |
| **方法** | `PushReviewedContent()` (第 34 行) |
| **当前状态** | 代码逻辑完整，无 Mock 分支。但依赖本机 `git` 命令行和远程仓库配置 |
| **前置条件** | 1. 服务器安装了 `git` CLI |
| | 2. SSH Key 或 HTTPS Token 已配置，能访问远程仓库 |
| | 3. `config.yaml` 中 `git.work_dir` 目录可写 |
| | 4. 项目已配置 `git_repo_url` 和 `git_branch` |
| **工作流程** | clone/pull -> 创建 `autotest/review-{id}` 分支 -> 写入脚本和文档文件 -> add + commit + push |
| **commit 格式** | `[AutoTestFlow] Review #{id} 通过 - {title}`，author 来自 `git.commit_author` / `git.commit_email` |
| **注意事项** | 首次运行需确保 `git clone` 正常；Windows 环境下路径分隔符需注意 |

### 配置项

```yaml
git:
  work_dir: ""       # Git 本地工作目录
  commit_author: ""  # 提交作者名
  commit_email: ""   # 提交作者邮箱
```

---

## TODO 与 [MOCK] 标记汇总

以下是代码库中所有 `TODO` 和 `[MOCK]` 注释的完整列表:

### [MOCK] 标记 (7 处)

| 文件 | 行号 | 内容 |
|------|------|------|
| `zentao_service.go` | 141 | `// [MOCK] 此方法为Mock实现，真实环境需替换为 syncFromZentao` |
| `gentest_service.go` | 268 | `// [MOCK] AI未配置时使用Mock输出` |
| `gentest_service.go` | 478 | `// [MOCK] 此方法为Mock实现` |
| `gentest_service.go` | 578 | `// [MOCK] 当前返回占位内容，后续需要从 Git 工作区或远程读取` |
| `notification_service.go` | 32 | `// [MOCK] 邮件未配置，仅记录日志` |
| `ci_service.go` | 71 | `// [MOCK] 当 GitLab CI 未配置时，模拟执行` |
| `ci_service.go` | 95 | `// [MOCK]` |

### TODO 标记 (3 处，不含 Mock 生成的 pytest 模板)

| 文件 | 行号 | 内容 |
|------|------|------|
| `gentest_service.go` | 241 | `// TODO: 当前使用占位内容，后续通过 Git clone + os.ReadFile 读取真实文件` |
| `gentest_service.go` | 580 | `// TODO: 实现真实文件读取` |
| `ci_service.go` | 81 | `// TODO: 从项目配置中获取 GitLab project ID 和 trigger token` |

> 注: `gentest_service.go` 第 532/537/542/547 行的 `# TODO` 是 Mock 输出的 pytest 模板内容（Python 注释），不属于 Go 代码 TODO。

---

## 替换优先级

| 优先级 | 模块 | 原因 |
|--------|------|------|
| **P0** | AI 生成测试 | 核心功能，Mock 数据无实际测试价值 |
| **P0** | 禅道同步 | 数据源，决定系统能否真正工作 |
| **P0** | 文档读取 (`readDocFile`) | 始终为 Mock，即使 AI 已配置也只传占位文本，严重影响 AI 生成质量 |
| **P1** | Git 推送 | 联调需要真实仓库 |
| **P1** | CI 执行 + Pipeline 触发 | 闭环必需；`triggerGitLabAPI()` 已写好但未接入 |
| **P2** | 邮件通知 | 非阻塞，可后期接入 |

---

## 快速替换检查清单

```yaml
# config.yaml 中需要填入的真实配置

# ---- P0: 禅道同步 ----
zentao:
  base_url: "http://your-zentao-server"      # <- 替换，为空则触发 Mock
  api_token: "your-api-token"                 # <- 替换，为空则触发 Mock
  sync_interval: "30m"                        # 同步间隔

# ---- P0: AI 生成 ----
ai:
  provider: "claude"                          # "claude" 或 "openai"
  api_key: "sk-xxx"                           # <- 替换，为空则触发 Mock
  base_url: "https://api.anthropic.com"       # Claude 基础地址(OpenAI 默认 https://api.openai.com)
  model: "claude-sonnet-4-20250514"             # <- 按需调整
  max_tokens: 4096                            # 最大输出 Token 数
  temperature: 0.7                            # 温度(仅 OpenAI 使用)

# ---- P0: 文档读取 ----
# 无配置项，需要编码实现 readDocFile() 函数
# 依赖 git.work_dir 指向的本地仓库目录

# ---- P1: Git 推送 ----
git:
  work_dir: "/data/auto-test-flow/repos"      # <- 替换为服务器实际路径
  commit_author: "AutoTestFlow"               # Git 提交作者
  commit_email: "autotest@your-company.com"   # Git 提交邮箱

# ---- P1: CI 执行 ----
# 需要在项目中配置 git_repo_url（项目级别，非全局配置）
# 需要编码: 在 triggerGitLabPipeline() 中调用 triggerGitLabAPI()
# 环境变量: ATF_CALLBACK_URL=http://your-server/api/ci/callback

# ---- P2: 邮件通知 ----
mail:
  host: "smtp.your-company.com"               # <- 替换，为空则触发 Mock
  port: 465                                   # SMTP 端口
  username: "autotest@your-company.com"       # <- 替换
  password: "your-smtp-password"              # <- 替换
  from: "autotest@your-company.com"           # <- 替换
  use_ssl: true                               # 是否启用 SSL
```

### 需要编码修改的项（非纯配置可解决）

| 项 | 文件 | 工作量 | 说明 |
|----|------|--------|------|
| `readDocFile()` 真实实现 | `gentest_service.go:579` | 小 | 用 `os.ReadFile` 从 `git.work_dir` 读取，代码注释已给出方案 |
| `triggerGitLabPipeline()` 接入 | `ci_service.go:72` | 小 | 需要获取 GitLab project ID + trigger token，调用已有的 `triggerGitLabAPI()` |

---

## setting_service.go 说明

`backend/internal/service/setting_service.go` **不包含任何 Mock 实现**。该文件提供:

- `GetSettings()` / `SaveSettings()` -- 系统设置的 CRUD
- `TestZentaoConnection()` -- 测试禅道连接并自动获取/保存 Token
- `TestGitLabConnection()` -- 测试 GitLab 连接（`GET /api/v4/user`）
- `GetZentaoToken()` -- 获取禅道 Token（支持过期自动刷新）

这些是真实实现的管理功能，配置页面可通过它们在线测试外部系统连通性。
