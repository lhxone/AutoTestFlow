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
