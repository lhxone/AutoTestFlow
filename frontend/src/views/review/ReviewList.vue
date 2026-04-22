<template>
  <div>
    <a-page-header v-if="!embedded" :title="t('review.list.title')" />

    <a-row :gutter="16" style="margin-bottom: 16px">
      <a-col :span="4">
        <a-select v-model:value="query.status" :placeholder="t('review.list.statusPlaceholder')" allowClear style="width: 100%">
          <a-select-option v-for="(v, k) in reviewStatusMap" :key="k" :value="k">{{ v.label }}</a-select-option>
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
          <a-tag :color="reviewStatusMap[record.status]?.color || 'default'">
            {{ reviewStatusMap[record.status]?.label || record.status }}
          </a-tag>
        </template>
        <template v-if="column.key === 'reviewer'">
          {{ record.reviewer?.real_name || t('common.unassigned') }}
        </template>
        <template v-if="column.key === 'action'">
          <a-button type="link" size="small" @click="$router.push(`/test-cases/tasks/${record.test_task_id}/edit?review_id=${record.id}`)">
            {{ record.status === 'pending' ? t('review.list.goReview') : t('review.list.view') }}
          </a-button>
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { getReviewList } from '@/api/review'
import { getReviewStatusMap } from '@/types'
import type { ReviewTask } from '@/types'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const route = useRoute()
withDefaults(defineProps<{ embedded?: boolean }>(), {
  embedded: false,
})
const list = ref<ReviewTask[]>([])
const loading = ref(false)
const query = reactive({ status: undefined as string | undefined })
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })
const reviewStatusMap = computed(() => getReviewStatusMap(t))

const columns = computed(() => [
  { title: t('common.id'), dataIndex: 'id', key: 'id', width: 60 },
  { title: t('review.list.columns.title'), dataIndex: 'title', key: 'title', ellipsis: true },
  { title: t('review.list.columns.issue'), key: 'issue', ellipsis: true },
  { title: t('review.list.columns.status'), key: 'status', width: 100 },
  { title: t('review.list.columns.reviewer'), key: 'reviewer', width: 100 },
  { title: t('review.list.columns.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 170 },
  { title: t('review.list.columns.action'), key: 'action', width: 100 },
])

onMounted(fetchData)
watch(() => route.query.task_id, () => {
  pagination.current = 1
  fetchData()
})

async function fetchData() {
  loading.value = true
  try {
    const params = {
      ...query,
      task_id: route.query.task_id ? Number(route.query.task_id) : undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    }
    const res = await getReviewList(params)
    const data = res.data.data
    list.value = data.list || []
    pagination.total = data.total
  } finally {
    loading.value = false
  }
}

function handleTableChange(pag: any) {
  pagination.current = pag.current
  pagination.pageSize = pag.pageSize
  fetchData()
}
</script>
