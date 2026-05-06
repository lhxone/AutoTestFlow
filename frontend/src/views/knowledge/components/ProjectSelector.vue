<template>
  <a-select
    v-model:value="current"
    show-search
    :loading="loading"
    :options="options"
    option-filter-prop="label"
    placeholder="选择项目"
    class="project-selector"
    @change="onChange"
  />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { getProjectList } from '@/api/project'

const props = defineProps<{ value?: number }>()
const emit = defineEmits<{ (e: 'update:value', value: number): void; (e: 'change', value: number): void }>()

const loading = ref(false)
const projects = ref<any[]>([])
const current = computed({
  get: () => props.value,
  set: (value?: number) => {
    if (value) emit('update:value', value)
  },
})
const options = computed(() => projects.value.map((item) => ({ label: item.name, value: item.id })))

function onChange(value: number) {
  emit('change', value)
}

async function loadProjects() {
  loading.value = true
  try {
    const res = await getProjectList({ page: 1, page_size: 100, status: 1 })
    projects.value = res.data.data?.list || []
    if (!props.value && projects.value.length > 0) {
      emit('update:value', projects.value[0].id)
      emit('change', projects.value[0].id)
    }
  } finally {
    loading.value = false
  }
}

onMounted(loadProjects)
</script>

<style scoped>
.project-selector {
  width: 260px;
}
</style>
