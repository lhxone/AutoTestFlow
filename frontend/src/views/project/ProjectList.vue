<template>
  <div>
    <!-- 项目列表视图 -->
    <template v-if="!showSyncLogs">
      <a-page-header :title="t('project.list.title')" />

      <a-row :gutter="16" style="margin-bottom: 16px">
        <a-col :span="6">
          <a-input v-model:value="query.keyword" :placeholder="t('project.list.searchPlaceholder')" allowClear @pressEnter="fetchData" />
        </a-col>
        <a-col>
          <a-button type="primary" @click="fetchData">{{ t('common.query') }}</a-button>
          <a-button style="margin-left: 8px" @click="openCreate">{{ t('project.list.create') }}</a-button>
        </a-col>
      </a-row>

      <a-table :dataSource="list" :columns="columns" :loading="loading" :pagination="pagination"
               @change="handleTableChange" rowKey="id" size="middle">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'zentao'">
            {{ record.zentao_project_name || '-' }}
          </template>
          <template v-if="column.key === 'owner'">
            {{ record.owner?.real_name || '-' }}
          </template>
          <template v-if="column.key === 'status'">
            <a-tag :color="record.status === 1 ? 'green' : 'red'">{{ record.status === 1 ? t('common.enabled') : t('common.disabled') }}</a-tag>
          </template>
          <template v-if="column.key === 'action'">
            <a-button
              type="link"
              size="small"
              :loading="syncingProjectId === record.id"
              :disabled="!record.zentao_project_id"
              @click="handleSyncIssues(record)"
            >
              {{ t('project.list.syncIssues') }}
            </a-button>
            <a-button type="link" size="small" @click="showProjectSyncLogs(record)">
              {{ t('project.list.viewSyncLogs') }}
            </a-button>
            <a-button type="link" size="small" @click="handleEdit(record)">{{ t('common.edit') }}</a-button>
            <a-popconfirm :title="t('common.confirmDelete')" @confirm="handleDelete(record.id)">
              <a-button type="link" size="small" danger>{{ t('common.delete') }}</a-button>
            </a-popconfirm>
          </template>
        </template>
      </a-table>

      <a-modal v-model:open="showModal" :title="editing ? t('project.list.edit') : t('project.list.create')" @ok="handleSubmit"
               :confirmLoading="submitting" width="640px">
        <a-form layout="vertical">
          <a-form-item :label="t('project.list.form.name')" required>
            <a-input v-model:value="form.name" />
          </a-form-item>
          <a-form-item :label="t('project.list.form.description')">
            <a-textarea v-model:value="form.description" :rows="2" />
          </a-form-item>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('project.list.form.funcDocPath')">
                <a-input v-model:value="form.func_doc_path" placeholder="docs/function.md" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('project.list.form.designDocPath')">
                <a-input v-model:value="form.design_doc_path" placeholder="docs/design.md" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('project.list.form.dbDocPath')">
                <a-input v-model:value="form.db_doc_path" placeholder="docs/database.md" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('project.list.form.testDocPath')">
                <a-input v-model:value="form.test_doc_path" placeholder="docs/test.md" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="16">
            <a-col :span="16">
              <a-form-item :label="t('project.list.form.gitRepoUrl')">
                <a-input v-model:value="form.git_repo_url" placeholder="https://gitlab.example.com/group/repo.git" />
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('project.list.form.gitBranch')">
                <a-input v-model:value="form.git_branch" placeholder="main" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('project.list.form.gitPullInterval')">
                <a-input-number
                  v-model:value="form.git_pull_interval"
                  :min="0"
                  :max="1440"
                  :step="5"
                  style="width: 100%"
                />
                <div style="color: #999; font-size: 12px; margin-top: 2px">{{ t('project.list.form.gitPullIntervalHint') }}</div>
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('project.list.form.zentaoProject')">
                <a-select v-model:value="selectedZentaoProject" :placeholder="t('project.list.form.selectZentaoProject')"
                          :loading="zentaoProjectsLoading" show-search :filter-option="filterOption"
                          allowClear @change="onZentaoProjectChange" @focus="fetchZentaoProjects">
                  <a-select-option v-for="p in zentaoProjects" :key="p.id" :value="p.id" :label="p.label">
                    {{ p.name }} (ID: {{ p.id }})
                  </a-select-option>
                </a-select>
                <div style="color: #999; font-size: 12px; margin-top: 2px">{{ t('project.list.form.zentaoProjectHint') }}</div>
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('project.list.form.zentaoBranch')">
                <a-select v-model:value="form.zentao_branch" :placeholder="t('project.list.form.selectZentaoBranch')"
                          :loading="zentaoBranchesLoading" allowClear
                          :disabled="!selectedZentaoProject" @focus="() => fetchZentaoBranches()">
                  <a-select-option v-for="b in zentaoBranches" :key="b.id" :value="String(b.id)" :label="b.name">
                    {{ b.name }}
                  </a-select-option>
                </a-select>
                <div style="color: #999; font-size: 12px; margin-top: 2px">{{ t('project.list.form.zentaoBranchHint') }}</div>
              </a-form-item>
            </a-col>
          </a-row>
        </a-form>
      </a-modal>
    </template>

    <!-- 采集记录视图 -->
    <template v-else>
      <a-page-header :title="t('syncLog.list.title') + ' - ' + currentProject?.name" @back="closeSyncLogs">
        <template #extra>
          <a-button @click="closeSyncLogs">{{ t('common.back') }}</a-button>
        </template>
      </a-page-header>

      <!-- 采集记录列表 -->
      <a-card :title="t('syncLog.list.logListTitle')" size="small" style="margin-bottom: 16px">
        <a-table
          :dataSource="syncLogs"
          :columns="syncLogColumns"
          :loading="syncLoading"
          :pagination="syncPagination"
          rowKey="id"
          size="small"
          @change="handleSyncTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <a-tag :color="getSyncStatusColor(record.status)">{{ translateSyncStatus(record.status) }}</a-tag>
            </template>
            <template v-if="column.key === 'mode'">
              <a-tag :color="record.full_sync ? 'purple' : 'blue'">
                {{ record.full_sync ? t('syncLog.mode.full') : t('syncLog.mode.incremental') }}
              </a-tag>
            </template>
            <template v-if="column.key === 'counts'">
              <span>{{ t('syncLog.summary.added') }}: {{ record.added_count }} / {{ t('syncLog.summary.updated') }}: {{ record.updated_count }} / {{ t('syncLog.summary.deleted') }}: {{ record.deleted_count }}</span>
            </template>
            <template v-if="column.key === 'action'">
              <a-button type="link" size="small" @click="selectSyncLog(record)">
                {{ selectedSyncLog?.id === record.id ? t('syncLog.viewing') : t('syncLog.viewDetail') }}
              </a-button>
            </template>
          </template>
        </a-table>
      </a-card>

      <!-- 采集详情 -->
      <a-card v-if="selectedSyncLog" :title="t('syncLog.list.detailTitle')" size="small">
        <a-descriptions :column="4" bordered size="small" style="margin-bottom: 16px">
          <a-descriptions-item :label="t('common.id')">{{ selectedSyncLog.id }}</a-descriptions-item>
          <a-descriptions-item :label="t('common.status')">
            <a-tag :color="getSyncStatusColor(selectedSyncLog.status)">{{ translateSyncStatus(selectedSyncLog.status) }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item :label="t('syncLog.summary.mode')">
            {{ selectedSyncLog.full_sync ? t('syncLog.mode.full') : t('syncLog.mode.incremental') }}
          </a-descriptions-item>
          <a-descriptions-item :label="t('syncLog.summary.startedAt')">
            {{ selectedSyncLog.started_at || '-' }}
          </a-descriptions-item>
          <a-descriptions-item :label="t('syncLog.summary.completedAt')">
            {{ selectedSyncLog.completed_at || '-' }}
          </a-descriptions-item>
          <a-descriptions-item :label="t('syncLog.summary.totalChanges')">
            {{ selectedSyncLog.added_count + selectedSyncLog.updated_count + selectedSyncLog.deleted_count }}
          </a-descriptions-item>
          <a-descriptions-item :label="t('syncLog.summary.added')">
            {{ selectedSyncLog.added_count }}
          </a-descriptions-item>
          <a-descriptions-item :label="t('syncLog.summary.updated')">
            {{ selectedSyncLog.updated_count }}
          </a-descriptions-item>
        </a-descriptions>

        <a-alert
          v-if="selectedSyncLog?.error_message"
          type="error"
          show-icon
          :message="t('syncLog.summary.errorMessage')"
          :description="selectedSyncLog.error_message"
          style="margin-bottom: 16px"
        />

        <a-table
          :dataSource="syncDetails"
          :columns="syncDetailColumns"
          :loading="detailLoading"
          :pagination="detailPagination"
          rowKey="id"
          size="small"
          @change="handleDetailTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'action'">
              <a-tag :color="getSyncActionColor(record.action)">{{ translateSyncAction(record.action) }}</a-tag>
            </template>
            <template v-if="column.key === 'changedFields'">
              <div
                v-if="record.action === 'updated' && record.changed_fields?.length"
                style="white-space: normal; word-break: break-word"
              >
                <div v-for="change in record.changed_fields" :key="`${record.id}-${change.field}`" style="margin-bottom: 8px">
                  <div style="font-weight: 500">{{ change.field_label }}</div>
                  <div style="color: #666">{{ t('syncLog.detail.oldValue') }}: {{ displayChangeValue(change.old_value) }}</div>
                  <div>{{ t('syncLog.detail.newValue') }}: {{ displayChangeValue(change.new_value) }}</div>
                </div>
              </div>
              <span v-else-if="record.action === 'added'">{{ t('syncLog.detail.addedHint') }}</span>
              <span v-else>{{ t('syncLog.detail.deletedHint') }}</span>
            </template>
          </template>
        </a-table>
      </a-card>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import {
  getProjectList,
  createProject,
  updateProject,
  deleteProject,
  getProjectIssueSyncLogs,
  getProjectIssueSyncLogDetail,
} from '@/api/project'
import { syncIssues } from '@/api/issue'
import { getZentaoProducts, getZentaoBranches } from '@/api/zentao'
import type { Project, ProjectIssueSyncLog, ProjectIssueSyncDetail } from '@/types'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const list = ref<Project[]>([])
const loading = ref(false)
const showModal = ref(false)
const submitting = ref(false)
const syncingProjectId = ref<number | null>(null)
const editing = ref<Project | null>(null)
const query = reactive({ keyword: '' })
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })

const zentaoProjects = ref<{ id: number; name: string; label: string }[]>([])
const zentaoBranches = ref<{ id: string; name: string }[]>([])
const zentaoProjectsLoading = ref(false)
const zentaoBranchesLoading = ref(false)
const selectedZentaoProject = ref<number | null>(null)

const form = reactive({
  name: '',
  description: '',
  func_doc_path: '',
  design_doc_path: '',
  db_doc_path: '',
  test_doc_path: '',
  git_repo_url: '',
  git_branch: 'main',
  git_pull_interval: 0,
  zentao_project_id: null as number | null,
  zentao_project_name: '',
  zentao_branch: '',
})

// 采集记录相关状态
const showSyncLogs = ref(false)
const currentProject = ref<Project | null>(null)
const syncLoading = ref(false)
const detailLoading = ref(false)
const syncLogs = ref<ProjectIssueSyncLog[]>([])
const syncDetails = ref<ProjectIssueSyncDetail[]>([])
const selectedSyncLog = ref<ProjectIssueSyncLog | null>(null)
const syncPagination = reactive({ current: 1, pageSize: 10, total: 0 })
const detailPagination = reactive({ current: 1, pageSize: 20, total: 0 })

const columns = computed(() => [
  { title: t('common.id'), dataIndex: 'id', key: 'id', width: 60 },
  { title: t('project.list.columns.name'), dataIndex: 'name', key: 'name' },
  { title: t('project.list.columns.zentaoProject'), key: 'zentao' },
  { title: t('project.list.columns.owner'), key: 'owner', width: 100 },
  { title: t('project.list.columns.status'), key: 'status', width: 80 },
  { title: t('project.list.columns.action'), key: 'action', width: 300 },
])

const syncLogColumns = computed(() => [
  { title: t('common.id'), dataIndex: 'id', key: 'id', width: 70 },
  { title: t('common.status'), key: 'status', width: 100 },
  { title: t('syncLog.summary.mode'), key: 'mode', width: 100 },
  { title: t('syncLog.summary.totalChanges'), key: 'counts' },
  { title: t('syncLog.summary.startedAt'), dataIndex: 'started_at', key: 'started_at', width: 160 },
  { title: t('common.action'), key: 'action', width: 100 },
])

const syncDetailColumns = computed(() => [
  { title: t('syncLog.detail.action'), key: 'action', width: 90 },
  { title: t('syncLog.detail.zentaoId'), dataIndex: 'zentao_id', key: 'zentao_id', width: 90 },
  { title: t('syncLog.detail.issueTitle'), dataIndex: 'issue_title', key: 'issue_title', ellipsis: true },
  { title: t('syncLog.detail.changedFields'), key: 'changedFields' },
])

onMounted(fetchData)

async function fetchData() {
  loading.value = true
  try {
    const res = await getProjectList({ ...query, page: pagination.current, page_size: pagination.pageSize })
    const data = res.data.data
    list.value = data.list || []
    pagination.total = data.total
  } finally {
    loading.value = false
  }
}

function handleTableChange(pag: any) {
  pagination.current = pag.current
  fetchData()
}

function openCreate() {
  editing.value = null
  selectedZentaoProject.value = null
  zentaoBranches.value = []
  Object.assign(form, {
    name: '',
    description: '',
    func_doc_path: '',
    design_doc_path: '',
    db_doc_path: '',
    test_doc_path: '',
    git_repo_url: '',
    git_branch: 'main',
    git_pull_interval: 0,
    zentao_project_id: null,
    zentao_project_name: '',
    zentao_branch: '',
  })
  showModal.value = true
}

function handleEdit(record: Project) {
  editing.value = record
  selectedZentaoProject.value = record.zentao_project_id
  zentaoBranches.value = []
  Object.assign(form, {
    name: record.name,
    description: record.description,
    func_doc_path: record.func_doc_path,
    design_doc_path: record.design_doc_path,
    db_doc_path: record.db_doc_path,
    test_doc_path: record.test_doc_path,
    git_repo_url: record.git_repo_url,
    git_branch: record.git_branch,
    git_pull_interval: record.git_pull_interval || 0,
    zentao_project_id: record.zentao_project_id,
    zentao_project_name: record.zentao_project_name,
    zentao_branch: record.zentao_branch,
  })
  void fetchZentaoProjects()
  if (record.zentao_project_id) {
    void fetchZentaoBranches(record.zentao_project_id)
  }
  showModal.value = true
}

async function fetchZentaoProjects() {
  if (zentaoProjects.value.length > 0) return
  zentaoProjectsLoading.value = true
  try {
    const res = await getZentaoProducts()
    zentaoProjects.value = (res.data.data || []).map((item: { id: number; name: string }) => ({
      ...item,
      label: `${item.name} (ID: ${item.id})`,
    }))
  } catch {
    message.warning(t('project.list.messages.loadProductsFailed'))
  } finally {
    zentaoProjectsLoading.value = false
  }
}

function onZentaoProjectChange(projectId: number | null | undefined) {
  const normalizedProjectId = projectId ?? null
  const project = zentaoProjects.value.find((p) => p.id === normalizedProjectId)
  form.zentao_project_id = normalizedProjectId
  form.zentao_project_name = project?.name || ''
  form.zentao_branch = ''
  zentaoBranches.value = []
  if (normalizedProjectId) {
    void fetchZentaoBranches(normalizedProjectId)
  }
}

async function fetchZentaoBranches(projectId?: number | null | Event) {
  const normalizedProjectId = typeof projectId === 'number'
    ? projectId
    : selectedZentaoProject.value
  if (!normalizedProjectId) return
  zentaoBranchesLoading.value = true
  try {
    const res = await getZentaoBranches(normalizedProjectId)
    zentaoBranches.value = res.data.data || []
    const matchedBranch = zentaoBranches.value.find(
      (branch) => String(branch.id) === form.zentao_branch || branch.name === form.zentao_branch,
    )
    if (matchedBranch) {
      form.zentao_branch = String(matchedBranch.id)
    }
  } catch {
    message.warning(t('project.list.messages.loadBranchesFailed'))
  } finally {
    zentaoBranchesLoading.value = false
  }
}

function filterOption(input: string, option: any) {
  const label = option.label ?? option.value ?? ''
  return String(label).toLowerCase().includes(input.toLowerCase())
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (editing.value) {
      await updateProject(editing.value.id, form)
      message.success(t('common.updateSuccess'))
    } else {
      await createProject(form)
      message.success(t('common.createSuccess'))
    }
    showModal.value = false
    fetchData()
  } finally {
    submitting.value = false
  }
}

async function handleSyncIssues(record: Project) {
  if (!record.zentao_project_id) {
    message.warning(t('project.list.messages.syncDisabled'))
    return
  }

  syncingProjectId.value = record.id
  try {
    await syncIssues({ project_id: record.id, full_sync: false })
    message.success(t('project.list.messages.syncTriggered'))
    // 刷新采集记录列表
    if (showSyncLogs.value && currentProject.value?.id === record.id) {
      fetchSyncLogs()
    }
  } finally {
    syncingProjectId.value = null
  }
}

// 显示项目的采集记录
function showProjectSyncLogs(record: Project) {
  currentProject.value = record
  showSyncLogs.value = true
  syncPagination.current = 1
  selectedSyncLog.value = null
  syncDetails.value = []
  fetchSyncLogs()
}

// 关闭采集记录视图
function closeSyncLogs() {
  showSyncLogs.value = false
  currentProject.value = null
  syncLogs.value = []
  syncDetails.value = []
  selectedSyncLog.value = null
}

// 获取采集记录列表
async function fetchSyncLogs() {
  if (!currentProject.value) return
  syncLoading.value = true
  try {
    const res = await getProjectIssueSyncLogs(currentProject.value.id, {
      page: syncPagination.current,
      page_size: syncPagination.pageSize,
    })
    const data = res.data.data
    syncLogs.value = data.list || []
    syncPagination.total = data.total || 0
  } finally {
    syncLoading.value = false
  }
}

// 选择采集记录查看详情
function selectSyncLog(record: ProjectIssueSyncLog) {
  selectedSyncLog.value = record
  detailPagination.current = 1
  fetchSyncLogDetail()
}

// 获取采集记录详情
async function fetchSyncLogDetail() {
  if (!currentProject.value || !selectedSyncLog.value) return
  detailLoading.value = true
  try {
    const res = await getProjectIssueSyncLogDetail(currentProject.value.id, selectedSyncLog.value.id, {
      page: detailPagination.current,
      page_size: detailPagination.pageSize,
    })
    const data = res.data.data
    selectedSyncLog.value = data.log || selectedSyncLog.value
    syncDetails.value = data.details || []
    detailPagination.total = data.total || 0
  } finally {
    detailLoading.value = false
  }
}

function handleSyncTableChange(pag: any) {
  syncPagination.current = pag.current
  fetchSyncLogs()
}

function handleDetailTableChange(pag: any) {
  detailPagination.current = pag.current
  fetchSyncLogDetail()
}

function getSyncStatusColor(status: string) {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'error'
    case 'running':
      return 'processing'
    default:
      return 'default'
  }
}

function getSyncActionColor(action: string) {
  switch (action) {
    case 'added':
      return 'success'
    case 'updated':
      return 'processing'
    case 'deleted':
      return 'error'
    default:
      return 'default'
  }
}

function translateSyncStatus(status: string) {
  return t(`syncLog.status.${status}`)
}

function translateSyncAction(action: string) {
  return t(`syncLog.actionType.${action}`)
}

function displayChangeValue(value: string) {
  return value || t('syncLog.detail.emptyValue')
}

async function handleDelete(id: number) {
  await deleteProject(id)
  message.success(t('common.deleteSuccess'))
  fetchData()
}
</script>
