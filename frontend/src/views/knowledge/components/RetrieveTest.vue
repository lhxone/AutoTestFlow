<template>
  <a-space direction="vertical" class="retrieve-test" :size="12">
    <a-input-search v-model:value="query" enter-button="检索" :loading="loading" placeholder="输入测试生成相关问题、规范或关键词" @search="runQuery" />
    <a-list :data-source="results" bordered>
      <template #renderItem="{ item }">
        <a-list-item>
          <a-list-item-meta :title="`Score ${Number(item.score || 0).toFixed(4)} · Chunk ${item.metadata?.chunk_id || '-'}`">
            <template #description>
              <div class="result-content">{{ item.content }}</div>
            </template>
          </a-list-item-meta>
        </a-list-item>
      </template>
    </a-list>
  </a-space>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { queryKnowledgeBase } from '@/api/knowledge'

const props = defineProps<{ kbId: number; projectId: number }>()
const loading = ref(false)
const query = ref('')
const results = ref<any[]>([])

async function runQuery() {
  if (!query.value.trim()) return
  loading.value = true
  try {
    const res = await queryKnowledgeBase(props.kbId, { project_id: props.projectId, query: query.value })
    results.value = res.data.data || []
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.retrieve-test {
  width: 100%;
}
.result-content {
  white-space: pre-wrap;
  color: #344054;
}
</style>
