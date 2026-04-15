<template>
  <div class="code-editor">
    <div ref="editorRef" class="editor-surface" :style="{ height: `${height}px` }"></div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api'
import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
import 'monaco-editor/esm/vs/basic-languages/javascript/javascript.contribution'
import 'monaco-editor/esm/vs/basic-languages/typescript/typescript.contribution'
import 'monaco-editor/esm/vs/basic-languages/python/python.contribution'
import 'monaco-editor/esm/vs/basic-languages/markdown/markdown.contribution'

const props = withDefaults(defineProps<{
  modelValue: string
  language?: string
  height?: number
  readOnly?: boolean
}>(), {
  language: 'typescript',
  height: 360,
  readOnly: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

self.MonacoEnvironment = {
  getWorker() {
    return new editorWorker()
  },
}

const editorRef = ref<HTMLElement | null>(null)
const editor = shallowRef<monaco.editor.IStandaloneCodeEditor | null>(null)
const suppressSync = ref(false)

onMounted(() => {
  if (!editorRef.value) {
    return
  }

  editor.value = monaco.editor.create(editorRef.value, {
    value: props.modelValue,
    language: normalizeLanguage(props.language),
    theme: 'vs',
    automaticLayout: true,
    readOnly: props.readOnly,
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    fontSize: 13,
    lineHeight: 22,
    wordWrap: 'on',
    padding: { top: 12, bottom: 12 },
  })

  editor.value.onDidChangeModelContent(() => {
    if (props.readOnly || suppressSync.value || !editor.value) {
      return
    }
    emit('update:modelValue', editor.value.getValue())
  })
})

onBeforeUnmount(() => {
  editor.value?.dispose()
})

watch(
  () => props.modelValue,
  (value) => {
    const instance = editor.value
    if (!instance || instance.getValue() === value) {
      return
    }
    suppressSync.value = true
    instance.setValue(value)
    suppressSync.value = false
  },
)

watch(
  () => props.language,
  (value) => {
    const model = editor.value?.getModel()
    if (!model) {
      return
    }
    monaco.editor.setModelLanguage(model, normalizeLanguage(value))
  },
)

watch(
  () => props.readOnly,
  (value) => {
    editor.value?.updateOptions({ readOnly: value })
  },
)

function normalizeLanguage(language?: string) {
  const normalized = (language || '').toLowerCase()
  if (normalized === 'ts' || normalized === 'typescript') {
    return 'typescript'
  }
  if (normalized === 'js' || normalized === 'javascript') {
    return 'javascript'
  }
  if (normalized === 'py' || normalized === 'python') {
    return 'python'
  }
  if (normalized === 'md' || normalized === 'markdown') {
    return 'markdown'
  }
  if (normalized === 'json') {
    return 'json'
  }
  return 'plaintext'
}
</script>

<style scoped>
.code-editor {
  border: 1px solid #d9d9d9;
  border-radius: 8px;
  overflow: hidden;
  background: #fff;
}

.editor-surface {
  width: 100%;
}
</style>
