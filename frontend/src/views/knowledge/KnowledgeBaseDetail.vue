<template>
  <a-space direction="vertical" class="kb-detail" :size="16">
    <div class="detail-header">
      <a-button @click="$emit('back')">返回</a-button>
      <div>
        <h2>{{ kb.name }}</h2>
        <p>{{ kb.description || '暂无描述' }}</p>
      </div>
      <a-popconfirm title="确认删除该知识库?" @confirm="$emit('deleted')">
        <a-button danger>删除</a-button>
      </a-popconfirm>
    </div>
    <a-tabs v-model:activeKey="activeTab" @change="onTabChange">
      <a-tab-pane key="documents" tab="文档管理">
        <DocumentUpload :kb-id="kb.id" :project-id="projectId" @uploaded="loadDocuments" />
        <DocumentTable class="table-block" :documents="documents" :loading="docsLoading" :action-id="actionId" @rebuild="rebuildDoc" @delete="deleteDoc" />
      </a-tab-pane>
      <a-tab-pane key="retrieve" tab="检索测试">
        <RetrieveTest :kb-id="kb.id" :project-id="projectId" />
      </a-tab-pane>
      <a-tab-pane key="stats" tab="统计">
        <a-row :gutter="16">
          <a-col v-for="item in statItems" :key="item.label" :xs="12" :md="6">
            <a-statistic :title="item.label" :value="item.value" />
          </a-col>
        </a-row>
      </a-tab-pane>
      <a-tab-pane key="graph" tab="知识图谱">
        <KnowledgeGraph :graph="graph" />
      </a-tab-pane>
    </a-tabs>
  </a-space>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import {
  deleteKnowledgeDocument,
  getKnowledgeDocuments,
  getKnowledgeGraph,
  getKnowledgeStats,
  rebuildKnowledgeDocument,
  type KnowledgeBase,
  type KnowledgeDocument,
  type KnowledgeGraphData,
  type KnowledgeStats,
} from '@/api/knowledge'
import DocumentUpload from './components/DocumentUpload.vue'
import DocumentTable from './components/DocumentTable.vue'
import RetrieveTest from './components/RetrieveTest.vue'
import KnowledgeGraph from './components/KnowledgeGraph.vue'

const props = defineProps<{ kb: KnowledgeBase; projectId: number }>()
defineEmits<{ (e: 'back'): void; (e: 'deleted'): void }>()

const activeTab = ref('documents')
const docsLoading = ref(false)
const actionId = ref<number>()
const documents = ref<KnowledgeDocument[]>([])
const stats = ref<KnowledgeStats>()
const graph = ref<KnowledgeGraphData>()

const statItems = computed(() => [
  { label: '文档数', value: stats.value?.document_count || 0 },
  { label: 'Chunk 数', value: stats.value?.chunk_count || 0 },
  { label: '向量数', value: stats.value?.vector_count || 0 },
  { label: '图谱边', value: stats.value?.graph_edges || 0 },
])

async function loadDocuments() {
  docsLoading.value = true
  try {
    const res = await getKnowledgeDocuments(props.kb.id, { project_id: props.projectId, page: 1, page_size: 100 })
    documents.value = res.data.data?.list || []
  } finally {
    docsLoading.value = false
  }
}

async function loadStats() {
  const res = await getKnowledgeStats(props.kb.id, props.projectId)
  stats.value = res.data.data
}

async function loadGraph() {
  const res = await getKnowledgeGraph(props.kb.id, props.projectId)
  graph.value = res.data.data
}

async function rebuildDoc(id: number) {
  actionId.value = id
  try {
    await rebuildKnowledgeDocument(props.kb.id, id, props.projectId)
    message.success('索引已重建')
    await loadDocuments()
  } finally {
    actionId.value = undefined
  }
}

async function deleteDoc(id: number) {
  await deleteKnowledgeDocument(props.kb.id, id, props.projectId)
  message.success('文档已删除')
  await loadDocuments()
}

function onTabChange(key: string) {
  if (key === 'stats') loadStats()
  if (key === 'graph') loadGraph()
}

onMounted(() => {
  loadDocuments()
  loadStats()
})
</script>

<style scoped>
.kb-detail {
  width: 100%;
}
.detail-header {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 16px;
  align-items: center;
}
.detail-header h2 {
  margin: 0 0 4px;
  font-size: 20px;
}
.detail-header p {
  margin: 0;
  color: #667085;
}
.table-block {
  margin-top: 16px;
}
</style>
