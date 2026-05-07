import request from '@/utils/request'

// 禅道设置
export function getZentaoSettings() {
  return request.get('/settings/zentao')
}

export function saveZentaoSettings(data: { settings: any[] }) {
  return request.put('/settings/zentao', data)
}

export function testZentaoConnection(data: { base_url: string; account: string; password: string }) {
  return request.post('/settings/zentao/test', data)
}

// GitLab设置
export function getGitLabSettings() {
  return request.get('/settings/gitlab')
}

export function saveGitLabSettings(data: { settings: any[] }) {
  return request.put('/settings/gitlab', data)
}

export function testGitLabConnection(data: { base_url: string; access_token: string }) {
  return request.post('/settings/gitlab/test', data)
}

// 邮件设置
export function getMailSettings() {
  return request.get('/settings/mail')
}

export function saveMailSettings(data: { settings: any[] }) {
  return request.put('/settings/mail', data)
}

export function testMailConnection(data: {
  host: string
  port: number
  username: string
  password: string
  from: string
  use_ssl: boolean
}) {
  return request.post('/settings/mail/test', data)
}

export function sendTestMail(data: {
  recipient: string
  template_type: 'review_result' | 'test_report'
  subject_template?: string
  body_template?: string
}) {
  return request.post('/settings/mail/send-test', data)
}

// CLI Runtime 设置
export function getCLIRuntimeSettings() {
  return request.get('/settings/cli-runtime')
}

export function saveCLIRuntimeSettings(data: { settings: any[] }) {
  return request.put('/settings/cli-runtime', data)
}
