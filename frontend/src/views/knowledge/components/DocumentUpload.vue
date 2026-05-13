<template>
  <a-space direction="vertical" class="document-upload" :size="12">
    <a-segmented v-model:value="mode" :options="modeOptions" />
    <a-upload-dragger
      v-if="mode === 'file'"
      multiple
      :before-upload="beforeUpload"
      :show-upload-list="false"
      accept=".md,.markdown,.go,.txt,.ts,.js,.vue"
    >
      <p class="upload-title">拖拽或点击上传文档</p>
      <p class="upload-hint">支持 Markdown、Go、文本和常见源码文件，可一次选择多个文件</p>
    </a-upload-dragger>
    <a-form v-else layout="vertical" :model="form">
      <a-form-item label="标题">
        <a-input v-model:value="form.title" />
      </a-form-item>
      <a-form-item :label="mode === 'url' ? '网页 URL' : '内容'">
        <a-input v-if="mode === 'url'" v-model:value="form.source_path" />
        <a-textarea v-else v-model:value="form.content" :rows="6" />
      </a-form-item>
      <a-button type="primary" :loading="loading" @click="submitManual">导入</a-button>
    </a-form>
  </a-space>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { addKnowledgeDocument, batchAddKnowledgeDocuments } from '@/api/knowledge'

const props = defineProps<{ kbId: number; projectId: number }>()
const emit = defineEmits<{ (e: 'uploaded'): void }>()

const activeUploads = ref(0)
const mode = ref<'file' | 'manual' | 'url'>('file')
const modeOptions = [
  { label: '文件', value: 'file' },
  { label: '手动', value: 'manual' },
  { label: 'URL', value: 'url' },
]
const form = reactive({ title: '', content: '', source_path: '' })
const loading = computed(() => activeUploads.value > 0)
const pendingBatches = new Set<string>()

function detectSourceType(fileName: string) {
  return fileName.endsWith('.go') ? 'code' : 'markdown'
}

function getBatchKey(files: File[]) {
  return files
    .map((file) => `${file.name}:${file.size}:${file.lastModified}`)
    .sort()
    .join('|')
}

async function uploadFiles(files: File[]) {
  activeUploads.value += 1
  try {
    if (files.length === 1) {
      const [file] = files
      await addKnowledgeDocument(props.kbId, {
        project_id: props.projectId,
        source_type: detectSourceType(file.name),
        source_path: file.name,
        title: file.name,
        content: await file.text(),
      })
      message.success('文档已加入处理队列')
    } else {
      const documents = await Promise.all(
        files.map(async (file) => ({
          project_id: props.projectId,
          source_type: detectSourceType(file.name),
          source_path: file.name,
          title: file.name,
          content: await file.text(),
        })),
      )
      await batchAddKnowledgeDocuments(props.kbId, {
        project_id: props.projectId,
        documents,
      })
      message.success(`${files.length} 个文档已加入处理队列`)
    }
    emit('uploaded')
  } finally {
    activeUploads.value -= 1
  }
}

function beforeUpload(file: File, fileList: File[]) {
  const files = fileList.length > 0 ? fileList : [file]
  const batchKey = getBatchKey(files)
  if (pendingBatches.has(batchKey)) {
    return false
  }
  pendingBatches.add(batchKey)
  void uploadFiles(files).finally(() => {
    pendingBatches.delete(batchKey)
  })
  return false
}

async function submitManual() {
  activeUploads.value += 1
  try {
    await addKnowledgeDocument(props.kbId, {
      project_id: props.projectId,
      source_type: mode.value,
      source_path: form.source_path,
      title: form.title,
      content: form.content,
    })
    message.success('文档已加入处理队列')
    form.title = ''
    form.content = ''
    form.source_path = ''
    emit('uploaded')
  } finally {
    activeUploads.value -= 1
  }
}
</script>

<style scoped>
.document-upload {
  width: 100%;
}
.upload-title {
  margin: 8px 0 4px;
  font-weight: 600;
}
.upload-hint {
  margin: 0;
  color: #667085;
}
</style>
