<template>
  <div>
    <a-page-header :title="t('settings.gitlab.title')" :sub-title="t('settings.gitlab.subtitle')" />

    <a-spin :spinning="loading">
      <a-card :title="t('settings.gitlab.connection')" style="margin-bottom: 16px">
        <a-form :model="form" layout="vertical" style="max-width: 600px">
          <a-form-item :label="t('settings.gitlab.baseUrl')" required>
            <a-input v-model:value="form.base_url" placeholder="https://gitlab.example.com" />
            <div class="field-hint">{{ t('settings.gitlab.baseUrlHint') }}</div>
          </a-form-item>
          <a-form-item :label="t('settings.gitlab.accessToken')" required>
            <a-input-password v-model:value="form.access_token" placeholder="glpat-xxxxxxxxxxxx" />
            <div class="field-hint">
              {{ t('settings.gitlab.accessTokenHint') }}
            </div>
          </a-form-item>
          <a-form-item :label="t('settings.gitlab.apiVersion')">
            <a-select v-model:value="form.api_version" style="width: 200px">
              <a-select-option value="v4">v4 ({{ t('settings.gitlab.recommended') }})</a-select-option>
              <a-select-option value="v3">v3</a-select-option>
            </a-select>
          </a-form-item>
        </a-form>

        <a-space>
          <a-button type="primary" ghost @click="handleTestConnection" :loading="testing">
            {{ t('settings.gitlab.testConnection') }}
          </a-button>
        </a-space>

        <a-alert v-if="testResult" :type="testResult.success ? 'success' : 'error'"
                 :message="testResult.message" style="margin-top: 12px" show-icon closable />
      </a-card>

      <a-button type="primary" size="large" @click="handleSave" :loading="saving">
        {{ t('settings.gitlab.save') }}
      </a-button>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { getGitLabSettings, saveGitLabSettings, testGitLabConnection } from '@/api/setting'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const testResult = ref<{ success: boolean; message: string } | null>(null)

const form = reactive({
  base_url: '',
  access_token: '',
  api_version: 'v4',
})

onMounted(fetchSettings)

async function fetchSettings() {
  loading.value = true
  try {
    const res = await getGitLabSettings()
    const list = res.data.data as any[]
    for (const item of list) {
      if (item.key in form) {
        (form as any)[item.key] = item.value
      }
    }
  } finally {
    loading.value = false
  }
}

async function handleTestConnection() {
  if (!form.base_url || !form.access_token) {
    message.warning(t('settings.gitlab.messages.required'))
    return
  }
  testing.value = true
  testResult.value = null
  try {
    const res = await testGitLabConnection({
      base_url: form.base_url,
      access_token: form.access_token === '******' ? '' : form.access_token,
    })
    testResult.value = res.data.data
  } catch {
    testResult.value = { success: false, message: t('common.requestFailed') }
  } finally {
    testing.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    const settings = [
      { key: 'base_url', value: form.base_url, encrypted: 0, description: 'GitLab服务器地址' },
      { key: 'access_token', value: form.access_token, encrypted: 1, description: 'GitLab Personal Access Token' },
      { key: 'api_version', value: form.api_version, encrypted: 0, description: 'GitLab API版本' },
    ]
    await saveGitLabSettings({ settings })
    message.success(t('common.saveSuccess'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.field-hint {
  color: #999;
  font-size: 12px;
  margin-top: 4px;
}
</style>
