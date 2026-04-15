<template>
  <div>
    <a-page-header :title="pageTitle" @back="$router.back()">
      <template #extra>
        <a-tag :color="taskStatusColor(task?.status || '')">
          {{ translateTaskStatus(t, task?.status || '') || '-' }}
        </a-tag>
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <a-card :title="t('testCase.detail.issueInfo')" size="small" style="margin-bottom: 16px">
        <a-descriptions :column="1" bordered size="small">
          <a-descriptions-item :label="t('testCase.detail.taskId')">{{ task?.id || '-' }}</a-descriptions-item>
          <a-descriptions-item :label="t('testCase.detail.issueTitle')">{{ task?.issue?.title || '-' }}</a-descriptions-item>
          <a-descriptions-item :label="t('testCase.detail.issueId')">{{ task?.issue_id || '-' }}</a-descriptions-item>
          <a-descriptions-item :label="t('common.status')">
            <a-tag :color="taskStatusColor(task?.status || '')">
              {{ translateTaskStatus(t, task?.status || '') || '-' }}
            </a-tag>
          </a-descriptions-item>
        </a-descriptions>
      </a-card>

      <a-card :title="t('testCase.detail.selfTestReportTitle')" size="small" style="margin-bottom: 16px">
        <a-alert
          v-if="selfTestReport"
          :type="selfTestReport.passed === false ? 'error' : 'success'"
          :message="selfTestReport.summary || t('testCase.detail.selfTestNoSummary')"
          show-icon
          style="margin-bottom: 12px"
        />
        <a-alert
          v-else
          type="info"
          :message="t('testCase.detail.selfTestNoReport')"
          show-icon
          style="margin-bottom: 12px"
        />

        <a-descriptions :column="1" size="small" bordered style="margin-bottom: 12px">
          <a-descriptions-item :label="t('testCase.detail.selfTestChecks')">
            <ul v-if="selfTestChecks.length" style="margin: 0; padding-left: 18px">
              <li v-for="(item, index) in selfTestChecks" :key="`check-${index}`">{{ item }}</li>
            </ul>
            <span v-else>-</span>
          </a-descriptions-item>
        </a-descriptions>

        <a-row :gutter="12">
          <a-col :span="12">
            <a-card size="small" :title="t('testCase.detail.playwrightReport')">
              <template v-if="playwrightReport">
                <a-tag :color="frameworkTagColor(playwrightReport.passed)">{{ frameworkStatusLabel(playwrightReport.passed) }}</a-tag>
                <div v-if="playwrightReport.report_path" style="color:#8c8c8c; font-size:12px; margin-top: 6px; word-break: break-all">
                  {{ playwrightReport.report_path }}
                </div>
                <p v-if="playwrightReport.summary" style="margin: 8px 0">{{ playwrightReport.summary }}</p>
                <ul v-if="frameworkChecks('playwright').length" style="margin: 0; padding-left: 18px">
                  <li v-for="(item, index) in frameworkChecks('playwright')" :key="`pw-${index}`">{{ item }}</li>
                </ul>
              </template>
              <a-empty v-else :description="t('testCase.detail.selfTestNoFrameworkReport')" />
            </a-card>
          </a-col>
          <a-col :span="12">
            <a-card size="small" :title="t('testCase.detail.midsceneReport')">
              <template v-if="midsceneReport">
                <a-tag :color="frameworkTagColor(midsceneReport.passed)">{{ frameworkStatusLabel(midsceneReport.passed) }}</a-tag>
                <div v-if="midsceneReport.report_path" style="color:#8c8c8c; font-size:12px; margin-top: 6px; word-break: break-all">
                  {{ midsceneReport.report_path }}
                </div>
                <p v-if="midsceneReport.summary" style="margin: 8px 0">{{ midsceneReport.summary }}</p>
                <ul v-if="frameworkChecks('midscene').length" style="margin: 0; padding-left: 18px">
                  <li v-for="(item, index) in frameworkChecks('midscene')" :key="`mid-${index}`">{{ item }}</li>
                </ul>
              </template>
              <a-empty v-else :description="t('testCase.detail.selfTestNoFrameworkReport')" />
            </a-card>
          </a-col>
        </a-row>
      </a-card>

      <a-card :title="t('testCase.detail.caseGroupTitle')" size="small">
        <a-collapse v-if="editableCases.length" v-model:activeKey="activeCaseKeys">
          <a-collapse-panel
            v-for="editableCase in editableCases"
            :key="editableCase.id"
            :header="`[${translateTestCaseCategory(t, editableCase.category)}] ${editableCase.title}`"
          >
            <a-form layout="vertical">
              <a-row :gutter="16">
                <a-col :span="24">
                  <a-form-item :label="t('testCase.detail.caseTitle')">
                    <a-input v-model:value="editableCase.title" :disabled="!canIntervene" />
                  </a-form-item>
                </a-col>
              </a-row>

              <a-row :gutter="16">
                <a-col :span="16">
                  <a-form-item :label="t('testCase.detail.precondition')">
                    <a-textarea v-model:value="editableCase.precondition" :rows="3" :disabled="!canIntervene" />
                  </a-form-item>
                </a-col>
                <a-col :span="8">
                  <a-form-item :label="t('review.detail.selfTestResult')">
                    <a-tag :color="editableCase.self_test_result === 'pass' ? 'green' : editableCase.self_test_result === 'fail' ? 'red' : 'default'">
                      {{ translateSelfTestResult(t, editableCase.self_test_result) }}
                    </a-tag>
                  </a-form-item>
                  <a-form-item :label="t('review.detail.source')">
                    <a-tag :color="editableCase.source === 'ai' ? 'blue' : 'orange'">
                      {{ translateCaseSource(t, editableCase.source) }}
                    </a-tag>
                  </a-form-item>
                </a-col>
              </a-row>

              <a-form-item :label="t('testCase.detail.stepTableTitle')">
                <a-table :dataSource="editableCase.stepRows" :columns="stepColumns" :pagination="false" size="small" rowKey="key">
                  <template #bodyCell="{ column, record }">
                    <template v-if="column.key === 'step'">
                      <a-textarea v-model:value="record.step" :rows="2" :disabled="!canIntervene" />
                    </template>
                    <template v-if="column.key === 'expected'">
                      <a-textarea v-model:value="record.expected" :rows="2" :disabled="!canIntervene" />
                    </template>
                    <template v-if="column.key === 'action'">
                      <a-button type="link" danger size="small" @click="removeStepRow(editableCase.id, record.key)" :disabled="!canIntervene">
                        {{ t('common.delete') }}
                      </a-button>
                    </template>
                  </template>
                </a-table>
                <a-button type="dashed" block style="margin-top: 12px" @click="appendStepRow(editableCase.id)" :disabled="!canIntervene">
                  {{ t('testCase.detail.addStep') }}
                </a-button>
              </a-form-item>

              <a-form-item :label="t('testCase.detail.changeNote')" required v-if="canIntervene">
                <a-input
                  v-model:value="editableCase.changeNote"
                  :placeholder="t('testCase.detail.changeNotePlaceholder')"
                />
              </a-form-item>

              <a-space v-if="canIntervene">
                <a-button type="primary" :loading="savingCaseId === editableCase.id" @click="saveCase(editableCase)">
                  {{ t('common.save') }}
                </a-button>
              </a-space>
            </a-form>
          </a-collapse-panel>
        </a-collapse>
        <a-empty v-else :description="t('review.detail.noTestCases')" />
      </a-card>

      <a-card :title="t('testCase.detail.scriptGroupTitle')" size="small" style="margin-top: 16px">
        <a-collapse v-if="editableScripts.length">
          <a-collapse-panel
            v-for="editableScript in editableScripts"
            :key="editableScript.id"
            :header="`${editableScript.file_path} (${editableScript.language})`"
          >
            <a-form layout="vertical">
              <a-form-item :label="t('testCase.detail.scriptContent')">
                <CodeEditor
                  v-model="editableScript.file_content"
                  :language="scriptEditorLanguage(editableScript)"
                  :height="380"
                  :readOnly="!canIntervene"
                />
              </a-form-item>
              <a-form-item :label="t('testCase.detail.changeNote')" required v-if="canIntervene">
                <a-input
                  v-model:value="editableScript.changeNote"
                  :placeholder="t('testCase.detail.changeNotePlaceholder')"
                />
              </a-form-item>
              <a-button type="primary" :loading="savingScriptId === editableScript.id" @click="saveScript(editableScript)" v-if="canIntervene">
                {{ t('common.save') }}
              </a-button>
            </a-form>
          </a-collapse-panel>
        </a-collapse>
        <a-empty v-else :description="t('review.detail.noTestScripts')" />
      </a-card>

      <a-card :title="t('testCase.detail.reviewTitle')" size="small" style="margin-top: 16px" v-if="canReviewList && reviewInfo">
        <a-descriptions :column="1" bordered size="small" style="margin-bottom: 12px">
          <a-descriptions-item :label="t('common.status')">
            <a-tag :color="reviewStatusMap[reviewInfo.status]?.color || 'default'">
              {{ reviewStatusMap[reviewInfo.status]?.label || reviewInfo.status }}
            </a-tag>
          </a-descriptions-item>
          <a-descriptions-item :label="t('review.detail.comment')">
            {{ reviewInfo.review_note || '-' }}
          </a-descriptions-item>
        </a-descriptions>

        <a-timeline v-if="reviewDetail?.records?.length" style="margin-bottom: 12px">
          <a-timeline-item
            v-for="record in reviewDetail.records"
            :key="record.id"
            :color="record.action === 'approve' ? 'green' : record.action === 'reject' ? 'red' : 'blue'"
          >
            <strong>{{ record.reviewer_name }}</strong> {{ reviewActionLabel(record.action) }}
            <span style="color: #999; margin-left: 8px">{{ record.created_at }}</span>
            <p v-if="record.comment" style="margin: 4px 0 0; color: #666">{{ record.comment }}</p>
          </a-timeline-item>
        </a-timeline>

        <a-empty v-else :description="t('review.detail.noRecords')" style="margin-bottom: 12px" />

        <a-form layout="vertical" v-if="canReviewApprove && (reviewInfo.status === 'pending' || reviewInfo.status === 'changes_requested')">
          <a-form-item :label="t('review.detail.comment')">
            <a-textarea v-model:value="reviewComment" :rows="3" :placeholder="t('review.detail.commentPlaceholder')" />
          </a-form-item>
          <a-space>
            <a-button type="primary" @click="submitReview('approve')" :loading="reviewSubmitting">{{ t('review.detail.approve') }}</a-button>
            <a-button danger @click="submitReview('reject')" :loading="reviewSubmitting">{{ t('review.detail.reject') }}</a-button>
            <a-button @click="submitReview('request_changes')" :loading="reviewSubmitting">{{ t('review.detail.requestChanges') }}</a-button>
            <a-button @click="submitReview('comment')" :loading="reviewSubmitting">{{ t('review.detail.onlyComment') }}</a-button>
          </a-space>
        </a-form>
      </a-card>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import CodeEditor from '@/components/CodeEditor.vue'
import { getTestCases, getTestScripts, getTestTaskById, updateTestCase, updateTestScript } from '@/api/testTask'
import { doReview, getReviewDetail, getReviewList } from '@/api/review'
import { useUserStore } from '@/stores/user'
import type { ReviewDetail, ReviewTask, SelfTestFrameworkReport, SelfTestReport, TestCaseVO, TestScriptVO, TestTask } from '@/types'
import { getReviewStatusMap, translateCaseSource, translateSelfTestResult, translateTaskStatus, translateTestCaseCategory } from '@/types'
import { useI18n } from 'vue-i18n'

type StepRow = {
  key: string
  step: string
  expected: string
}

type EditableCase = TestCaseVO & {
  stepRows: StepRow[]
  changeNote: string
}

type EditableScript = TestScriptVO & {
  changeNote: string
}

const route = useRoute()
const { t } = useI18n()
const userStore = useUserStore()

const taskId = Number(route.params.id)
const loading = ref(false)
const savingCaseId = ref<number | null>(null)
const savingScriptId = ref<number | null>(null)
const reviewSubmitting = ref(false)
const task = ref<TestTask | null>(null)
const editableCases = ref<EditableCase[]>([])
const editableScripts = ref<EditableScript[]>([])
const activeCaseKeys = ref<Array<number | string>>([])
const reviewInfo = ref<ReviewTask | null>(null)
const reviewDetail = ref<ReviewDetail | null>(null)
const reviewComment = ref('')
const canIntervene = computed(() => userStore.hasPermission('test:intervene'))
const canReviewList = computed(() => userStore.hasPermission('review:list'))
const canReviewApprove = computed(() => userStore.hasPermission('review:approve'))
const reviewStatusMap = computed(() => getReviewStatusMap(t))
const selfTestReport = computed<SelfTestReport | null>(() => {
  const report = task.value?.ai_output?.self_test
  if (!report || typeof report !== 'object') return null
  return report
})
const selfTestChecks = computed<string[]>(() => {
  if (!selfTestReport.value?.checks || !Array.isArray(selfTestReport.value.checks)) {
    return []
  }
  return selfTestReport.value.checks
})
const playwrightReport = computed<SelfTestFrameworkReport | null>(() => frameworkReport('playwright'))
const midsceneReport = computed<SelfTestFrameworkReport | null>(() => frameworkReport('midscene'))

const pageTitle = computed(() => task.value?.issue?.title || t('testCase.detail.title'))
const stepColumns = computed(() => [
  { title: t('testCase.detail.columns.step'), key: 'step' },
  { title: t('testCase.detail.columns.expected'), key: 'expected' },
  { title: t('common.action'), key: 'action', width: 90 },
])

onMounted(fetchDetail)
watch(
  () => route.query.caseId,
  () => syncActiveCase(),
)

async function fetchDetail() {
  loading.value = true
  try {
    const [taskRes, casesRes, scriptsRes] = await Promise.allSettled([
      getTestTaskById(taskId),
      getTestCases(taskId),
      getTestScripts(taskId),
    ])

    if (taskRes.status !== 'fulfilled' || casesRes.status !== 'fulfilled') {
      throw new Error('load_failed')
    }

    task.value = taskRes.value.data.data
    editableCases.value = (casesRes.value.data.data || []).map((item: TestCaseVO) => ({
      ...item,
      stepRows: parseStepRows(item.steps, item.expected),
      changeNote: '',
    }))
    syncActiveCase()
    if (scriptsRes.status === 'fulfilled') {
      editableScripts.value = (scriptsRes.value.data.data || []).map((item: TestScriptVO) => ({
        ...item,
        changeNote: '',
      }))
    } else {
      editableScripts.value = []
    }
    await fetchReviewData()
  } catch {
    message.error(t('common.requestFailed'))
  } finally {
    loading.value = false
  }
}

async function fetchReviewData() {
  if (!canReviewList.value) {
    reviewInfo.value = null
    reviewDetail.value = null
    return
  }

  try {
    const reviewRes = await getReviewList({ task_id: taskId, page: 1, page_size: 1 })
    const review = reviewRes.data.data?.list?.[0]
    if (!review?.id) {
      reviewInfo.value = null
      reviewDetail.value = null
      return
    }
    reviewInfo.value = review
    const detailRes = await getReviewDetail(review.id)
    reviewDetail.value = detailRes.data.data
  } catch {
    reviewInfo.value = null
    reviewDetail.value = null
  }
}

function syncActiveCase() {
  const rawCaseId = route.query.caseId
  const caseId = Number(Array.isArray(rawCaseId) ? rawCaseId[0] : rawCaseId)
  if (caseId && editableCases.value.some((item) => item.id === caseId)) {
    activeCaseKeys.value = [caseId]
    return
  }
  activeCaseKeys.value = editableCases.value.length ? [editableCases.value[0].id] : []
}

function parseStepRows(steps: string, expected: string) {
  const stepLines = normalizeLines(steps)
  const expectedLines = normalizeLines(expected)
  const maxLength = Math.max(stepLines.length, expectedLines.length, 1)
  const rows: StepRow[] = []
  for (let index = 0; index < maxLength; index += 1) {
    rows.push({
      key: `row-${index}-${Date.now()}`,
      step: stepLines[index] || '',
      expected: expectedLines[index] || '',
    })
  }
  return rows
}

function normalizeLines(value: string) {
  return value
    .split('\n')
    .map((item) => item.trim())
    .filter((item) => item.length > 0)
}

function appendStepRow(caseId: number) {
  const currentCase = editableCases.value.find((item) => item.id === caseId)
  if (!currentCase) {
    return
  }
  currentCase.stepRows.push({
    key: `row-${caseId}-${currentCase.stepRows.length}-${Date.now()}`,
    step: '',
    expected: '',
  })
}

function removeStepRow(caseId: number, rowKey: string) {
  const currentCase = editableCases.value.find((item) => item.id === caseId)
  if (!currentCase) {
    return
  }
  currentCase.stepRows = currentCase.stepRows.filter((item) => item.key !== rowKey)
  if (!currentCase.stepRows.length) {
    appendStepRow(caseId)
  }
}

async function saveCase(editableCase: EditableCase) {
  if (!editableCase.changeNote.trim()) {
    message.warning(t('testCase.detail.changeNoteRequired'))
    return
  }

  const filteredRows = editableCase.stepRows.filter((item) => item.step.trim() || item.expected.trim())
  if (!filteredRows.length) {
    message.warning(t('testCase.detail.stepRequired'))
    return
  }

  savingCaseId.value = editableCase.id
  try {
    await updateTestCase(editableCase.id, {
      title: editableCase.title,
      precondition: editableCase.precondition,
      steps: filteredRows.map((item) => item.step.trim()).join('\n'),
      expected: filteredRows.map((item) => item.expected.trim()).join('\n'),
      change_note: editableCase.changeNote,
    })
    message.success(t('testCase.detail.saveSuccess'))
    editableCase.changeNote = ''
    await fetchDetail()
  } finally {
    savingCaseId.value = null
  }
}

async function saveScript(editableScript: EditableScript) {
  if (!editableScript.changeNote.trim()) {
    message.warning(t('testCase.detail.changeNoteRequired'))
    return
  }

  savingScriptId.value = editableScript.id
  try {
    await updateTestScript(editableScript.id, {
      file_content: editableScript.file_content,
      change_note: editableScript.changeNote,
    })
    message.success(t('testCase.detail.saveSuccess'))
    editableScript.changeNote = ''
    await fetchDetail()
  } finally {
    savingScriptId.value = null
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

function scriptEditorLanguage(script: EditableScript) {
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

function frameworkReport(key: 'playwright' | 'midscene'): SelfTestFrameworkReport | null {
  const report = selfTestReport.value?.[key]
  if (!report || typeof report !== 'object') return null
  return report as SelfTestFrameworkReport
}

function frameworkChecks(key: 'playwright' | 'midscene'): string[] {
  const report = frameworkReport(key)
  if (report?.checks && Array.isArray(report.checks) && report.checks.length > 0) {
    return report.checks
  }

  const allChecks = selfTestChecks.value
  const keyword = key === 'playwright' ? /playwright/i : /midscene/i
  return allChecks.filter((item) => keyword.test(item))
}

function frameworkTagColor(passed: boolean | undefined) {
  if (passed === true) return 'green'
  if (passed === false) return 'red'
  return 'default'
}

function frameworkStatusLabel(passed: boolean | undefined) {
  if (passed === true) return t('testCase.detail.selfTestPass')
  if (passed === false) return t('testCase.detail.selfTestFail')
  return t('testCase.detail.selfTestUnknown')
}

function reviewActionLabel(action: string) {
  const map: Record<string, string> = {
    approve: t('review.detail.actions.approve'),
    reject: t('review.detail.actions.reject'),
    request_changes: t('review.detail.actions.requestChanges'),
    comment: t('review.detail.actions.comment'),
  }
  return map[action] || action
}

async function submitReview(action: string) {
  if (!reviewInfo.value) return
  reviewSubmitting.value = true
  try {
    await doReview(reviewInfo.value.id, { action, comment: reviewComment.value })
    message.success(action === 'approve' ? t('review.detail.approveSuccess') : t('review.detail.actionSuccess'))
    reviewComment.value = ''
    await fetchReviewData()
    await fetchDetail()
  } catch {
    message.error(t('common.operationFailed'))
  } finally {
    reviewSubmitting.value = false
  }
}
</script>
