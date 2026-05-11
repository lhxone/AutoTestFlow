<template>
  <div class="issue-page">
    <div class="issue-page-heading">
      <a-page-header :title="$t('issue.list.title')" />
      <a-radio-group v-model:value="viewMode" button-style="solid" class="view-switch">
        <a-radio-button value="table">{{ $t('issue.list.view.table') }}</a-radio-button>
        <a-radio-button value="board">{{ $t('issue.list.view.board') }}</a-radio-button>
      </a-radio-group>
    </div>

    <a-row :gutter="16" class="issue-filter-bar">
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

    <a-table v-if="viewMode === 'table'" :dataSource="list" :columns="columns" :loading="loading" :pagination="pagination"
             @change="handleTableChange" rowKey="id" size="middle" :scroll="{ x: 1520 }">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'project'">
          {{ record.project?.name || '-' }}
        </template>
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
        <template v-if="column.key === 'resolved_at'">
          {{ formatDateTime(record.resolved_at) }}
        </template>
        <template v-if="column.key === 'created_at'">
          {{ formatDateTime(record.created_at) }}
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

    <a-spin v-else :spinning="loading">
      <section class="bug-board">
        <div class="board-hero">
          <div>
            <div class="board-kicker">{{ $t('issue.list.board.kicker') }}</div>
            <h1>{{ $t('issue.list.board.title') }}</h1>
            <p>{{ $t('issue.list.board.subtitle') }}</p>
          </div>
          <div class="board-stats">
            <div class="board-stat-card board-stat-card--total">
              <span>{{ $t('issue.list.board.total') }}</span>
              <strong>{{ pagination.total }}</strong>
              <small>{{ $t('issue.list.board.currentPage', { count: list.length }) }}</small>
            </div>
            <div class="board-stat-card board-stat-card--running">
              <span>{{ $t('issue.list.board.inProgress') }}</span>
              <strong>{{ activeIssueCount }}</strong>
              <small>{{ $t('issue.list.board.needFollow') }}</small>
            </div>
            <div class="board-stat-card board-stat-card--time">
              <span>{{ $t('issue.list.board.snapshot') }}</span>
              <strong>{{ snapshotTime }}</strong>
              <small>{{ $t('issue.list.board.synced') }}</small>
            </div>
          </div>
        </div>

        <div class="board-columns">
          <article
            v-for="column in boardColumns"
            :key="column.key"
            class="board-column"
            :class="[`board-column--${column.tone}`, { 'is-collapsed': collapsedColumns[column.key] }]"
          >
            <header class="board-column-header">
              <div>
                <h2>{{ column.title }}</h2>
                <div class="board-column-count">{{ column.items.length }}</div>
              </div>
              <button class="board-collapse-btn" type="button" @click="toggleColumn(column.key)">
                {{ collapsedColumns[column.key] ? '›' : '‹' }}
              </button>
            </header>

            <template v-if="!collapsedColumns[column.key]">
              <div class="board-column-badges">
                <a-tag v-if="column.items.length === 0">{{ $t('issue.list.board.empty') }}</a-tag>
                <a-tag v-for="summary in column.summaries" v-else :key="summary.label" :color="summary.color">
                  {{ summary.label }} {{ summary.count }}
                </a-tag>
              </div>

              <a-input
                v-model:value="boardSearch[column.key]"
                :placeholder="$t('issue.list.board.search', { stage: column.title })"
                allowClear
                class="board-column-search"
              />

              <div v-if="column.filteredItems.length > 0" class="board-card-list">
                <button
                  v-for="issue in column.filteredItems"
                  :key="issue.id"
                  type="button"
                  class="bug-card"
                  @click="handleViewDetail(issue)"
                >
                  <div class="bug-card-head">
                    <a-tag :color="severityColor(issue.severity)" class="bug-card-severity">
                      {{ formatSeverity(issue.severity) }}
                    </a-tag>
                    <span class="bug-card-id">#{{ issue.zentao_id }}</span>
                  </div>
                  <h3>{{ issue.title }}</h3>
                  <p>{{ summarizeDescription(issue.description) }}</p>
                  <dl>
                    <div>
                      <dt>{{ $t('issue.list.columns.assignee') }}</dt>
                      <dd>{{ issue.assignee || '-' }}</dd>
                    </div>
                    <div>
                      <dt>{{ $t('issue.list.columns.reporter') }}</dt>
                      <dd>{{ issue.reporter || '-' }}</dd>
                    </div>
                  </dl>
                  <div class="bug-card-tags">
                    <a-tag>{{ formatZentaoStatus(issue.zentao_status) }}</a-tag>
                    <a-tag :color="testStatusMap[issue.test_status]?.color || 'default'">
                      {{ testStatusMap[issue.test_status]?.label || issue.test_status }}
                    </a-tag>
                  </div>
                  <footer>
                    <span>{{ formatDateTime(issue.created_at) }}</span>
                    <div class="bug-card-actions" @click.stop>
                      <a-button size="small" type="link" @click="handleGenTest(issue)" :disabled="!canGenerateTest(issue)">
                        {{ $t('issue.list.action.generateTest') }}
                      </a-button>
                      <a-button
                        v-if="issue.test_status !== 'pending'"
                        size="small"
                        type="link"
                        :loading="viewingTaskLoading === issue.id"
                        @click="handleViewTask(issue)"
                      >
                        {{ $t('issue.list.action.viewTask') }}
                      </a-button>
                    </div>
                  </footer>
                </button>
              </div>

              <div v-else class="board-empty">
                <strong>{{ $t('issue.list.board.noMatch') }}</strong>
                <span>{{ column.emptyText }}</span>
              </div>
            </template>
          </article>
        </div>
      </section>
    </a-spin>

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
            <a-descriptions-item :label="$t('issue.list.columns.resolvedAt')">{{ formatDateTime(detailIssue.resolved_at) }}</a-descriptions-item>
            <a-descriptions-item :label="$t('issue.list.columns.createdAt')">{{ formatDateTime(detailIssue.created_at) }}</a-descriptions-item>
            <a-descriptions-item :label="$t('issue.list.detailModal.branch')">{{ detailIssue.branch || '-' }}</a-descriptions-item>
            <a-descriptions-item :label="$t('issue.list.detailModal.updatedAt')">{{ formatDateTime(detailIssue.synced_at) }}</a-descriptions-item>
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
import dayjs from 'dayjs'
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
const viewMode = ref<'table' | 'board'>('table')
const query = reactive({ keyword: '', zentao_status: undefined, test_status: undefined, project_id: undefined as number | undefined, branch: undefined as string | undefined })
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showTotal: (total: number) => `共 ${total} 条` })
const boardSearch = reactive<Record<string, string>>({})
const collapsedColumns = reactive<Record<string, boolean>>({})

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
  { title: t('issue.list.project'), key: 'project', width: 180 },
  { title: t('issue.list.branch'), dataIndex: 'branch', key: 'branch', width: 140 },
  { title: t('issue.list.columns.type'), dataIndex: 'issue_type', key: 'issue_type', width: 70 },
  { title: t('issue.list.columns.severity'), key: 'severity', width: 90 },
  { title: t('issue.list.columns.zentaoStatus'), key: 'zentao_status', width: 100 },
  { title: t('issue.list.columns.testStatus'), key: 'test_status', width: 110 },
  { title: t('issue.list.columns.assignee'), dataIndex: 'assignee', key: 'assignee', width: 80 },
  { title: t('issue.list.columns.reporter'), dataIndex: 'reporter', key: 'reporter', width: 80 },
  { title: t('issue.list.columns.resolvedAt'), dataIndex: 'resolved_at', key: 'resolved_at', width: 170 },
  { title: t('issue.list.columns.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 170 },
  { title: t('issue.list.columns.action'), key: 'action', width: 280, fixed: 'right' as const, customHeaderCell: () => ({ style: 'text-align: center' }) },
])

const boardStageConfig = computed(() => [
  {
    key: 'pending',
    title: testStatusMap.value.pending?.label || t('issue.list.board.stages.pending'),
    statuses: ['pending'],
    tone: 'slate',
    emptyText: t('issue.list.board.emptyText.pending'),
  },
  {
    key: 'pending_upgrade',
    title: testStatusMap.value.pending_upgrade?.label || t('issue.list.board.stages.pendingUpgrade'),
    statuses: ['pending_upgrade'],
    tone: 'blue',
    emptyText: t('issue.list.board.emptyText.pendingUpgrade'),
  },
  {
    key: 'pending_generate',
    title: testStatusMap.value.pending_generate?.label || t('issue.list.board.stages.pendingGenerate'),
    statuses: ['pending_generate'],
    tone: 'cyan',
    emptyText: t('issue.list.board.emptyText.pendingGenerate'),
  },
  {
    key: 'generating',
    title: testStatusMap.value.generating?.label || t('issue.list.board.stages.generating'),
    statuses: ['generating'],
    tone: 'gold',
    emptyText: t('issue.list.board.emptyText.generating'),
  },
  {
    key: 'review',
    title: testStatusMap.value.review_pending?.label || t('issue.list.board.stages.review'),
    statuses: ['review_pending'],
    tone: 'purple',
    emptyText: t('issue.list.board.emptyText.review'),
  },
  {
    key: 'review_approved',
    title: testStatusMap.value.review_approved?.label || t('issue.list.board.stages.reviewApproved'),
    statuses: ['review_approved'],
    tone: 'green',
    emptyText: t('issue.list.board.emptyText.reviewApproved'),
  },
  {
    key: 'review_rejected',
    title: testStatusMap.value.review_rejected?.label || t('issue.list.board.stages.reviewRejected'),
    statuses: ['review_rejected'],
    tone: 'red',
    emptyText: t('issue.list.board.emptyText.reviewRejected'),
  },
])

const boardColumns = computed(() => {
  return boardStageConfig.value.map((stage) => {
    const items = list.value.filter((issue) => stage.statuses.includes(issue.test_status))
    const keyword = (boardSearch[stage.key] || '').trim().toLowerCase()
    const filteredItems = keyword
      ? items.filter((issue) => {
          const haystack = [
            issue.zentao_id,
            issue.title,
            issue.reporter,
            issue.assignee,
            issue.project?.name,
            issue.branch,
          ].join(' ').toLowerCase()
          return haystack.includes(keyword)
        })
      : items

    const summaries = stage.statuses
      .map((status) => ({
        label: testStatusMap.value[status]?.label || status,
        color: testStatusMap.value[status]?.color || 'default',
        count: items.filter((issue) => issue.test_status === status).length,
      }))
      .filter((summary) => summary.count > 0)

    return { ...stage, items, filteredItems, summaries }
  })
})

const activeIssueCount = computed(() =>
  list.value.filter((issue) => !['pending', 'review_approved', 'passed'].includes(issue.test_status)).length
)

const snapshotTime = computed(() => dayjs().format('YYYY/MM/DD HH:mm'))

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

function formatDateTime(value?: string | null) {
  if (!value) return '-'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : value
}

function canGenerateTest(issue: Issue) {
  // 允许对任意状态的Bug单生成测试，不限制禅道状态或测试状态
  return true
}

function toggleColumn(key: string) {
  collapsedColumns[key] = !collapsedColumns[key]
}

function summarizeDescription(value?: string) {
  if (!value) return t('issue.list.board.noDescription')
  const text = value.replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim()
  return text ? (text.length > 96 ? `${text.slice(0, 96)}...` : text) : t('issue.list.board.noDescription')
}
</script>

<style scoped>
.issue-page {
  min-width: 0;
}

.issue-page-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.issue-page-heading :deep(.ant-page-header) {
  flex: 1;
  padding-left: 0;
}

.view-switch {
  flex: none;
}

.issue-filter-bar {
  margin-bottom: 16px;
}

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

.bug-board {
  padding: 22px 0 8px;
  overflow: hidden;
  background:
    radial-gradient(circle at 12% 10%, rgba(65, 120, 255, 0.12), transparent 26%),
    linear-gradient(180deg, #f8fbff 0%, #f5f1e9 100%);
  border-radius: 20px;
}

.board-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  padding: 0 24px 22px;
}

.board-kicker {
  color: #7a8aa0;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.18em;
}

.board-hero h1 {
  margin: 8px 0 6px;
  color: #17233c;
  font-size: 30px;
  font-weight: 900;
}

.board-hero p {
  margin: 0;
  color: #6b778c;
}

.board-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(170px, 1fr));
  gap: 12px;
  min-width: 560px;
}

.board-stat-card {
  padding: 16px;
  border: 1px solid #d6e0ee;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.78);
  box-shadow: 0 18px 40px rgba(31, 44, 70, 0.08);
}

.board-stat-card--total {
  background: linear-gradient(135deg, #ffffff, #edf5ff);
}

.board-stat-card--running {
  background: linear-gradient(135deg, #fff8d7, #e7f6ff);
}

.board-stat-card--time {
  background: linear-gradient(135deg, #e9fff6, #f7ffdf);
}

.board-stat-card span,
.board-stat-card small {
  display: block;
  color: #61718a;
  font-size: 12px;
  font-weight: 700;
}

.board-stat-card strong {
  display: block;
  margin: 6px 0;
  color: #17233c;
  font-size: 26px;
  line-height: 1.1;
}

.board-columns {
  display: grid;
  grid-auto-columns: minmax(300px, 1fr);
  grid-auto-flow: column;
  gap: 16px;
  min-height: 560px;
  overflow-x: auto;
  padding: 0 24px 18px;
}

.board-column {
  display: flex;
  flex-direction: column;
  width: 100%;
  max-height: 72vh;
  padding: 16px;
  overflow: hidden;
  border: 1px solid #d8e1ee;
  border-top: 3px solid #6b7c93;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: 0 20px 46px rgba(31, 44, 70, 0.1);
}

.board-column.is-collapsed {
  width: 72px;
  min-width: 72px;
  align-items: center;
}

.board-column--blue {
  border-top-color: #28a9ff;
}

.board-column--gold {
  border-top-color: #d9a400;
}

.board-column--cyan {
  border-top-color: #13c2c2;
}

.board-column--green {
  border-top-color: #39c27f;
}

.board-column--red {
  border-top-color: #ff6b57;
}

.board-column-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
  border-bottom: 1px dashed #c6d1e0;
}

.board-column-header > div {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.board-column.is-collapsed .board-column-header {
  writing-mode: vertical-rl;
  border-bottom: 0;
}

.board-column h2 {
  margin: 0;
  color: #182640;
  font-size: 18px;
  font-weight: 900;
  white-space: nowrap;
}

.board-column-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 28px;
  height: 28px;
  padding: 0 8px;
  color: #637187;
  font-weight: 800;
  background: #edf2f7;
  border-radius: 999px;
}

.board-collapse-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  color: #53657c;
  cursor: pointer;
  background: transparent;
  border: 0;
  border-radius: 50%;
}

.board-collapse-btn:hover {
  background: #eef4fb;
}

.board-column-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-height: 28px;
  margin: 12px 0;
}

.board-column-search {
  margin-bottom: 14px;
}

.board-card-list {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 14px;
  min-height: 0;
  overflow-y: auto;
  padding-right: 2px;
}

.bug-card {
  width: 100%;
  padding: 16px;
  color: inherit;
  text-align: left;
  cursor: pointer;
  background: rgba(255, 255, 255, 0.94);
  border: 1px solid #d9e2ef;
  border-radius: 18px;
  box-shadow: 0 14px 28px rgba(37, 53, 80, 0.08);
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
}

.bug-card:hover {
  border-color: #7ea7ff;
  box-shadow: 0 18px 34px rgba(37, 53, 80, 0.14);
  transform: translateY(-2px);
}

.bug-card-head,
.bug-card footer,
.bug-card-tags,
.bug-card-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.bug-card-head {
  justify-content: space-between;
}

.bug-card-severity {
  margin-right: 0;
}

.bug-card-id {
  color: #7b8798;
  font-size: 12px;
  font-weight: 800;
}

.bug-card h3 {
  display: -webkit-box;
  margin: 10px 0 8px;
  overflow: hidden;
  color: #222b3a;
  font-size: 16px;
  font-weight: 900;
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.bug-card p {
  display: -webkit-box;
  min-height: 42px;
  margin: 0 0 12px;
  overflow: hidden;
  color: #6d788a;
  font-size: 13px;
  line-height: 1.6;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.bug-card dl {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin: 0 0 12px;
}

.bug-card dt {
  color: #8a96a8;
  font-size: 12px;
}

.bug-card dd {
  margin: 2px 0 0;
  color: #405066;
  font-size: 13px;
  font-weight: 700;
}

.bug-card-tags {
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.bug-card footer {
  justify-content: space-between;
  color: #7b8798;
  font-size: 12px;
}

.bug-card-actions {
  justify-content: flex-end;
}

.board-empty {
  display: flex;
  flex: 1;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 200px;
  padding: 22px;
  color: #8a96a8;
  text-align: center;
  border: 1px dashed #d5deeb;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.5);
}

.board-empty strong {
  margin-bottom: 8px;
  color: #65758d;
  font-size: 18px;
}

@media (max-width: 1200px) {
  .board-hero {
    flex-direction: column;
  }

  .board-stats {
    width: 100%;
    min-width: 0;
  }
}

@media (max-width: 768px) {
  .issue-page-heading {
    align-items: stretch;
    flex-direction: column;
  }

  .view-switch {
    align-self: flex-start;
  }

  .board-hero,
  .board-columns {
    padding-right: 14px;
    padding-left: 14px;
  }

  .board-stats {
    grid-template-columns: 1fr;
  }

  .board-columns {
    grid-auto-columns: minmax(280px, 86vw);
  }
}
</style>
