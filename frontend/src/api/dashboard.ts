import request from '@/utils/request'

export function getDashboardStats() {
  return request.get('/dashboard/stats')
}

export function getRecentActivities() {
  return request.get('/dashboard/recent-activities')
}
