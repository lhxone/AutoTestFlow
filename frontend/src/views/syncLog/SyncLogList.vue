<template>
  <div>
    <a-page-header :title="t('syncLog.list.title')" />

    <a-row :gutter="16" style="margin-bottom: 16px">
      <a-col :span="6">
        <a-select
          v-model:value="query.project_id"
          :placeholder="t('syncLog.list.selectProject')"
          allowClear
          show-search
          :filter-option="filterOption"
          @change="handleQuery"
        >
          <a-select-option v-for="p in projects" :key="p.id" :value="p.id">
            {{ p.name }}
          </a-select-option>
        </a-select>
      </a-col>
      <a-col>
        <a-button type="primary" @click="handleQuery">{{ t('common.query') }}</a-button>
      </a-col>
    </a-row>

    <a-row :gutter="16">
      <a-col :span="10">
        <a-card :title="t('syncLog.list.logListTitle')" size="small">
          <a-table
            :dataSource="syncLogs"
            :columns="syncLogColumns"
            :loading="loading"
            :pagination="pagination"
            rowKey="id"
            size="small"
            @change="handleTableChange"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'project'">
                {{ getProjectName(record.project_id) }}
              </template>
              <template v-if="column.key === 'status'">
                <a-tag :color="getSyncStatusColor(record.status)">{{ translateSyncStatus(record.status) }}</a-tag>
              </template>
              <template v-if="column.key === 'mode'">
                <a-tag :color="record.full_sync ? 'purple' : 'blue'">
                  {{ record.full_sync ? t('syncLog.mode.full') : t('syncLog.mode.incremental') }}
                </a-tag>
              </template>
              <template v-if="column.key === 'counts'">
                <div>{{ t('syncLog.summary.added') }}: {{ record.added_count }}</div>
                <div>{{ t('syncLog.summary.updated') }}: {{ record.updated_count }}</div>
                <div>{{ t('syncLog.summary.deleted') }}: {{ record.deleted_count }}</div>
              </template>
              <template v-if="column.key === 'action'">
                <a-button type="link" size="small" @click="fetchSyncLogDetail(record.id)">
                  {{ selectedSyncLog?.id === record.id ? t('syncLog.viewing') : t('syncLog.viewDetail') }}
                </a-button>
              </template>
            </template>
          </a-table>
        </a-card>
      </a-col>
      <a-col :span="14">
        <a-card :title="t('syncLog.list.detailTitle')" size="small">
          <a-empty v-if="!selectedSyncLog && !detailLoading" :description="t('syncLog.list.noRecords')" />
          <template v-else>
            <a-descriptions v-if="selectedSyncLog" :column="2" bordered size="small" style="margin-bottom: 16px">
              <a-descriptions-item :label="t('common.id')">{{ selectedSyncLog.id }}</a-descriptions-item>
              <a-descriptions-item :label="t('common.status')">
                <a-tag :color="getSyncStatusColor(selectedSyncLog.status)">{{ translateSyncStatus(selectedSyncLog.status) }}</a-tag>
              </a-descriptions-item>
              <a-descriptions-item :label="t('syncLog.summary.project')">
                {{ getProjectName(selectedSyncLog.project_id) }}
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
              <a-descriptions-item :label="t('syncLog.summary.deleted')">
                {{ selectedSyncLog.deleted_count }}
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
              v-if="selectedSyncLog"
              :dataSource="syncDetails"
              :columns="syncDetailColumns"
              :loading="detailLoading"
              :pagination="false"
              rowKey="id"
              size="small"
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
          </template>
        </a-card>
      </a-col>
    </a-row>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  getAllIssueSyncLogs,
  getIssueSyncLogDetail,
  getProjectList,
} from '@/api/project'
import type { Project, ProjectIssueSyncLog, ProjectIssueSyncDetail } from '@/types'

const route = useRoute()
const { t } = useI18n()
const loading = ref(false)
const detailLoading = ref(false)
const syncLogs = ref<ProjectIssueSyncLog[]>([])
const syncDetails = ref<ProjectIssueSyncDetail[]>([])
const selectedSyncLog = ref<ProjectIssueSyncLog | null>(null)
const projects = ref<Project[]>([])
const query = reactive<{ project_id: number | null }>({ project_id: null })
const pagination = reactive({ current: 1, pageSize: 8, total: 0 })

const syncLogColumns = computed(() => [
  { title: t('common.id'), dataIndex: 'id', key: 'id', width: 70 },
  { title: t('syncLog.summary.project'), key: 'project', width: 120 },
  { title: t('common.status'), key: 'status', width: 100 },
  { title: t('syncLog.summary.mode'), key: 'mode', width: 100 },
  { title: t('syncLog.summary.totalChanges'), key: 'counts', width: 150 },
  { title: t('syncLog.summary.startedAt'), dataIndex: 'started_at', key: 'started_at', width: 160 },
  { title: t('common.action'), key: 'action', width: 100 },
])

const syncDetailColumns = computed(() => [
  { title: t('syncLog.detail.action'), key: 'action', width: 90 },
  { title: t('syncLog.detail.zentaoId'), dataIndex: 'zentao_id', key: 'zentao_id', width: 90 },
  { title: t('syncLog.detail.issueTitle'), dataIndex: 'issue_title', key: 'issue_title', ellipsis: true },
  { title: t('syncLog.detail.changedFields'), key: 'changedFields' },
])

onMounted(async () => {
  // 从 URL 参数中读取 project_id
  const projectIdParam = route.query.project_id
  if (projectIdParam) {
    query.project_id = Number(projectIdParam)
  }
  await Promise.all([fetchProjects(), fetchData()])
})

async function fetchProjects() {
  try {
    const res = await getProjectList({ page: 1, page_size: 1000 })
    projects.value = res.data.data?.list || []
  } catch {
    // ignore
  }
}

async function fetchData(targetLogId?: number) {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (query.project_id) {
      params.project_id = query.project_id
    }
    const res = await getAllIssueSyncLogs(params)
    const data = res.data.data
    syncLogs.value = data.list || []
    pagination.total = data.total || 0

    const nextLogId = targetLogId ?? selectedSyncLog.value?.id ?? syncLogs.value[0]?.id
    if (nextLogId) {
      await fetchSyncLogDetail(nextLogId)
    } else {
      selectedSyncLog.value = null
      syncDetails.value = []
    }
  } finally {
    loading.value = false
  }
}

async function fetchSyncLogDetail(logId: number) {
  detailLoading.value = true
  try {
    const res = await getIssueSyncLogDetail(logId)
    selectedSyncLog.value = res.data.data?.log || null
    syncDetails.value = res.data.data?.details || []
  } finally {
    detailLoading.value = false
  }
}

function handleTableChange(pag: any) {
  pagination.current = pag.current
  fetchData(selectedSyncLog.value?.id)
}

function handleQuery() {
  pagination.current = 1
  fetchData()
}

function filterOption(input: string, option: any) {
  const label = option.children?.[0]?.children || ''
  return String(label).toLowerCase().includes(input.toLowerCase())
}

function getProjectName(projectId: number) {
  return projects.value.find(p => p.id === projectId)?.name || '-'
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
</script>
