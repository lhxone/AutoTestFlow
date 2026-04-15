<template>
  <div class="task-run-page">
    <a-page-header :title="`${t('taskRun.drawerTitle')} #${taskId}`" @back="$router.back()" />

    <a-skeleton v-if="initializing" active :paragraph="{ rows: 8 }" />

    <template v-else>
      <a-steps :current="currentStageIndex" size="small" class="task-run-page__steps">
        <a-step
          v-for="s in stages"
          :key="s.key"
          :title="s.label"
          :status="getStageStatus(s.key)"
        />
      </a-steps>

      <div class="task-run-page__status-row">
        <a-tag :color="connTagColor">{{ connectionStatusText }}</a-tag>
        <template v-if="taskInfo">
          <a-tag :color="taskStatusColor(taskInfo.status)">{{ translateTaskStatus(t, taskInfo.status) }}</a-tag>
          <span v-if="taskInfo.error_message" class="task-run-page__error">{{ taskInfo.error_message }}</span>
        </template>
      </div>

      <a-card size="small" class="terminal-card" :bodyStyle="{ padding: terminalExpanded ? '12px' : '8px 12px' }">
        <template #title>
          终端输出
        </template>
        <template #extra>
          <a-button type="link" size="small" @click="terminalExpanded = !terminalExpanded">
            {{ terminalExpanded ? '收起' : '展开' }}
          </a-button>
        </template>
        <div v-if="terminalExpanded" class="terminal" ref="terminalRef">
          <div v-if="logs.length === 0" class="terminal__empty">{{ t('taskRun.noLogs') }}</div>
          <div
            v-for="(line, idx) in logs"
            :key="idx"
            :class="['terminal__line', `terminal__line--${line.type}`]"
          >
            <span v-if="line.time" class="terminal__time">{{ line.time }}</span>
            <span class="terminal__msg" v-html="formatMsg(line.msg)"></span>
          </div>
        </div>
        <div v-else class="terminal-collapsed-hint">
          已默认折叠，点击右上角“展开”查看完整终端输出
        </div>
      </a-card>

      <CLIInteractionPanel :taskId="taskId" :taskEvents="taskEvents" />

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
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import {
  createTestTaskEventSource,
  getTaskLogs,
  getTestCases,
  getTestScripts,
  getTestTaskById,
} from '@/api/testTask'
import type { TestCaseVO, TestScriptVO, TestTask, TestTaskEvent } from '@/types'
import { translateSelfTestResult, translateTaskStatus, translateTestCaseCategory } from '@/types'
import CLIInteractionPanel from '@/components/CLIInteractionPanel.vue'
import { useUserStore } from '@/stores/user'

interface LogLine {
  time: string
  msg: string
  type: 'log' | 'error' | 'stage' | 'status'
}

const STAGE_ORDER = ['workspace_prepared', 'cli_started', 'artifacts_synced', 'review_pending']
const LOG_RENDER_LIMIT = 200
const EVENT_LIMIT = 400
const LOG_CHUNK_SIZE = 40

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const { t } = useI18n()

const taskId = computed(() => Number(route.params.id))

const initializing = ref(true)
const logs = ref<LogLine[]>([])
const taskEvents = ref<TestTaskEvent[]>([])
const terminalRef = ref<HTMLDivElement | null>(null)
const terminalExpanded = ref(false)
const reachedStage = ref('')
const connStatus = ref<'idle' | 'connecting' | 'connected' | 'closed' | 'error'>('idle')
const taskInfo = ref<TestTask | null>(null)
const testCases = ref<TestCaseVO[]>([])
const testScripts = ref<TestScriptVO[]>([])
const resultsVisible = ref(false)
const scriptModal = ref(false)
const viewingScript = ref<TestScriptVO | null>(null)
const canEditCase = computed(() => userStore.hasPermission('test:intervene'))
let eventSource: EventSource | null = null

const stages = computed(() => [
  { key: 'workspace_prepared', label: t('taskRun.stages.workspace') },
  { key: 'cli_started', label: t('taskRun.stages.cliStarted') },
  { key: 'artifacts_synced', label: t('taskRun.stages.artifacts') },
  { key: 'review_pending', label: t('taskRun.stages.done') },
])

const currentStageIndex = computed(() => {
  if (!reachedStage.value) return -1
  return STAGE_ORDER.indexOf(reachedStage.value)
})

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
  logs.value = []
  taskEvents.value = []
  terminalExpanded.value = false
  reachedStage.value = ''
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
    taskEvents.value = events.slice(-EVENT_LIMIT)
    await renderLogsChunked(events.slice(-LOG_RENDER_LIMIT))
    syncReachedStageFromEvents(events)
  } catch {
    // ignore
  }
}

async function renderLogsChunked(events: TestTaskEvent[]) {
  logs.value = []
  const mapped = events.map((e) => ({
    time: e.timestamp ? new Date(e.timestamp).toLocaleTimeString() : '',
    msg: e.message || '',
    type: mapEventType(e.type),
  }))
  for (let i = 0; i < mapped.length; i += LOG_CHUNK_SIZE) {
    logs.value = logs.value.concat(mapped.slice(i, i + LOG_CHUNK_SIZE))
    await waitFrame()
  }
}

function syncReachedStageFromEvents(events: TestTaskEvent[]) {
  for (const e of events) {
    if (e.stage && STAGE_ORDER.includes(e.stage)) {
      const currentIdx = STAGE_ORDER.indexOf(reachedStage.value)
      const newIdx = STAGE_ORDER.indexOf(e.stage)
      if (newIdx > currentIdx) reachedStage.value = e.stage
    }
  }
}

async function waitFrame() {
  return new Promise((resolve) => setTimeout(resolve, 0))
}

function openEventSource(id: number) {
  closeEventSource()
  eventSource = createTestTaskEventSource(id)
  eventSource.addEventListener('task-event', async (rawEvent) => {
    connStatus.value = 'connected'
    const payload = JSON.parse((rawEvent as MessageEvent<string>).data) as TestTaskEvent
    appendLog(payload)
    if (payload.stage && STAGE_ORDER.includes(payload.stage)) {
      const currentIdx = STAGE_ORDER.indexOf(reachedStage.value)
      const newIdx = STAGE_ORDER.indexOf(payload.stage)
      if (newIdx > currentIdx) reachedStage.value = payload.stage
    }
    if (payload.type !== 'log') {
      await refreshTask(id)
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
  const type = mapEventType(event.type)
  const msg = event.message || (event.type === 'stage' ? `[${event.stage}]` : '')
  if (!msg) return
  logs.value = [...logs.value, {
    time: event.timestamp ? new Date(event.timestamp).toLocaleTimeString() : '',
    msg,
    type,
  }].slice(-LOG_RENDER_LIMIT)
  taskEvents.value = [...taskEvents.value, event].slice(-EVENT_LIMIT)
  nextTick(() => {
    if (terminalRef.value) terminalRef.value.scrollTop = terminalRef.value.scrollHeight
  })
}

function mapEventType(type: string): LogLine['type'] {
  if (type === 'error') return 'error'
  if (type === 'stage') return 'stage'
  if (type === 'status') return 'status'
  return 'log'
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
  } catch {
    // ignore
  }
}

function closeEventSource() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
}

function formatMsg(msg: string): string {
  if (!msg) return ''
  return msg
    .replace(/\n/g, '<br/>')
    .replace(/(https?:\/\/[^\s<]+)/g, '<span class="terminal__link">$1</span>')
}

function getStageStatus(key: string): 'wait' | 'process' | 'finish' | 'error' {
  if (!reachedStage.value) return 'wait'
  const reached = STAGE_ORDER.indexOf(reachedStage.value)
  const current = STAGE_ORDER.indexOf(key)
  if (taskInfo.value?.status === 'failed') {
    if (current <= reached) return current < reached ? 'finish' : 'error'
    return 'wait'
  }
  if (current < reached) return 'finish'
  if (current === reached) return reachedStage.value === 'review_pending' ? 'finish' : 'process'
  return 'wait'
}

function taskStatusColor(s: string) {
  const map: Record<string, string> = {
    pending: 'default', running: 'processing', completed: 'success', failed: 'error',
  }
  return map[s] || 'default'
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

.task-run-page__steps {
  margin-bottom: 14px;
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

.terminal {
  min-height: 320px;
  max-height: 480px;
  overflow-y: auto;
  background: #1e1e1e;
  border-radius: 6px;
  padding: 12px 16px;
  font-family: 'Cascadia Code', Consolas, 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.6;
}

.terminal-card {
  margin-bottom: 12px;
}

.terminal-collapsed-hint {
  color: #8c8c8c;
  font-size: 12px;
}

.terminal__empty {
  color: #666;
  font-style: italic;
}

.terminal__line {
  display: flex;
  gap: 10px;
  white-space: pre-wrap;
  word-break: break-all;
}

.terminal__time {
  color: #569cd6;
  flex-shrink: 0;
  user-select: none;
  font-size: 11px;
  padding-top: 1px;
}

.terminal__msg {
  color: #d4d4d4;
}

.terminal__msg :deep(.terminal__link) {
  color: #4fc1ff;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.terminal__line--error .terminal__msg {
  color: #f48771;
}

.terminal__line--stage .terminal__msg {
  color: #4ec9b0;
  font-weight: 600;
}

.terminal__line--status .terminal__msg {
  color: #dcdcaa;
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
</style>
