<template>
  <div class="login-container">
    <div class="login-card">
      <div class="login-locale" :aria-label="t('common.language')">
        <GlobalOutlined class="locale-icon" />
        <button
          type="button"
          class="locale-pill"
          :class="{ active: currentLocale === 'zh-CN' }"
          @click="currentLocale = 'zh-CN'"
        >
          中
        </button>
        <button
          type="button"
          class="locale-pill"
          :class="{ active: currentLocale === 'en-US' }"
          @click="currentLocale = 'en-US'"
        >
          EN
        </button>
      </div>
      <h1 class="login-title">{{ t('login.title') }}</h1>
      <p class="login-subtitle">{{ t('login.subtitle') }}</p>
      <a-form :model="form" @finish="handleLogin" layout="vertical">
        <a-form-item name="username" :rules="[{ required: true, message: t('login.usernameRequired') }]">
          <a-input v-model:value="form.username" size="large" :placeholder="t('login.username')">
            <template #prefix><UserOutlined /></template>
          </a-input>
        </a-form-item>
        <a-form-item name="password" :rules="[{ required: true, message: t('login.passwordRequired') }]">
          <a-input-password v-model:value="form.password" size="large" :placeholder="t('login.password')">
            <template #prefix><LockOutlined /></template>
          </a-input-password>
        </a-form-item>
        <a-form-item>
          <a-button type="primary" html-type="submit" size="large" block :loading="loading">
            {{ t('login.submit') }}
          </a-button>
        </a-form-item>
      </a-form>
      <p class="login-hint">{{ t('login.defaultHint') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { UserOutlined, LockOutlined, GlobalOutlined } from '@ant-design/icons-vue'
import { useUserStore } from '@/stores/user'
import { login } from '@/api/auth'
import { useI18n } from 'vue-i18n'
import { setAppLocale } from '@/locales'

const router = useRouter()
const userStore = useUserStore()
const { t, locale } = useI18n()
const loading = ref(false)
const form = reactive({ username: '', password: '' })
const currentLocale = computed({
  get: () => locale.value,
  set: (value: string) => setAppLocale(value as 'zh-CN' | 'en-US'),
})

async function handleLogin() {
  loading.value = true
  try {
    const res = await login(form)
    const data = res.data.data
    userStore.setToken(data.access_token, data.refresh_token)
    await userStore.fetchUserInfo()
    message.success(t('login.success'))
    router.push('/dashboard')
  } catch {
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}
.login-card {
  position: relative;
  width: 400px;
  padding: 40px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
}
.login-locale {
  position: absolute;
  top: 14px;
  right: 14px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px;
  border-radius: 999px;
  background: rgba(102, 126, 234, 0.08);
  border: 1px solid rgba(102, 126, 234, 0.22);
}
.locale-icon {
  color: #667eea;
  font-size: 14px;
  margin: 0 4px 0 6px;
}
.locale-pill {
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: #667085;
  font-size: 12px;
  font-weight: 700;
  min-width: 34px;
  height: 26px;
  padding: 0 10px;
  cursor: pointer;
  transition: all 0.2s ease;
}
.locale-pill:hover {
  color: #344054;
}
.locale-pill.active {
  background: linear-gradient(135deg, #667eea, #7f56d9);
  color: #fff;
  box-shadow: 0 6px 14px rgba(102, 126, 234, 0.28);
}
.login-title {
  text-align: center;
  font-size: 28px;
  font-weight: bold;
  margin-bottom: 4px;
  color: #1a1a2e;
}
.login-subtitle {
  text-align: center;
  color: #888;
  margin-bottom: 32px;
}
.login-hint {
  text-align: center;
  color: #aaa;
  font-size: 12px;
  margin-top: 16px;
}
</style>
