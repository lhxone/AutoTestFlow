import request from '@/utils/request'

export function getZentaoProjects() {
  return request.get('/zentao/projects')
}

export function getZentaoProducts() {
  return request.get('/zentao/products')
}

export function getZentaoBranches(productId: number) {
  return request.get(`/zentao/products/${productId}/branches`)
}

// 禅道用例相关接口
export function getZentaoTestCaseList(params: any) {
  return request.get('/test-cases/zentao', { params })
}

export function getZentaoTestCaseById(id: number) {
  return request.get(`/test-cases/zentao/${id}`)
}

export function syncZentaoTestCases(data: { project_id: number; full_sync?: boolean }) {
  return request.post('/test-cases/zentao/sync', data)
}

export function generateTestScript(data: { test_case_id: number; agent_id?: number }) {
  return request.post('/test-cases/zentao/generate', data)
}
