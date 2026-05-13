<template>
  <div class="graph-shell">
    <div class="graph-toolbar">
      <a-space wrap>
        <a-segmented v-model:value="displayMode" size="small" :options="displayModeOptions" />
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
type GraphDisplayMode = 'auto' | 'full' | 'sample' | 'aggregate'
type GraphNodeType = SourceNode['type'] | 'aggregate'
type GraphNode = SourceNode & {
  type: GraphNodeType
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
const displayMode = ref<GraphDisplayMode>('auto')
const visibleTypes = ref<GraphNodeType[]>(['document', 'chunk', 'tag', 'aggregate'])
const showLabels = ref(false)
const hoveredNode = ref<GraphNode | null>(null)
const displayModeOptions = [
  { label: '自动', value: 'auto' },
  { label: '完整', value: 'full' },
  { label: '抽稀', value: 'sample' },
  { label: '聚合', value: 'aggregate' },
]
const typeOptions = [
  { label: '文档', value: 'document' },
  { label: 'Chunk', value: 'chunk' },
  { label: '标签', value: 'tag' },
  { label: '聚合', value: 'aggregate' },
]

const autoFullThreshold = 220
const autoSampleThreshold = 600
const maxDocumentsInSample = 36
const maxChunksPerDocumentInSample = 8
const maxTagsPerChunkInSample = 3
const maxDocumentsInAggregate = 24
const maxChunksPerDocumentInAggregate = 4
const maxTagsPerChunkInAggregate = 2

let graph: any = null
let frame = 0
let resizeObserver: ResizeObserver | null = null

const displayGraph = computed(() => {
  const source = props.graph || { nodes: [], edges: [] }
  const visibleSet = new Set(visibleTypes.value)
  const totalNodeCount = source.nodes.length
  const effectiveMode =
    displayMode.value === 'auto'
      ? totalNodeCount <= autoFullThreshold
        ? 'full'
        : totalNodeCount <= autoSampleThreshold
          ? 'sample'
          : 'aggregate'
      : displayMode.value

  if (effectiveMode === 'full') {
    return buildFullGraph(source, visibleSet)
  }

  const limits =
    effectiveMode === 'sample'
      ? { maxDocs: maxDocumentsInSample, maxChunks: maxChunksPerDocumentInSample, maxTags: maxTagsPerChunkInSample }
      : { maxDocs: maxDocumentsInAggregate, maxChunks: maxChunksPerDocumentInAggregate, maxTags: maxTagsPerChunkInAggregate }

  return buildSummarizedGraph(source, visibleSet, limits)
})

const filteredCounts = computed(() => {
  const rawNodes = props.graph?.nodes.length || 0
  const rawEdges = props.graph?.edges.length || 0
  const shownNodes = displayGraph.value.nodes.length
  const shownEdges = displayGraph.value.links.length
  const label = displayMode.value === 'auto' ? '自动' : displayModeOptions.find((item) => item.value === displayMode.value)?.label || '图谱'
  if (!rawNodes && !rawEdges) {
    return `${label} 0 节点 / 0 边`
  }
  return `${label} ${shownNodes}/${rawNodes} 节点 · ${shownEdges}/${rawEdges} 边`
})

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
  graph.graphData(displayGraph.value)
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
  const spriteAny = sprite as any
  sprite.color = node.color
  sprite.textHeight = node.type === 'document' ? 11 : node.type === 'aggregate' ? 9 : 8
  spriteAny.backgroundColor = 'rgba(255,255,255,0.84)'
  spriteAny.borderColor = 'rgba(15,23,42,0.2)'
  spriteAny.borderWidth = 1
  spriteAny.borderRadius = 6
  sprite.padding = 3
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
  if (type === 'aggregate') return '#7c3aed'
  return '#8c8c8c'
}

function edgeColor(type: string) {
  if (type === 'similar') return '#7c3aed'
  if (type === 'tag') return '#d48806'
  if (type === 'contains') return '#98a2b3'
  return '#667085'
}

function nodeSize(node: SourceNode) {
  if (node.type === 'aggregate') {
    return Math.max(10, Math.min(24, 10 + Math.log2((node.value || 1) + 1) * 3))
  }
  const base = node.type === 'document' ? 9 : node.type === 'tag' ? 4 : 6
  return Math.max(base, Math.min(18, node.value || base))
}

function nodeTypeLabel(type: string) {
  if (type === 'document') return '文档'
  if (type === 'chunk') return 'Chunk'
  if (type === 'tag') return '标签'
  if (type === 'aggregate') return '聚合'
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

function buildFullGraph(source: KnowledgeGraphData, visibleSet: Set<GraphNodeType>) {
  const nodes = source.nodes.filter((node) => visibleSet.has(node.type as GraphNodeType)).map(toGraphNode)
  const nodeIds = new Set(nodes.map((node) => node.id))
  const links = source.edges
    .filter((edge) => nodeIds.has(edge.source) && nodeIds.has(edge.target))
    .map((edge) => ({ ...edge, color: edgeColor(edge.type) }))
  return { nodes, links }
}

function buildSummarizedGraph(
  source: KnowledgeGraphData,
  visibleSet: Set<GraphNodeType>,
  limits: { maxDocs: number; maxChunks: number; maxTags: number },
) {
  const documents = source.nodes.filter((node) => node.type === 'document' && visibleSet.has('document')).map(toGraphNode)
  const chunks = source.nodes.filter((node) => node.type === 'chunk' && visibleSet.has('chunk')).map(toGraphNode)
  const tags = source.nodes.filter((node) => node.type === 'tag' && visibleSet.has('tag')).map(toGraphNode)
  const documentMap = new Map(documents.map((node) => [node.id, node]))
  const chunkMap = new Map(chunks.map((node) => [node.id, node]))
  const tagMap = new Map(tags.map((node) => [node.id, node]))

  const chunkByDoc = new Map<string, GraphNode[]>()
  const tagLinksByChunk = new Map<string, KnowledgeGraphData['edges']>()
  const similarityEdges: KnowledgeGraphData['edges'] = []

  for (const edge of source.edges) {
    if (edge.type === 'contains') {
      const chunkNode = chunkMap.get(edge.target)
      const docNode = documentMap.get(edge.source)
      if (docNode && chunkNode) {
        const list = chunkByDoc.get(docNode.id) || []
        list.push(chunkNode)
        chunkByDoc.set(docNode.id, list)
      }
      continue
    }
    if (edge.type === 'tag' && visibleSet.has('tag')) {
      const list = tagLinksByChunk.get(edge.source) || []
      list.push(edge)
      tagLinksByChunk.set(edge.source, list)
      continue
    }
    if (edge.type === 'similar') {
      similarityEdges.push(edge)
    }
  }

  const keptDocs = documents
    .map((doc) => ({
      doc,
      chunks: chunkByDoc.get(doc.id) || [],
    }))
    .sort((left, right) => right.chunks.length - left.chunks.length || right.doc.size - left.doc.size)
    .slice(0, limits.maxDocs)

  const keptDocIds = new Set(keptDocs.map((item) => item.doc.id))
  const keptChunkIds = new Set<string>()
  const keptTagIds = new Set<string>()
  const nodes = new Map<string, GraphNode>()
  const links: GraphLink[] = []

  for (const { doc, chunks: docChunks } of keptDocs) {
    nodes.set(doc.id, doc)
    const sortedChunks = [...docChunks].sort((left, right) => (right.value || 0) - (left.value || 0)).slice(0, limits.maxChunks)
    const hiddenChunkCount = Math.max(0, docChunks.length - sortedChunks.length)

    for (const chunk of sortedChunks) {
      keptChunkIds.add(chunk.id)
      nodes.set(chunk.id, chunk)
      links.push({ source: doc.id, target: chunk.id, type: 'contains', score: 1, color: edgeColor('contains') })

      const tagsForChunk = tagLinksByChunk.get(chunk.id) || []
      const visibleTagIds = tagsForChunk
        .map((edge) => edge.target)
        .filter((tagId) => visibleSet.has('tag') && tagMap.has(tagId))
        .slice(0, limits.maxTags)

      for (const tagId of visibleTagIds) {
        const tagNode = tagMap.get(tagId)
        if (!tagNode) continue
        nodes.set(tagNode.id, tagNode)
        links.push({ source: chunk.id, target: tagNode.id, type: 'tag', score: 1, color: edgeColor('tag') })
      }
    }

    if (hiddenChunkCount > 0) {
      const aggregateId = `aggregate-doc-${doc.id}`
      const aggregateNode: GraphNode = {
        id: aggregateId,
        name: `+${hiddenChunkCount} 个 chunk`,
        type: 'aggregate',
        category: 'aggregate',
        value: hiddenChunkCount,
        meta: { doc_id: doc.id, hidden_chunks: hiddenChunkCount },
        color: nodeColor('aggregate'),
        size: nodeSize({ id: aggregateId, name: '', type: 'aggregate', category: 'aggregate', value: hiddenChunkCount, meta: {} }),
      }
      nodes.set(aggregateId, aggregateNode)
      links.push({ source: doc.id, target: aggregateId, type: 'contains', score: 1, color: edgeColor('contains') })
    }
  }

  if (documents.length > keptDocs.length) {
    const hiddenDocCount = documents.length - keptDocs.length
    const aggregateId = 'aggregate-documents'
    const aggregateNode: GraphNode = {
      id: aggregateId,
      name: `+${hiddenDocCount} 个文档`,
      type: 'aggregate',
      category: 'aggregate',
      value: hiddenDocCount,
      meta: { hidden_documents: hiddenDocCount },
      color: nodeColor('aggregate'),
      size: nodeSize({ id: aggregateId, name: '', type: 'aggregate', category: 'aggregate', value: hiddenDocCount, meta: {} }),
    }
    nodes.set(aggregateId, aggregateNode)
  }

  for (const edge of similarityEdges) {
    if (!keptChunkIds.has(edge.source) || !keptChunkIds.has(edge.target)) {
      continue
    }
    if (!visibleSet.has('chunk')) {
      continue
    }
    links.push({ ...edge, color: edgeColor(edge.type) })
  }

  return { nodes: [...nodes.values()], links }
}

watch([displayGraph, showLabels], scheduleRender, { flush: 'post' })

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
  font-size: 15px;
  font-weight: 500;
}
.graph-stage {
  position: relative;
  height: calc(100vh - 320px);
  min-height: 600px;
  border: 1px solid #eaecf0;
  border-radius: 10px;
  overflow: hidden;
  background: #fff;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
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
  padding: 12px 14px;
  color: #475467;
  background: rgba(255, 255, 255, 0.96);
  border: 1px solid #eaecf0;
  border-radius: 10px;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.16);
  pointer-events: none;
  font-size: 15px;
  line-height: 1.6;
}
.tooltip-title {
  margin-bottom: 4px;
  color: #101828;
  font-weight: 700;
  font-size: 17px;
}
.graph-shell :deep(.ant-spin-nested-loading),
.graph-shell :deep(.ant-spin-container) {
  flex: 1;
  min-height: 0;
}
</style>
