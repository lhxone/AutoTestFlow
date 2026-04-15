import request from '@/utils/request'

export function getIssueList(params: any) {
  return request.get('/issues', { params })
}

export function getIssueById(id: number) {
  return request.get(`/issues/${id}`)
}

export function syncIssues(data: { project_id: number; full_sync: boolean }) {
  return request.post('/issues/sync', data)
}

export function updateTestStatus(id: number, data: { test_status: string; remark?: string }) {
  return request.put(`/issues/${id}/test-status`, data)
}

export function getInterventionHistory(issueId: number) {
  return request.get(`/issues/${issueId}/interventions`)
}
