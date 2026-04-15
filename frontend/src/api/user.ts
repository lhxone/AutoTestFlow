import request from '@/utils/request'

export function getUserList(params: any) {
  return request.get('/users', { params })
}

export function getLoginLogList(params: any) {
  return request.get('/users/login-logs', { params })
}

export function getUserById(id: number) {
  return request.get(`/users/${id}`)
}

export function createUser(data: any) {
  return request.post('/users', data)
}

export function updateUser(id: number, data: any) {
  return request.put(`/users/${id}`, data)
}

export function deleteUser(id: number) {
  return request.delete(`/users/${id}`)
}
