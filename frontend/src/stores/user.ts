import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { UserInfo } from '@/types'
import { getCurrentUser } from '@/api/auth'

export const useUserStore = defineStore('user', () => {
  const userInfo = ref<UserInfo | null>(null)
  const token = ref(localStorage.getItem('access_token') || '')

  const isLoggedIn = computed(() => !!token.value)
  const permissions = computed(() => userInfo.value?.permissions || [])
  const roles = computed(() => userInfo.value?.roles || [])

  function setToken(accessToken: string, refreshToken: string) {
    token.value = accessToken
    localStorage.setItem('access_token', accessToken)
    localStorage.setItem('refresh_token', refreshToken)
  }

  async function fetchUserInfo() {
    try {
      const res = await getCurrentUser()
      userInfo.value = res.data.data
    } catch {
      logout()
    }
  }

  function hasPermission(perm: string): boolean {
    if (roles.value.includes('admin')) return true
    return permissions.value.includes(perm)
  }

  function logout() {
    token.value = ''
    userInfo.value = null
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
  }

  return { userInfo, token, isLoggedIn, permissions, roles, setToken, fetchUserInfo, hasPermission, logout }
})
