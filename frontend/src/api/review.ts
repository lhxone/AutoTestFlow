import request from '@/utils/request'

export interface ReviewActionResult {
  new_task_id?: number
}

export function getReviewList(params: any) {
  return request.get('/reviews', { params })
}

export function getReviewDetail(id: number) {
  return request.get(`/reviews/${id}`)
}

export function doReview(id: number, data: { action: string; comment?: string }) {
  return request.post<{ data: ReviewActionResult }>(`/reviews/${id}/review`, data)
}
