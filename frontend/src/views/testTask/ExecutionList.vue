<template>
  <div>
    <a-page-header v-if="!embedded" :title="t('execution.list.title')" />

    <a-row :gutter="16" style="margin-bottom: 16px">
      <a-col :span="4">
        <a-select v-model:value="query.status" :placeholder="t('execution.list.statusPlaceholder')" allowClear style="width: 100%">
          <a-select-option value="pending">{{ translateExecutionStatus(t, 'pending') }}</a-select-option>
          <a-select-option value="running">{{ translateExecutionStatus(t, 'running') }}</a-select-option>
          <a-select-option value="passed">{{ translateExecutionStatus(t, 'passed') }}</a-select-option>
          <a-select-option value="failed">{{ translateExecutionStatus(t, 'failed') }}</a-select-option>
          <a-select-option value="error">{{ translateExecutionStatus(t, 'error') }}</a-select-option>
        </a-select>
      </a-col>
      <a-col>
        <a-button type="primary" @click="fetchData">{{ t('common.query') }}</a-button>
      </a-col>
    </a-row>

    <a-table :dataSource="list" :columns="columns" :loading="loading" :pagination="pagination"
             @change="handleTableChange" rowKey="id" size="middle">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'status'">
          <a-tag :color="execStatusColor(record.status)">{{ translateExecutionStatus(t, record.status) }}</a-tag>
        </template>
        <template v-if="column.key === 'pass_rate'">
          <a-progress :percent="record.pass_rate" :status="record.pass_rate >= 100 ? 'success' : 'active'" size="small" />
        </template>
        <template v-if="column.key === 'duration'">
          {{ record.duration_sec ? `${record.duration_sec}s` : '-' }}
        </template>
        <template v-if="column.key === 'trigger'">
          <a-tag>{{ translateTriggerType(t, record.trigger_type) }}</a-tag>
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { getExecutionList } from '@/api/testTask'
import type { TestExecution } from '@/types'
import { translateExecutionStatus, translateTriggerType } from '@/types'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const route = useRoute()
withDefaults(defineProps<{ embedded?: boolean }>(), {
  embedded: false,
})
const list = ref<TestExecution[]>([])
const loading = ref(false)
const query = reactive({ status: undefined as string | undefined })
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })

const columns = computed(() => [
  { title: t('common.id'), dataIndex: 'id', key: 'id', width: 60 },
  { title: t('execution.list.columns.trigger'), key: 'trigger', width: 90 },
  { title: t('execution.list.columns.branch'), dataIndex: 'branch', key: 'branch', width: 120 },
  { title: t('execution.list.columns.status'), key: 'status', width: 90 },
  { title: t('execution.list.columns.totalCases'), dataIndex: 'total_cases', key: 'total_cases', width: 80 },
  { title: t('execution.list.columns.passedCases'), dataIndex: 'passed_cases', key: 'passed_cases', width: 60 },
  { title: t('execution.list.columns.failedCases'), dataIndex: 'failed_cases', key: 'failed_cases', width: 60 },
  { title: t('execution.list.columns.passRate'), key: 'pass_rate', width: 160 },
  { title: t('execution.list.columns.duration'), key: 'duration', width: 80 },
  { title: t('execution.list.columns.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 170 },
])

onMounted(fetchData)
watch(() => route.query.task_id, () => {
  pagination.current = 1
  fetchData()
})

async function fetchData() {
  loading.value = true
  try {
    const res = await getExecutionList({
      ...query,
      task_id: route.query.task_id ? Number(route.query.task_id) : undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
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

function execStatusColor(s: string) {
  const map: Record<string, string> = {
    pending: 'default', running: 'processing', passed: 'success', failed: 'error', error: 'volcano',
  }
  return map[s] || 'default'
}
</script>
