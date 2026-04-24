<template>
  <div>
    <a-page-header :title="$t('issue.list.title')" />

    <a-row :gutter="16" style="margin-bottom: 16px">
      <a-col :span="5">
        <a-input v-model:value="query.keyword" :placeholder="$t('issue.list.search')" allowClear @pressEnter="fetchData" />
      </a-col>
      <a-col :span="4">
        <a-select v-model:value="query.zentao_status" :placeholder="$t('issue.list.zentaoStatus')" allowClear style="width: 100%">
          <a-select-option value="active">{{ $t('issue.list.status.active') }}</a-select-option>
          <a-select-option value="resolved">{{ $t('issue.list.status.resolved') }}</a-select-option>
          <a-select-option value="closed">{{ $t('issue.list.status.closed') }}</a-select-option>
        </a-select>
      </a-col>
      <a-col :span="4">
        <a-select v-model:value="query.test_status" :placeholder="$t('issue.list.testStatus')" allowClear style="width: 100%">
          <a-select-option v-for="(v, k) in testStatusMap" :key="k" :value="k">{{ v.label }}</a-select-option>
        </a-select>
      </a-col>
      <a-col :span="4">
        <a-select v-model:value="query.project_id" :placeholder="$t('issue.list.project')" allowClear style="width: 100%" @change="handleProjectChange">
          <a-select-option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</a-select-option>
        </a-select>
      </a-col>
      <a-col :span="4">
        <a-select v-model:value="query.branch" :placeholder="$t('issue.list.branch')" allowClear style="width: 100%">
          <a-select-option v-for="b in branches" :key="b" :value="b">{{ b }}</a-select-option>
        </a-select>
      </a-col>
      <a-col>
        <a-button type="primary" @click="fetchData">{{ $t('common.query') }}</a-button>
        <a-button style="margin-left: 8px" @click="handleSync">{{ $t('issue.list.sync') }}</a-button>
      </a-col>
    </a-row>

    <a-table :dataSource="list" :columns="columns" :loading="loading" :pagination="pagination"
             @change="handleTableChange" rowKey="id" size="middle" :scroll="{ x: 1200 }">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'zentao_status'">
          <a-tag>{{ formatZentaoStatus(record.zentao_status) }}</a-tag>
        </template>
        <template v-if="column.key === 'test_status'">
          <a-tag :color="testStatusMap[record.test_status]?.color || 'default'">
            {{ testStatusMap[record.test_status]?.label || record.test_status }}
          </a-tag>
        </template>
        <template v-if="column.key === 'issue_type'">
          <a-tag>{{ formatIssueType(record.issue_type) }}</a-tag>
        </template>
        <template v-if="column.key === 'severity'">
          <a-tag :color="severityColor(record.severity)">{{ formatSeverity(record.severity) }}</a-tag>
        </template>
        <template v-if="column.key === 'action'">
          <a-button type="link" size="small" @click="handleViewDetail(record)">
            {{ $t('issue.list.action.viewDetail') }}
          </a-button>
          <a-button type="link" size="small" @click="handleGenTest(record)"
                    :disabled="!canGenerateTest(record)">
            {{ $t('issue.list.action.generateTest') }}
          </a-button>
          <a-button
            v-if="record.test_status !== 'pending'"
            type="link" size="small"
            :loading="viewingTaskLoading === record.id"
            @click="handleViewTask(record)"
          >
            {{ $t('issue.list.action.viewTask') }}
          </a-button>
        </template>
      </template>
    </a-table>

    <!-- Sync modal -->
    <a-modal v-model:open="syncModal" :title="$t('issue.list.syncModal.title')" @ok="doSync" :confirmLoading="syncing">
      <a-form layout="vertical">
        <a-form-item :label="$t('issue.list.syncModal.selectProject')" required>
          <a-select v-model:value="syncForm.project_id" :placeholder="$t('issue.list.syncModal.selectProject')" :loading="projectListLoading" show-search :filter-option="filterOption">
            <a-select-option v-for="p in projectList" :key="p.id" :value="p.id">
              {{ p.name }}<span v-if="p.zentao_project_name" style="color: #999; margin-left: 8px">（{{ p.zentao_project_name }}）</span>
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item>
          <a-checkbox v-model:checked="syncForm.full_sync">{{ $t('issue.list.syncModal.fullSync') }}</a-checkbox>
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- Generate test dialog -->
    <a-modal
      v-model:open="genTestModal"
      :title="$t('issue.list.genTestModal.title')"
      @ok="doGenTest"
      :confirmLoading="genTestLoading"
      :ok-text="$t('issue.list.genTestModal.start')"
    >
      <div class="gen-test-issue-info" v-if="genTestIssue">
        <span class="gen-test-issue-info__title">{{ genTestIssue.title }}</span>
      </div>
      <a-form layout="vertical" style="margin-top: 16px">
        <a-form-item :label="$t('issue.list.genTestModal.selectAgent')">
          <a-select
            v-model:value="genTestForm.agent_id"
            :placeholder="$t('issue.list.genTestModal.agentPlaceholder')"
            :loading="agentListLoading"
            allowClear
            style="width: 100%"
          >
            <a-select-option v-for="a in agentList" :key="a.id" :value="a.id">
              {{ a.name }}
              <span v-if="a.description" style="color: #999; margin-left: 6px; font-size: 12px">{{ a.description }}</span>
            </a-select-option>
          </a-select>
          <div class="ant-form-item-extra">{{ $t('issue.list.genTestModal.agentHint') }}</div>
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- Issue detail dialog -->
    <a-modal
      v-model:open="detailModal"
      :title="$t('issue.list.detailModal.title')"
      :footer="null"
      :width="920"
      destroyOnClose
    >
      <a-spin :spinning="detailLoading">
        <template v-if="detailIssue">
          <div class="issue-detail-toolbar">
            <a-button
              type="primary"
              ghost
              @click="handleOpenZentao"
              :disabled="!detailIssue.zentao_url"
            >
              {{ $t('issue.list.detailModal.openInZentao') }}
            </a-button>
          </div>
          <a-descriptions :column="2" bordered size="small" style="margin-bottom: 16px">
            <a-descriptions-item :label="$t('issue.list.columns.id')">{{ detailIssue.zentao_id }}</a-descriptions-item>
            <a-descriptions-item :label="$t('issue.list.columns.title')">{{ detailIssue.title }}</a-descriptions-item>
            <a-descriptions-item :label="$t('issue.list.columns.zentaoStatus')">
              <a-tag>{{ formatZentaoStatus(detailIssue.zentao_status) }}</a-tag>
            </a-descriptions-item>
            <a-descriptions-item :label="$t('issue.list.columns.testStatus')">
              <a-tag :color="testStatusMap[detailIssue.test_status]?.color || 'default'">
                {{ testStatusMap[detailIssue.test_status]?.label || detailIssue.test_status }}
              </a-tag>
            </a-descriptions-item>
            <a-descriptions-item :label="$t('issue.list.detailModal.assignee')">
              <div>{{ detailIssue.assignee || '-' }}</div>
              <div class="issue-detail-email">{{ detailIssue.assignee_email || '-' }}</div>
            </a-descriptions-item>
            <a-descriptions-item :label="$t('issue.list.detailModal.reporter')">
              <div>{{ detailIssue.reporter || '-' }}</div>
              <div class="issue-detail-email">{{ detailIssue.reporter_email || '-' }}</div>
            </a-descriptions-item>
            <a-descriptions-item :label="$t('issue.list.detailModal.branch')">{{ detailIssue.branch || '-' }}</a-descriptions-item>
            <a-descriptions-item :label="$t('issue.list.detailModal.updatedAt')">{{ detailIssue.synced_at || '-' }}</a-descriptions-item>
          </a-descriptions>

          <div class="issue-detail-section-title">{{ $t('issue.list.detailModal.description') }}</div>
          <div
            v-if="detailIssue.description"
            class="issue-detail-content"
            v-html="detailIssue.description"
          />
          <a-empty v-else :description="$t('issue.list.detailModal.noDescription')" />
        </template>
      </a-spin>
    </a-modal>

  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useRouter } from 'vue-router'
import { getIssueById, getIssueList, syncIssues } from '@/api/issue'
import { getProjectList } from '@/api/project'
import { createTestTask, getTestTaskList } from '@/api/testTask'
import { getAgentList } from '@/api/agent'
import { getTestStatusMap } from '@/types'
import type { Issue } from '@/types'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const router = useRouter()

const list = ref<Issue[]>([])
const loading = ref(false)
const query = reactive({ keyword: '', zentao_status: undefined, test_status: undefined, project_id: undefined as number | undefined, branch: undefined as string | undefined })
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showTotal: (total: number) => `共 ${total} 条` })

const syncModal = ref(false)
const syncing = ref(false)
const syncForm = reactive({ project_id: undefined as number | undefined, full_sync: false })
const projectList = ref<{ id: number; name: string; zentao_project_name?: string }[]>([])
const projectListLoading = ref(false)
const projects = ref<{ id: number; name: string }[]>([])
const branches = ref<string[]>([])
const testStatusMap = computed(() => getTestStatusMap(t))

// Generate test dialog
const genTestModal = ref(false)
const genTestLoading = ref(false)
const genTestIssue = ref<Issue | null>(null)
const genTestForm = reactive({ agent_id: undefined as number | undefined })
const agentList = ref<{ id: number; name: string; description?: string; is_default?: boolean }[]>([])
const agentListLoading = ref(false)

// 本地缓存 issueId -> taskId，避免重新查询
const issueTaskCache = reactive<Record<number, number>>({})
const viewingTaskLoading = ref<number | null>(null)

const detailModal = ref(false)
const detailLoading = ref(false)
const detailIssue = ref<Issue | null>(null)

const columns = computed(() => [
  { title: t('issue.list.columns.id'), dataIndex: 'zentao_id', key: 'zentao_id', width: 70 },
  { title: t('issue.list.columns.title'), dataIndex: 'title', key: 'title', ellipsis: true },
  { title: t('issue.list.columns.type'), dataIndex: 'issue_type', key: 'issue_type', width: 70 },
  { title: t('issue.list.columns.severity'), key: 'severity', width: 90 },
  { title: t('issue.list.columns.zentaoStatus'), key: 'zentao_status', width: 100 },
  { title: t('issue.list.columns.testStatus'), key: 'test_status', width: 110 },
  { title: t('issue.list.columns.assignee'), dataIndex: 'assignee', key: 'assignee', width: 80 },
  { title: t('issue.list.columns.reporter'), dataIndex: 'reporter', key: 'reporter', width: 80 },
  { title: t('issue.list.columns.action'), key: 'action', width: 280, fixed: 'right' as const, customHeaderCell: () => ({ style: 'text-align: center' }) },
])

onMounted(() => {
  fetchData()
  fetchProjects()
})

async function fetchData() {
  loading.value = true
  try {
    const res = await getIssueList({ ...query, page: pagination.current, page_size: pagination.pageSize })
    const data = res.data.data
    list.value = data.list || []
    pagination.total = data.total

    const allBranches = new Set<string>()
    list.value.forEach(issue => {
      if (issue.branch) {
        allBranches.add(issue.branch)
      }
    })
    branches.value = Array.from(allBranches)
  } finally {
    loading.value = false
  }
}

function handleTableChange(pag: any) {
  pagination.current = pag.current
  pagination.pageSize = pag.pageSize
  fetchData()
}

function handleSync() {
  syncModal.value = true
  fetchProjectList()
}

async function fetchProjectList() {
  if (projectList.value.length > 0) return
  projectListLoading.value = true
  try {
    const res = await getProjectList({ page: 1, page_size: 100 })
    projectList.value = res.data.data?.list || []
  } finally {
    projectListLoading.value = false
  }
}

async function fetchProjects() {
  if (projects.value.length > 0) return
  try {
    const res = await getProjectList({ page: 1, page_size: 100 })
    projects.value = res.data.data?.list?.map((project: any) => ({ id: project.id, name: project.name })) || []
  } catch (error) {
    console.error(t('issue.list.messages.projectLoadFailed'), error)
  }
}

function filterOption(input: string, option: any) {
  const label = option.children?.[0]?.children?.[0]?.children || ''
  return String(label).toLowerCase().includes(input.toLowerCase())
}

async function handleProjectChange(value: number | undefined) {
  query.project_id = value
  query.branch = undefined
  pagination.current = 1
  await fetchData()
}

async function doSync() {
  if (syncForm.project_id == null) {
    message.warning(t('issue.list.messages.selectProjectWarning'))
    return
  }
  syncing.value = true
  try {
    await syncIssues({ project_id: syncForm.project_id, full_sync: syncForm.full_sync })
    message.success(t('issue.list.messages.syncTriggered'))
    syncModal.value = false
  } finally {
    syncing.value = false
  }
}

async function handleGenTest(record: Issue) {
  genTestIssue.value = record
  genTestForm.agent_id = undefined
  genTestModal.value = true
  fetchAgentList()
}

async function fetchAgentList() {
  if (agentList.value.length > 0) {
    applyDefaultAgentSelection()
    return
  }
  agentListLoading.value = true
  try {
    const res = await getAgentList({ status: 1, page: 1, page_size: 100 })
    agentList.value = res.data.data?.list || []
    applyDefaultAgentSelection()
  } finally {
    agentListLoading.value = false
  }
}

function applyDefaultAgentSelection() {
  const defaultAgent = agentList.value.find((agent) => !!agent.is_default)
  if (defaultAgent) {
    genTestForm.agent_id = defaultAgent.id
  }
}

async function doGenTest() {
  if (!genTestIssue.value) return
  genTestLoading.value = true
  try {
    const payload: { issue_id: number; agent_id?: number } = { issue_id: genTestIssue.value.id }
    if (genTestForm.agent_id) payload.agent_id = genTestForm.agent_id
    const res = await createTestTask(payload)
    const task = res.data.data
    genTestModal.value = false
    message.success(t('issue.list.genTestModal.taskCreated'))
    if (task?.id && genTestIssue.value) {
      issueTaskCache[genTestIssue.value.id] = task.id
    }
    if (task?.id) {
      router.push({ name: 'TaskRunDetail', params: { id: String(task.id) } })
    }
    fetchData()
  } finally {
    genTestLoading.value = false
  }
}

async function handleViewTask(record: Issue) {
  // 优先用本地缓存，没有则查最新任务
  if (issueTaskCache[record.id]) {
    router.push({ name: 'TaskRunDetail', params: { id: String(issueTaskCache[record.id]) } })
    return
  }
  viewingTaskLoading.value = record.id
  try {
    const res = await getTestTaskList({ issue_id: record.id, page: 1, page_size: 1 })
    const tasks = res.data.data?.list || []
    if (tasks.length === 0) {
      message.warning(t('issue.list.messages.noTaskFound'))
      return
    }
    const taskId = tasks[0].id
    issueTaskCache[record.id] = taskId
    router.push({ name: 'TaskRunDetail', params: { id: String(taskId) } })
  } finally {
    viewingTaskLoading.value = null
  }
}

async function handleViewDetail(record: Issue) {
  detailModal.value = true
  detailLoading.value = true
  detailIssue.value = null
  try {
    const res = await getIssueById(record.id)
    detailIssue.value = res.data.data || null
  } catch (error) {
    message.error(t('issue.list.messages.loadDetailFailed'))
    detailModal.value = false
  } finally {
    detailLoading.value = false
  }
}

function handleOpenZentao() {
  const url = detailIssue.value?.zentao_url
  if (!url) {
    message.warning(t('issue.list.messages.zentaoUrlUnavailable'))
    return
  }
  window.open(url, '_blank', 'noopener,noreferrer')
}

function severityColor(s: string) {
  const map: Record<string, string> = { critical: 'red', major: 'orange', normal: 'blue', minor: 'default' }
  return map[s] || 'default'
}

function formatZentaoStatus(status: string) {
  const statusMap: Record<string, string> = {
    'active': t('issue.list.status.active'),
    'resolved': t('issue.list.status.resolved'),
    'closed': t('issue.list.status.closed')
  }
  return statusMap[status] || status
}

function formatIssueType(type: string) {
  const typeMap: Record<string, string> = {
    'bug': t('issue.list.type.bug'),
    'task': t('issue.list.type.task'),
    'story': t('issue.list.type.story'),
    'feedback': t('issue.list.type.feedback')
  }
  return typeMap[type] || type
}

function formatSeverity(severity: string) {
  const severityMap: Record<string, string> = {
    'critical': t('issue.list.severity.critical'),
    'major': t('issue.list.severity.major'),
    'normal': t('issue.list.severity.normal'),
    'minor': t('issue.list.severity.minor')
  }
  return severityMap[severity] || severity
}

function canGenerateTest(issue: Issue) {
  // 审核驳回、异常状态允许重新生成
  if (issue.test_status === 'review_rejected' || issue.test_status === 'error') {
    return issue.zentao_status === 'resolved'
  }

  if (issue.test_status === 'pending') {
    return issue.zentao_status === 'resolved' || issue.zentao_status === 'active'
  }

  return false
}
</script>

<style scoped>
.gen-test-issue-info {
  background: #f5f5f5;
  border-radius: 6px;
  padding: 10px 14px;
}

.gen-test-issue-info__title {
  font-size: 14px;
  color: #333;
}

.issue-detail-section-title {
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 600;
}

.issue-detail-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}

.issue-detail-email {
  color: #8c8c8c;
  font-size: 12px;
}

.issue-detail-content {
  max-height: 460px;
  overflow: auto;
  padding: 12px;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  background: #fff;
}

.issue-detail-content :deep(img) {
  max-width: 100%;
  height: auto;
}

.issue-detail-content :deep(pre),
.issue-detail-content :deep(code) {
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
