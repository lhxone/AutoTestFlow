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
        <a-tabs size="small">
          <a-tab-pane key="cases" :tab="t('taskRun.tabs.testCases')">
            <a-table
              :dataSource="testCases"
              :columns="caseColumns"
              rowKey="id"
              size="small"
              :pagination="false"
              :scroll="{ y: 260 }"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'category'">
                  {{ translateTestCaseCategory(t, record.category) }}
                </template>
                <template v-if="column.key === 'self_test_result'">
                  <a-tag :color="record.self_test_result === 'pass' ? 'green' : 'orange'">
                    {{ translateSelfTestResult(t, record.self_test_result) }}
                  </a-tag>
                </template>
                <template v-if="column.key === 'action'">
                  <a-button type="link" size="small" @click="goToEditCase(record)" v-if="canEditCase">{{ t('common.edit') }}</a-button>
                </template>
              </template>
            </a-table>
          </a-tab-pane>
          <a-tab-pane key="scripts" :tab="t('taskRun.tabs.testScripts')">
            <a-table
              :dataSource="testScripts"
              :columns="scriptColumns"
              rowKey="id"
              size="small"
              :pagination="false"
              :scroll="{ y: 260 }"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'content'">
                  <a-button type="link" size="small" @click="viewScript(record)">{{ t('taskRun.viewScript') }}</a-button>
                </template>
              </template>
            </a-table>
          </a-tab-pane>
          <a-tab-pane key="self-test" :tab="t('taskRun.tabs.selfTest')">
            <div class="self-test-report">
              <a-alert
                v-if="selfTestReport"
                :type="selfTestReport.passed === false ? 'error' : 'success'"
                :message="selfTestReport.summary || t('taskRun.selfTest.noSummary')"
                show-icon
              />
              <a-alert
                v-else
                type="info"
                :message="t('taskRun.selfTest.noReport')"
                show-icon
              />

              <a-card size="small" class="self-test-report__card" :title="t('taskRun.selfTest.generalChecks')">
                <ul v-if="selfTestChecks.length" class="self-test-report__list">
                  <li v-for="(item, idx) in selfTestChecks" :key="`general-${idx}`">{{ item }}</li>
                </ul>
                <a-empty v-else :description="t('taskRun.selfTest.noChecks')" />
              </a-card>

              <a-card size="small" class="self-test-report__card" :title="t('taskRun.selfTest.playwright')">
                <template v-if="playwrightReport">
                  <div class="self-test-report__meta">
                    <a-tag :color="getFrameworkTagColor(playwrightReport.passed)">
                      {{ formatFrameworkStatus(playwrightReport.passed) }}
                    </a-tag>
                    <span v-if="playwrightReport.report_path" class="self-test-report__path">
                      {{ playwrightReport.report_path }}
                    </span>
                  </div>
                  <p v-if="playwrightReport.summary" class="self-test-report__summary">{{ playwrightReport.summary }}</p>
                  <ul v-if="getFrameworkChecks().length" class="self-test-report__list">
                    <li v-for="(item, idx) in getFrameworkChecks()" :key="`playwright-${idx}`">{{ item }}</li>
                  </ul>
                </template>
                <a-empty v-if="!playwrightReport && !playwrightReportHtml" :description="t('taskRun.selfTest.noFrameworkReport')" />
                <iframe v-if="playwrightReportHtml" class="report-frame" :srcdoc="playwrightReportHtml" sandbox="allow-scripts allow-same-origin allow-modals allow-popups allow-forms allow-downloads" />
              </a-card>
            </div>
          </a-tab-pane>
        </a-tabs>
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
import {
  createTestTaskEventSource,
  getSelfTestReport,
  getTaskLogs,
  getTestCases,
  getTestScripts,
  getTestTaskById,
} from '@/api/testTask'
import type { SelfTestFrameworkReport, SelfTestReport, TestCaseVO, TestScriptVO, TestTask, TestTaskEvent } from '@/types'
import { translateSelfTestResult, translateTaskStatus, translateTestCaseCategory } from '@/types'
import EinoWorkflowPanel from '@/components/EinoWorkflowPanel.vue'
import { useUserStore } from '@/stores/user'

const EVENT_LOG_LIMIT = 120
const TASK_REFRESH_DEBOUNCE_MS = 300

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
const testCases = ref<TestCaseVO[]>([])
const testScripts = ref<TestScriptVO[]>([])
const resultsVisible = ref(false)
const scriptModal = ref(false)
const viewingScript = ref<TestScriptVO | null>(null)
const playwrightReportHtml = ref('')
const canEditCase = computed(() => userStore.hasPermission('test:intervene'))
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
const playwrightReport = computed<SelfTestFrameworkReport | null>(() => getFrameworkReport())
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

const caseColumns = computed(() => [
  { title: t('testTask.list.caseColumns.title'), dataIndex: 'title', key: 'title', ellipsis: true },
  { title: t('testTask.list.caseColumns.category'), dataIndex: 'category', key: 'category', width: 100 },
  { title: t('testTask.list.caseColumns.selfTest'), key: 'self_test_result', width: 80 },
  { title: t('testTask.list.caseColumns.action'), key: 'action', width: 70 },
])

const scriptColumns = computed(() => [
  { title: t('taskRun.scriptName'), dataIndex: 'file_path', key: 'file_path', ellipsis: true },
  { title: t('taskRun.scriptLang'), dataIndex: 'language', key: 'language', width: 90 },
  { title: t('common.action'), key: 'content', width: 70 },
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
  testCases.value = []
  testScripts.value = []
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
    testCases.value = caseRes.data.data || []
    testScripts.value = scriptRes.data.data || []
    resultsVisible.value = true
    await loadPlaywrightHtml(id)
  } catch {
    // ignore
  }
}

async function loadPlaywrightHtml(id: number) {
  playwrightReportHtml.value = ''
  try {
    const res = await getSelfTestReport(id, 'playwright')
    const data = res.data.data || {}
    if (data.content_type?.includes('html') && data.content) {
      const html = injectSandboxScript(data.content, id)
      playwrightReportHtml.value = html
    }
  } catch (e) {
    console.error('loadPlaywrightHtml failed:', e)
  }
}

function injectSandboxScript(html: string, taskId: number): string {
  const headClose = `<\x2fhead>`
  const scriptTag = `<\x2fscript>`
  // Playwright 报告的相对资源路径（video、trace 等），注入 base 标签指向后端工作区接口
  const basePath = `/api/test-tasks/${taskId}/workspace/playwright-report/`
  const baseTag = `<base href="${basePath}">`

  // 拦截 Playwright SPA hash 路由，防止 srcdoc iframe 跳出
  const script = `
<script>
(function(){
  try{
    var _ps=history.pushState.bind(history);
    var _rs=history.replaceState.bind(history);
    history.pushState=function(s,t,u){if(u){var h=parseHash(u);if(h){window.location.hash=h;u='';}_ps(null,t,u);}else{_ps(null,t,'');}};
    history.replaceState=function(s,t,u){if(u){var h=parseHash(u);if(h){window.location.hash=h;u='';}_rs(null,t,u);}else{_rs(null,t,'');}};
    function parseHash(u){try{var p=new URL(u,'http://x');return p.hash||p.search?'#'+(p.hash||p.search.slice(1)):null}catch(e){return null}}
  }catch(e){}
  document.addEventListener('click',function(e){
    var a=e.target.closest('a');
    if(!a)return;
    var href=a.getAttribute('href');
    if(href&&href.indexOf('#')===0){
      e.preventDefault();
      window.location.hash=href.slice(1);
    }
  },true);
})();
${scriptTag}`
  const injected = `${baseTag}${script}`
  if (html.includes(headClose)) {
    return html.replace(headClose, `${injected}${headClose}`)
  }
  return injected + html
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

function goToEditCase(tc: TestCaseVO) {
  router.push({
    name: 'TestCaseEdit',
    params: { id: String(taskId.value) },
    query: { caseId: String(tc.id) },
  })
}

function viewScript(script: TestScriptVO) {
  viewingScript.value = script
  scriptModal.value = true
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

.report-frame {
  width: 100%;
  height: 70vh;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  background: #fff;
}

.self-test-report {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.self-test-report__card {
  margin-top: 0;
}

.self-test-report__list {
  margin: 0;
  padding-left: 18px;
}

.self-test-report__summary {
  margin: 8px 0;
  color: #595959;
}

.self-test-report__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.self-test-report__path {
  color: #8c8c8c;
  font-size: 12px;
  word-break: break-all;
}

@media (max-width: 960px) {
  .self-test-report :deep(.ant-col) {
    width: 100%;
    max-width: 100%;
    flex: 0 0 100%;
  }
}
</style>
