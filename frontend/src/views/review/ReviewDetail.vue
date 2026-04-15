<template>
  <div>
    <a-page-header :title="detail?.title || t('review.detail.title')" @back="$router.back()">
      <template #extra>
        <a-tag :color="reviewStatusMap[detail?.status || '']?.color">
          {{ reviewStatusMap[detail?.status || '']?.label || detail?.status }}
        </a-tag>
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <a-card :title="t('review.detail.issue')" size="small" style="margin-bottom: 16px">
        <p>{{ detail?.issue_title || '-' }}</p>
      </a-card>

      <a-card :title="t('review.detail.testCases')" size="small" style="margin-bottom: 16px">
        <a-collapse v-if="detail?.test_cases?.length">
          <a-collapse-panel v-for="tc in detail.test_cases" :key="tc.id"
                            :header="`[${translateTestCaseCategory(t, tc.category)}] ${tc.title}`">
            <a-descriptions :column="1" size="small" bordered>
              <a-descriptions-item :label="t('review.detail.precondition')">{{ tc.precondition }}</a-descriptions-item>
              <a-descriptions-item :label="t('review.detail.steps')">
                <a-table
                  :dataSource="buildStepRows(tc.steps, tc.expected)"
                  :columns="stepColumns"
                  :pagination="false"
                  size="small"
                  rowKey="key"
                />
              </a-descriptions-item>
              <a-descriptions-item :label="t('review.detail.selfTestResult')">
                <a-tag :color="tc.self_test_result === 'pass' ? 'green' : 'red'">{{ translateSelfTestResult(t, tc.self_test_result) }}</a-tag>
              </a-descriptions-item>
              <a-descriptions-item :label="t('review.detail.source')">
                <a-tag :color="tc.source === 'ai' ? 'blue' : 'orange'">{{ translateCaseSource(t, tc.source) }}</a-tag>
              </a-descriptions-item>
            </a-descriptions>
          </a-collapse-panel>
        </a-collapse>
        <a-empty v-else :description="t('review.detail.noTestCases')" />
      </a-card>

      <a-card :title="t('review.detail.testScripts')" size="small" style="margin-bottom: 16px">
        <div v-for="ts in detail?.test_scripts" :key="ts.id" style="margin-bottom: 12px">
          <p><strong>{{ ts.file_path }}</strong> ({{ ts.language }})</p>
          <CodeEditor
            :modelValue="ts.file_content"
            :language="scriptViewerLanguage(ts)"
            :height="320"
            readOnly
          />
        </div>
        <a-empty v-if="!detail?.test_scripts?.length" :description="t('review.detail.noTestScripts')" />
      </a-card>

      <a-card :title="t('review.detail.testDocs')" size="small" style="margin-bottom: 16px">
        <div v-for="doc in detail?.test_docs" :key="doc.id">
          <h4>{{ doc.title }}</h4>
          <pre style="white-space: pre-wrap; background: #f5f5f5; padding: 12px; border-radius: 4px">{{ doc.content }}</pre>
        </div>
        <a-empty v-if="!detail?.test_docs?.length" :description="t('review.detail.noTestDocs')" />
      </a-card>

      <a-card :title="t('review.detail.records')" size="small" style="margin-bottom: 16px">
        <a-timeline v-if="detail?.records?.length">
          <a-timeline-item v-for="r in detail.records" :key="r.id"
                           :color="r.action === 'approve' ? 'green' : r.action === 'reject' ? 'red' : 'blue'">
            <strong>{{ r.reviewer_name }}</strong> {{ actionLabel(r.action) }}
            <span style="color: #999; margin-left: 8px">{{ r.created_at }}</span>
            <p v-if="r.comment" style="margin: 4px 0 0; color: #666">{{ r.comment }}</p>
          </a-timeline-item>
        </a-timeline>
        <a-empty v-else :description="t('review.detail.noRecords')" />
      </a-card>

      <a-card :title="t('review.detail.execute')" size="small" v-if="detail?.status === 'pending' || detail?.status === 'changes_requested'">
        <a-form layout="vertical">
          <a-form-item :label="t('review.detail.comment')">
            <a-textarea v-model:value="reviewComment" :rows="3" :placeholder="t('review.detail.commentPlaceholder')" />
          </a-form-item>
          <a-form-item>
            <a-space>
              <a-button type="primary" @click="handleReview('approve')" :loading="submitting">{{ t('review.detail.approve') }}</a-button>
              <a-button danger @click="handleReview('reject')" :loading="submitting">{{ t('review.detail.reject') }}</a-button>
              <a-button @click="handleReview('request_changes')" :loading="submitting">{{ t('review.detail.requestChanges') }}</a-button>
              <a-button @click="handleReview('comment')" :loading="submitting">{{ t('review.detail.onlyComment') }}</a-button>
            </a-space>
          </a-form-item>
        </a-form>
      </a-card>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import CodeEditor from '@/components/CodeEditor.vue'
import { getReviewDetail, doReview } from '@/api/review'
import { getReviewStatusMap, translateCaseSource, translateSelfTestResult, translateTestCaseCategory } from '@/types'
import type { ReviewDetail } from '@/types'
import { useI18n } from 'vue-i18n'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const detail = ref<ReviewDetail | null>(null)
const loading = ref(false)
const submitting = ref(false)
const reviewComment = ref('')

const reviewId = Number(route.params.id)
const reviewStatusMap = computed(() => getReviewStatusMap(t))
const stepColumns = computed(() => [
  { title: t('review.detail.stepColumns.step'), dataIndex: 'step', key: 'step' },
  { title: t('review.detail.stepColumns.expected'), dataIndex: 'expected', key: 'expected' },
])

onMounted(fetchDetail)

async function fetchDetail() {
  loading.value = true
  try {
    const res = await getReviewDetail(reviewId)
    detail.value = res.data.data
  } finally {
    loading.value = false
  }
}

async function handleReview(action: string) {
  submitting.value = true
  try {
    await doReview(reviewId, { action, comment: reviewComment.value })
    message.success(action === 'approve' ? t('review.detail.approveSuccess') : t('review.detail.actionSuccess'))
    reviewComment.value = ''
    fetchDetail()
  } finally {
    submitting.value = false
  }
}

function actionLabel(action: string) {
  const map: Record<string, string> = {
    approve: t('review.detail.actions.approve'),
    reject: t('review.detail.actions.reject'),
    request_changes: t('review.detail.actions.requestChanges'),
    comment: t('review.detail.actions.comment'),
  }
  return map[action] || action
}

function buildStepRows(steps: string, expected: string) {
  const stepLines = normalizeLines(steps)
  const expectedLines = normalizeLines(expected)
  const maxLength = Math.max(stepLines.length, expectedLines.length, 1)
  return Array.from({ length: maxLength }, (_, index) => ({
    key: `${index}-${stepLines[index] || ''}-${expectedLines[index] || ''}`,
    step: stepLines[index] || '-',
    expected: expectedLines[index] || '-',
  }))
}

function normalizeLines(value: string) {
  return (value || '')
    .split('\n')
    .map((item) => item.trim())
    .filter((item) => item.length > 0)
}

function scriptViewerLanguage(script: ReviewDetail['test_scripts'][number]) {
  const filePath = script.file_path.toLowerCase()
  if (filePath.endsWith('.spec.ts') || filePath.endsWith('.ts')) {
    return 'typescript'
  }
  if (filePath.endsWith('.js')) {
    return 'javascript'
  }
  if (filePath.endsWith('.py')) {
    return 'python'
  }
  return script.language || 'typescript'
}
</script>
