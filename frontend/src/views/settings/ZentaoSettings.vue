<template>
  <div>
    <a-page-header :title="t('settings.zentao.title')" :sub-title="t('settings.zentao.subtitle')" />

    <a-spin :spinning="loading">
      <a-card :title="t('settings.zentao.connection')" style="margin-bottom: 16px">
        <a-form :model="form" layout="vertical" style="max-width: 600px">
          <a-form-item :label="t('settings.zentao.baseUrl')" required>
            <a-input v-model:value="form.base_url" placeholder="https://10.110.63.81:30008" />
            <div class="field-hint">{{ t('settings.zentao.baseUrlHint') }}</div>
          </a-form-item>
          <a-form-item :label="t('settings.zentao.account')" required>
            <a-input v-model:value="form.account" placeholder="admin" />
          </a-form-item>
          <a-form-item :label="t('settings.zentao.password')" required>
            <a-input-password v-model:value="form.password" :placeholder="t('settings.zentao.passwordPlaceholder')" />
          </a-form-item>
          <a-form-item :label="t('settings.zentao.token')">
            <a-input v-model:value="form.token" disabled :placeholder="t('settings.zentao.tokenPlaceholder')" />
            <div class="field-hint">{{ t('settings.zentao.tokenHint') }}</div>
          </a-form-item>
        </a-form>

        <a-space>
          <a-button type="primary" ghost @click="handleTestConnection" :loading="testing">
            {{ t('settings.zentao.testConnection') }}
          </a-button>
        </a-space>

        <a-alert v-if="testResult" :type="testResult.success ? 'success' : 'error'"
                 :message="testResult.message" style="margin-top: 12px" show-icon closable />
      </a-card>

      <a-card :title="t('settings.zentao.syncStrategy')" style="margin-bottom: 16px">
        <a-form layout="vertical" style="max-width: 600px">
          <a-form-item :label="t('settings.zentao.autoSync')">
            <a-switch v-model:checked="syncEnabled" :checked-children="t('settings.zentao.syncEnabled')" :un-checked-children="t('settings.zentao.syncDisabled')" />
          </a-form-item>
          <a-form-item :label="t('settings.zentao.syncInterval')">
            <a-input-number v-model:value="form.sync_interval" :min="5" :max="1440" style="width: 200px" />
            <div class="field-hint">{{ t('settings.zentao.syncIntervalHint') }}</div>
          </a-form-item>
        </a-form>
      </a-card>

      <a-button type="primary" size="large" @click="handleSave" :loading="saving">
        {{ t('settings.zentao.save') }}
      </a-button>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { message } from 'ant-design-vue'
import { getZentaoSettings, saveZentaoSettings, testZentaoConnection } from '@/api/setting'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const testResult = ref<{ success: boolean; message: string } | null>(null)

const form = reactive({
  base_url: '',
  account: '',
  password: '',
  token: '',
  token_expire_at: '',
  sync_interval: '30',
  sync_enabled: '1',
})

const syncEnabled = computed({
  get: () => form.sync_enabled === '1',
  set: (v: boolean) => { form.sync_enabled = v ? '1' : '0' },
})

onMounted(fetchSettings)

async function fetchSettings() {
  loading.value = true
  try {
    const res = await getZentaoSettings()
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
  if (!form.base_url || !form.account || !form.password) {
    message.warning(t('settings.zentao.messages.required'))
    return
  }
  testing.value = true
  testResult.value = null
  try {
    const res = await testZentaoConnection({
      base_url: form.base_url,
      account: form.account,
      password: form.password === '******' ? '' : form.password,
    })
    testResult.value = res.data.data
    if (res.data.data.success && res.data.data.token) {
      form.token = '******'
    }
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
      { key: 'base_url', value: form.base_url, encrypted: 0, description: '禅道服务器地址' },
      { key: 'account', value: form.account, encrypted: 0, description: '禅道登录账号' },
      { key: 'password', value: form.password, encrypted: 1, description: '禅道登录密码' },
      { key: 'sync_interval', value: String(form.sync_interval), encrypted: 0, description: '同步频率(分钟)' },
      { key: 'sync_enabled', value: form.sync_enabled, encrypted: 0, description: '是否启用自动同步' },
    ]
    await saveZentaoSettings({ settings })
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
