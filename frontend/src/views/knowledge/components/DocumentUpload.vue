<template>
  <a-space direction="vertical" class="document-upload" :size="12">
    <a-segmented v-model:value="mode" :options="modeOptions" />
    <a-upload-dragger v-if="mode === 'file'" :before-upload="beforeUpload" :show-upload-list="false" accept=".md,.markdown,.go,.txt,.ts,.js,.vue">
      <p class="upload-title">拖拽或点击上传文档</p>
      <p class="upload-hint">支持 Markdown、Go、文本和常见源码文件</p>
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
import { reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { addKnowledgeDocument } from '@/api/knowledge'

const props = defineProps<{ kbId: number; projectId: number }>()
const emit = defineEmits<{ (e: 'uploaded'): void }>()

const loading = ref(false)
const mode = ref<'file' | 'manual' | 'url'>('file')
const modeOptions = [
  { label: '文件', value: 'file' },
  { label: '手动', value: 'manual' },
  { label: 'URL', value: 'url' },
]
const form = reactive({ title: '', content: '', source_path: '' })

async function beforeUpload(file: File) {
  loading.value = true
  try {
    const content = await file.text()
    await addKnowledgeDocument(props.kbId, {
      project_id: props.projectId,
      source_type: file.name.endsWith('.go') ? 'code' : 'markdown',
      source_path: file.name,
      title: file.name,
      content,
    })
    message.success('文档已导入')
    emit('uploaded')
  } finally {
    loading.value = false
  }
  return false
}

async function submitManual() {
  loading.value = true
  try {
    await addKnowledgeDocument(props.kbId, {
      project_id: props.projectId,
      source_type: mode.value,
      source_path: form.source_path,
      title: form.title,
      content: form.content,
    })
    message.success('文档已导入')
    form.title = ''
    form.content = ''
    form.source_path = ''
    emit('uploaded')
  } finally {
    loading.value = false
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
