// ====== 通用 ======
export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

export interface PageResult<T = any> {
  list: T[]
  total: number
  page: number
  page_size: number
}

export interface PageQuery {
  page: number
  page_size: number
}

export interface DashboardStats {
  projects: number | null
  pending_reviews: number | null
  intervention_needed: number | null
  pass_rate: number | null
  issue_sync_projects?: DashboardProjectSyncStatus[]
}

export interface DashboardProjectSyncStatus {
  project_id: number
  project_name: string
  status: string
  status_label: string
  added_count: number
  updated_count: number
  deleted_count: number
  started_at?: string
  completed_at?: string
  error_message?: string
}

// ====== 用户 ======
export interface UserInfo {
  id: number
  username: string
  real_name: string
  email: string
  phone: string
  avatar: string
  status: number
  roles: string[]
  permissions: string[]
}

export interface User {
  id: number
  username: string
  real_name: string
  email: string
  phone: string
  status: number
  last_login_at: string | null
  created_at: string
  roles: Role[]
}

export interface LoginLog {
  id: number
  user_id: number | null
  username: string
  module: string
  action: string
  target_type: string
  target_id: number | null
  detail: {
    result?: string
    reason?: string
  } | null
  ip: string
  user_agent: string
  created_at: string
}

export interface Role {
  id: number
  code: string
  name: string
  description: string
}

// ====== 登录 ======
export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  access_token: string
  refresh_token: string
  expires_in: number
  user: UserInfo
}

// ====== 项目 ======
export interface Project {
  id: number
  name: string
  description: string
  func_doc_path: string
  design_doc_path: string
  db_doc_path: string
  test_doc_path: string
  extra_files_path: string
  git_repo_url: string
  git_branch: string
  git_pull_interval: number
  git_last_pull_at: string | null
  zentao_project_id: number | null
  zentao_project_name: string
  zentao_branch: string
  status: number
  owner_id: number | null
  owner: User | null
  created_at: string
}

export interface ProjectIssueSyncLog {
  id: number
  project_id: number
  status: string
  full_sync: boolean
  added_count: number
  updated_count: number
  deleted_count: number
  error_message: string
  started_at: string
  completed_at: string | null
}

export interface IssueSyncFieldChange {
  field: string
  field_label: string
  old_value: string
  new_value: string
}

export interface ProjectIssueSyncDetail {
  id: number
  sync_log_id: number
  project_id: number
  issue_id: number | null
  zentao_id: number
  issue_title: string
  action: string
  changed_fields: IssueSyncFieldChange[]
  created_at: string
}

// ====== 问题单 ======
export interface Issue {
  id: number
  zentao_id: number
  project_id: number
  title: string
  description: string
  issue_type: string
  zentao_status: string
  test_status: string
  severity: string
  priority: number
  reporter: string
  reporter_email: string
  assignee: string
  assignee_email: string
  branch: string
  resolved_at: string | null
  synced_at: string | null
  created_at: string
  zentao_url?: string
}

// ====== Agent ======
export interface Agent {
  id: number
  name: string
  description: string
  is_default: boolean
  model_provider: string
  model_name: string
  api_key_ref: string
  base_url: string
  max_tokens: number
  temperature: number
  status: number
  config_json: any
  workflows: Workflow[]
  mcp_servers: MCPServer[]
  created_at: string
}

export interface Workflow {
  id: number
  name: string
  description: string
  workflow_type: string
  prompt_template: string
  input_schema?: any
  output_schema?: any
  config_json?: any
  status: number
  created_at?: string
  updated_at?: string
}

export interface MCPServer {
  id: number
  name: string
  description: string
  server_type: string
  command: string
  args?: any
  url: string
  env_vars?: any
  status: number
  created_at?: string
  updated_at?: string
}

// ====== Review ======
export interface ReviewTask {
  id: number
  test_task_id: number
  issue_id: number
  project_id: number
  title: string
  status: string
  reviewer_id: number | null
  review_note: string
  reviewed_at: string | null
  created_at: string
  issue: Issue | null
  reviewer: User | null
}

export interface ReviewDetail {
  id: number
  title: string
  status: string
  issue_title: string
  test_cases: TestCaseVO[]
  test_scripts: TestScriptVO[]
  test_docs: TestDocVO[]
  records: ReviewRecordVO[]
}

export interface TestCaseVO {
  id: number
  title: string
  category: string
  precondition: string
  steps: string
  expected: string
  self_test_result: string
  source: string
}

export interface TestCase {
  id: number
  task_id: number
  issue_id: number
  project_id: number
  title: string
  category: string
  precondition: string
  steps: string
  expected: string
  actual: string
  self_test_result: string
  priority: number
  current_version: number
  source: string
  created_at: string
  updated_at: string
}

export interface TestScriptVO {
  id: number
  file_path: string
  file_content: string
  language: string
  source: string
}

export interface TestDocVO {
  id: number
  title: string
  file_path?: string
  content: string
  doc_type: string
  source: string
}

export interface SelfTestFrameworkReport {
  passed?: boolean
  summary?: string
  checks?: string[]
  report_path?: string
  [key: string]: any
}

export interface SelfTestReport {
  passed?: boolean
  summary?: string
  checks?: string[]
  playwright?: SelfTestFrameworkReport
  midscene?: SelfTestFrameworkReport
  [key: string]: any
}

export interface TestTaskAIOutput {
  self_test?: SelfTestReport
  [key: string]: any
}

export interface ReviewRecordVO {
  id: number
  reviewer_name: string
  action: string
  comment: string
  created_at: string
}

// ====== 测试任务 ======
export interface TestTask {
  id: number
  issue_id: number
  project_id: number
  agent_id: number | null
  workflow_name: string
  status: string
  error_message: string
  retry_count: number
  started_at: string | null
  completed_at: string | null
  created_at: string
  ai_output?: TestTaskAIOutput | null
  issue: Issue | null
}

export interface TestTaskEvent {
  id: number
  task_id: number
  type: string
  stage?: string
  status?: string
  message?: string
  timestamp: string
  data?: Record<string, any>
}

// ====== 测试执行 ======
export interface TestExecution {
  id: number
  project_id: number
  trigger_type: string
  branch: string
  status: string
  total_cases: number
  passed_cases: number
  failed_cases: number
  pass_rate: number
  duration_sec: number
  started_at: string | null
  completed_at: string | null
  created_at: string
}

// ====== 人工介入 ======
export interface ManualIntervention {
  id: number
  issue_id: number
  operator_id: number
  intervention_type: string
  description: string
  status: string
  created_at: string
  operator: User | null
}

type Translator = (key: string) => string

// ====== 测试状态 ======
export function getTestStatusMap(t: Translator): Record<string, { label: string; color: string }> {
  return {
    pending: { label: t('status.test.pending'), color: 'default' },
    generating: { label: t('status.test.generating'), color: 'processing' },
    review_pending: { label: t('status.test.review_pending'), color: 'warning' },
    review_approved: { label: t('status.test.review_approved'), color: 'cyan' },
    review_rejected: { label: t('status.test.review_rejected'), color: 'error' },
    testing: { label: t('status.test.testing'), color: 'processing' },
    passed: { label: t('status.test.passed'), color: 'success' },
    partial_passed: { label: t('status.test.partial_passed'), color: 'warning' },
    all_failed: { label: t('status.test.all_failed'), color: 'error' },
    intervention_needed: { label: t('status.test.intervention_needed'), color: 'volcano' },
    intervention_in_progress: { label: t('status.test.intervention_in_progress'), color: 'orange' },
    error: { label: t('status.test.error'), color: 'error' },
  }
}

export function getReviewStatusMap(t: Translator): Record<string, { label: string; color: string }> {
  return {
    pending: { label: t('status.review.pending'), color: 'warning' },
    approved: { label: t('status.review.approved'), color: 'success' },
    rejected: { label: t('status.review.rejected'), color: 'error' },
    changes_requested: { label: t('status.review.changes_requested'), color: 'orange' },
  }
}

export function translateTestCaseCategory(t: Translator, category: string) {
  return category ? t(`testCase.category.${category}`) : category
}

export function translateSelfTestResult(t: Translator, result: string) {
  return result ? t(`testCase.selfTest.${result}`) : result
}

export function translateCaseSource(t: Translator, source: string) {
  return source ? t(`testCase.source.${source}`) : source
}

export function translateTaskStatus(t: Translator, status: string) {
  return status ? t(`status.task.${status}`) : status
}

export function translateExecutionStatus(t: Translator, status: string) {
  return status ? t(`status.execution.${status}`) : status
}

export function translateTriggerType(t: Translator, triggerType: string) {
  return triggerType ? t(`status.trigger.${triggerType}`) : triggerType
}

export function translateWorkflowType(t: Translator, workflowType: string) {
  return workflowType ? t(`status.workflowType.${workflowType}`) : workflowType
}
