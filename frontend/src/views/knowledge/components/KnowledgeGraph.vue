<template>
  <div class="graph-shell">
    <a-space class="graph-toolbar">
      <a-checkbox-group v-model:value="visibleTypes" :options="typeOptions" />
      <a-button @click="exportPng">导出 PNG</a-button>
    </a-space>
    <div ref="chartEl" class="graph-canvas"></div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import type { KnowledgeGraphData } from '@/api/knowledge'

const props = defineProps<{ graph?: KnowledgeGraphData }>()
const chartEl = ref<HTMLDivElement>()
const visibleTypes = ref(['document', 'chunk', 'tag'])
const typeOptions = [
  { label: '文档', value: 'document' },
  { label: 'Chunk', value: 'chunk' },
  { label: '标签', value: 'tag' },
]
let chart: echarts.ECharts | null = null

const option = computed(() => {
  const source = props.graph || { nodes: [], edges: [] }
  const nodes = source.nodes.filter((node) => visibleTypes.value.includes(node.type))
  const nodeIds = new Set(nodes.map((node) => node.id))
  const links = source.edges.filter((edge) => nodeIds.has(edge.source) && nodeIds.has(edge.target))
  return {
    tooltip: { trigger: 'item' },
    legend: [{ data: ['document', 'chunk', 'tag'] }],
    series: [
      {
        type: 'graph',
        layout: 'force',
        roam: true,
        draggable: true,
        categories: [{ name: 'document' }, { name: 'chunk' }, { name: 'tag' }],
        force: { repulsion: 180, edgeLength: 90 },
        label: { show: true, fontSize: 11 },
        data: nodes.map((node) => ({
          id: node.id,
          name: node.name,
          category: node.category,
          value: node.value,
          symbolSize: Math.max(18, node.value * 2),
        })),
        links: links.map((edge) => ({ source: edge.source, target: edge.target, value: edge.score, label: { show: edge.type !== 'contains', formatter: edge.type } })),
        emphasis: { focus: 'adjacency' },
      },
    ],
  }
})

function render() {
  if (!chartEl.value) return
  if (!chart) chart = echarts.init(chartEl.value)
  chart.setOption(option.value, true)
}

function exportPng() {
  if (!chart) return
  const url = chart.getDataURL({ type: 'png', pixelRatio: 2, backgroundColor: '#fff' })
  const link = document.createElement('a')
  link.href = url
  link.download = 'knowledge-graph.png'
  link.click()
}

function resize() {
  chart?.resize()
}

watch(option, () => nextTick(render), { deep: true })
onMounted(() => {
  render()
  window.addEventListener('resize', resize)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', resize)
  chart?.dispose()
})
</script>

<style scoped>
.graph-shell {
  min-height: 520px;
}
.graph-toolbar {
  margin-bottom: 12px;
}
.graph-canvas {
  height: 480px;
  border: 1px solid #eaecf0;
  border-radius: 6px;
}
</style>
