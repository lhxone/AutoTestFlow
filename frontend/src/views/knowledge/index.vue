<template>
  <a-space direction="vertical" class="knowledge-page" :size="16">
    <div class="page-header">
      <div>
        <h1>知识库</h1>
        <p>按项目管理规范、历史用例和最佳实践，供测试生成流程检索使用。</p>
      </div>
      <ProjectSelector v-model:value="projectId" @change="handleProjectChange" />
    </div>
    <a-tabs v-model:activeKey="activeKey">
      <a-tab-pane key="bases" tab="知识库">
        <KnowledgeBaseDetail
          v-if="selectedKB"
          :kb="selectedKB"
          :project-id="projectId!"
          @back="selectedKB = undefined"
          @deleted="deleteSelected"
        />
        <KnowledgeBaseList v-else :project-id="projectId" @select="selectedKB = $event" />
      </a-tab-pane>
      <a-tab-pane key="settings" tab="配置">
        <Settings />
      </a-tab-pane>
    </a-tabs>
  </a-space>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { deleteKnowledgeBase, type KnowledgeBase } from '@/api/knowledge'
import ProjectSelector from './components/ProjectSelector.vue'
import KnowledgeBaseList from './KnowledgeBaseList.vue'
import KnowledgeBaseDetail from './KnowledgeBaseDetail.vue'
import Settings from './Settings.vue'

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
  message.success('知识库已删除')
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
