import request from '@/utils/request'

export interface KnowledgeBaseConfig {
  enabled: boolean
  vector_store_type: string
  vector_store_host: string
  vector_store_port: number
  vector_store_collection: string
  embedding_provider: string
  embedding_api_key: string
  embedding_base_url: string
  embedding_model: string
  embedding_dimension: number
  embedding_batch_size: number
  chunk_size: number
  chunk_overlap: number
  top_k: number
  similarity_threshold: number
}

export interface KnowledgeBase {
  id: number
  project_id: number
  name: string
  description: string
  status: number
  chunk_size: number
  chunk_overlap: number
  created_at: string
  updated_at: string
}

export interface KnowledgeDocument {
  id: number
  kb_id: number
  source_type: string
  source_path: string
  title: string
  content_size: number
  chunk_count: number
  status: string
  error_msg: string
  created_at: string
  updated_at: string
}

export interface KnowledgeStats {
  document_count: number
  chunk_count: number
  vector_count: number
  graph_nodes: number
  graph_edges: number
}

export interface KnowledgeGraphData {
  nodes: Array<{ id: string; name: string; type: string; category: string; value: number; meta: Record<string, any> }>
  edges: Array<{ source: string; target: string; type: string; score: number }>
}

export interface KnowledgeSearchResult {
  id: string
  content: string
  score: number
  metadata: Record<string, any>
}

export interface KnowledgeChatMessage {
  role: 'user' | 'assistant'
  content: string
}

export interface KnowledgeChatResponse {
  answer: string
  sources: KnowledgeSearchResult[]
  agent?: {
    id: number
    name: string
    provider: string
    model: string
  }
}

export function getKnowledgeConfig() {
  return request.get('/knowledge-base/config')
}

export function saveKnowledgeConfig(data: KnowledgeBaseConfig) {
  return request.put('/knowledge-base/config', data)
}

export function createKnowledgeBase(data: Partial<KnowledgeBase>) {
  return request.post('/knowledge-bases', data)
}

export function getKnowledgeBases(params: { project_id: number; keyword?: string; page?: number; page_size?: number }) {
  return request.get('/knowledge-bases', { params })
}

export function getKnowledgeBase(id: number, projectId: number) {
  return request.get(`/knowledge-bases/${id}`, { params: { project_id: projectId } })
}

export function updateKnowledgeBase(id: number, data: Partial<KnowledgeBase>) {
  return request.put(`/knowledge-bases/${id}`, data)
}

export function deleteKnowledgeBase(id: number, projectId: number) {
  return request.delete(`/knowledge-bases/${id}`, { params: { project_id: projectId } })
}

export function getKnowledgeStats(id: number, projectId: number) {
  return request.get(`/knowledge-bases/${id}/stats`, { params: { project_id: projectId } })
}

export function addKnowledgeDocument(id: number, data: any) {
  return request.post(`/knowledge-bases/${id}/documents`, data)
}

export function batchAddKnowledgeDocuments(id: number, data: any) {
  return request.post(`/knowledge-bases/${id}/documents/batch`, data)
}

export function getKnowledgeDocuments(id: number, params: { project_id: number; page?: number; page_size?: number }) {
  return request.get(`/knowledge-bases/${id}/documents`, { params })
}

export function rebuildKnowledgeDocument(id: number, docId: number, projectId: number) {
  return request.post(`/knowledge-bases/${id}/documents/${docId}/rebuild`, null, { params: { project_id: projectId } })
}

export function deleteKnowledgeDocument(id: number, docId: number, projectId: number) {
  return request.delete(`/knowledge-bases/${id}/documents/${docId}`, { params: { project_id: projectId } })
}

export function rebuildKnowledgeBase(id: number, projectId: number) {
  return request.post(`/knowledge-bases/${id}/chunks/rebuild`, null, { params: { project_id: projectId } })
}

export function queryKnowledgeBase(id: number, data: { project_id: number; query: string; top_k?: number; keywords?: string[] }) {
  return request.post(`/knowledge-bases/${id}/query`, data)
}

export function chatKnowledgeBase(
  id: number,
  data: { project_id: number; query: string; top_k?: number; keywords?: string[]; agent_id?: number; messages?: KnowledgeChatMessage[] },
) {
  return request.post(`/knowledge-bases/${id}/chat`, data, { timeout: 180000 })
}

export function chatKnowledgeBaseStream(
  id: number,
  data: { project_id: number; query: string; top_k?: number; keywords?: string[]; agent_id?: number; messages?: KnowledgeChatMessage[] },
  onEvent: (event: ChatStreamEvent) => void,
  onError?: (error: Error) => void,
) {
  const token = localStorage.getItem('access_token') || ''
  fetch(`/api/knowledge-bases/${id}/chat/stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(data),
  })
    .then(async (response) => {
      if (!response.ok) {
        const text = await response.text()
        throw new Error(text || `HTTP ${response.status}`)
      }
      const reader = response.body?.getReader()
      if (!reader) throw new Error('无法读取流')
      const decoder = new TextDecoder()
      let buffer = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const event = JSON.parse(line.slice(6)) as ChatStreamEvent
              onEvent(event)
            } catch {}
          }
        }
      }
    })
    .catch((err) => {
      onError?.(err)
    })
}

export interface ChatStreamEvent {
  type: 'sources' | 'thinking' | 'content' | 'done' | 'error'
  sources?: KnowledgeSearchResult[]
  content?: string
  agent?: {
    id: number
    name: string
    provider: string
    model: string
  }
}

export function getKnowledgeGraph(id: number, projectId: number) {
  return request.get(`/knowledge-bases/${id}/graph`, { params: { project_id: projectId } })
}
