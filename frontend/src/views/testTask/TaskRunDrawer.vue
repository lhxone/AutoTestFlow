<template>
  <a-drawer
    :open="open"
    @update:open="$emit('update:open', $event)"
    :title="t('taskRun.drawerTitle')"
    width="860"
    :destroyOnClose="false"
    @close="handleClose"
  >
    <div class="task-run-drawer">
      <a-steps :current="currentStageIndex" size="small" class="task-run-drawer__steps">
        <a-step
          v-for="s in stages"
          :key="s.key"
          :title="s.label"
          :status="getStageStatus(s.key)"
        />
      </a-steps>

      <div class="task-run-drawer__status-row">
        <a-tag :color="connTagColor">{{ connectionStatusText }}</a-tag>
        <template v-if="taskInfo">
          <a-tag :color="taskStatusColor(taskInfo.status)">{{ translateTaskStatus(t, taskInfo.status) }}</a-tag>
          <span v-if="taskInfo.error_message" class="task-run-drawer__error">{{ taskInfo.error_message }}</span>
        </template>
      </div>

      <div class="terminal" ref="terminalRef">
        <div v-if="processedLogs.length === 0" class="terminal__empty">{{ t('taskRun.noLogs') }}</div>

        <template v-for="(item, idx) in processedLogs" :key="idx">
          <CollapsibleLog v-if="item.collapsible" :title="item.title" :icon="item.icon" :defaultOpen="item.defaultOpen" :iconColor="item.iconColor">
            <div
              v-for="(line, lIdx) in item.children"
              :key="lIdx"
              :class="['terminal__line', `terminal__line--${line.type}`]"
            >
              <span v-if="line.time" class="terminal__time">{{ line.time }}</span>
              <span class="terminal__msg" v-html="formatMsg(line.msg)"></span>
            </div>
          </CollapsibleLog>

          <div
            v-else
            :class="['terminal__line', `terminal__line--${item.type}`]"
          >
            <span v-if="item.time" class="terminal__time">{{ item.time }}</span>
            <span class="terminal__msg" v-html="formatMsg(item.msg || '')"></span>
          </div>
        </template>
      </div>

      <EinoWorkflowPanel v-if="taskId" :taskId="taskId" :taskEvents="taskEvents" :taskInfo="taskInfo" ref="interactionPanelRef" />

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
    </div>
    <a-modal
      v-model:open="scriptModal"
      :title="viewingScript?.file_path || t('taskRun.scriptPreview')"
      width="800px"
      :footer="null"
    >
      <pre class="script-preview">{{ viewingScript?.file_content }}</pre>
    </a-modal>
  </a-drawer>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onBeforeUnmount, h, defineComponent } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import {
  createTestTaskEventSource,
  getTestTaskById,
  getTestCases,
  getTestScripts,
  getTaskLogs,
} from '@/api/testTask'
import type { TestTask, TestCaseVO, TestTaskEvent, TestScriptVO } from '@/types'
import { translateTaskStatus, translateTestCaseCategory, translateSelfTestResult } from '@/types'
import EinoWorkflowPanel from '@/components/EinoWorkflowPanel.vue'
import { useUserStore } from '@/stores/user'

interface LogLine {
  time: string
  msg: string
  type: 'log' | 'error' | 'stage' | 'status'
}

interface LogItem {
  collapsible?: boolean
  title?: string
  icon?: string
  iconColor?: string
  defaultOpen?: boolean
  children?: LogLine[]
  time?: string
  msg?: string
  type?: 'log' | 'error' | 'stage' | 'status'
}

const CollapsibleLog = defineComponent({
  props: {
    title: String,
    icon: String,
    defaultOpen: { type: Boolean, default: false },
    iconColor: { type: String, default: '#4ec9b0' },
  },
  emits: [],
  setup(props, { slots }) {
    const collapsed = ref(!props.defaultOpen)
    return () => h('div', { class: 'terminal__section' }, [
      h('div', {
        class: 'terminal__section-header',
        onClick: () => { collapsed.value = !collapsed.value },
      }, [
        h('span', { class: 'terminal__section-arrow', style: { transform: collapsed.value ? 'rotate(-90deg)' : 'rotate(0)' } }, '\u25B6'),
        h('span', { class: 'terminal__section-icon', style: { color: props.iconColor } }, props.icon),
        h('span', { class: 'terminal__section-title' }, props.title),
      ]),
      !collapsed.value && h('div', { class: 'terminal__section-body' }, slots.default?.()),
    ])
  },
})

const props = defineProps<{
  open: boolean
  taskId: number | null
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  'task-done': []
  'refresh-list': []
}>()

const { t } = useI18n()
const router = useRouter()
const userStore = useUserStore()

const stages = computed(() => [
  { key: 'workspace_prepared', label: t('taskRun.stages.workspace') },
  { key: 'cli_started', label: t('taskRun.stages.cliStarted') },
  { key: 'artifacts_synced', label: t('taskRun.stages.artifacts') },
  { key: 'review_pending', label: t('taskRun.stages.done') },
])

const STAGE_ORDER = ['workspace_prepared', 'cli_started', 'artifacts_synced', 'review_pending']

const logs = ref<LogLine[]>([])
const terminalRef = ref<HTMLDivElement | null>(null)
const interactionPanelRef = ref<InstanceType<typeof EinoWorkflowPanel> | null>(null)
const reachedStage = ref<string>('')
const connStatus = ref<'idle' | 'connecting' | 'connected' | 'closed' | 'error'>('idle')
const taskInfo = ref<TestTask | null>(null)
const testCases = ref<TestCaseVO[]>([])
const testScripts = ref<TestScriptVO[]>([])
const taskEvents = ref<TestTaskEvent[]>([])
const resultsVisible = ref(false)
let eventSource: EventSource | null = null

const scriptModal = ref(false)
const viewingScript = ref<TestScriptVO | null>(null)
const canEditCase = computed(() => userStore.hasPermission('test:intervene'))

const processedLogs = computed<LogItem[]>(() => {
  const items: LogItem[] = []
  const lines = logs.value

  let i = 0
  while (i < lines.length) {
    const line = lines[i]
    const msg = line.msg

    if (msg.includes('git clone') || msg.includes('仓库目录') || msg.includes('跳过 clone')) {
      const sectionLines: LogLine[] = []
      while (i < lines.length && !isCollapsibleHeader(lines[i].msg)) {
        sectionLines.push(lines[i])
        i++
      }
      items.push({
        collapsible: true,
        title: 'Git Clone',
        icon: '\uD83D\uDCE6',
        iconColor: '#5db8fe',
        defaultOpen: false,
        children: sectionLines,
      })
      continue
    }

    if (msg.includes('\uD83D\uDE80') || msg.includes('初始化会话')) {
      const sectionLines: LogLine[] = []
      while (i < lines.length && !isCollapsibleHeader(lines[i].msg) && i < lines.length) {
        if (lines[i].msg.includes('\uD83D\uDE80') || lines[i].msg.includes('初始化会话') || lines[i].msg.includes('\u2139\uFE0F')) {
          sectionLines.push(lines[i])
          i++
          continue
        }
        break
      }
      items.push({
        collapsible: true,
        title: '会话初始化',
        icon: '\uD83D\uDE80',
        iconColor: '#5db8fe',
        defaultOpen: false,
        children: sectionLines,
      })
      continue
    }

    if (msg.includes('CLI 命令已启动')) {
      items.push(line)
      i++
      continue
    }

    if (line.type === 'stage') {
      items.push(line)
      i++
      continue
    }

    items.push(line)
    i++
  }

  return items
})

function isCollapsibleHeader(msg: string): boolean {
  if (msg.includes('CLI 命令已启动')) return true
  if (msg.includes('CLI 命令执行完成')) return true
  if (msg.includes('已解析 CLI 结果文件')) return true
  if (msg.includes('测试脚本和测试文档已同步')) return true
  if (msg.includes('CLI Runtime 已完成')) return true
  if (msg.includes('测试任务已生成完成')) return true
  if (msg.includes('\u2714\uFE0F 完成')) return true
  if (msg.includes('\u2705 完成')) return true
  return false
}

function formatMsg(msg: string): string {
  if (!msg) return ''
  return msg
    .replace(/\n/g, '<br/>')
    .replace(/(https?:\/\/[^\s<]+)/g, '<span class="terminal__link">$1</span>')
}

const currentStageIndex = computed(() => {
  if (!reachedStage.value) return -1
  const idx = STAGE_ORDER.indexOf(reachedStage.value)
  return idx
})

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

function taskStatusColor(s: string) {
  const map: Record<string, string> = {
    pending: 'default', running: 'processing', completed: 'success', failed: 'error',
  }
  return map[s] || 'default'
}

onBeforeUnmount(closeEventSource)

let lastInitedTaskId: number | null = null

watch(
  [() => props.open, () => props.taskId],
  ([open, taskId], [, prevTaskId]) => {
    if (open && taskId) {
      if (taskId !== lastInitedTaskId) {
        resetState()
      } else {
        closeEventSource()
      }
      lastInitedTaskId = taskId
      initTask(taskId)
    } else if (!open) {
      closeEventSource()
    }
  },
  { immediate: true },
)

function resetState() {
  logs.value = []
  taskEvents.value = []
  reachedStage.value = ''
  connStatus.value = 'connecting'
  taskInfo.value = null
  testCases.value = []
  testScripts.value = []
  resultsVisible.value = false
  closeEventSource()
}

async function initTask(taskId: number) {
  try {
    const res = await getTestTaskById(taskId)
    taskInfo.value = res.data.data
  } catch {
    // ignore
  }

  const isDone = taskInfo.value?.status === 'completed' || taskInfo.value?.status === 'failed'

  if (isDone) {
    connStatus.value = 'closed'
    await Promise.all([
      loadHistoryLogs(taskId),
      loadResults(taskId),
    ])
    return
  }

  await loadResults(taskId)
  openEventSource(taskId)
}

async function loadHistoryLogs(taskId: number) {
  try {
    const res = await getTaskLogs(taskId)
    const events: TestTaskEvent[] = res.data.data || []
    taskEvents.value = events.slice(-400)
    logs.value = events.slice(-200).map(e => ({
      time: e.timestamp ? new Date(e.timestamp).toLocaleTimeString() : '',
      msg: e.message || '',
      type: mapEventType(e.type),
    }))

    for (const e of events) {
      if (e.stage && STAGE_ORDER.includes(e.stage)) {
        const currentIdx = STAGE_ORDER.indexOf(reachedStage.value)
        const newIdx = STAGE_ORDER.indexOf(e.stage)
        if (newIdx > currentIdx) {
          reachedStage.value = e.stage
        }
      }
      if (e.status === 'failed' || e.stage === 'review_pending') {
        if (e.status) taskInfo.value = { ...taskInfo.value!, status: e.status } as TestTask
      }
    }
  } catch {
    // ignore
  }
}

function mapEventType(type: string): LogLine['type'] {
  if (type === 'error') return 'error'
  if (type === 'stage') return 'stage'
  if (type === 'status') return 'status'
  return 'log'
}

function openEventSource(taskId: number) {
  closeEventSource()
  connStatus.value = 'connecting'
  eventSource = createTestTaskEventSource(taskId)

  eventSource.addEventListener('task-event', async (rawEvent) => {
    connStatus.value = 'connected'
    const payload = JSON.parse((rawEvent as MessageEvent<string>).data) as TestTaskEvent

    appendLog(payload)

    if (payload.stage && STAGE_ORDER.includes(payload.stage)) {
      const currentIdx = STAGE_ORDER.indexOf(reachedStage.value)
      const newIdx = STAGE_ORDER.indexOf(payload.stage)
      if (newIdx > currentIdx) {
        reachedStage.value = payload.stage
      }
    }

    if (payload.type !== 'log') {
      await refreshTask(taskId, payload.stage)
    }

    if (payload.status === 'failed' || payload.stage === 'review_pending') {
      closeEventSource()
      emit('task-done')
      emit('refresh-list')
    }
  })

  eventSource.onerror = () => {
    connStatus.value = 'error'
  }
}

function appendLog(event: TestTaskEvent) {
  let type: LogLine['type'] = 'log'
  if (event.type === 'error') type = 'error'
  else if (event.type === 'stage') type = 'stage'
  else if (event.type === 'status') type = 'status'

  const msg = event.message || (event.type === 'stage' ? `[${event.stage}]` : '')
  if (!msg) return

  logs.value = [...logs.value, {
    time: event.timestamp ? new Date(event.timestamp).toLocaleTimeString() : '',
    msg,
    type,
  }].slice(-200)
  taskEvents.value = [...taskEvents.value, event].slice(-400)

  nextTick(() => {
    if (terminalRef.value) {
      terminalRef.value.scrollTop = terminalRef.value.scrollHeight
    }
  })
}

async function refreshTask(taskId: number, stage?: string) {
  try {
    const res = await getTestTaskById(taskId)
    taskInfo.value = res.data.data || taskInfo.value
  } catch {
    // ignore
  }

  if (stage === 'artifacts_synced' || stage === 'review_pending' || stage === 'runtime_completed') {
    await loadResults(taskId)
  }
}

async function loadResults(taskId: number) {
  try {
    const [caseRes, scriptRes] = await Promise.all([
      getTestCases(taskId),
      getTestScripts(taskId),
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
    if (props.open) {
      connStatus.value = 'closed'
    } else {
      connStatus.value = 'idle'
    }
  }
}

function handleClose() {
  closeEventSource()
  emit('update:open', false)
}

function goToEditCase(tc: TestCaseVO) {
  if (!props.taskId) {
    return
  }
  closeEventSource()
  emit('update:open', false)
  router.push({
    name: 'TestCaseEdit',
    params: { id: String(props.taskId) },
    query: { caseId: String(tc.id) },
  })
}

function viewScript(script: TestScriptVO) {
  viewingScript.value = script
  scriptModal.value = true
}

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
</script>

<style scoped>
.task-run-drawer {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.task-run-drawer__steps {
  margin-bottom: 14px;
}

.task-run-drawer__status-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  flex-wrap: wrap;
}

.task-run-drawer__error {
  color: #ff4d4f;
  font-size: 12px;
}

.terminal {
  flex: 1;
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

.terminal__section {
  margin: 4px 0;
}

.terminal__section-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 0;
  cursor: pointer;
  user-select: none;
  border-radius: 4px;
  transition: background 0.15s;
}

.terminal__section-header:hover {
  background: rgba(255, 255, 255, 0.05);
}

.terminal__section-arrow {
  color: #808080;
  font-size: 10px;
  transition: transform 0.15s;
  display: inline-block;
  width: 14px;
}

.terminal__section-icon {
  font-size: 14px;
}

.terminal__section-title {
  color: #cccccc;
  font-weight: 600;
  font-size: 12px;
}

.terminal__section-body {
  padding-left: 20px;
  border-left: 2px solid #333;
  margin-left: 6px;
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
