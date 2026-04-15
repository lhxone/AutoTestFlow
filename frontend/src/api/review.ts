import request from '@/utils/request'

export function getReviewList(params: any) {
  return request.get('/reviews', { params })
}

export function getReviewDetail(id: number) {
  return request.get(`/reviews/${id}`)
}

export function doReview(id: number, data: { action: string; comment?: string }) {
  return request.post(`/reviews/${id}/review`, data)
}
