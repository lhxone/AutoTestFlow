<template>
  <a-card :title="t('taskRun.workflow.title')" size="small" class="workflow-panel">
    <template #extra>
      <a-space :size="8" wrap>
        <a-tag color="blue">{{ completedNodeCount }}/{{ workflowNodes.length }}</a-tag>
        <a-tag :color="pendingCount > 0 ? 'orange' : 'green'">
          {{ t('taskRun.workflow.pendingInteractions', { count: pendingCount }) }}
        </a-tag>
      </a-space>
    </template>

    <a-spin :spinning="loading">
      <div class="workflow-panel__summary">
        <div>
          <div class="workflow-panel__name">{{ workflowName }}</div>
          <div class="workflow-panel__meta">{{ workflowSummary }}</div>
        </div>
        <a-tag v-if="selectedNode" :color="nodeStatusColor(selectedNode.status)">
          {{ selectedNode.label }}
        </a-tag>
      </div>

      <div class="workflow-graph" role="tablist" :aria-label="t('taskRun.workflow.title')">
        <template v-for="(node, index) in workflowNodes" :key="node.key">
          <button
            type="button"
            class="workflow-node"
            :class="[
              `workflow-node--${node.status}`,
              { 'workflow-node--selected': node.key === selectedNodeKey },
            ]"
            @click="handleSelectNode(node.key)"
          >
            <span class="workflow-node__badge">{{ index + 1 }}</span>
            <span class="workflow-node__title">{{ node.label }}</span>
            <span class="workflow-node__desc">{{ node.shortDescription }}</span>
            <span class="workflow-node__footer">
              <span>{{ t(`taskRun.workflow.status.${node.status}`) }}</span>
              <span>{{ node.events.length }} {{ t('taskRun.workflow.events') }}</span>
            </span>
          </button>
          <div v-if="index < workflowNodes.length - 1" class="workflow-graph__arrow" aria-hidden="true">
            <span></span>
          </div>
        </template>
      </div>

      <a-row :gutter="16" class="workflow-panel__body">
        <a-col :xs="24" :xl="16">
          <a-card v-if="selectedNode" size="small" class="workflow-detail" :bordered="false">
            <template #title>
              <div class="workflow-detail__title-row">
                <span>{{ selectedNode.label }}</span>
                <a-tag :color="nodeStatusColor(selectedNode.status)">
                  {{ t(`taskRun.workflow.status.${selectedNode.status}`) }}
                </a-tag>
              </div>
            </template>

            <div class="workflow-detail__description">{{ selectedNode.description }}</div>

            <a-tabs size="small">
              <a-tab-pane key="output" :tab="t('taskRun.workflow.nodeOutput')">
                <div v-if="selectedNode.outputEntries.length" class="workflow-output-list">
                  <div
                    v-for="entry in selectedNode.outputEntries"
                    :key="entry.id"
                    class="workflow-output"
                    :class="`workflow-output--${entry.role}`"
                  >
                    <div class="workflow-output__header">
                      <a-space :size="8" wrap>
                        <a-tag size="small" :color="entryRoleColor(entry.role)">
                          {{ entry.title }}
                        </a-tag>
                        <span class="workflow-output__time">{{ formatTime(entry.timestamp) }}</span>
                      </a-space>
                    </div>
                    <div class="workflow-output__text">{{ entry.text }}</div>
                    <pre v-if="entry.detail" class="workflow-output__detail">{{ entry.detail }}</pre>
                  </div>
                </div>
                <a-empty v-else :description="t('taskRun.workflow.noNodeOutput')" />
              </a-tab-pane>

              <a-tab-pane key="events" :tab="t('taskRun.workflow.rawEvents')">
                <a-timeline v-if="selectedNode.events.length">
                  <a-timeline-item
                    v-for="event in selectedNode.events"
                    :key="`${event.id}-${event.timestamp}-${event.stage}`"
                    :color="eventTimelineColor(event)"
                  >
                    <div class="workflow-event">
                      <div class="workflow-event__head">
                        <span class="workflow-event__stage">{{ eventTitle(event) }}</span>
                        <span class="workflow-event__time">{{ formatTime(event.timestamp) }}</span>
                      </div>
                      <div v-if="event.message" class="workflow-event__message">{{ event.message }}</div>
                      <pre v-if="hasEventData(event)" class="workflow-event__data">{{ formatJSON(event.data) }}</pre>
                    </div>
                  </a-timeline-item>
                </a-timeline>
                <a-empty v-else :description="t('taskRun.workflow.noEvents')" />
              </a-tab-pane>

              <a-tab-pane key="interactions" :tab="t('taskRun.workflow.interactions')">
                <div class="interaction-list">
                  <div v-if="interactions.length === 0" class="empty-state">
                    <a-empty :description="t('taskRun.workflow.noInteractions')" />
                  </div>
                  <a-timeline v-else>
                    <a-timeline-item v-for="interaction in interactions" :key="interaction.id">
                      <div class="interaction-item">
                        <div class="interaction-item__header">
                          <a-tag :color="getInteractionTypeColor(interaction.interaction_type)">
                            {{ getInteractionTypeLabel(interaction.interaction_type) }}
                          </a-tag>
                          <a-tag :color="getStatusColor(interaction.status)">
                            {{ getStatusLabel(interaction.status) }}
                          </a-tag>
                          <span class="interaction-item__time">{{ formatTime(interaction.created_at) }}</span>
                        </div>

                        <div class="interaction-item__content">{{ interaction.content }}</div>

                        <div v-if="interaction.status === 'pending'" class="interaction-item__actions">
                          <a-input
                            v-model:value="responses[interaction.id]"
                            :placeholder="t('taskRun.workflow.replyPlaceholder')"
                            :disabled="loading"
                            @pressEnter="handleReply(interaction.id)"
                          >
                            <template #suffix>
                              <a-button
                                type="link"
                                size="small"
                                :loading="actionLoading[interaction.id]"
                                @click="handleReply(interaction.id)"
                              >
                                {{ t('taskRun.workflow.replyAction') }}
                              </a-button>
                            </template>
                          </a-input>

                          <a-space v-if="interaction.interaction_type === 'permission_request'" style="margin-top: 8px">
                            <a-button
                              type="primary"
                              size="small"
                              :loading="actionLoading[interaction.id]"
                              @click="handleApprove(interaction.id)"
                            >
                              {{ t('taskRun.workflow.approve') }}
                            </a-button>
                            <a-button
                              danger
                              size="small"
                              :loading="actionLoading[interaction.id]"
                              @click="handleReject(interaction.id)"
                            >
                              {{ t('taskRun.workflow.reject') }}
                            </a-button>
                          </a-space>
                        </div>

                        <div v-else-if="interaction.user_response" class="interaction-item__response">
                          <div class="interaction-item__response-label">{{ t('taskRun.workflow.replyLabel') }}</div>
                          <div>{{ interaction.user_response }}</div>
                        </div>
                      </div>
                    </a-timeline-item>
                  </a-timeline>
                </div>
              </a-tab-pane>
            </a-tabs>
          </a-card>
        </a-col>

        <a-col :xs="24" :xl="8">
          <a-card size="small" class="workflow-sidecard" :title="t('taskRun.workflow.overview')">
            <a-descriptions size="small" :column="1">
              <a-descriptions-item :label="t('taskRun.workflow.engine')">
                {{ workflowEngine }}
              </a-descriptions-item>
              <a-descriptions-item :label="t('taskRun.workflow.currentNode')">
                {{ selectedNode?.label || '-' }}
              </a-descriptions-item>
              <a-descriptions-item :label="t('taskRun.workflow.taskStatus')">
                {{ props.taskInfo?.status || '-' }}
              </a-descriptions-item>
              <a-descriptions-item :label="t('taskRun.workflow.retryCount')">
                {{ props.taskInfo?.retry_count ?? 0 }}
              </a-descriptions-item>
              <a-descriptions-item :label="t('taskRun.workflow.chromeMcp')">
                {{ chromeMCPText }}
              </a-descriptions-item>
            </a-descriptions>
          </a-card>

          <a-card size="small" class="workflow-sidecard" :title="t('taskRun.workflow.legend')">
            <div class="workflow-legend">
              <div v-for="status in legendStatuses" :key="status" class="workflow-legend__item">
                <span class="workflow-legend__dot" :class="`workflow-legend__dot--${status}`"></span>
                <span>{{ t(`taskRun.workflow.status.${status}`) }}</span>
              </div>
            </div>
          </a-card>
        </a-col>
      </a-row>
    </a-spin>
  </a-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import {
  approveInteraction,
  getCLIInteractions,
  getPendingInteractions,
  getTaskLogs,
  rejectInteraction,
  replyInteraction,
  type CLIInteraction,
} from '@/api/testTask'
import type { TestTask, TestTaskEvent } from '@/types'

type WorkflowNodeKey = 'prepare_context' | 'mcp_preflight' | 'generate_assets' | 'self_test' | 'finalize_task'
type WorkflowNodeStatus = 'wait' | 'running' | 'completed' | 'failed'
type OutputRole = 'assistant' | 'tool' | 'system' | 'event' | 'error'

interface WorkflowNodeDefinition {
  key: WorkflowNodeKey
  labelKey: string
  shortDescriptionKey: string
  descriptionKey: string
}

interface OutputEntry {
  id: string
  role: OutputRole
  title: string
  text: string
  detail?: string
  timestamp: string
}

interface WorkflowNodeViewModel extends WorkflowNodeDefinition {
  label: string
  shortDescription: string
  description: string
  status: WorkflowNodeStatus
  events: TestTaskEvent[]
  outputEntries: OutputEntry[]
}

const NODE_DEFINITIONS: WorkflowNodeDefinition[] = [
  {
    key: 'prepare_context',
    labelKey: 'taskRun.workflow.nodes.prepareContext',
    shortDescriptionKey: 'taskRun.workflow.nodeHints.prepareContext',
    descriptionKey: 'taskRun.workflow.nodeDescriptions.prepareContext',
  },
  {
    key: 'mcp_preflight',
    labelKey: 'taskRun.workflow.nodes.mcpPreflight',
    shortDescriptionKey: 'taskRun.workflow.nodeHints.mcpPreflight',
    descriptionKey: 'taskRun.workflow.nodeDescriptions.mcpPreflight',
  },
  {
    key: 'generate_assets',
    labelKey: 'taskRun.workflow.nodes.generateAssets',
    shortDescriptionKey: 'taskRun.workflow.nodeHints.generateAssets',
    descriptionKey: 'taskRun.workflow.nodeDescriptions.generateAssets',
  },
  {
    key: 'self_test',
    labelKey: 'taskRun.workflow.nodes.selfTest',
    shortDescriptionKey: 'taskRun.workflow.nodeHints.selfTest',
    descriptionKey: 'taskRun.workflow.nodeDescriptions.selfTest',
  },
  {
    key: 'finalize_task',
    labelKey: 'taskRun.workflow.nodes.finalizeTask',
    shortDescriptionKey: 'taskRun.workflow.nodeHints.finalizeTask',
    descriptionKey: 'taskRun.workflow.nodeDescriptions.finalizeTask',
  },
]

const GENERATE_ASSET_STAGES = new Set([
  'generate_assets',
  'runtime_start',
  'workspace_prepared',
  'control_files_written',
  'mcp_runtime',
  'runtime_started',
  'cli_output',
  'cli_output_raw',
  'artifacts_synced',
  'result_loaded',
  'runtime_completed',
  'runtime_failed',
])

const legendStatuses: WorkflowNodeStatus[] = ['wait', 'running', 'completed', 'failed']

const props = defineProps<{
  taskId: number
  taskEvents?: TestTaskEvent[]
  taskInfo?: TestTask | null
}>()

const { t } = useI18n()

const loading = ref(false)
const interactions = ref<CLIInteraction[]>([])
const taskLogs = ref<TestTaskEvent[]>([])
const pendingCount = ref(0)
const actionLoading = ref<Record<number, boolean>>({})
const responses = ref<Record<number, string>>({})
const selectedNodeKey = ref<WorkflowNodeKey>('prepare_context')
const userSelectedNode = ref(false)

const sourceEvents = computed<TestTaskEvent[]>(() => {
  if (props.taskEvents && props.taskEvents.length > 0) {
    return props.taskEvents
  }
  return taskLogs.value
})

const workflowMeta = computed<Record<string, any> | null>(() => {
  const meta = props.taskInfo?.ai_output?.workflow
  if (!meta || typeof meta !== 'object') return null
  return meta as Record<string, any>
})

const workflowName = computed(() => {
  const raw = workflowMeta.value?.name
  return typeof raw === 'string' && raw.trim() ? raw : 'gen-test-eino-workflow'
})

const workflowEngine = computed(() => {
  const raw = workflowMeta.value?.engine
  return typeof raw === 'string' && raw.trim() ? raw : 'eino'
})

const workflowSummary = computed(() => {
  const raw = workflowMeta.value?.mcp_capability_summary
  if (typeof raw === 'string' && raw.trim()) {
    return raw.trim()
  }
  return t('taskRun.workflow.defaultSummary')
})

const chromeMCPText = computed(() => {
  const servers = workflowMeta.value?.chrome_mcp_servers
  if (Array.isArray(servers) && servers.length > 0) {
    return servers.join(', ')
  }
  return t('taskRun.workflow.notEnabled')
})

const workflowNodes = computed<WorkflowNodeViewModel[]>(() => NODE_DEFINITIONS.map((definition) => {
  const events = sourceEvents.value.filter((event) => belongsToNode(event, definition.key))
  return {
    ...definition,
    label: t(definition.labelKey),
    shortDescription: t(definition.shortDescriptionKey),
    description: t(definition.descriptionKey),
    status: resolveNodeStatus(definition.key, events, props.taskInfo?.status || ''),
    events,
    outputEntries: buildOutputEntries(events),
  }
}))

const completedNodeCount = computed(() => workflowNodes.value.filter((node) => node.status === 'completed').length)

const preferredNodeKey = computed<WorkflowNodeKey>(() => {
  const failed = workflowNodes.value.find((node) => node.status === 'failed')
  if (failed) return failed.key
  const running = workflowNodes.value.find((node) => node.status === 'running')
  if (running) return running.key
  const completed = [...workflowNodes.value].reverse().find((node) => node.status === 'completed')
  if (completed) return completed.key
  return 'prepare_context'
})

const selectedNode = computed(() => workflowNodes.value.find((node) => node.key === selectedNodeKey.value) || workflowNodes.value[0])

onMounted(() => {
  loadPanelData()
})

watch(() => props.taskId, () => {
  taskLogs.value = []
  interactions.value = []
  pendingCount.value = 0
  userSelectedNode.value = false
  loadPanelData()
})

watch(workflowNodes, (nodes) => {
  if (!nodes.length) return
  const exists = nodes.some((node) => node.key === selectedNodeKey.value)
  if (!exists || !userSelectedNode.value) {
    selectedNodeKey.value = preferredNodeKey.value
  }
}, { immediate: true })

watch(() => props.taskEvents, (events) => {
  if (events && events.length) {
    taskLogs.value = events.slice(-400)
  }
}, { immediate: true })

async function loadPanelData() {
  if (!props.taskId) return
  loading.value = true
  try {
    const jobs: Promise<unknown>[] = [loadInteractions(), loadPendingInteractions()]
    if (!props.taskEvents || props.taskEvents.length === 0) {
      jobs.push(loadTaskLogs())
    }
    await Promise.all(jobs)
  } finally {
    loading.value = false
  }
}

async function loadInteractions() {
  if (!props.taskId) return
  try {
    const res = await getCLIInteractions(props.taskId)
    interactions.value = res.data?.data || []
    pendingCount.value = interactions.value.filter((item) => item.status === 'pending').length
  } catch {
    message.error(t('taskRun.workflow.loadInteractionsFailed'))
  }
}

async function loadPendingInteractions() {
  if (!props.taskId) return
  try {
    const res = await getPendingInteractions(props.taskId)
    pendingCount.value = (res.data?.data || []).length
  } catch {
    pendingCount.value = interactions.value.filter((item) => item.status === 'pending').length
  }
}

async function loadTaskLogs() {
  if (!props.taskId) return
  try {
    const res = await getTaskLogs(props.taskId)
    taskLogs.value = res.data?.data || []
  } catch {
    taskLogs.value = []
  }
}

function belongsToNode(event: TestTaskEvent, nodeKey: WorkflowNodeKey): boolean {
  const stage = String(event.stage || '')
  switch (nodeKey) {
    case 'prepare_context':
      return stage === 'context_loaded'
    case 'mcp_preflight':
      return stage === 'mcp_preflight'
    case 'generate_assets':
      return GENERATE_ASSET_STAGES.has(stage)
    case 'self_test':
      return stage === 'self_test_started' || stage === 'self_test_completed' || stage === 'self_test'
    case 'finalize_task':
      return stage === 'finalize_task' || stage === 'review_pending' || stage === 'workflow_completed' || stage === 'self_test_failed'
    default:
      return false
  }
}

function resolveNodeStatus(nodeKey: WorkflowNodeKey, events: TestTaskEvent[], taskStatus: string): WorkflowNodeStatus {
  const stages = events.map((event) => String(event.stage || ''))
  const hasFailedEvent = events.some((event) => event.status === 'failed' || event.type === 'error')

  if (nodeKey === 'generate_assets' && (hasFailedEvent || stages.includes('runtime_failed'))) {
    return 'failed'
  }
  if (nodeKey === 'finalize_task' && stages.includes('self_test_failed')) {
    return 'failed'
  }

  switch (nodeKey) {
    case 'prepare_context':
      return stages.includes('context_loaded') ? 'completed' : 'wait'
    case 'mcp_preflight':
      return stages.includes('mcp_preflight') ? 'completed' : previousNodeReady('prepare_context')
    case 'generate_assets':
      if (stages.includes('result_loaded') || stages.includes('runtime_completed') || stages.includes('artifacts_synced')) {
        return 'completed'
      }
      if (events.length > 0) {
        return 'running'
      }
      return previousNodeReady('mcp_preflight')
    case 'self_test':
      if (stages.includes('self_test_completed')) {
        return 'completed'
      }
      if (stages.includes('self_test_started') || events.length > 0) {
        return 'running'
      }
      return previousNodeReady('generate_assets')
    case 'finalize_task':
      if (stages.includes('review_pending') || stages.includes('workflow_completed')) {
        return taskStatus === 'failed' ? 'failed' : 'completed'
      }
      if (events.length > 0) {
        return taskStatus === 'failed' ? 'failed' : 'running'
      }
      return previousNodeReady('self_test')
    default:
      return 'wait'
  }

  function previousNodeReady(previousKey: WorkflowNodeKey): WorkflowNodeStatus {
    const previous = workflowNodes.value.find((node) => node.key === previousKey)
    if (!previous) return 'wait'
    return previous.status === 'completed' ? 'wait' : previous.status === 'failed' ? 'failed' : 'wait'
  }
}

function buildOutputEntries(events: TestTaskEvent[]): OutputEntry[] {
  return events.flatMap((event) => parseStructuredEvent(event)).slice(-80)
}

function parseStructuredEvent(event: TestTaskEvent): OutputEntry[] {
  if (!event.message) {
    return [buildPlainEventEntry(event)]
  }

  const raw = event.message.trim()
  if ((event.stage === 'cli_output' || event.stage === 'cli_output_raw') && raw.startsWith('{')) {
    try {
      const payload = JSON.parse(raw) as Record<string, any>
      return parseRuntimePayload(event, payload)
    } catch {
      return [buildPlainEventEntry(event)]
    }
  }

  return [buildPlainEventEntry(event)]
}

function parseRuntimePayload(event: TestTaskEvent, payload: Record<string, any>): OutputEntry[] {
  const baseId = `${event.id}-${event.timestamp}`
  const payloadType = String(payload.type || '')
  const timestamp = event.timestamp

  if (payloadType === 'assistant') {
    const content = Array.isArray(payload.message?.content) ? payload.message.content : []
    const entries: OutputEntry[] = []
    for (let index = 0; index < content.length; index += 1) {
      const block = content[index]
      if (block?.type === 'text') {
        entries.push({
          id: `${baseId}-assistant-${index}`,
          role: 'assistant',
          title: t('taskRun.workflow.roles.assistant'),
          text: String(block.text || '').trim(),
          timestamp,
        })
      }
      if (block?.type === 'tool_use') {
        entries.push({
          id: `${baseId}-tool-${index}`,
          role: 'tool',
          title: String(block.name || t('taskRun.workflow.roles.tool')),
          text: t('taskRun.workflow.toolCall'),
          detail: formatJSON(block.input),
          timestamp,
        })
      }
    }
    return entries.length ? entries : [buildPlainEventEntry(event)]
  }

  if (payloadType === 'user') {
    const content = Array.isArray(payload.message?.content) ? payload.message.content : []
    const entries = content.map((block: any, index: number) => ({
      id: `${baseId}-result-${index}`,
      role: block?.is_error ? 'error' as const : 'tool' as const,
      title: block?.is_error ? t('taskRun.workflow.roles.error') : t('taskRun.workflow.roles.toolResult'),
      text: String(block?.content || '').trim(),
      timestamp,
    }))
    return entries.length ? entries : [buildPlainEventEntry(event)]
  }

  if (payloadType === 'system') {
    return [{
      id: `${baseId}-system`,
      role: 'system',
      title: t('taskRun.workflow.roles.system'),
      text: String(payload.message || t('taskRun.workflow.systemEvent')),
      timestamp,
    }]
  }

  if (payloadType === 'result') {
    return [{
      id: `${baseId}-runtime-result`,
      role: payload.is_error ? 'error' : 'system',
      title: payload.is_error ? t('taskRun.workflow.roles.error') : t('taskRun.workflow.roles.system'),
      text: payload.is_error ? t('taskRun.workflow.runtimeFailed') : t('taskRun.workflow.runtimeCompleted'),
      detail: payload.result ? formatJSON(payload.result) : undefined,
      timestamp,
    }]
  }

  return [buildPlainEventEntry(event)]
}

function buildPlainEventEntry(event: TestTaskEvent): OutputEntry {
  return {
    id: `${event.id}-${event.timestamp}-${event.stage || event.type}`,
    role: event.type === 'error' || event.status === 'failed' ? 'error' : 'event',
    title: eventTitle(event),
    text: event.message || t('taskRun.workflow.noMessage'),
    detail: hasEventData(event) ? formatJSON(event.data) : undefined,
    timestamp: event.timestamp,
  }
}

function eventTitle(event: TestTaskEvent): string {
  const stage = String(event.stage || '')
  const stageMap: Record<string, string> = {
    context_loaded: t('taskRun.workflow.eventTitles.contextLoaded'),
    mcp_preflight: t('taskRun.workflow.eventTitles.mcpPreflight'),
    runtime_start: t('taskRun.workflow.eventTitles.runtimeStart'),
    workspace_prepared: t('taskRun.workflow.eventTitles.workspacePrepared'),
    control_files_written: t('taskRun.workflow.eventTitles.controlFilesWritten'),
    runtime_started: t('taskRun.workflow.eventTitles.runtimeStarted'),
    cli_output: t('taskRun.workflow.eventTitles.agentOutput'),
    cli_output_raw: t('taskRun.workflow.eventTitles.agentTrace'),
    artifacts_synced: t('taskRun.workflow.eventTitles.artifactsSynced'),
    result_loaded: t('taskRun.workflow.eventTitles.resultLoaded'),
    self_test_started: t('taskRun.workflow.eventTitles.selfTestStarted'),
    self_test_completed: t('taskRun.workflow.eventTitles.selfTestCompleted'),
    self_test_failed: t('taskRun.workflow.eventTitles.selfTestFailed'),
    review_pending: t('taskRun.workflow.eventTitles.reviewPending'),
    workflow_completed: t('taskRun.workflow.eventTitles.workflowCompleted'),
    runtime_failed: t('taskRun.workflow.eventTitles.runtimeFailed'),
  }
  return stageMap[stage] || stage || event.type
}

function formatTime(value: string) {
  return value ? new Date(value).toLocaleString('zh-CN') : '-'
}

function handleSelectNode(nodeKey: WorkflowNodeKey) {
  userSelectedNode.value = true
  selectedNodeKey.value = nodeKey
}

function formatJSON(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function hasEventData(event: TestTaskEvent) {
  return !!event.data && Object.keys(event.data).length > 0
}

function nodeStatusColor(status: WorkflowNodeStatus) {
  const map: Record<WorkflowNodeStatus, string> = {
    wait: 'default',
    running: 'processing',
    completed: 'success',
    failed: 'error',
  }
  return map[status]
}

function entryRoleColor(role: OutputRole) {
  const map: Record<OutputRole, string> = {
    assistant: 'blue',
    tool: 'cyan',
    system: 'gold',
    event: 'default',
    error: 'red',
  }
  return map[role]
}

function eventTimelineColor(event: TestTaskEvent) {
  if (event.type === 'error' || event.status === 'failed') return 'red'
  if (event.type === 'stage' || event.type === 'status') return 'blue'
  return 'gray'
}

async function handleReply(interactionId: number) {
  const response = responses.value[interactionId]
  if (!response?.trim()) {
    message.warning(t('taskRun.workflow.replyEmpty'))
    return
  }

  actionLoading.value[interactionId] = true
  try {
    await replyInteraction(props.taskId, interactionId, response)
    message.success(t('taskRun.workflow.replySuccess'))
    responses.value[interactionId] = ''
    await loadInteractions()
  } catch {
    message.error(t('taskRun.workflow.replyFailed'))
  } finally {
    actionLoading.value[interactionId] = false
  }
}

async function handleApprove(interactionId: number) {
  actionLoading.value[interactionId] = true
  try {
    await approveInteraction(props.taskId, interactionId)
    message.success(t('taskRun.workflow.approveSuccess'))
    await loadInteractions()
  } catch {
    message.error(t('taskRun.workflow.approveFailed'))
  } finally {
    actionLoading.value[interactionId] = false
  }
}

async function handleReject(interactionId: number) {
  actionLoading.value[interactionId] = true
  try {
    await rejectInteraction(props.taskId, interactionId, t('taskRun.workflow.userRejected'))
    message.success(t('taskRun.workflow.rejectSuccess'))
    await loadInteractions()
  } catch {
    message.error(t('taskRun.workflow.rejectFailed'))
  } finally {
    actionLoading.value[interactionId] = false
  }
}

function getInteractionTypeColor(type: string) {
  switch (type) {
    case 'ai_question':
      return 'blue'
    case 'permission_request':
      return 'orange'
    default:
      return 'default'
  }
}

function getInteractionTypeLabel(type: string) {
  switch (type) {
    case 'ai_question':
      return t('taskRun.workflow.interactionTypes.aiQuestion')
    case 'permission_request':
      return t('taskRun.workflow.interactionTypes.permissionRequest')
    default:
      return type
  }
}

function getStatusColor(status: string) {
  switch (status) {
    case 'pending':
      return 'orange'
    case 'approved':
      return 'green'
    case 'rejected':
      return 'red'
    case 'answered':
      return 'blue'
    default:
      return 'default'
  }
}

function getStatusLabel(status: string) {
  switch (status) {
    case 'pending':
      return t('taskRun.workflow.interactionStatus.pending')
    case 'approved':
      return t('taskRun.workflow.interactionStatus.approved')
    case 'rejected':
      return t('taskRun.workflow.interactionStatus.rejected')
    case 'answered':
      return t('taskRun.workflow.interactionStatus.answered')
    default:
      return status
  }
}
</script>

<style scoped>
.workflow-panel {
  margin-top: 16px;
}

.workflow-panel__summary {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.workflow-panel__name {
  font-size: 15px;
  font-weight: 600;
  color: #1f2937;
}

.workflow-panel__meta {
  margin-top: 4px;
  color: #667085;
  font-size: 12px;
  white-space: pre-line;
}

.workflow-graph {
  display: flex;
  align-items: stretch;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 8px;
}

.workflow-graph__arrow {
  min-width: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.workflow-graph__arrow span {
  position: relative;
  display: block;
  width: 100%;
  height: 2px;
  background: linear-gradient(90deg, #d1d5db, #94a3b8);
}

.workflow-graph__arrow span::after {
  content: '';
  position: absolute;
  right: -1px;
  top: -4px;
  border-left: 8px solid #94a3b8;
  border-top: 5px solid transparent;
  border-bottom: 5px solid transparent;
}

.workflow-node {
  min-width: 180px;
  border: 1px solid #d8e2f1;
  border-radius: 18px;
  padding: 14px 14px 12px;
  background: linear-gradient(180deg, #ffffff 0%, #f8fbff 100%);
  display: flex;
  flex-direction: column;
  gap: 8px;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}

.workflow-node:hover {
  transform: translateY(-1px);
  border-color: #91caff;
}

.workflow-node--selected {
  border-color: #1677ff;
  box-shadow: 0 10px 24px rgba(22, 119, 255, 0.14);
}

.workflow-node__badge {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: #e6f4ff;
  color: #1677ff;
  font-size: 12px;
  font-weight: 700;
}

.workflow-node__title {
  font-size: 14px;
  font-weight: 600;
  color: #111827;
}

.workflow-node__desc {
  min-height: 34px;
  color: #667085;
  font-size: 12px;
  line-height: 1.45;
}

.workflow-node__footer {
  margin-top: auto;
  display: flex;
  justify-content: space-between;
  gap: 8px;
  color: #475467;
  font-size: 11px;
}

.workflow-node--wait .workflow-node__badge,
.workflow-legend__dot--wait {
  background: #f2f4f7;
  color: #667085;
}

.workflow-node--running .workflow-node__badge,
.workflow-legend__dot--running {
  background: #e6f4ff;
  color: #1677ff;
}

.workflow-node--completed .workflow-node__badge,
.workflow-legend__dot--completed {
  background: #f6ffed;
  color: #389e0d;
}

.workflow-node--failed .workflow-node__badge,
.workflow-legend__dot--failed {
  background: #fff1f0;
  color: #cf1322;
}

.workflow-panel__body {
  margin-top: 12px;
}

.workflow-detail,
.workflow-sidecard {
  border-radius: 16px;
  background: #fbfdff;
}

.workflow-sidecard + .workflow-sidecard {
  margin-top: 12px;
}

.workflow-detail__title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.workflow-detail__description {
  margin-bottom: 12px;
  color: #475467;
  line-height: 1.65;
}

.workflow-output-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.workflow-output {
  padding: 12px;
  border-radius: 14px;
  border: 1px solid #e5e7eb;
  background: #fff;
}

.workflow-output--assistant {
  border-color: #d6e4ff;
}

.workflow-output--tool {
  border-color: #d9f7be;
}

.workflow-output--system {
  border-color: #ffe58f;
}

.workflow-output--error {
  border-color: #ffccc7;
}

.workflow-output__header {
  margin-bottom: 8px;
}

.workflow-output__time,
.workflow-event__time,
.interaction-item__time {
  color: #98a2b3;
  font-size: 12px;
}

.workflow-output__text,
.workflow-event__message,
.interaction-item__content {
  white-space: pre-line;
  color: #111827;
  line-height: 1.6;
}

.workflow-output__detail,
.workflow-event__data {
  margin: 10px 0 0;
  padding: 12px;
  border-radius: 12px;
  background: #0f172a;
  color: #e2e8f0;
  overflow: auto;
  font-size: 12px;
}

.workflow-event__head,
.interaction-item__header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
}

.workflow-event__stage {
  font-weight: 600;
  color: #111827;
}

.interaction-item__actions {
  margin-top: 10px;
}

.interaction-item__response {
  margin-top: 10px;
  padding: 10px 12px;
  border-radius: 12px;
  background: #f8fafc;
}

.interaction-item__response-label {
  margin-bottom: 4px;
  font-size: 12px;
  color: #667085;
}

.workflow-legend {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.workflow-legend__item {
  display: flex;
  align-items: center;
  gap: 10px;
}

.workflow-legend__dot {
  width: 12px;
  height: 12px;
  border-radius: 999px;
  display: inline-block;
}

@media (max-width: 1199px) {
  .workflow-panel__summary {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>