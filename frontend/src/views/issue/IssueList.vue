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
        <a-select v-model:value="query.project_set_id" :placeholder="$t('issue.list.projectSet')" allowClear style="width: 100%" @change="handleProjectSetChange">
          <a-select-option v-for="p in projectSets" :key="p.id" :value="p.id">{{ p.name }}</a-select-option>
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
          <a-button type="link" size="small" @click="handleGenTest(record)"
                    :disabled="record.zentao_status !== 'resolved' || record.test_status !== 'pending'">
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

  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useRouter } from 'vue-router'
import { getIssueList, syncIssues } from '@/api/issue'
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
const query = reactive({ keyword: '', zentao_status: undefined, test_status: undefined, project_set_id: undefined, branch: undefined })
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })

const syncModal = ref(false)
const syncing = ref(false)
const syncForm = reactive({ project_id: undefined as number | undefined, full_sync: false })
const projectList = ref<{ id: number; name: string; zentao_project_name?: string }[]>([])
const projectListLoading = ref(false)
const projectSets = ref<{ id: number; name: string }[]>([])
const branches = ref<string[]>([])
const testStatusMap = computed(() => getTestStatusMap(t))

// Generate test dialog
const genTestModal = ref(false)
const genTestLoading = ref(false)
const genTestIssue = ref<Issue | null>(null)
const genTestForm = reactive({ agent_id: undefined as number | undefined })
const agentList = ref<{ id: number; name: string; description?: string }[]>([])
const agentListLoading = ref(false)

// 本地缓存 issueId -> taskId，避免重新查询
const issueTaskCache = reactive<Record<number, number>>({})
const viewingTaskLoading = ref<number | null>(null)

const columns = computed(() => [
  { title: t('issue.list.columns.id'), dataIndex: 'zentao_id', key: 'zentao_id', width: 70 },
  { title: t('issue.list.columns.title'), dataIndex: 'title', key: 'title', ellipsis: true },
  { title: t('issue.list.columns.type'), dataIndex: 'issue_type', key: 'issue_type', width: 70 },
  { title: t('issue.list.columns.severity'), key: 'severity', width: 90 },
  { title: t('issue.list.columns.zentaoStatus'), key: 'zentao_status', width: 100 },
  { title: t('issue.list.columns.testStatus'), key: 'test_status', width: 110 },
  { title: t('issue.list.columns.assignee'), dataIndex: 'assignee', key: 'assignee', width: 80 },
  { title: t('issue.list.columns.reporter'), dataIndex: 'reporter', key: 'reporter', width: 80 },
  { title: t('issue.list.columns.action'), key: 'action', width: 160, fixed: 'right' as const },
])

onMounted(() => {
  fetchData()
  fetchProjectSets()
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

async function fetchProjectSets() {
  if (projectSets.value.length > 0) return
  try {
    const res = await getProjectList({ page: 1, page_size: 100 })
    projectSets.value = res.data.data?.list?.map((p: any) => ({ id: p.id, name: p.name })) || []
  } catch (error) {
    console.error(t('issue.list.messages.projectSetLoadFailed'), error)
  }
}

function filterOption(input: string, option: any) {
  const label = option.children?.[0]?.children?.[0]?.children || ''
  return String(label).toLowerCase().includes(input.toLowerCase())
}

async function handleProjectSetChange(value: number | undefined) {
  if (!value) {
    branches.value = []
    return
  }
  try {
    const res = await getProjectList({ project_set_id: value, page: 1, page_size: 100 })
    const projects = res.data.data?.list || []
    const allBranches = new Set<string>()
    projects.forEach((project: any) => {
      if (project.branch) allBranches.add(project.branch)
    })
    branches.value = Array.from(allBranches)
  } catch (error) {
    console.error(t('issue.list.messages.branchLoadFailed'), error)
    branches.value = []
  }
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
  if (agentList.value.length > 0) return
  agentListLoading.value = true
  try {
    const res = await getAgentList({ status: 1, page: 1, page_size: 100 })
    agentList.value = res.data.data?.list || []
  } finally {
    agentListLoading.value = false
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
</style>
