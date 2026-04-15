<template>
  <div>
    <a-page-header v-if="!embedded" :title="t('testCase.list.title')" />

    <a-row :gutter="16" style="margin-bottom: 16px">
      <a-col :span="8">
        <a-input v-model:value="query.keyword" :placeholder="t('testCase.list.searchPlaceholder')" allowClear @pressEnter="fetchData" />
      </a-col>
      <a-col :span="4">
        <a-select v-model:value="query.status" :placeholder="t('testCase.list.statusPlaceholder')" allowClear style="width: 100%">
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

    <a-table
      :dataSource="list"
      :columns="columns"
      :loading="loading"
      :pagination="pagination"
      @change="handleTableChange"
      rowKey="id"
      size="middle"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'issue'">
          {{ record.issue?.title || '-' }}
        </template>
        <template v-if="column.key === 'status'">
          <a-tag :color="taskStatusColor(record.status)">{{ translateTaskStatus(t, record.status) }}</a-tag>
        </template>
        <template v-if="column.key === 'action'">
          <a-button type="link" size="small" @click="goToEdit(record)" v-if="canIntervene">{{ t('common.edit') }}</a-button>
          <a-button type="link" size="small" @click="goToReview(record)" v-if="canReview">{{ t('review.list.goReview') }}</a-button>
          <a-button type="link" size="small" @click="goToExecutions(record)" v-if="canViewExecutions">{{ t('execution.list.title') }}</a-button>
          <a-popconfirm
            :title="t('testCase.list.messages.regenerateConfirm')"
            @confirm="handleRegenerate(record)"
            v-if="canTrigger"
          >
            <a-button type="link" size="small" danger :loading="regeneratingId === record.id">{{ t('testCase.list.regenerate') }}</a-button>
          </a-popconfirm>
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { getReviewList } from '@/api/review'
import { getTestTaskList, createTestTask } from '@/api/testTask'
import type { TestTask } from '@/types'
import { translateTaskStatus } from '@/types'
import { useI18n } from 'vue-i18n'

const userStore = useUserStore()
const router = useRouter()
withDefaults(defineProps<{ embedded?: boolean }>(), {
  embedded: false,
})
const canIntervene = computed(() => userStore.hasPermission('test:intervene'))
const canReview = computed(() => userStore.hasPermission('review:list'))
const canViewExecutions = computed(() => userStore.hasPermission('test:list'))
const canTrigger = computed(() => userStore.hasPermission('test:trigger'))
const { t } = useI18n()

const list = ref<TestTask[]>([])
const loading = ref(false)
const query = reactive({
  keyword: '',
  status: undefined as string | undefined,
})
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })

const columns = computed(() => [
  { title: t('common.id'), dataIndex: 'id', key: 'id', width: 80 },
  { title: t('testCase.list.columns.issue'), key: 'issue', ellipsis: true },
  { title: t('testCase.list.columns.issueId'), dataIndex: 'issue_id', key: 'issue_id', width: 100 },
  { title: t('testCase.list.columns.taskStatus'), key: 'status', width: 110 },
  { title: t('testCase.list.columns.updatedAt'), dataIndex: 'updated_at', key: 'updated_at', width: 180 },
  { title: t('testCase.list.columns.action'), key: 'action', width: 220 },
])

onMounted(fetchData)

async function fetchData() {
  loading.value = true
  try {
    const res = await getTestTaskList({
      ...query,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    const data = res.data.data
    list.value = data.list || []
    pagination.total = data.total || 0
  } finally {
    loading.value = false
  }
}

function handleTableChange(pag: any) {
  pagination.current = pag.current
  pagination.pageSize = pag.pageSize
  fetchData()
}

function goToEdit(record: TestTask) {
  router.push(`/test-cases/tasks/${record.id}/edit`)
}

async function goToReview(record: TestTask) {
  try {
    const res = await getReviewList({
      task_id: record.id,
      page: 1,
      page_size: 1,
    })
    const review = res.data.data?.list?.[0]
    if (!review?.id) {
      message.warning(t('review.list.messages.notFound'))
      return
    }
    router.push(`/reviews/${review.id}`)
  } catch {
    message.error(t('common.operationFailed'))
  }
}

function goToExecutions(record: TestTask) {
  router.push({
    path: '/executions',
    query: { task_id: String(record.id) },
  })
}

const regeneratingId = ref<number | null>(null)

async function handleRegenerate(record: TestTask) {
  regeneratingId.value = record.id
  try {
    await createTestTask({ issue_id: record.issue_id })
    message.success(t('testCase.list.messages.regenerateSuccess'))
    fetchData()
  } catch {
    message.error(t('common.operationFailed'))
  } finally {
    regeneratingId.value = null
  }
}

function taskStatusColor(status: string) {
  const map: Record<string, string> = {
    pending: 'default',
    running: 'processing',
    completed: 'success',
    failed: 'error',
  }
  return map[status] || 'default'
}
</script>
