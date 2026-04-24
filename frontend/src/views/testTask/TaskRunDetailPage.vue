<template>
  <div class="task-run-page">
    <a-page-header :title="`${t('taskRun.drawerTitle')} #${taskId}`" @back="handleBack" />

    <a-skeleton v-if="initializing" active :paragraph="{ rows: 8 }" />

    <template v-else>
      <div class="task-run-page__status-row">
        <a-tag :color="connTagColor">{{ connectionStatusText }}</a-tag>
        <template v-if="taskInfo">
          <a-tag :color="taskStatusColor(taskInfo.status)">{{ translateTaskStatus(t, taskInfo.status) }}</a-tag>
          <span v-if="taskInfo.error_message" class="task-run-page__error">{{ taskInfo.error_message }}</span>
        </template>
      </div>

      <EinoWorkflowPanel :taskId="taskId" :taskEvents="taskEvents" :taskInfo="taskInfo" />

      <template v-if="resultsVisible">
        <a-divider style="margin: 16px 0 8px" />

        <!-- 回归用例 -->
        <a-collapse :defaultActiveKey="activeCaseKeys" style="margin-bottom: 16px">
          <a-collapse-panel
            v-for="editableCase in editableCases"
            :key="editableCase.id"
            :header="`[${translateTestCaseCategory(t, editableCase.category)}] ${editableCase.title}`"
          >
            <a-descriptions :column="1" size="small" bordered style="margin-bottom: 12px">
              <a-descriptions-item :label="t('testCase.detail.precondition')">{{ editableCase.precondition || '-' }}</a-descriptions-item>
              <a-descriptions-item :label="t('testCase.detail.steps')">
                <a-table
                  :dataSource="editableCase.stepRows"
                  :columns="stepColumns"
                  :pagination="false"
                  size="small"
                  rowKey="key"
                >
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
              </a-descriptions-item>
              <a-descriptions-item :label="t('review.detail.selfTestResult')">
                <a-tag :color="editableCase.self_test_result === 'pass' ? 'green' : editableCase.self_test_result === 'fail' ? 'red' : 'default'">
                  {{ translateSelfTestResult(t, editableCase.self_test_result) }}
                </a-tag>
              </a-descriptions-item>
              <a-descriptions-item :label="t('review.detail.source')">
                <a-tag :color="editableCase.source === 'ai' ? 'blue' : 'orange'">
                  {{ translateCaseSource(t, editableCase.source) }}
                </a-tag>
              </a-descriptions-item>
            </a-descriptions>

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
          </a-collapse-panel>
        </a-collapse>
        <a-empty v-if="!editableCases.length" :description="t('review.detail.noTestCases')" style="margin-bottom: 16px" />

        <!-- 测试脚本 -->
        <a-collapse :defaultActiveKey="['scripts']" style="margin-bottom: 16px">
          <a-collapse-panel key="scripts" :header="t('taskRun.tabs.testScripts')">
            <template v-if="editableScripts.length">
              <a-form v-for="editableScript in editableScripts" :key="editableScript.id" layout="vertical" style="margin-bottom: 16px">
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
            </template>
            <a-empty v-else :description="t('review.detail.noTestScripts')" />
          </a-collapse-panel>
        </a-collapse>

        <!-- Playwright 报告 -->
        <a-collapse :defaultActiveKey="['playwright']" style="margin-bottom: 16px">
          <a-collapse-panel key="playwright" :header="t('taskRun.selfTest.playwright')">
            <div v-if="playwrightReportUrl" class="report-container">
              <iframe
                :key="playwrightReportFrameKey"
                class="playwright-report-frame"
                :src="playwrightReportUrl"
                sandbox="allow-scripts allow-same-origin allow-modals allow-popups allow-forms allow-downloads"
                @load="playwrightReportLoading = false"
              />
            </div>
            <div v-else-if="playwrightReportLoading" class="report-loading">
              <a-spin :tip="t('common.loading')" />
            </div>
            <a-empty v-else :description="t('taskRun.selfTest.noReportContent')" />
          </a-collapse-panel>
        </a-collapse>

        <!-- 回归审批 -->
        <a-card :title="t('taskRun.regression.title')" size="small" v-if="canReviewList && reviewInfo">
          <a-descriptions :column="1" bordered size="small" style="margin-bottom: 12px">
            <a-descriptions-item :label="t('taskRun.regression.reviewStatus')">
              <a-tag :color="reviewStatusMap[reviewInfo.status]?.color || 'default'">
                {{ reviewStatusMap[reviewInfo.status]?.label || reviewInfo.status }}
              </a-tag>
            </a-descriptions-item>
            <a-descriptions-item :label="t('taskRun.regression.reviewComment')">
              {{ reviewInfo.review_note || '-' }}
            </a-descriptions-item>
          </a-descriptions>

          <a-timeline v-if="reviewDetail?.records?.length" style="margin-bottom: 12px">
            <a-timeline-item
              v-for="record in reviewDetail.records"
              :key="record.id"
              :color="record.action === 'approve' || record.action === 'fail_regression' ? 'green' : record.action === 'reject' ? 'red' : 'blue'"
            >
              <strong>{{ record.reviewer_name }}</strong> {{ regressionActionLabel(record.action) }}
              <span style="color: #999; margin-left: 8px">{{ record.created_at }}</span>
              <p v-if="record.comment" style="margin: 4px 0 0; color: #666">{{ record.comment }}</p>
            </a-timeline-item>
          </a-timeline>
          <a-empty v-else :description="t('taskRun.regression.noRecords')" style="margin-bottom: 12px" />

          <a-form layout="vertical" v-if="canReviewApprove && (reviewInfo.status === 'pending' || reviewInfo.status === 'changes_requested')">
            <a-form-item :label="t('taskRun.regression.comment')">
              <a-textarea v-model:value="regressionComment" :rows="3" :placeholder="t('taskRun.regression.commentPlaceholder')" />
            </a-form-item>
            <a-space>
              <a-button type="primary" @click="submitRegression('approve')" :loading="regressionSubmitting">{{ t('taskRun.regression.confirmSuccess') }}</a-button>
              <a-button danger @click="submitRegression('fail_regression')" :loading="regressionSubmitting">{{ t('taskRun.regression.failRegression') }}</a-button>
              <a-button @click="submitRegression('request_changes')" :loading="regressionSubmitting">{{ t('taskRun.regression.reject') }}</a-button>
            </a-space>
          </a-form>
        </a-card>
      </template>
    </template>

    <a-modal
      v-model:open="scriptModal"
      :title="viewingScript?.file_path || t('taskRun.scriptPreview')"
      width="800px"
      :footer="null"
    >
      <pre class="script-preview">{{ viewingScript?.file_content }}</pre>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import CodeEditor from '@/components/CodeEditor.vue'
import {
  createTestTaskEventSource,
  getSelfTestReport,
  getTaskLogs,
  getTestCases,
  getTestScripts,
  getTestTaskById,
  getWorkspaceFileUrl,
  updateTestCase,
  updateTestScript,
} from '@/api/testTask'
import { doReview, getReviewDetail, getReviewList } from '@/api/review'
import type { ReviewDetail, ReviewTask, SelfTestFrameworkReport, SelfTestReport, TestCaseVO, TestScriptVO, TestTask, TestTaskEvent } from '@/types'
import { translateSelfTestResult, translateTaskStatus, translateTestCaseCategory, translateCaseSource, getReviewStatusMap } from '@/types'
import EinoWorkflowPanel from '@/components/EinoWorkflowPanel.vue'
import { useUserStore } from '@/stores/user'

const EVENT_LOG_LIMIT = 120
const TASK_REFRESH_DEBOUNCE_MS = 300

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
const router = useRouter()
const userStore = useUserStore()
const { t } = useI18n()

const taskId = computed(() => Number(route.params.id))

function handleBack() {
  router.push({ name: 'IssueList' })
}

const initializing = ref(true)
const taskEvents = ref<TestTaskEvent[]>([])
const connStatus = ref<'idle' | 'connecting' | 'connected' | 'closed' | 'error'>('idle')
const taskInfo = ref<TestTask | null>(null)
const editableCases = ref<EditableCase[]>([])
const editableScripts = ref<EditableScript[]>([])
const activeCaseKeys = ref<(number | string)[]>([])
const resultsVisible = ref(false)
const scriptModal = ref(false)
const viewingScript = ref<TestScriptVO | null>(null)
const playwrightReportUrl = ref('')
const playwrightReportLoading = ref(false)
const playwrightReportFrameKey = ref(0)
const playwrightReport = computed<SelfTestFrameworkReport | null>(() => getFrameworkReport())
const selfTestReport = computed<SelfTestReport | null>(() => {
  const report = taskInfo.value?.ai_output?.self_test
  if (!report || typeof report !== 'object') return null
  return report
})
const selfTestChecks = computed<string[]>(() => {
  if (!selfTestReport.value?.checks || !Array.isArray(selfTestReport.value.checks)) {
    return []
  }
  return selfTestReport.value.checks
})

// Review / Regression
const reviewInfo = ref<ReviewTask | null>(null)
const reviewDetail = ref<ReviewDetail | null>(null)
const regressionComment = ref('')
const regressionSubmitting = ref(false)
const reviewStatusMap = computed(() => getReviewStatusMap(t))
const canIntervene = computed(() => userStore.hasPermission('test:intervene'))
const canReviewList = computed(() => userStore.hasPermission('review:list'))
const canReviewApprove = computed(() => userStore.hasPermission('review:approve'))

const savingCaseId = ref<number | null>(null)
const savingScriptId = ref<number | null>(null)

let eventSource: EventSource | null = null
let taskRefreshTimer: number | null = null

const connectionStatusText = computed(() => {
  const map: Record<string, string> = {
    idle: t('taskRun.connStatus.idle'),
    connecting: t('taskRun.connStatus.connecting'),
    connected: t('taskRun.connStatus.connected'),
    closed: t('taskRun.connStatus.closed'),
    error: t('taskRun.connStatus.error'),
  }
  return map[connStatus.value] || map.idle
})

const connTagColor = computed(() => {
  const map: Record<string, string> = {
    idle: 'default', connecting: 'processing', connected: 'success', closed: 'warning', error: 'error',
  }
  return map[connStatus.value] || 'default'
})

const stepColumns = computed(() => [
  { title: t('testCase.detail.columns.step'), key: 'step' },
  { title: t('testCase.detail.columns.expected'), key: 'expected' },
  { title: t('common.action'), key: 'action', width: 90 },
])

onMounted(() => {
  if (taskId.value > 0) {
    initTask(taskId.value)
  }
})

watch(taskId, (id, prev) => {
  if (!id || id === prev) return
  resetState()
  initTask(id)
})

onBeforeUnmount(closeEventSource)

function resetState() {
  initializing.value = true
  taskEvents.value = []
  connStatus.value = 'connecting'
  taskInfo.value = null
  editableCases.value = []
  editableScripts.value = []
  resultsVisible.value = false
  closeEventSource()
}

async function initTask(id: number) {
  connStatus.value = 'connecting'
  try {
    await refreshTask(id)
    await Promise.all([
      loadHistoryLogs(id),
      loadResults(id),
    ])
    const isDone = taskInfo.value?.status === 'completed' || taskInfo.value?.status === 'failed'
    if (isDone) {
      connStatus.value = 'closed'
    } else {
      openEventSource(id)
    }
  } finally {
    initializing.value = false
  }
}

async function loadHistoryLogs(id: number) {
  try {
    const res = await getTaskLogs(id)
    const events: TestTaskEvent[] = res.data.data || []
    taskEvents.value = compactTaskEvents(events)
  } catch {
    // ignore
  }
}

function openEventSource(id: number) {
  closeEventSource()
  eventSource = createTestTaskEventSource(id)
  eventSource.addEventListener('task-event', async (rawEvent) => {
    connStatus.value = 'connected'
    const payload = JSON.parse((rawEvent as MessageEvent<string>).data) as TestTaskEvent
    appendLog(payload)
    if (payload.type !== 'log') {
      if (shouldRefreshTaskImmediately(payload)) {
        await flushTaskRefresh(id)
      } else {
        scheduleTaskRefresh(id)
      }
    }
    if (payload.stage === 'artifacts_synced' || payload.stage === 'review_pending' || payload.stage === 'runtime_completed') {
      await loadResults(id)
    }
    if (payload.status === 'failed' || payload.stage === 'review_pending') {
      closeEventSource()
    }
  })
  eventSource.onerror = () => {
    connStatus.value = 'error'
  }
}

function appendLog(event: TestTaskEvent) {
  taskEvents.value = compactTaskEvents([...taskEvents.value, event])
}

function compactTaskEvents(events: TestTaskEvent[]) {
  const dedupedEvents = dedupeTaskEvents(events)
  const retainedLogEvents = dedupedEvents.filter((event) => !shouldAlwaysKeepTaskEvent(event)).slice(-EVENT_LOG_LIMIT)
  const retainedLogIds = new Set(retainedLogEvents.map((event) => event.id))
  return dedupedEvents.filter((event) => shouldAlwaysKeepTaskEvent(event) || retainedLogIds.has(event.id))
}

function dedupeTaskEvents(events: TestTaskEvent[]) {
  const seen = new Set<string>()
  const deduped: TestTaskEvent[] = []

  for (const event of events) {
    const signature = buildTaskEventSignature(event)
    if (seen.has(signature)) {
      continue
    }
    seen.add(signature)
    deduped.push(event)
  }

  return deduped
}

function buildTaskEventSignature(event: TestTaskEvent) {
  if (shouldAlwaysKeepTaskEvent(event)) {
    return [
      event.type,
      event.stage || '',
      event.status || '',
      event.message || '',
      JSON.stringify(event.data || {}),
    ].join('::')
  }

  return [
    event.id,
    event.type,
    event.stage || '',
    event.status || '',
    event.timestamp || '',
    event.message || '',
    JSON.stringify(event.data || {}),
  ].join('::')
}

function shouldAlwaysKeepTaskEvent(event: TestTaskEvent) {
  return event.type !== 'log' || Boolean(event.stage) || Boolean(event.status)
}

function scheduleTaskRefresh(id: number) {
  if (taskRefreshTimer !== null) return
  taskRefreshTimer = window.setTimeout(async () => {
    taskRefreshTimer = null
    await refreshTask(id)
  }, TASK_REFRESH_DEBOUNCE_MS)
}

async function flushTaskRefresh(id: number) {
  if (taskRefreshTimer !== null) {
    window.clearTimeout(taskRefreshTimer)
    taskRefreshTimer = null
  }
  await refreshTask(id)
}

function shouldRefreshTaskImmediately(event: TestTaskEvent) {
  return event.status === 'failed' || event.stage === 'review_pending' || event.stage === 'runtime_completed'
}

async function refreshTask(id: number) {
  try {
    const res = await getTestTaskById(id)
    taskInfo.value = res.data.data || taskInfo.value
  } catch {
    // ignore
  }
}

async function loadResults(id: number) {
  try {
    const [caseRes, scriptRes] = await Promise.all([
      getTestCases(id),
      getTestScripts(id),
    ])
    editableCases.value = (caseRes.data.data || []).map((item: TestCaseVO) => ({
      ...item,
      stepRows: parseStepRows(item.steps, item.expected),
      changeNote: '',
    }))
    editableScripts.value = (scriptRes.data.data || []).map((item: TestScriptVO) => ({
      ...item,
      changeNote: '',
    }))
    activeCaseKeys.value = editableCases.value.map(c => c.id)
    resultsVisible.value = true
    await loadPlaywrightHtml(id)
    await fetchReviewData()
  } catch {
    // ignore
  }
}

async function loadPlaywrightHtml(id: number) {
  playwrightReportUrl.value = ''
  playwrightReportLoading.value = true
  try {
    // 使用静默模式，任务运行中未产出报告是正常情况，不显示错误提示
    const res = await getSelfTestReport(id, 'playwright', true)
    const data = res.data.data || {}
    if (data.content_type?.includes('html') && data.report_path) {
      playwrightReportUrl.value = getWorkspaceFileUrl(id, data.report_path)
      playwrightReportFrameKey.value += 1
    } else {
      playwrightReportLoading.value = false
    }
  } catch {
    // 任务运行中未产出报告是正常情况，静默处理
    playwrightReportLoading.value = false
  }
}

async function fetchReviewData() {
  if (!canReviewList.value) {
    reviewInfo.value = null
    reviewDetail.value = null
    return
  }

  try {
    const reviewRes = await getReviewList({ task_id: taskId.value, page: 1, page_size: 1 })
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

function closeEventSource() {
  if (taskRefreshTimer !== null) {
    window.clearTimeout(taskRefreshTimer)
    taskRefreshTimer = null
  }
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
}

function taskStatusColor(s: string) {
  const map: Record<string, string> = {
    pending: 'default', running: 'processing', completed: 'success', failed: 'error', warning: 'warning',
  }
  return map[s] || 'default'
}

function getFrameworkReport(): SelfTestFrameworkReport | null {
  const report = selfTestReport.value?.['playwright']
  if (!report || typeof report !== 'object') return null
  return report as SelfTestFrameworkReport
}

function getFrameworkChecks(): string[] {
  const report = getFrameworkReport()
  if (report?.checks && Array.isArray(report.checks) && report.checks.length > 0) {
    return report.checks
  }

  const checks = selfTestChecks.value
  return checks.filter((item) => /playwright/i.test(item))
}

function getFrameworkTagColor(passed: boolean | undefined) {
  if (passed === true) return 'green'
  if (passed === false) return 'red'
  return 'default'
}

function formatFrameworkStatus(passed: boolean | undefined) {
  if (passed === true) return t('taskRun.selfTest.pass')
  if (passed === false) return t('taskRun.selfTest.fail')
  return t('taskRun.selfTest.unknown')
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
  return (value || '')
    .split('\n')
    .map((item) => item.trim())
    .filter((item) => item.length > 0)
}

function appendStepRow(caseId: number) {
  const currentCase = editableCases.value.find((item) => item.id === caseId)
  if (!currentCase) return
  currentCase.stepRows.push({
    key: `row-${caseId}-${currentCase.stepRows.length}-${Date.now()}`,
    step: '',
    expected: '',
  })
}

function removeStepRow(caseId: number, rowKey: string) {
  const currentCase = editableCases.value.find((item) => item.id === caseId)
  if (!currentCase) return
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

function regressionActionLabel(action: string) {
  const map: Record<string, string> = {
    approve: t('taskRun.regression.actions.approve'),
    fail_regression: t('taskRun.regression.actions.fail_regression'),
    request_changes: t('taskRun.regression.actions.request_changes'),
    comment: t('taskRun.regression.actions.comment'),
  }
  return map[action] || action
}

async function submitRegression(action: string) {
  if (!reviewInfo.value) return
  regressionSubmitting.value = true
  try {
    await doReview(reviewInfo.value.id, { action, comment: regressionComment.value })
    const msgMap: Record<string, string> = {
      approve: t('taskRun.regression.approveSuccess'),
      fail_regression: t('taskRun.regression.failSuccess'),
      request_changes: t('taskRun.regression.rejectSuccess'),
    }
    message.success(msgMap[action] || t('taskRun.regression.actionSuccess'))
    regressionComment.value = ''
    await fetchReviewData()
    await fetchDetail()
  } catch {
    message.error(t('taskRun.regression.actionFailed'))
  } finally {
    regressionSubmitting.value = false
  }
}

async function fetchDetail() {
  try {
    const [caseRes, scriptRes] = await Promise.allSettled([
      getTestCases(taskId.value),
      getTestScripts(taskId.value),
    ])
    if (caseRes.status === 'fulfilled') {
      editableCases.value = (caseRes.value.data.data || []).map((item: TestCaseVO) => ({
        ...item,
        stepRows: parseStepRows(item.steps, item.expected),
        changeNote: '',
      }))
    }
    if (scriptRes.status === 'fulfilled') {
      editableScripts.value = (scriptRes.value.data.data || []).map((item: TestScriptVO) => ({
        ...item,
        changeNote: '',
      }))
    }
    await fetchReviewData()
  } catch {
    // ignore
  }
}
</script>

<style scoped>
.task-run-page {
  padding: 0 12px 12px;
}

.task-run-page__status-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  flex-wrap: wrap;
}

.task-run-page__error {
  color: #ff4d4f;
  font-size: 12px;
}

.script-preview {
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 16px;
  border-radius: 6px;
  font-family: Consolas, monospace;
  font-size: 12px;
  max-height: 60vh;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

.playwright-report-frame {
  width: 100%;
  height: 70vh;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  background: #fff;
}

.report-container {
  width: 100%;
}

.report-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 180px;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  background: #fff;
}
</style>
