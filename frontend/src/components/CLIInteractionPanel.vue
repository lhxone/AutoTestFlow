<template>
  <a-card title="AI 交互面板" size="small" style="margin-top: 16px">
    <template #extra>
      <a-tag :color="pendingCount > 0 ? 'orange' : 'green'">
        待处理: {{ pendingCount }}
      </a-tag>
    </template>

    <a-spin :spinning="loading">
      <div class="chat-section">
        <div class="section-title">AI Agent 对话</div>
        <div v-if="chatMessages.length === 0" class="empty-state chat-empty">
          <a-empty description="暂无 AI 对话输出" />
        </div>
        <div v-else class="chat-list">
          <div
            v-for="item in chatMessages"
            :key="`${item.id}-${item.timestamp}`"
            :class="['chat-row', `chat-row--${item.role}`]"
          >
            <div :class="['chat-bubble', `chat-bubble--${item.role}`]">
              <div class="chat-meta">
                <a-tag size="small" :color="chatRoleColor(item.role)">{{ chatRoleLabel(item.role) }}</a-tag>
                <span class="chat-time">{{ formatTime(item.timestamp) }}</span>
              </div>
              <div class="chat-main-row">
                <div class="chat-text">{{ item.text }}</div>
                <a-button
                  v-if="item.detail"
                  type="link"
                  size="small"
                  class="chat-detail-toggle"
                  @click="toggleDetail(item)"
                >
                  {{ isDetailVisible(item) ? '收起详情' : '展开详情' }}
                </a-button>
              </div>
              <pre v-if="item.detail && isDetailVisible(item)" class="chat-detail">{{ item.detail }}</pre>
            </div>
          </div>
        </div>
      </div>

      <a-divider style="margin: 12px 0" />

      <div class="interaction-section">
        <div class="section-title">待处理交互</div>
        <div class="interaction-list">
          <div v-if="interactions.length === 0" class="empty-state">
            <a-empty description="暂无交互记录" />
          </div>
          <a-timeline v-else>
            <a-timeline-item v-for="interaction in interactions" :key="interaction.id">
              <div class="interaction-item">
                <div class="interaction-header">
                  <a-tag :color="getInteractionTypeColor(interaction.interaction_type)">
                    {{ getInteractionTypeLabel(interaction.interaction_type) }}
                  </a-tag>
                  <a-tag :color="getStatusColor(interaction.status)">
                    {{ getStatusLabel(interaction.status) }}
                  </a-tag>
                  <span class="interaction-time">{{ formatTime(interaction.created_at) }}</span>
                </div>

                <div class="interaction-content">
                  <div class="ai-message">
                    {{ interaction.content }}
                  </div>

                  <div v-if="interaction.status === 'pending'" class="interaction-actions">
                    <a-input
                      v-model="responses[interaction.id]"
                      placeholder="请输入回复..."
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
                          回复
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
                        批准
                      </a-button>
                      <a-button
                        danger
                        size="small"
                        :loading="actionLoading[interaction.id]"
                        @click="handleReject(interaction.id)"
                      >
                        拒绝
                      </a-button>
                    </a-space>
                  </div>

                  <div v-else-if="interaction.user_response" class="user-response">
                    <div class="response-label">您的回复:</div>
                    <div class="response-content">{{ interaction.user_response }}</div>
                  </div>
                </div>
              </div>
            </a-timeline-item>
          </a-timeline>
        </div>
      </div>
    </a-spin>
  </a-card>
</template>

<script setup lang="ts">
import { computed, reactive, ref, onMounted, watch } from 'vue'
import { message } from 'ant-design-vue'
import {
  getCLIInteractions,
  getPendingInteractions,
  getTaskLogs,
  replyInteraction,
  approveInteraction,
  rejectInteraction,
  type CLIInteraction,
} from '@/api/testTask'
import type { TestTaskEvent } from '@/types'

interface Props {
  taskId: number
  taskEvents?: TestTaskEvent[]
}

const props = defineProps<Props>()

const loading = ref(false)
const actionLoading = ref<Record<number, boolean>>({})
const interactions = ref<CLIInteraction[]>([])
const taskLogs = ref<TestTaskEvent[]>([])
const responses = ref<Record<number, string>>({})
const pendingCount = ref(0)

const loadInteractions = async () => {
  if (!props.taskId) return
  try {
    const res = await getCLIInteractions(props.taskId)
    interactions.value = res.data?.data || []
    pendingCount.value = interactions.value.filter(i => i.status === 'pending').length
  } catch (error) {
    message.error('加载交互记录失败')
  }
}

const loadTaskLogs = async () => {
  if (!props.taskId) return
  try {
    const res = await getTaskLogs(props.taskId)
    taskLogs.value = res.data?.data || []
  } catch {
    taskLogs.value = []
  }
}

const loadPanelData = async () => {
  loading.value = true
  try {
    const jobs: Promise<unknown>[] = [loadInteractions()]
    if (!props.taskEvents || props.taskEvents.length === 0) {
      jobs.push(loadTaskLogs())
    }
    await Promise.all(jobs)
  } finally {
    loading.value = false
  }
}

const loadPendingInteractions = async () => {
  if (!props.taskId) return
  try {
    const res = await getPendingInteractions(props.taskId)
    const pending = res.data?.data || []
    pendingCount.value = pending.length
  } catch (error) {
    console.error('加载待处理交互失败:', error)
  }
}

const handleReply = async (interactionId: number) => {
  const response = responses.value[interactionId]
  if (!response?.trim()) {
    message.warning('请输入回复内容')
    return
  }

  actionLoading.value[interactionId] = true
  try {
    await replyInteraction(props.taskId, interactionId, response)
    message.success('回复成功')
    responses.value[interactionId] = ''
    await loadInteractions()
  } catch (error) {
    message.error('回复失败')
  } finally {
    actionLoading.value[interactionId] = false
  }
}

const handleApprove = async (interactionId: number) => {
  actionLoading.value[interactionId] = true
  try {
    await approveInteraction(props.taskId, interactionId)
    message.success('批准成功')
    await loadInteractions()
  } catch (error) {
    message.error('批准失败')
  } finally {
    actionLoading.value[interactionId] = false
  }
}

const handleReject = async (interactionId: number) => {
  actionLoading.value[interactionId] = true
  try {
    await rejectInteraction(props.taskId, interactionId, '用户拒绝')
    message.success('已拒绝')
    await loadInteractions()
  } catch (error) {
    message.error('拒绝失败')
  } finally {
    actionLoading.value[interactionId] = false
  }
}

const getInteractionTypeColor = (type: string) => {
  switch (type) {
    case 'ai_question':
      return 'blue'
    case 'permission_request':
      return 'orange'
    default:
      return 'default'
  }
}

const getInteractionTypeLabel = (type: string) => {
  switch (type) {
    case 'ai_question':
      return 'AI 提问'
    case 'permission_request':
      return '权限请求'
    default:
      return type
  }
}

const getStatusColor = (status: string) => {
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

const getStatusLabel = (status: string) => {
  switch (status) {
    case 'pending':
      return '待处理'
    case 'approved':
      return '已批准'
    case 'rejected':
      return '已拒绝'
    case 'answered':
      return '已回复'
    default:
      return status
  }
}

const formatTime = (time: string) => {
  return new Date(time).toLocaleString('zh-CN')
}

onMounted(() => {
  loadPanelData()
})

watch(() => props.taskId, () => {
  if (props.taskId) {
    loadPanelData()
  }
})

watch(
  () => props.taskEvents,
  (events) => {
    if (events && events.length) {
      taskLogs.value = events.slice(-400)
    }
  },
  { immediate: true },
)

type ChatRole = 'assistant' | 'user' | 'tool' | 'system' | 'error'

type ChatMessage = {
  id: string
  role: ChatRole
  text: string
  detail?: string
  detailCollapsedDefault?: boolean
  timestamp: string
}
const detailVisibleState = reactive<Record<string, boolean>>({})

const chatMessages = computed<ChatMessage[]>(() => {
  const sourceEvents = (props.taskEvents && props.taskEvents.length > 0) ? props.taskEvents : taskLogs.value
  const rows: ChatMessage[] = []
  for (const event of sourceEvents) {
    if (event.type !== 'log' || !event.message) {
      continue
    }
    if (event.stage !== 'cli_output' && event.stage !== 'cli_output_raw') {
      continue
    }
    const parsed = parseAgentEvent(event)
    if (parsed.length) {
      rows.push(...parsed)
    }
  }
  return rows.slice(-120)
})

function parseAgentEvent(event: TestTaskEvent): ChatMessage[] {
  const raw = (event.message || '').trim()
  if (!raw || raw[0] !== '{') {
    return []
  }

  let payload: any
  try {
    payload = JSON.parse(raw)
  } catch {
    return []
  }

  const ts = event.timestamp
  const baseId = String(event.id || `${event.task_id}-${ts}`)
  const type = String(payload?.type || '')
  if (type === 'assistant') {
    return parseAssistantPayload(payload, ts, baseId)
  }
  if (type === 'user') {
    return parseUserPayload(payload, ts, baseId)
  }
  if (type === 'system') {
    const content = String(payload?.message || payload?.subtype || '系统事件')
    return [{ id: `${baseId}-system`, role: 'system', text: content, timestamp: ts }]
  }
  if (type === 'result') {
    const isError = Boolean(payload?.is_error)
    const text = isError ? 'AI 运行失败' : 'AI 运行完成'
    const detail = stringifyBrief(payload?.result)
    return [{ id: `${baseId}-result`, role: isError ? 'error' : 'system', text, detail, timestamp: ts }]
  }
  if (type.includes('.')) {
    return parseCodexEvent(payload, ts, baseId)
  }
  return []
}

function parseCodexEvent(payload: any, timestamp: string, baseId: string): ChatMessage[] {
  const eventType = String(payload?.type || '')
  if (!eventType.includes('.')) {
    return []
  }

  if (eventType === 'thread.started') {
    return [{ id: `${baseId}-codex-thread-started`, role: 'system', text: 'Codex 会话已启动', timestamp }]
  }
  if (eventType === 'thread.completed') {
    return [{ id: `${baseId}-codex-thread-completed`, role: 'system', text: 'Codex 会话已完成', timestamp }]
  }
  if (eventType === 'turn.started' || eventType === 'turn.completed') {
    return []
  }

  if (!eventType.startsWith('item.')) {
    return []
  }

  const item = payload?.item || {}
  const itemType = String(item?.type || '')
  const eventState = eventType.slice('item.'.length)

  if (itemType === 'reasoning') {
    if (eventState !== 'completed') {
      return []
    }
    const text = String(item?.text || '').trim()
    if (!text) {
      return []
    }
    const lines = text.split('\n').filter(Boolean)
    return [{
      id: `${baseId}-codex-reasoning`,
      role: 'assistant',
      text: lines[0].length > 180 ? `${lines[0].slice(0, 180)}...` : lines[0],
      detail: text,
      detailCollapsedDefault: true,
      timestamp,
    }]
  }

  if (itemType === 'todo_list') {
    const todos = Array.isArray(item?.items) ? item.items : []
    if (!todos.length) {
      return []
    }
    const completed = todos.filter((t: any) => Boolean(t?.completed)).length
    const summary = `任务进度 ${completed}/${todos.length}`
    const detail = todos
      .map((t: any) => `${t?.completed ? '[x]' : '[ ]'} ${String(t?.text || '').trim()}`)
      .join('\n')
    return [{
      id: `${baseId}-codex-todo`,
      role: 'system',
      text: summary,
      detail,
      detailCollapsedDefault: true,
      timestamp,
    }]
  }

  if (itemType === 'command_execution') {
    const command = String(item?.command || '').trim()
    const output = String(item?.aggregated_output || '').trim()
    const exitCode = typeof item?.exit_code === 'number' ? item.exit_code : null
    const status = String(item?.status || '').trim()

    // started 事件只显示命令摘要；completed 事件展示结果。
    if (eventState === 'started') {
      if (!command) {
        return []
      }
      return [{
        id: `${baseId}-codex-cmd-started`,
        role: 'tool',
        text: `执行命令: ${command.length > 140 ? `${command.slice(0, 140)}...` : command}`,
        detailCollapsedDefault: true,
        timestamp,
      }]
    }

    if (eventState === 'completed') {
      const ok = exitCode === 0
      const text = ok
        ? `命令执行成功${command ? `: ${command.length > 100 ? `${command.slice(0, 100)}...` : command}` : ''}`
        : `命令执行失败${exitCode == null ? '' : ` (exit=${exitCode})`}`
      return [{
        id: `${baseId}-codex-cmd-completed`,
        role: ok ? 'tool' : 'error',
        text,
        detail: stringifyBrief({ status, command, exit_code: exitCode, aggregated_output: output }),
        detailCollapsedDefault: true,
        timestamp,
      }]
    }
  }

  if (itemType === 'message') {
    const text = String(item?.text || '').trim()
    if (!text) {
      return []
    }
    return [{ id: `${baseId}-codex-message`, role: 'assistant', text, timestamp }]
  }

  return []
}

function parseAssistantPayload(payload: any, timestamp: string, baseId: string): ChatMessage[] {
  const results: ChatMessage[] = []
  const model = String(payload?.message?.model || '').trim()
  const contents = Array.isArray(payload?.message?.content) ? payload.message.content : []
  for (let idx = 0; idx < contents.length; idx += 1) {
    const item = contents[idx] || {}
    const kind = String(item?.type || '')
    if (kind === 'text') {
      const text = String(item?.text || '').trim()
      if (!text) continue
      results.push({
        id: `${baseId}-assistant-text-${idx}`,
        role: 'assistant',
        text: text.length > 600 ? `${text.slice(0, 600)}...` : text,
        timestamp,
      })
      continue
    }
    if (kind === 'thinking') {
      const thinking = String(item?.thinking || '').trim()
      if (!thinking) continue
      results.push({
        id: `${baseId}-assistant-thinking-${idx}`,
        role: 'assistant',
        text: model ? `思考中 (${model})` : '思考中',
        detail: thinking.length > 800 ? `${thinking.slice(0, 800)}...` : thinking,
        timestamp,
      })
      continue
    }
    if (kind === 'tool_use') {
      const name = String(item?.name || 'unknown_tool')
      results.push({
        id: `${baseId}-assistant-tool-${idx}`,
        role: 'assistant',
        text: `调用工具: ${name}`,
        detail: stringifyBrief(item?.input),
        detailCollapsedDefault: true,
        timestamp,
      })
    }
  }
  return results
}

function parseUserPayload(payload: any, timestamp: string, baseId: string): ChatMessage[] {
  const results: ChatMessage[] = []
  const contents = Array.isArray(payload?.message?.content) ? payload.message.content : []
  for (let idx = 0; idx < contents.length; idx += 1) {
    const item = contents[idx] || {}
    const kind = String(item?.type || '')
    if (kind === 'tool_result') {
      const isError = Boolean(item?.is_error)
      results.push({
        id: `${baseId}-tool-result-${idx}`,
        role: isError ? 'error' : 'tool',
        text: isError ? '工具返回错误' : '工具执行结果',
        detail: stringifyBrief(item?.content),
        detailCollapsedDefault: true,
        timestamp,
      })
      continue
    }
    if (kind === 'text') {
      const text = String(item?.text || '').trim()
      if (!text) continue
      results.push({
        id: `${baseId}-user-text-${idx}`,
        role: 'user',
        text,
        timestamp,
      })
    }
  }
  return results
}

function stringifyBrief(value: any): string {
  if (value == null) {
    return ''
  }
  if (typeof value === 'string') {
    return value.length > 1200 ? `${value.slice(0, 1200)}...` : value
  }
  try {
    const formatted = JSON.stringify(value, null, 2)
    return formatted.length > 1200 ? `${formatted.slice(0, 1200)}...` : formatted
  } catch {
    return String(value)
  }
}

const chatRoleLabel = (role: ChatRole) => {
  switch (role) {
    case 'assistant':
      return 'AI'
    case 'user':
      return 'User'
    case 'tool':
      return 'Tool'
    case 'system':
      return 'System'
    case 'error':
      return 'Error'
    default:
      return 'Event'
  }
}

const chatRoleColor = (role: ChatRole) => {
  switch (role) {
    case 'assistant':
      return 'blue'
    case 'user':
      return 'cyan'
    case 'tool':
      return 'purple'
    case 'system':
      return 'default'
    case 'error':
      return 'red'
    default:
      return 'default'
  }
}

const isDetailVisible = (item: ChatMessage) => {
  const current = detailVisibleState[item.id]
  if (typeof current === 'boolean') {
    return current
  }
  return !item.detailCollapsedDefault
}

const toggleDetail = (item: ChatMessage) => {
  const current = detailVisibleState[item.id]
  if (typeof current === 'boolean') {
    detailVisibleState[item.id] = !current
    return
  }
  detailVisibleState[item.id] = !!item.detailCollapsedDefault
}

defineExpose({
  loadPanelData,
  loadTaskLogs,
  loadInteractions,
  loadPendingInteractions
})
</script>

<style scoped>
.section-title {
  font-size: 13px;
  font-weight: 600;
  color: #333;
  margin-bottom: 8px;
}

.empty-state {
  padding: 24px;
  text-align: center;
}

.chat-empty {
  padding: 8px 0 12px;
}

.chat-list {
  max-height: 680px;
  overflow-y: auto;
  padding-right: 4px;
}

.interaction-list {
  max-height: 180px;
  overflow-y: auto;
  padding-right: 4px;
}

.chat-row {
  display: flex;
  margin-bottom: 10px;
}

.chat-row--assistant,
.chat-row--system,
.chat-row--error,
.chat-row--tool {
  justify-content: flex-start;
}

.chat-row--user {
  justify-content: flex-end;
}

.chat-bubble {
  max-width: 85%;
  border-radius: 10px;
  padding: 8px 10px;
  line-height: 1.45;
  border: 1px solid #e8e8e8;
  background: #fff;
}

.chat-bubble--assistant {
  background: #f0f5ff;
  border-color: #d6e4ff;
}

.chat-bubble--user {
  background: #e6fffb;
  border-color: #b5f5ec;
}

.chat-bubble--tool {
  background: #f9f0ff;
  border-color: #efdbff;
}

.chat-bubble--system {
  background: #fafafa;
  border-color: #e8e8e8;
}

.chat-bubble--error {
  background: #fff1f0;
  border-color: #ffccc7;
}

.chat-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
}

.chat-time {
  font-size: 12px;
  color: #999;
}

.chat-text {
  flex: 1;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 13px;
}

.chat-main-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.chat-detail-toggle {
  margin-top: 0;
  height: 20px;
  line-height: 20px;
  font-size: 12px;
  flex-shrink: 0;
  padding-left: 0;
}

.chat-detail {
  margin-top: 8px;
  background: #1f1f1f;
  color: #d9d9d9;
  padding: 8px;
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.5;
  max-height: 180px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

.interaction-item {
  margin-bottom: 16px;
}

.interaction-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.interaction-time {
  color: #999;
  font-size: 12px;
  margin-left: auto;
}

.interaction-content {
  background: #f5f5f5;
  padding: 12px;
  border-radius: 4px;
}

.ai-message {
  margin-bottom: 8px;
  line-height: 1.6;
}

.interaction-actions {
  margin-top: 12px;
  border-top: 1px solid #e8e8e8;
  padding-top: 12px;
}

.user-response {
  margin-top: 12px;
  border-top: 1px solid #e8e8e8;
  padding-top: 12px;
}

.response-label {
  font-size: 12px;
  color: #999;
  margin-bottom: 4px;
}

.response-content {
  color: #52c41a;
  padding: 8px;
  background: #f6ffed;
  border: 1px solid #b7eb8f;
  border-radius: 4px;
}
</style>
