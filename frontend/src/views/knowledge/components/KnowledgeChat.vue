<template>
  <div class="knowledge-chat">
    <div class="chat-toolbar">
      <a-select
        v-model:value="selectedAgentId"
        class="agent-select"
        allow-clear
        :loading="agentsLoading"
        placeholder="默认 Agent"
        :options="agentOptions"
      />
      <a-input-number v-model:value="topK" :min="1" :max="20" addon-before="Top K" />
      <a-button :disabled="messages.length === 0 || loading" @click="clearMessages">清空</a-button>
    </div>

    <div ref="messageListRef" class="message-list">
      <a-empty v-if="messages.length === 0" description="输入问题后，AI 会结合知识库检索结果生成回答" />
      <div v-for="item in messages" :key="item.id" :class="['message-row', `message-row--${item.role}`]">
        <div :class="['message-bubble', `message-bubble--${item.role}`]">
          <div class="message-meta">
            <a-tag size="small" :color="item.role === 'assistant' ? 'geekblue' : 'green'">
              {{ item.role === 'assistant' ? 'AI' : '我' }}
            </a-tag>
            <span v-if="item.agentName" class="agent-name">{{ item.agentName }}</span>
          </div>
          <div v-if="item.role === 'assistant'" class="markdown-body" v-html="renderMarkdown(item.content)" />
          <div v-else class="plain-text">{{ item.content }}</div>
          <a-collapse v-if="item.sources?.length" class="sources-collapse" ghost>
            <a-collapse-panel key="sources" :header="`引用来源 ${item.sources.length}`">
              <a-list :data-source="item.sources" size="small">
                <template #renderItem="{ item: source, index }">
                  <a-list-item>
                    <div class="source-item">
                      <div class="source-title">
                        #{{ index + 1 }} · Score {{ Number(source.score || 0).toFixed(4) }} · Chunk {{ source.metadata?.chunk_id || '-' }}
                      </div>
                      <div class="source-content">{{ source.content }}</div>
                    </div>
                  </a-list-item>
                </template>
              </a-list>
            </a-collapse-panel>
          </a-collapse>
        </div>
      </div>
      <div v-if="loading" class="message-row message-row--assistant">
        <div class="message-bubble message-bubble--assistant">
          <a-spin size="small" />
          <span class="thinking-text">正在检索知识库并调用 Agent...</span>
        </div>
      </div>
    </div>

    <div class="composer">
      <a-textarea
        v-model:value="draft"
        :auto-size="{ minRows: 2, maxRows: 5 }"
        placeholder="询问知识库内容，例如：这个项目的测试生成规范是什么？"
        :disabled="loading"
        @keydown="onComposerKeydown"
      />
      <a-button type="primary" :loading="loading" :disabled="!draft.trim()" @click="sendMessage">
        <template #icon><SendOutlined /></template>
        发送
      </a-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { SendOutlined } from '@ant-design/icons-vue'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'
import mermaid from 'mermaid'
import { getAgentList } from '@/api/agent'
import { chatKnowledgeBase, type KnowledgeSearchResult } from '@/api/knowledge'

const props = defineProps<{ kbId: number; projectId: number }>()

type ChatRole = 'user' | 'assistant'

interface ChatMessage {
  id: string
  role: ChatRole
  content: string
  sources?: KnowledgeSearchResult[]
  agentName?: string
}

interface AgentOption {
  label: string
  value: number
  isDefault?: boolean
}

const markdownRenderer = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
})

const defaultFenceRenderer = markdownRenderer.renderer.rules.fence?.bind(markdownRenderer.renderer.rules)
markdownRenderer.renderer.rules.fence = (tokens, idx, options, env, self) => {
  const token = tokens[idx]
  const lang = token.info.trim().split(/\s+/)[0]?.toLowerCase()
  if (lang === 'mermaid') {
    const encoded = encodeURIComponent(token.content)
    return `<div class="mermaid-block" data-mermaid="${encoded}"></div>`
  }
  if (defaultFenceRenderer) {
    return defaultFenceRenderer(tokens, idx, options, env, self)
  }
  return self.renderToken(tokens, idx, options)
}

mermaid.initialize({
  startOnLoad: false,
  securityLevel: 'strict',
  theme: 'default',
  flowchart: {
    htmlLabels: false,
  },
})

const loading = ref(false)
const agentsLoading = ref(false)
const draft = ref('')
const topK = ref(5)
const selectedAgentId = ref<number>()
const agentOptions = ref<AgentOption[]>([])
const messages = ref<ChatMessage[]>([])
const messageListRef = ref<HTMLElement>()

const historyPayload = computed(() =>
  messages.value
    .filter((item) => item.role === 'user' || item.role === 'assistant')
    .slice(-12)
    .map((item) => ({ role: item.role, content: item.content })),
)

async function loadAgents() {
  agentsLoading.value = true
  try {
    const res = await getAgentList({ page: 1, page_size: 100, status: 1 })
    const list = res.data.data?.list || []
    agentOptions.value = list.map((item: any) => ({
      label: item.is_default ? `${item.name}（默认）` : item.name,
      value: item.id,
      isDefault: !!item.is_default,
    }))
    if (!selectedAgentId.value && agentOptions.value.length > 0) {
      selectedAgentId.value = agentOptions.value.find((item) => item.isDefault)?.value || agentOptions.value[0].value
    }
  } finally {
    agentsLoading.value = false
  }
}

async function sendMessage() {
  const content = draft.value.trim()
  if (!content || loading.value) return

  const history = historyPayload.value
  messages.value.push({ id: messageId(), role: 'user', content })
  draft.value = ''
  await nextTick()
  loading.value = true
  await scrollToBottom()

  try {
    const res = await chatKnowledgeBase(props.kbId, {
      project_id: props.projectId,
      query: content,
      top_k: topK.value,
      agent_id: selectedAgentId.value,
      messages: history,
    })
    const data = res.data.data || {}
    messages.value.push({
      id: messageId(),
      role: 'assistant',
      content: data.answer || '未生成有效回答。',
      sources: data.sources || [],
      agentName: data.agent?.name,
    })
    await renderMermaidBlocks()
  } catch {
    // The shared request interceptor already surfaces the failure.
  } finally {
    loading.value = false
    await scrollToBottom()
  }
}

function clearMessages() {
  messages.value = []
}

function renderMarkdown(text: string) {
  return DOMPurify.sanitize(markdownRenderer.render(text || ''), {
    ADD_ATTR: ['data-mermaid'],
  })
}

function onComposerKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
    event.preventDefault()
    sendMessage()
  }
}

function messageId() {
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

async function scrollToBottom() {
  await nextTick()
  const el = messageListRef.value
  if (el) {
    el.scrollTop = el.scrollHeight
  }
}

async function renderMermaidBlocks() {
  await nextTick()
  const root = messageListRef.value
  if (!root) return
  const blocks = Array.from(root.querySelectorAll<HTMLElement>('.mermaid-block[data-mermaid]:not([data-rendered])'))
  for (const block of blocks) {
    const source = decodeURIComponent(block.dataset.mermaid || '').trim()
    if (!source) continue
    block.dataset.rendered = 'true'
    try {
      const id = `kb-mermaid-${Date.now()}-${Math.random().toString(16).slice(2)}`
      const rendered = await mermaid.render(id, source)
      block.innerHTML = rendered.svg
    } catch (error: any) {
      block.classList.add('mermaid-block--error')
      block.textContent = `Mermaid 渲染失败：${error?.message || error}`
    }
  }
}

watch(
  () => messages.value.map((item) => item.content).join('\n---\n'),
  () => {
    renderMermaidBlocks()
  },
  { flush: 'post' },
)

onMounted(loadAgents)
</script>

<style scoped>
.knowledge-chat {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  gap: 12px;
  height: max(520px, calc(100dvh - 260px));
  min-height: 0;
  overflow: hidden;
}

.chat-toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.agent-select {
  width: 260px;
}

.message-list {
  min-height: 0;
  overflow: auto;
  padding: 16px;
  border: 1px solid #eaecf0;
  background: #f8fafc;
}

.message-row {
  display: flex;
  margin-bottom: 14px;
}

.message-row--user {
  justify-content: flex-end;
}

.message-row--assistant {
  justify-content: flex-start;
}

.message-bubble {
  width: min(780px, 84%);
  padding: 12px 14px;
  border: 1px solid #eaecf0;
  border-radius: 8px;
  background: #fff;
}

.message-bubble--user {
  border-color: #b7eb8f;
  background: #f6ffed;
}

.message-meta {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}

.agent-name,
.thinking-text {
  color: #667085;
  font-size: 12px;
}

.thinking-text {
  margin-left: 8px;
}

.plain-text,
.markdown-body {
  white-space: pre-wrap;
  color: #1f2937;
  line-height: 1.7;
}

.markdown-body :deep(p) {
  margin: 0 0 8px;
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin: 0 0 8px 20px;
  padding: 0;
}

.markdown-body :deep(table) {
  width: 100%;
  margin: 8px 0;
  border-collapse: collapse;
  font-size: 14px;
}

.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid #e5e7eb;
  padding: 8px 12px;
  text-align: left;
  white-space: normal;
  word-break: break-word;
}

.markdown-body :deep(th) {
  background: #f3f4f6;
  font-weight: 600;
}

.markdown-body :deep(tr:nth-child(even) td) {
  background: #f9fafb;
}

.markdown-body :deep(pre) {
  overflow: auto;
  padding: 12px;
  border-radius: 6px;
  background: #111827;
  color: #f9fafb;
}

.markdown-body :deep(code) {
  font-family: Consolas, Monaco, 'Courier New', monospace;
}

.markdown-body :deep(.mermaid-block) {
  overflow: auto;
  max-height: 500px;
  margin: 12px 0;
  padding: 12px;
  border: 1px solid #d0d5dd;
  border-radius: 8px;
  background: #fff;
}

.markdown-body :deep(.mermaid-block svg) {
  max-width: 100% !important;
  height: auto;
}

.markdown-body :deep(.mermaid-block--error) {
  color: #b42318;
  white-space: pre-wrap;
}

.sources-collapse {
  margin-top: 8px;
}

.source-item {
  width: 100%;
}

.source-title {
  margin-bottom: 4px;
  color: #475467;
  font-size: 12px;
}

.source-content {
  max-height: 120px;
  overflow: auto;
  white-space: pre-wrap;
  color: #344054;
  font-size: 13px;
}

.composer {
  position: sticky;
  bottom: 0;
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 12px;
  align-items: end;
  padding-top: 4px;
  background: #fff;
}

@media (max-height: 760px) {
  .knowledge-chat {
    height: calc(100dvh - 210px);
    min-height: 400px;
  }

  .chat-toolbar {
    gap: 8px;
  }

  .message-list {
    padding: 12px;
  }
}
</style>
