<template>
  <div class="knowledge-chat">
    <div class="chat-toolbar">
      <a-select
        v-model:value="selectedAgentId"
        class="agent-select"
        allow-clear
        :loading="agentsLoading"
        :placeholder="t('knowledgeChat.defaultAgent')"
        :options="agentOptions"
      />
      <a-input-number v-model:value="topK" :min="1" :max="20" addon-before="Top K" />
      <a-button :disabled="messages.length === 0 || loading" @click="clearMessages">{{ t('knowledgeChat.clear') }}</a-button>
    </div>

    <div ref="messageListRef" class="message-list">
      <a-empty v-if="messages.length === 0" :description="t('knowledgeChat.emptyHint')" />
      <div v-for="item in messages" :key="item.id" :class="['message-row', `message-row--${item.role}`]">
        <div :class="['message-bubble', `message-bubble--${item.role}`]">
          <div class="message-meta">
            <a-tag size="small" :color="item.role === 'assistant' ? 'geekblue' : 'green'">
              {{ item.role === 'assistant' ? t('knowledgeChat.ai') : t('knowledgeChat.me') }}
            </a-tag>
            <span v-if="item.agentName" class="agent-name">{{ item.agentName }}</span>
          </div>
          <div v-if="item.sources?.length" class="sources-section">
            <div class="sources-header">
              <FileSearchOutlined />
              <span>{{ t('knowledgeChat.searchSources', { count: item.sources.length }) }}</span>
            </div>
            <a-collapse class="sources-collapse" ghost>
              <a-collapse-panel key="sources" :header="t('knowledgeChat.viewSources')">
                <a-list :data-source="item.sources" size="small">
                  <template #renderItem="{ item: source, index }">
                    <a-list-item>
                      <div class="source-item">
                        <div class="source-title">
                          <span class="source-file">{{ source.metadata?.title || source.metadata?.source_path || t('knowledgeChat.unknownFile') }}</span>
                          <span class="source-score">Score {{ Number(source.score || 0).toFixed(4) }}</span>
                        </div>
                        <div class="source-content">{{ source.content }}</div>
                      </div>
                    </a-list-item>
                  </template>
                </a-list>
              </a-collapse-panel>
            </a-collapse>
          </div>
          <div v-if="item.thinking" class="thinking-section">
            <a-spin size="small" />
            <span class="thinking-text">{{ item.thinking }}</span>
          </div>
          <div v-if="item.role === 'assistant'" class="markdown-body" v-html="renderMarkdown(item.content)" />
          <div v-else class="plain-text">{{ item.content }}</div>
        </div>
      </div>
      <div v-if="loading" class="message-row message-row--assistant">
        <div class="message-bubble message-bubble--assistant">
          <a-spin size="small" />
          <span class="thinking-text">{{ t('knowledgeChat.thinking') }}</span>
        </div>
      </div>
    </div>

    <div class="composer">
      <a-textarea
        v-model:value="draft"
        :auto-size="{ minRows: 2, maxRows: 5 }"
        :placeholder="t('knowledgeChat.placeholder')"
        :disabled="loading"
        @keydown="onComposerKeydown"
      />
      <a-button type="primary" :loading="loading" :disabled="!draft.trim()" @click="sendMessage">
        <template #icon><SendOutlined /></template>
        {{ t('knowledgeChat.send') }}
      </a-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { SendOutlined, FileSearchOutlined, CopyOutlined, CheckOutlined, DownloadOutlined, ZoomInOutlined, ZoomOutOutlined, ExpandOutlined } from '@ant-design/icons-vue'
import { useI18n } from 'vue-i18n'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'
import mermaid from 'mermaid'
import hljs from 'highlight.js'
import 'highlight.js/styles/github-dark.css'
import svgPanZoom from 'svg-pan-zoom'
import { getAgentList } from '@/api/agent'
import { chatKnowledgeBaseStream, type ChatStreamEvent, type KnowledgeSearchResult } from '@/api/knowledge'

const props = defineProps<{ kbId: number; projectId: number }>()
const { t } = useI18n()

type ChatRole = 'user' | 'assistant'

interface ChatMessage {
  id: string
  role: ChatRole
  content: string
  sources?: KnowledgeSearchResult[]
  agentName?: string
  thinking?: string
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
  highlight(str: string, lang: string): string {
    const validLang = lang && hljs.getLanguage(lang)
    const highlighted: string = validLang
      ? hljs.highlight(str, { language: lang, ignoreIllegals: true }).value
      : markdownRenderer.utils.escapeHtml(str)
    const codeId = `code-${Date.now()}-${Math.random().toString(16).slice(2)}`
    return `<div class="code-block" data-code-id="${codeId}">
      <div class="code-header">
        <span class="code-lang">${lang || 'text'}</span>
        <button class="code-copy-btn" onclick="window.__copyCode('${codeId}')" title="${t('knowledgeChat.copy')}">
          <span class="copy-icon">${t('knowledgeChat.copy')}</span>
        </button>
      </div>
      <pre><code class="hljs language-${lang || 'text'}">${highlighted}</code></pre>
    </div>`
  },
})

const defaultFenceRenderer = markdownRenderer.renderer.rules.fence?.bind(markdownRenderer.renderer.rules)
markdownRenderer.renderer.rules.fence = (tokens: any, idx: number, options: any, env: any, self: any): string => {
  const token = tokens[idx]
  const lang = token.info.trim().split(/\s+/)[0]?.toLowerCase()
  if (lang === 'mermaid') {
    const content = token.content.trim()
    if (!content) {
      return ''
    }
    const encoded = encodeURIComponent(content)
    return `<div class="mermaid-block" data-mermaid="${encoded}" data-pending="true">
      <div class="mermaid-toolbar">
        <button class="mermaid-btn mermaid-zoom-in" title="${t('knowledgeChat.zoomIn')}"><span>+</span></button>
        <button class="mermaid-btn mermaid-zoom-out" title="${t('knowledgeChat.zoomOut')}"><span>-</span></button>
        <button class="mermaid-btn mermaid-reset" title="${t('knowledgeChat.reset')}"><span>⟲</span></button>
        <button class="mermaid-btn mermaid-export" title="${t('knowledgeChat.exportPng')}"><span>↓</span></button>
      </div>
      <div class="mermaid-content"></div>
    </div>`
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
const topK = ref(10)
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

  const assistantMsg = reactive<ChatMessage>({
    id: messageId(),
    role: 'assistant',
    content: '',
    sources: [],
    agentName: '',
    thinking: '',
  })
  messages.value.push(assistantMsg)
  await scrollToBottom()

  chatKnowledgeBaseStream(
    props.kbId,
    {
      project_id: props.projectId,
      query: content,
      top_k: topK.value,
      agent_id: selectedAgentId.value,
      messages: history,
    },
    async (event: ChatStreamEvent) => {
      switch (event.type) {
        case 'sources':
          assistantMsg.sources = event.sources || []
          assistantMsg.agentName = event.agent?.name
          break
        case 'thinking':
          assistantMsg.thinking = event.content || ''
          break
        case 'content':
          assistantMsg.content += event.content || ''
          await scrollToBottom()
          break
        case 'done':
          assistantMsg.thinking = ''
          loading.value = false
          markMermaidReady()
          await renderMermaidBlocks(true)
          await scrollToBottom()
          break
        case 'error':
          assistantMsg.content = event.content || '发生错误'
          assistantMsg.thinking = ''
          loading.value = false
          await scrollToBottom()
          break
      }
    },
    async (err) => {
      assistantMsg.content = `请求失败: ${err.message}`
      assistantMsg.thinking = ''
      loading.value = false
      await scrollToBottom()
    },
  )
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

function initGlobalHelpers() {
  (window as any).__copyCode = (codeId: string) => {
    const block = document.querySelector(`[data-code-id="${codeId}"]`)
    if (!block) return
    const code = block.querySelector('code')?.textContent || ''
    navigator.clipboard.writeText(code).then(() => {
      const btn = block.querySelector('.code-copy-btn')
      if (btn) {
        const span = btn.querySelector('.copy-icon')
        if (span) {
          span.textContent = t('knowledgeChat.copied')
          setTimeout(() => { span.textContent = t('knowledgeChat.copy') }, 2000)
        }
      }
    })
  }

  const panZoomInstances = new WeakMap<Element, any>()

  document.addEventListener('click', (e: Event) => {
    const btn = (e.target as HTMLElement).closest('.mermaid-btn')
    if (!btn) return
    const container = btn.closest('.mermaid-block')
    if (!container) return

    if (btn.classList.contains('mermaid-zoom-in')) {
      const svgEl = container.querySelector('svg')
      if (!svgEl) return
      let instance = panZoomInstances.get(svgEl)
      if (!instance) {
        instance = svgPanZoom(svgEl as unknown as SVGElement, {
          zoomEnabled: true,
          controlIconsEnabled: false,
          fit: true,
          center: true,
          minZoom: 0.1,
          maxZoom: 10,
        })
        panZoomInstances.set(svgEl, instance)
      }
      instance.zoomBy(1.3)
    } else if (btn.classList.contains('mermaid-zoom-out')) {
      const svgEl = container.querySelector('svg')
      if (!svgEl) return
      let instance = panZoomInstances.get(svgEl)
      if (!instance) {
        instance = svgPanZoom(svgEl as unknown as SVGElement, {
          zoomEnabled: true,
          controlIconsEnabled: false,
          fit: true,
          center: true,
          minZoom: 0.1,
          maxZoom: 10,
        })
        panZoomInstances.set(svgEl, instance)
      }
      instance.zoomBy(0.7)
    } else if (btn.classList.contains('mermaid-reset')) {
      const svgEl = container.querySelector('svg')
      if (!svgEl) return
      const instance = panZoomInstances.get(svgEl)
      if (instance) {
        instance.reset()
      }
    } else if (btn.classList.contains('mermaid-export')) {
      const svgEl = container.querySelector('.mermaid-content svg')
      if (!svgEl) return
      const clone = svgEl.cloneNode(true) as SVGSVGElement
      clone.removeAttribute('style')
      const viewBox = svgEl.getAttribute('viewBox')
      if (viewBox) {
        clone.setAttribute('viewBox', viewBox)
      }
      const width = svgEl.clientWidth || 800
      const height = svgEl.clientHeight || 600
      clone.setAttribute('width', String(width * 2))
      clone.setAttribute('height', String(height * 2))
      const svgData = new XMLSerializer().serializeToString(clone)
      const canvas = document.createElement('canvas')
      const ctx = canvas.getContext('2d')
      if (!ctx) return
      canvas.width = width * 2
      canvas.height = height * 2
      const img = new Image()
      const base64Svg = 'data:image/svg+xml;base64,' + btoa(unescape(encodeURIComponent(svgData)))
      img.onload = () => {
        ctx.fillStyle = '#ffffff'
        ctx.fillRect(0, 0, canvas.width, canvas.height)
        ctx.drawImage(img, 0, 0, canvas.width, canvas.height)
        const pngUrl = canvas.toDataURL('image/png')
        const a = document.createElement('a')
        a.href = pngUrl
        a.download = `mermaid-${Date.now()}.png`
        a.click()
      }
      img.onerror = () => {
        console.error('Failed to load SVG for export')
      }
      img.src = base64Svg
    }
  })
}

async function renderMermaidBlocks(forceAll = false) {
  await nextTick()
  const root = messageListRef.value
  if (!root) return
  const selector = forceAll
    ? '.mermaid-block[data-mermaid]:not([data-rendered])'
    : '.mermaid-block[data-mermaid][data-pending]:not([data-rendered])'
  const blocks = Array.from(root.querySelectorAll<HTMLElement>(selector))
  for (const block of blocks) {
    const source = decodeURIComponent(block.dataset.mermaid || '').trim()
    if (!source) continue
    if (!forceAll && block.dataset.pending === 'true') {
      continue
    }
    block.dataset.rendered = 'true'
    block.removeAttribute('data-pending')
    try {
      const id = `kb-mermaid-${Date.now()}-${Math.random().toString(16).slice(2)}`
      const rendered = await mermaid.render(id, source)
      const contentEl = block.querySelector('.mermaid-content')
      if (contentEl) {
        contentEl.innerHTML = rendered.svg
      } else {
        block.innerHTML += rendered.svg
      }
    } catch (error: any) {
      block.classList.add('mermaid-block--error')
      block.textContent = `Mermaid 渲染失败：${error?.message || error}`
    }
  }
}

function markMermaidReady() {
  const root = messageListRef.value
  if (!root) return
  const blocks = root.querySelectorAll<HTMLElement>('.mermaid-block[data-pending]')
  blocks.forEach(block => {
    block.removeAttribute('data-pending')
  })
}

watch(
  () => messages.value.map((item) => item.content).join('\n---\n'),
  () => {
    renderMermaidBlocks(false)
  },
  { flush: 'post' },
)

onMounted(() => {
  initGlobalHelpers()
  loadAgents()
})
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

.thinking-section {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  margin-bottom: 8px;
  background: #f0f5ff;
  border-radius: 6px;
  border: 1px solid #d6e4ff;
}

.thinking-text {
  color: #1677ff;
}

.sources-section {
  margin-bottom: 12px;
  padding: 10px 12px;
  background: #fafafa;
  border-radius: 6px;
  border: 1px solid #e8e8e8;
}

.sources-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
  color: #595959;
  font-size: 13px;
  font-weight: 500;
}

.sources-collapse {
  margin-top: 0;
}

.thinking-text {
  margin-left: 8px;
}

.plain-text,
.markdown-body {
  white-space: pre-wrap;
  color: #1f2937;
  line-height: 1.38;
}

.markdown-body :deep(p) {
  margin: 0 0 4px;
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin: 0 0 4px 18px;
  padding: 0;
}

.markdown-body :deep(li) {
  margin-bottom: 0;
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
  padding: 6px 10px;
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

.markdown-body :deep(.code-block) {
  margin: 6px 0;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid #2d3748;
}

.markdown-body :deep(.code-header) {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 12px;
  background: #1a202c;
  border-bottom: 1px solid #2d3748;
}

.markdown-body :deep(.code-lang) {
  color: #a0aec0;
  font-size: 12px;
  font-family: Consolas, Monaco, 'Courier New', monospace;
}

.markdown-body :deep(.code-copy-btn) {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  background: transparent;
  border: 1px solid #4a5568;
  border-radius: 4px;
  color: #a0aec0;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.markdown-body :deep(.code-copy-btn:hover) {
  background: #2d3748;
  border-color: #718096;
  color: #e2e8f0;
}

.markdown-body :deep(pre) {
  margin: 0;
  overflow: auto;
  padding: 10px 12px;
  background: #1a202c;
  color: #e2e8f0;
}

.markdown-body :deep(pre code) {
  font-family: Consolas, Monaco, 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.5;
}

.markdown-body :deep(code) {
  font-family: Consolas, Monaco, 'Courier New', monospace;
}

.markdown-body :deep(.mermaid-block) {
  position: relative;
  margin: 12px 0;
  border: 1px solid #d0d5dd;
  border-radius: 8px;
  background: #fff;
  overflow: hidden;
}

.markdown-body :deep(.mermaid-toolbar) {
  display: flex;
  gap: 4px;
  padding: 6px 8px;
  background: #f8fafc;
  border-bottom: 1px solid #e5e7eb;
}

.markdown-body :deep(.mermaid-btn) {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  background: #fff;
  border: 1px solid #d0d5dd;
  border-radius: 4px;
  color: #475467;
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s;
}

.markdown-body :deep(.mermaid-btn:hover) {
  background: #f0f5ff;
  border-color: #1677ff;
  color: #1677ff;
}

.markdown-body :deep(.mermaid-content) {
  padding: 12px;
  overflow: auto;
  max-height: 800px;
}

.markdown-body :deep(.mermaid-content svg) {
  max-width: 100% !important;
  height: auto;
}

.markdown-body :deep(.mermaid-block--error) {
  color: #b42318;
  white-space: pre-wrap;
  padding: 12px;
}

.sources-collapse {
  margin-top: 8px;
}

.source-item {
  width: 100%;
}

.source-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
  color: #475467;
  font-size: 12px;
}

.source-file {
  font-weight: 500;
  color: #1677ff;
}

.source-score {
  color: #8c8c8c;
  font-size: 11px;
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
