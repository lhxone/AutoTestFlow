import request from '@/utils/request'
import type { LoginRequest, LoginResponse, UserInfo } from '@/types'

export function login(data: LoginRequest) {
  return request.post<any, { data: { data: LoginResponse } }>('/auth/login', data)
}

export function refreshToken(refresh_token: string) {
  return request.post('/auth/refresh', { refresh_token })
}

export function getCurrentUser() {
  return request.get<any, { data: { data: UserInfo } }>('/auth/me')
}

export function changePassword(data: { old_password: string; new_password: string }) {
  return request.put('/auth/password', data)
}

export function logout() {
  return request.post('/auth/logout')
}
