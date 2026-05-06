<template>
  <a-space direction="vertical" class="kb-list" :size="16">
    <div class="list-header">
      <a-input-search v-model:value="keyword" placeholder="搜索知识库" class="search-input" @search="load" />
      <a-button type="primary" @click="openCreate">新建知识库</a-button>
    </div>
    <a-row :gutter="[16, 16]">
      <a-col v-for="kb in items" :key="kb.id" :xs="24" :md="12" :xl="8">
        <a-card hoverable :body-style="{ padding: '16px' }" @click="$emit('select', kb)">
          <div class="kb-card-title">
            <span>{{ kb.name }}</span>
            <a-tag :color="kb.status === 1 ? 'success' : 'default'">{{ kb.status === 1 ? '启用' : '停用' }}</a-tag>
          </div>
          <p class="kb-desc">{{ kb.description || '暂无描述' }}</p>
          <a-space class="kb-meta">
            <span>Chunk {{ kb.chunk_size }}</span>
            <span>Overlap {{ kb.chunk_overlap }}</span>
          </a-space>
        </a-card>
      </a-col>
    </a-row>
    <a-empty v-if="!loading && items.length === 0" />
    <a-modal v-model:open="modalOpen" title="新建知识库" @ok="submit" :confirm-loading="saving">
      <a-form layout="vertical" :model="form">
        <a-form-item label="名称" required>
          <a-input v-model:value="form.name" />
        </a-form-item>
        <a-form-item label="描述">
          <a-textarea v-model:value="form.description" :rows="3" />
        </a-form-item>
        <a-form-item label="Chunk 大小">
          <a-input-number v-model:value="form.chunk_size" :min="100" :max="4000" class="number-input" />
        </a-form-item>
        <a-form-item label="Chunk 重叠">
          <a-input-number v-model:value="form.chunk_overlap" :min="0" :max="1000" class="number-input" />
        </a-form-item>
      </a-form>
    </a-modal>
  </a-space>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import { createKnowledgeBase, getKnowledgeBases, type KnowledgeBase } from '@/api/knowledge'

const props = defineProps<{ projectId?: number }>()
defineEmits<{ (e: 'select', kb: KnowledgeBase): void }>()

const loading = ref(false)
const saving = ref(false)
const modalOpen = ref(false)
const keyword = ref('')
const items = ref<KnowledgeBase[]>([])
const form = reactive({ name: '', description: '', chunk_size: 500, chunk_overlap: 50 })

async function load() {
  if (!props.projectId) return
  loading.value = true
  try {
    const res = await getKnowledgeBases({ project_id: props.projectId, keyword: keyword.value, page: 1, page_size: 100 })
    items.value = res.data.data?.list || []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.name = ''
  form.description = ''
  modalOpen.value = true
}

async function submit() {
  if (!props.projectId || !form.name.trim()) return
  saving.value = true
  try {
    await createKnowledgeBase({ ...form, project_id: props.projectId, status: 1 })
    message.success('知识库已创建')
    modalOpen.value = false
    await load()
  } finally {
    saving.value = false
  }
}

watch(() => props.projectId, load)
onMounted(load)
</script>

<style scoped>
.kb-list {
  width: 100%;
}
.list-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}
.search-input {
  max-width: 320px;
}
.kb-card-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-weight: 600;
}
.kb-desc {
  min-height: 44px;
  margin: 12px 0;
  color: #667085;
}
.kb-meta {
  color: #475467;
  font-size: 12px;
}
.number-input {
  width: 100%;
}
</style>
