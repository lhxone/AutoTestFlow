<template>
  <a-space direction="vertical" class="knowledge-page" :size="16">
    <div class="page-header">
      <div>
        <h1>{{ t('knowledge.title') }}</h1>
        <p>{{ t('knowledge.subtitle') }}</p>
      </div>
      <ProjectSelector v-model:value="projectId" @change="handleProjectChange" />
    </div>
    <a-tabs v-model:activeKey="activeKey">
      <a-tab-pane key="bases" :tab="t('knowledge.tabs.bases')">
        <KnowledgeBaseDetail
          v-if="selectedKB"
          :kb="selectedKB"
          :project-id="projectId!"
          @back="selectedKB = undefined"
          @deleted="deleteSelected"
        />
        <KnowledgeBaseList v-else :project-id="projectId" @select="selectedKB = $event" />
      </a-tab-pane>
      <a-tab-pane key="settings" :tab="t('knowledge.tabs.settings')">
        <Settings />
      </a-tab-pane>
    </a-tabs>
  </a-space>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { deleteKnowledgeBase, type KnowledgeBase } from '@/api/knowledge'
import ProjectSelector from './components/ProjectSelector.vue'
import KnowledgeBaseList from './KnowledgeBaseList.vue'
import KnowledgeBaseDetail from './KnowledgeBaseDetail.vue'
import Settings from './Settings.vue'

const { t } = useI18n()

const route = useRoute()
const router = useRouter()
const activeKey = ref('bases')
const projectId = ref<number>()
const selectedKB = ref<KnowledgeBase>()

function handleProjectChange(id: number) {
  selectedKB.value = undefined
  router.replace(`/knowledge/${id}`)
}

async function deleteSelected() {
  if (!selectedKB.value || !projectId.value) return
  await deleteKnowledgeBase(selectedKB.value.id, projectId.value)
  message.success(t('knowledge.detail.messages.deleteKbSuccess'))
  selectedKB.value = undefined
}

watch(
  () => route.params.projectId,
  (value) => {
    const id = Number(value)
    if (id > 0) projectId.value = id
  },
  { immediate: true },
)

onMounted(() => {
  const id = Number(route.params.projectId)
  if (id > 0) projectId.value = id
})
</script>

<style scoped>
.knowledge-page {
  width: 100%;
}
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.page-header h1 {
  margin: 0 0 4px;
  font-size: 24px;
}
.page-header p {
  margin: 0;
  color: #667085;
}
</style>
