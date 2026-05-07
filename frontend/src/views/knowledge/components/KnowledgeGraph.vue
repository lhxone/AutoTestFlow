<template>
  <div class="graph-shell">
    <div class="graph-toolbar">
      <a-space wrap>
        <a-checkbox-group v-model:value="visibleTypes" :options="typeOptions" />
        <a-switch v-model:checked="showLabels" size="small" />
        <span class="toolbar-label">显示标签</span>
      </a-space>
      <a-space>
        <span class="graph-count">{{ filteredCounts }}</span>
        <a-button @click="resetCamera">重置视角</a-button>
        <a-button @click="exportPng">导出 PNG</a-button>
      </a-space>
    </div>
    <a-spin :spinning="loading">
      <div class="graph-stage">
        <div ref="graphEl" class="graph-canvas"></div>
        <div v-if="hoveredNode" class="graph-tooltip">
          <div class="tooltip-title">{{ hoveredNode.name }}</div>
          <div>{{ nodeTypeLabel(hoveredNode.type) }}</div>
        </div>
      </div>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import ForceGraph3D from '3d-force-graph'
import SpriteText from 'three-spritetext'
import type { KnowledgeGraphData } from '@/api/knowledge'

const props = defineProps<{ graph?: KnowledgeGraphData; loading?: boolean }>()

type SourceNode = KnowledgeGraphData['nodes'][number]
type GraphNode = SourceNode & {
  color: string
  size: number
  x?: number
  y?: number
  z?: number
}
type GraphLink = KnowledgeGraphData['edges'][number] & {
  source: string
  target: string
  color: string
}

const graphEl = ref<HTMLDivElement>()
const visibleTypes = ref(['document', 'chunk', 'tag'])
const showLabels = ref(false)
const hoveredNode = ref<GraphNode | null>(null)
const typeOptions = [
  { label: '文档', value: 'document' },
  { label: 'Chunk', value: 'chunk' },
  { label: '标签', value: 'tag' },
]

let graph: any = null
let frame = 0
let resizeObserver: ResizeObserver | null = null

const filteredGraph = computed(() => {
  const source = props.graph || { nodes: [], edges: [] }
  const nodes = source.nodes.filter((node) => visibleTypes.value.includes(node.type)).map(toGraphNode)
  const nodeIds = new Set(nodes.map((node) => node.id))
  const links = source.edges
    .filter((edge) => nodeIds.has(edge.source) && nodeIds.has(edge.target))
    .map((edge) => ({ ...edge, color: edgeColor(edge.type) }))
  return { nodes, links }
})

const filteredCounts = computed(() => `${filteredGraph.value.nodes.length} 节点 / ${filteredGraph.value.links.length} 边`)

function initGraph() {
  if (!graphEl.value || graph) return
  graph = (ForceGraph3D as any)({ controlType: 'orbit' })(graphEl.value)
    .backgroundColor('#ffffff')
    .showNavInfo(false)
    .nodeId('id')
    .nodeLabel((node: GraphNode) => `${node.name}<br/>${nodeTypeLabel(node.type)}`)
    .nodeColor((node: GraphNode) => node.color)
    .nodeVal((node: GraphNode) => node.size)
    .linkSource('source')
    .linkTarget('target')
    .linkColor((link: GraphLink) => link.color)
    .linkOpacity(0.28)
    .linkWidth((link: GraphLink) => (link.type === 'similar' ? 1.4 : 0.8))
    .linkDirectionalParticles(0)
    .enableNodeDrag(true)
    .cooldownTicks(120)
    .warmupTicks(60)
    .d3VelocityDecay(0.45)
    .onNodeHover((node: GraphNode | null) => {
      hoveredNode.value = node
      if (graphEl.value) graphEl.value.style.cursor = node ? 'pointer' : 'grab'
    })
    .onNodeClick(focusNode)

  updateDimensions()
  renderGraph()
}

function renderGraph() {
  if (!graph) return
  graph.nodeThreeObject(showLabels.value ? buildNodeObject : null)
  graph.graphData(filteredGraph.value)
  updateDimensions()
}

function scheduleRender() {
  if (frame) cancelAnimationFrame(frame)
  frame = requestAnimationFrame(() => {
    frame = 0
    nextTick(renderGraph)
  })
}

function buildNodeObject(node: GraphNode) {
  const sprite = new SpriteText(node.name)
  sprite.color = node.color
  sprite.textHeight = node.type === 'document' ? 8 : 5
  sprite.backgroundColor = 'rgba(255,255,255,0.72)'
  sprite.padding = 2
  return sprite
}

function toGraphNode(node: SourceNode): GraphNode {
  return {
    ...node,
    color: nodeColor(node.type),
    size: nodeSize(node),
  }
}

function nodeColor(type: string) {
  if (type === 'document') return '#1677ff'
  if (type === 'chunk') return '#52c41a'
  if (type === 'tag') return '#faad14'
  return '#8c8c8c'
}

function edgeColor(type: string) {
  if (type === 'similar') return '#7c3aed'
  if (type === 'tag') return '#d48806'
  if (type === 'contains') return '#98a2b3'
  return '#667085'
}

function nodeSize(node: SourceNode) {
  const base = node.type === 'document' ? 9 : node.type === 'tag' ? 4 : 6
  return Math.max(base, Math.min(18, node.value || base))
}

function nodeTypeLabel(type: string) {
  if (type === 'document') return '文档'
  if (type === 'chunk') return 'Chunk'
  if (type === 'tag') return '标签'
  return type
}

function focusNode(node: GraphNode) {
  if (!graph || !node) return
  const distance = 160
  const length = Math.hypot(node.x || 0, node.y || 0, node.z || 0) || 1
  const distRatio = 1 + distance / length
  graph.cameraPosition(
    { x: (node.x || 0) * distRatio, y: (node.y || 0) * distRatio, z: (node.z || 0) * distRatio },
    node,
    900,
  )
}

function resetCamera() {
  graph?.cameraPosition({ x: 0, y: 0, z: 520 }, { x: 0, y: 0, z: 0 }, 700)
}

function exportPng() {
  const canvas = graphEl.value?.querySelector('canvas')
  if (!canvas) return
  const link = document.createElement('a')
  link.href = canvas.toDataURL('image/png')
  link.download = 'knowledge-graph-3d.png'
  link.click()
}

function updateDimensions() {
  if (!graph || !graphEl.value) return
  const rect = graphEl.value.getBoundingClientRect()
  graph.width(Math.max(320, Math.floor(rect.width)))
  graph.height(Math.max(520, Math.floor(rect.height)))
}

watch([filteredGraph, showLabels], scheduleRender, { flush: 'post' })

onMounted(() => {
  nextTick(initGraph)
  resizeObserver = new ResizeObserver(updateDimensions)
  if (graphEl.value) resizeObserver.observe(graphEl.value)
})

onBeforeUnmount(() => {
  if (frame) cancelAnimationFrame(frame)
  resizeObserver?.disconnect()
  graph?._destructor?.()
  graph = null
})
</script>

<style scoped>
.graph-shell {
  height: 100%;
  min-height: 640px;
  display: flex;
  flex-direction: column;
}
.graph-toolbar {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.toolbar-label,
.graph-count {
  color: #667085;
  font-size: 13px;
}
.graph-stage {
  position: relative;
  height: calc(100vh - 320px);
  min-height: 600px;
  border: 1px solid #eaecf0;
  border-radius: 6px;
  overflow: hidden;
  background: #fff;
}
.graph-canvas {
  width: 100%;
  height: 100%;
  cursor: grab;
}
.graph-tooltip {
  position: absolute;
  left: 12px;
  bottom: 12px;
  max-width: 320px;
  padding: 10px 12px;
  color: #475467;
  background: rgba(255, 255, 255, 0.92);
  border: 1px solid #eaecf0;
  border-radius: 6px;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12);
  pointer-events: none;
}
.tooltip-title {
  margin-bottom: 4px;
  color: #101828;
  font-weight: 600;
}
.graph-shell :deep(.ant-spin-nested-loading),
.graph-shell :deep(.ant-spin-container) {
  flex: 1;
  min-height: 0;
}
</style>
