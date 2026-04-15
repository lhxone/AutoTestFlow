import request from '@/utils/request'

export function getProjectList(params: any) {
  return request.get('/projects', { params })
}

export function getProjectById(id: number) {
  return request.get(`/projects/${id}`)
}

export function createProject(data: any) {
  return request.post('/projects', data)
}

export function updateProject(id: number, data: any) {
  return request.put(`/projects/${id}`, data)
}

export function deleteProject(id: number) {
  return request.delete(`/projects/${id}`)
}

export function getProjectIssueSyncLogs(id: number, params: any) {
  return request.get(`/projects/${id}/issue-sync-logs`, { params })
}

export function getProjectIssueSyncLogDetail(id: number, logId: number, params?: { page?: number; page_size?: number }) {
  return request.get(`/projects/${id}/issue-sync-logs/${logId}`, { params })
}

// 全局采集记录 API
export function getAllIssueSyncLogs(params: any) {
  return request.get('/issue-sync-logs', { params })
}

export function getIssueSyncLogDetail(logId: number) {
  return request.get(`/issue-sync-logs/${logId}`)
}
