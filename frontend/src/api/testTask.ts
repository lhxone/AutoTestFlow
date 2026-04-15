import request from '@/utils/request'

export function getTestTaskList(params: any) {
  return request.get('/test-tasks', { params })
}

export function getTestTaskById(id: number) {
  return request.get(`/test-tasks/${id}`)
}

export function createTestTask(data: { issue_id: number; agent_id?: number; workflow_name?: string }) {
  return request.post('/test-tasks', data)
}

export function getTestCases(taskId: number) {
  return request.get(`/test-tasks/${taskId}/cases`)
}

export function getTestScripts(taskId: number) {
  return request.get(`/test-tasks/${taskId}/scripts`)
}

export function createTestTaskEventSource(taskId: number) {
  const token = localStorage.getItem('access_token') || ''
  const query = token ? `?access_token=${encodeURIComponent(token)}` : ''
  return new EventSource(`/api/test-tasks/${taskId}/events${query}`)
}

export function updateTestCase(id: number, data: any) {
  return request.put(`/test-cases/${id}`, data)
}

export function getTestCaseList(params: any) {
  return request.get('/test-cases', { params })
}

export function updateTestScript(id: number, data: any) {
  return request.put(`/test-scripts/${id}`, data)
}

export function getExecutionList(params: any) {
  return request.get('/executions', { params })
}

export function getTaskLogs(taskId: number) {
  return request.get(`/test-tasks/${taskId}/logs`)
}

export function getSelfTestReport(taskId: number, framework: 'playwright' | 'midscene') {
  return request.get(`/test-tasks/${taskId}/self-test-report`, { params: { framework } })
}

export interface CLIInteraction {
  id: number;
  task_id: number;
  interaction_type: string;
  content: string;
  metadata: any;
  status: string;
  user_response?: string;
  user_id?: number;
  responded_at?: string;
  created_at: string;
  updated_at: string;
}

export function getCLIInteractions(taskId: number) {
  return request.get(`/test-tasks/${taskId}/interactions`)
}

export function getPendingInteractions(taskId: number) {
  return request.get(`/test-tasks/${taskId}/interactions/pending`)
}

export function replyInteraction(taskId: number, interactionId: number, response: string) {
  return request.post(`/test-tasks/${taskId}/interactions/${interactionId}/reply`, { response })
}

export function approveInteraction(taskId: number, interactionId: number) {
  return request.post(`/test-tasks/${taskId}/interactions/${interactionId}/approve`)
}

export function rejectInteraction(taskId: number, interactionId: number, reason: string) {
  return request.post(`/test-tasks/${taskId}/interactions/${interactionId}/reject`, { reason })
}
