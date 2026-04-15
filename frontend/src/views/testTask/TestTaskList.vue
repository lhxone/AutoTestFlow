<template>
  <div>
    <a-page-header v-if="!embedded" :title="t('testTask.list.title')" />

    <a-row :gutter="16" style="margin-bottom: 16px">
      <a-col :span="4">
        <a-select v-model:value="query.status" :placeholder="t('testTask.list.statusPlaceholder')" allowClear style="width: 100%">
          <a-select-option value="pending">{{ translateTaskStatus(t, 'pending') }}</a-select-option>
          <a-select-option value="running">{{ translateTaskStatus(t, 'running') }}</a-select-option>
          <a-select-option value="completed">{{ translateTaskStatus(t, 'completed') }}</a-select-option>
          <a-select-option value="failed">{{ translateTaskStatus(t, 'failed') }}</a-select-option>
        </a-select>
      </a-col>
      <a-col>
        <a-button type="primary" @click="fetchData">{{ t('common.query') }}</a-button>
      </a-col>
    </a-row>

    <a-table :dataSource="list" :columns="columns" :loading="loading" :pagination="pagination"
             @change="handleTableChange" rowKey="id" size="middle">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'issue'">
          {{ record.issue?.title || '-' }}
        </template>
        <template v-if="column.key === 'status'">
          <a-tag :color="statusColor(record.status)">{{ translateTaskStatus(t, record.status) }}</a-tag>
        </template>
        <template v-if="column.key === 'action'">
          <a-button type="link" size="small" @click="showDetail(record)">{{ t('testTask.list.detail') }}</a-button>
        </template>
      </template>
    </a-table>

  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { getTestTaskList } from '@/api/testTask'
import type { TestTask } from '@/types'
import { translateTaskStatus } from '@/types'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const router = useRouter()
withDefaults(defineProps<{ embedded?: boolean }>(), {
  embedded: false,
})
const list = ref<TestTask[]>([])
const loading = ref(false)
const query = reactive({ status: undefined as string | undefined })
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })

const columns = computed(() => [
  { title: t('common.id'), dataIndex: 'id', key: 'id', width: 60 },
  { title: t('testTask.list.columns.issue'), key: 'issue', ellipsis: true },
  { title: t('testTask.list.columns.workflow'), dataIndex: 'workflow_name', key: 'workflow_name', width: 140 },
  { title: t('testTask.list.columns.status'), key: 'status', width: 100 },
  { title: t('testTask.list.columns.retryCount'), dataIndex: 'retry_count', key: 'retry_count', width: 60 },
  { title: t('testTask.list.columns.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 170 },
  { title: t('testTask.list.columns.action'), key: 'action', width: 80 },
])

onMounted(fetchData)

async function fetchData() {
  loading.value = true
  try {
    const res = await getTestTaskList({ ...query, page: pagination.current, page_size: pagination.pageSize })
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

function showDetail(task: TestTask) {
  router.push({ name: 'TaskRunDetail', params: { id: String(task.id) } })
}

function statusColor(s: string) {
  const map: Record<string, string> = { pending: 'default', running: 'processing', completed: 'success', failed: 'error' }
  return map[s] || 'default'
}
</script>
