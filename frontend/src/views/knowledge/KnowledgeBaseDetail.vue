<template>
  <a-space direction="vertical" class="kb-detail" :size="16">
    <div class="detail-header">
      <a-button @click="$emit('back')">{{ t('knowledge.detail.back') }}</a-button>
      <div>
        <h2>{{ kb.name }}</h2>
        <p>{{ kb.description || t('knowledge.detail.noDescription') }}</p>
      </div>
      <a-popconfirm :title="t('knowledge.detail.deleteConfirm')" @confirm="$emit('deleted')">
        <a-button danger>{{ t('knowledge.detail.delete') }}</a-button>
      </a-popconfirm>
    </div>
    <a-tabs v-model:activeKey="activeTab" @change="onTabChange">
      <a-tab-pane key="documents" :tab="t('knowledge.detail.tabs.documents')">
        <DocumentUpload :kb-id="kb.id" :project-id="projectId" @uploaded="loadDocuments" />
        <DocumentTable class="table-block" :documents="documents" :loading="docsLoading" :action-id="actionId" @rebuild="rebuildDoc" @delete="deleteDoc" />
      </a-tab-pane>
      <a-tab-pane key="retrieve" :tab="t('knowledge.detail.tabs.retrieve')">
        <RetrieveTest :kb-id="kb.id" :project-id="projectId" />
      </a-tab-pane>
      <a-tab-pane key="chat" :tab="t('knowledge.detail.tabs.chat')">
        <KnowledgeChat :kb-id="kb.id" :project-id="projectId" />
      </a-tab-pane>
      <a-tab-pane key="stats" :tab="t('knowledge.detail.tabs.stats')">
        <a-row :gutter="16">
          <a-col v-for="item in statItems" :key="item.label" :xs="12" :md="6">
            <a-statistic :title="item.label" :value="item.value" />
          </a-col>
        </a-row>
      </a-tab-pane>
      <a-tab-pane key="graph" :tab="t('knowledge.detail.tabs.graph')">
        <div class="graph-tab">
          <KnowledgeGraph :graph="graph" :loading="graphLoading" />
        </div>
      </a-tab-pane>
    </a-tabs>
  </a-space>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
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
import KnowledgeChat from './components/KnowledgeChat.vue'
import KnowledgeGraph from './components/KnowledgeGraph.vue'

const { t } = useI18n()
const props = defineProps<{ kb: KnowledgeBase; projectId: number }>()
defineEmits<{ (e: 'back'): void; (e: 'deleted'): void }>()

const activeTab = ref('documents')
const docsLoading = ref(false)
const actionId = ref<number>()
const documents = ref<KnowledgeDocument[]>([])
const stats = ref<KnowledgeStats>()
const graph = ref<KnowledgeGraphData>()
const graphLoading = ref(false)
let docsRefreshTimer: ReturnType<typeof setTimeout> | undefined

const statItems = computed(() => [
  { label: t('knowledge.detail.stats.documents'), value: stats.value?.document_count || 0 },
  { label: t('knowledge.detail.stats.chunks'), value: stats.value?.chunk_count || 0 },
  { label: t('knowledge.detail.stats.vectors'), value: stats.value?.vector_count || 0 },
  { label: t('knowledge.detail.stats.edges'), value: stats.value?.graph_edges || 0 },
])

function clearDocsRefreshTimer() {
  if (docsRefreshTimer) {
    clearTimeout(docsRefreshTimer)
    docsRefreshTimer = undefined
  }
}

function hasPendingDocuments(list: KnowledgeDocument[]) {
  return list.some((item) => item.status === 'pending' || item.status === 'parsing')
}

function scheduleDocsRefresh() {
  clearDocsRefreshTimer()
  if (!hasPendingDocuments(documents.value)) {
    return
  }
  docsRefreshTimer = setTimeout(async () => {
    try {
      await Promise.all([loadDocuments(), loadStats()])
    } catch {
      scheduleDocsRefresh()
    }
  }, 3000)
}

async function loadDocuments() {
  docsLoading.value = true
  try {
    const res = await getKnowledgeDocuments(props.kb.id, { project_id: props.projectId, page: 1, page_size: 100 })
    documents.value = res.data.data?.list || []
    scheduleDocsRefresh()
  } finally {
    docsLoading.value = false
  }
}

async function loadStats() {
  const res = await getKnowledgeStats(props.kb.id, props.projectId)
  stats.value = res.data.data
}

async function loadGraph() {
  graphLoading.value = true
  try {
    const res = await getKnowledgeGraph(props.kb.id, props.projectId)
    graph.value = res.data.data
  } finally {
    graphLoading.value = false
  }
}

async function rebuildDoc(id: number) {
  actionId.value = id
  try {
    await rebuildKnowledgeDocument(props.kb.id, id, props.projectId)
    message.success(t('knowledge.detail.messages.rebuildSuccess'))
    await loadDocuments()
  } finally {
    actionId.value = undefined
  }
}

async function deleteDoc(id: number) {
  await deleteKnowledgeDocument(props.kb.id, id, props.projectId)
  message.success(t('knowledge.detail.messages.deleteSuccess'))
  await loadDocuments()
}

function onTabChange(key: string) {
  if (key === 'stats') loadStats()
  if (key === 'graph') loadGraph()
}

watch(
  () => [props.kb.id, props.projectId],
  () => {
    clearDocsRefreshTimer()
    loadDocuments()
    loadStats()
    if (activeTab.value === 'graph') {
      loadGraph()
    }
  },
)

onMounted(() => {
  loadDocuments()
  loadStats()
})

onBeforeUnmount(() => {
  clearDocsRefreshTimer()
})
</script>

<style scoped>
.kb-detail {
  width: 100%;
  min-height: calc(100vh - 172px);
}
.kb-detail :deep(.ant-space-item:last-child) {
  flex: 1;
  min-height: 0;
}
.kb-detail :deep(.ant-tabs) {
  height: 100%;
  display: flex;
  flex-direction: column;
}
.kb-detail :deep(.ant-tabs-content-holder),
.kb-detail :deep(.ant-tabs-content),
.kb-detail :deep(.ant-tabs-tabpane-active) {
  flex: 1;
  min-height: 0;
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
.graph-tab {
  height: calc(100vh - 260px);
  min-height: 640px;
}
</style>
