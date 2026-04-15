import request from '@/utils/request'

// Agent
export function getAgentList(params: any) {
  return request.get('/agents', { params })
}

export function getAgentById(id: number) {
  return request.get(`/agents/${id}`)
}

export function createAgent(data: any) {
  return request.post('/agents', data)
}

export function updateAgent(id: number, data: any) {
  return request.put(`/agents/${id}`, data)
}

export function deleteAgent(id: number) {
  return request.delete(`/agents/${id}`)
}

export function testAgentConnection(data: any) {
  return request.post('/agents/test', data)
}

// Workflow
export function getWorkflowList() {
  return request.get('/workflows')
}

export function createWorkflow(data: any) {
  return request.post('/workflows', data)
}

export function updateWorkflow(id: number, data: any) {
  return request.put(`/workflows/${id}`, data)
}

export function deleteWorkflow(id: number) {
  return request.delete(`/workflows/${id}`)
}

// MCP Server
export function getMCPServerList() {
  return request.get('/mcp-servers')
}

export function createMCPServer(data: any) {
  return request.post('/mcp-servers', data)
}
