<template>
  <div class="markdown-editor">
    <div class="editor-toolbar">
      <div class="toolbar-left">
        <a-button size="small" @click="insertHeading">H1</a-button>
        <a-button size="small" @click="() => wrapSelection('**', '**')">{{ t('markdownEditor.toolbar.bold') }}</a-button>
        <a-button size="small" @click="() => wrapSelection('*', '*')">{{ t('markdownEditor.toolbar.italic') }}</a-button>
        <a-button size="small" @click="insertBulletList">{{ t('markdownEditor.toolbar.list') }}</a-button>
        <a-button size="small" @click="insertQuote">{{ t('markdownEditor.toolbar.quote') }}</a-button>
        <a-button size="small" @click="insertCodeBlock">{{ t('markdownEditor.toolbar.codeBlock') }}</a-button>
        <a-button size="small" @click="insertLink">{{ t('markdownEditor.toolbar.link') }}</a-button>
        <a-button size="small" @click="insertTable">{{ t('markdownEditor.toolbar.table') }}</a-button>
      </div>
      <a-radio-group v-model:value="viewMode" size="small" button-style="solid">
        <a-radio-button value="edit">{{ t('markdownEditor.mode.edit') }}</a-radio-button>
        <a-radio-button value="split">{{ t('markdownEditor.mode.split') }}</a-radio-button>
        <a-radio-button value="preview">{{ t('markdownEditor.mode.preview') }}</a-radio-button>
      </a-radio-group>
    </div>

    <div class="editor-layout" :class="`mode-${viewMode}`">
      <div class="editor-pane" v-show="viewMode !== 'preview'">
        <div class="editor-shell">
          <div v-if="!modelValue.trim()" class="editor-placeholder">{{ resolvedPlaceholder }}</div>
          <div ref="editorRef" class="editor-surface" :style="editorSurfaceStyle"></div>
        </div>
      </div>

      <div class="preview-pane" v-show="viewMode !== 'edit'">
        <div
          v-if="modelValue.trim()"
          ref="previewContentRef"
          class="markdown-preview"
          :style="previewSurfaceStyle"
          v-html="previewHtml"
        ></div>
        <a-empty v-else :description="t('markdownEditor.previewEmpty')" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api'
import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
import 'monaco-editor/esm/vs/basic-languages/markdown/markdown.contribution'
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
}>(), {
  placeholder: undefined,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()
const { t } = useI18n()

self.MonacoEnvironment = {
  getWorker() {
    return new editorWorker()
  },
}

const editorRef = ref<HTMLElement | null>(null)
const previewContentRef = ref<HTMLElement | null>(null)
const viewMode = ref<'edit' | 'split' | 'preview'>('split')
const editor = shallowRef<monaco.editor.IStandaloneCodeEditor | null>(null)
const suppressSync = ref(false)
const splitHeight = ref(420)
let previewResizeObserver: ResizeObserver | null = null
const MAX_SPLIT_HEIGHT = 720

const md = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
})

const previewHtml = computed(() =>
  DOMPurify.sanitize(md.render(props.modelValue || '')),
)
const resolvedPlaceholder = computed(() => props.placeholder || t('markdownEditor.placeholder'))

const editorSurfaceStyle = computed(() =>
  viewMode.value === 'split'
    ? { height: `${Math.min(splitHeight.value, MAX_SPLIT_HEIGHT)}px` }
    : { height: '420px' },
)

const previewSurfaceStyle = computed(() =>
  viewMode.value === 'split'
    ? {
        minHeight: `${Math.min(splitHeight.value, MAX_SPLIT_HEIGHT)}px`,
        maxHeight: `${MAX_SPLIT_HEIGHT}px`,
      }
    : { minHeight: '420px' },
)

onMounted(() => {
  if (!editorRef.value) return

  editor.value = monaco.editor.create(editorRef.value, {
    value: props.modelValue,
    language: 'markdown',
    theme: 'vs',
    automaticLayout: true,
    wordWrap: 'on',
    lineNumbers: 'on',
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    fontSize: 14,
    lineHeight: 24,
    padding: { top: 16, bottom: 16 },
    smoothScrolling: true,
    renderLineHighlight: 'all',
    tabSize: 2,
  })

  editor.value.onDidChangeModelContent(() => {
    if (suppressSync.value || !editor.value) return
    emit('update:modelValue', editor.value.getValue())
  })

  editor.value.onDidContentSizeChange(() => {
    recalculateSplitHeight()
  })

  previewResizeObserver = new ResizeObserver(() => {
    recalculateSplitHeight()
  })

  if (previewContentRef.value) {
    previewResizeObserver.observe(previewContentRef.value)
  }

  recalculateSplitHeight()
})

onBeforeUnmount(() => {
  previewResizeObserver?.disconnect()
  editor.value?.dispose()
})

watch(
  () => props.modelValue,
  value => {
    const instance = editor.value
    if (!instance) return
    if (instance.getValue() === value) return
    suppressSync.value = true
    instance.setValue(value)
    suppressSync.value = false
    recalculateSplitHeight()
  },
)

watch(viewMode, async () => {
  await nextTick()
  recalculateSplitHeight()
})

watch(previewContentRef, (el, oldEl) => {
  if (oldEl) {
    previewResizeObserver?.unobserve(oldEl)
  }
  if (el) {
    previewResizeObserver?.observe(el)
  }
  recalculateSplitHeight()
})

function recalculateSplitHeight() {
  requestAnimationFrame(() => {
    const instance = editor.value
    if (!instance) return

    if (viewMode.value !== 'split') {
      instance.layout()
      return
    }

    const editorHeight = Math.max(420, Math.ceil(instance.getContentHeight()))
    const previewHeight = Math.max(420, Math.ceil(previewContentRef.value?.scrollHeight || 420))
    splitHeight.value = Math.min(Math.max(editorHeight, previewHeight), MAX_SPLIT_HEIGHT)

    nextTick(() => {
      instance.layout()
      requestAnimationFrame(() => instance.layout())
    })
  })
}

function focusEditor() {
  editor.value?.focus()
}

function updateValue(value: string, selectionStart?: number, selectionEnd?: number) {
  const instance = editor.value
  if (!instance) return

  suppressSync.value = true
  instance.setValue(value)
  suppressSync.value = false
  emit('update:modelValue', value)

  nextTick(() => {
    focusEditor()
    if (selectionStart != null && selectionEnd != null && instance.getModel()) {
      const start = instance.getModel()!.getPositionAt(selectionStart)
      const end = instance.getModel()!.getPositionAt(selectionEnd)
      instance.setSelection(new monaco.Selection(start.lineNumber, start.column, end.lineNumber, end.column))
    }
  })
}

function getSelectionOffsets() {
  const instance = editor.value
  const model = instance?.getModel()
  const selection = instance?.getSelection()
  if (!instance || !model || !selection) return null
  return {
    start: model.getOffsetAt(selection.getStartPosition()),
    end: model.getOffsetAt(selection.getEndPosition()),
  }
}

function wrapSelection(prefix: string, suffix: string, defaultText = t('markdownEditor.defaults.text')) {
  const offsets = getSelectionOffsets()
  if (!offsets) return

  const { start, end } = offsets
  const selected = props.modelValue.slice(start, end) || defaultText
  const nextValue = props.modelValue.slice(0, start) + prefix + selected + suffix + props.modelValue.slice(end)
  const cursorStart = start + prefix.length
  const cursorEnd = cursorStart + selected.length
  updateValue(nextValue, cursorStart, cursorEnd)
}

function prefixLines(prefix: string, defaultText: string) {
  const offsets = getSelectionOffsets()
  if (!offsets) return

  const { start, end } = offsets
  const selected = props.modelValue.slice(start, end) || defaultText
  const lines = selected.split('\n').map(line => `${prefix}${line}`).join('\n')
  const nextValue = props.modelValue.slice(0, start) + lines + props.modelValue.slice(end)
  updateValue(nextValue, start, start + lines.length)
}

function insertHeading() {
  prefixLines('# ', t('markdownEditor.defaults.heading'))
}

function insertBulletList() {
  prefixLines('- ', t('markdownEditor.defaults.listItem'))
}

function insertQuote() {
  prefixLines('> ', t('markdownEditor.defaults.quote'))
}

function insertCodeBlock() {
  wrapSelection('```markdown\n', '\n```', t('markdownEditor.defaults.code'))
}

function insertLink() {
  wrapSelection('[', '](https://example.com)', t('markdownEditor.defaults.linkText'))
}

function insertTable() {
  const table = t('markdownEditor.defaults.table')
  const offsets = getSelectionOffsets()
  if (!offsets) return
  const { start, end } = offsets
  const nextValue = props.modelValue.slice(0, start) + table + props.modelValue.slice(end)
  updateValue(nextValue, start, start + table.length)
}
</script>

<style scoped>
.markdown-editor {
  border: 1px solid #d9d9d9;
  border-radius: 8px;
  overflow: hidden;
  background: #fff;
}

.editor-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-bottom: 1px solid #f0f0f0;
  background: #fafafa;
}

.toolbar-left {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.editor-layout {
  display: grid;
  min-height: 420px;
  align-items: stretch;
}

.editor-layout.mode-split {
  grid-template-columns: 1fr 1fr;
}

.editor-layout.mode-edit,
.editor-layout.mode-preview {
  grid-template-columns: 1fr;
}

.editor-pane,
.preview-pane {
  min-height: 420px;
  height: 100%;
  max-height: 720px;
}

.editor-pane {
  border-right: 1px solid #f0f0f0;
  overflow: hidden;
}

.mode-edit .editor-pane,
.mode-preview .preview-pane {
  border-right: none;
}

.editor-shell {
  position: relative;
  min-height: 420px;
  height: 100%;
  max-height: 720px;
  background: #fff;
}

.editor-surface {
  height: 420px;
  max-height: 720px;
}

.editor-placeholder {
  position: absolute;
  top: 16px;
  left: 56px;
  z-index: 1;
  color: #bfbfbf;
  pointer-events: none;
}

.preview-pane {
  overflow: auto;
  max-height: 720px;
  background: linear-gradient(180deg, #fffdf8 0%, #ffffff 100%);
}

.markdown-preview {
  min-height: 420px;
  max-height: 720px;
  padding: 18px 20px;
  color: #262626;
  line-height: 1.8;
  word-break: break-word;
  overflow: auto;
}

.markdown-preview :deep(h1),
.markdown-preview :deep(h2),
.markdown-preview :deep(h3),
.markdown-preview :deep(h4) {
  margin: 1.2em 0 0.6em;
  color: #111827;
}

.markdown-preview :deep(p),
.markdown-preview :deep(ul),
.markdown-preview :deep(ol),
.markdown-preview :deep(blockquote) {
  margin: 0.8em 0;
}

.markdown-preview :deep(code) {
  padding: 0.15em 0.4em;
  border-radius: 4px;
  background: #fff1f0;
  color: #cf1322;
}

.markdown-preview :deep(pre) {
  padding: 14px 16px;
  border-radius: 8px;
  overflow: auto;
  background: #0f172a;
}

.markdown-preview :deep(pre code) {
  padding: 0;
  background: transparent;
  color: #e5e7eb;
}

.markdown-preview :deep(blockquote) {
  padding: 8px 12px;
  border-left: 4px solid #fa8c16;
  background: #fff7e6;
  color: #8c4a00;
}

.markdown-preview :deep(table) {
  width: 100%;
  border-collapse: collapse;
}

.markdown-preview :deep(th),
.markdown-preview :deep(td) {
  padding: 8px 10px;
  border: 1px solid #f0f0f0;
}

.markdown-preview :deep(th) {
  background: #fafafa;
  text-align: left;
}
</style>
