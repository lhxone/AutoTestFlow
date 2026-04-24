import axios from 'axios'
import type { ApiResponse } from '@/types'
import { message } from 'ant-design-vue'
import router from '@/router'
import i18n from '@/locales'

const request = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

// 扩展 axios 配置类型，支持 silent 模式
declare module 'axios' {
  interface AxiosRequestConfig {
    silent?: boolean
  }
}

// 请求拦截器: 注入 Token
request.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器: 统一错误处理
request.interceptors.response.use(
  (response) => {
    const data = response.data as ApiResponse
    if (data.code !== 0) {
      // silent 模式下不显示错误提示
      if (!response.config.silent) {
        message.error(data.message || i18n.global.t('common.requestFailed'))
      }
      // 未授权跳转登录
      if (data.code === 10002 || data.code === 401) {
        localStorage.removeItem('access_token')
        localStorage.removeItem('refresh_token')
        router.push('/login')
      }
      return Promise.reject(new Error(data.message))
    }
    return response
  },
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
      router.push('/login')
    }
    // silent 模式下不显示错误提示
    if (!error.config?.silent) {
      message.error(error.message || i18n.global.t('common.networkError'))
    }
    return Promise.reject(error)
  }
)

export default request
