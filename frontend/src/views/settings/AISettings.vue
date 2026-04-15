<template>
  <div>
    <a-page-header :title="t('settings.ai.title')" :sub-title="t('settings.ai.subtitle')" />

    <a-spin :spinning="loading">
      <a-card :title="t('settings.ai.configuration')" style="margin-bottom: 16px">
        <a-form :model="form" layout="vertical" style="max-width: 600px">
          <a-form-item :label="t('settings.ai.provider')" required>
            <a-select v-model:value="form.provider">
              <a-select-option value="claude">Claude (Anthropic)</a-select-option>
              <a-select-option value="openai">OpenAI</a-select-option>
            </a-select>
          </a-form-item>

          <a-form-item :label="t('settings.ai.apiKey')" required>
            <a-input-password v-model:value="form.api_key" :placeholder="t('settings.ai.apiKeyPlaceholder')" />
          </a-form-item>

          <a-form-item :label="t('settings.ai.baseUrl')">
            <a-input v-model:value="form.base_url" :placeholder="t('settings.ai.baseUrlPlaceholder')" />
            <div class="field-hint">{{ t('settings.ai.baseUrlHint') }}</div>
          </a-form-item>

          <a-form-item :label="t('settings.ai.model')">
            <a-input v-model:value="form.model" :placeholder="t('settings.ai.modelPlaceholder')" />
            <div class="field-hint">{{ t('settings.ai.modelHint') }}</div>
          </a-form-item>

          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('settings.ai.maxTokens')">
                <a-input-number v-model:value="form.max_tokens" :min="1" :max="128000" style="width: 100%" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('settings.ai.temperature')">
                <a-slider v-model:value="form.temperature" :min="0" :max="2" :step="0.1" />
              </a-form-item>
            </a-col>
          </a-row>
        </a-form>
      </a-card>

      <a-button type="primary" size="large" @click="handleSave" :loading="saving">
        {{ t('settings.ai.save') }}
      </a-button>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { getAISettings, saveAISettings } from '@/api/setting'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)

const form = reactive({
  provider: 'claude',
  api_key: '',
  base_url: '',
  model: '',
  max_tokens: 4096,
  temperature: 0.7,
})

onMounted(fetchSettings)

async function fetchSettings() {
  loading.value = true
  try {
    const res = await getAISettings()
    const list = res.data.data as Array<{ key: string; value: string }>
    for (const item of list) {
      if (!(item.key in form)) continue
      if (item.key === 'max_tokens') {
        form.max_tokens = Number(item.value || 4096)
      } else if (item.key === 'temperature') {
        form.temperature = Number(item.value || 0.7)
      } else {
        ;(form as Record<string, any>)[item.key] = item.value
      }
    }
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!form.provider || !form.api_key) {
    message.warning(t('settings.ai.messages.required'))
    return
  }
  saving.value = true
  try {
    const settings = [
      { key: 'provider', value: form.provider, encrypted: 0, description: 'AI Provider' },
      { key: 'api_key', value: form.api_key, encrypted: 1, description: 'AI API Key' },
      { key: 'base_url', value: form.base_url, encrypted: 0, description: 'AI API Base URL' },
      { key: 'model', value: form.model, encrypted: 0, description: 'AI Model' },
      { key: 'max_tokens', value: String(form.max_tokens), encrypted: 0, description: 'Max Tokens' },
      { key: 'temperature', value: String(form.temperature), encrypted: 0, description: 'Temperature' },
    ]
    await saveAISettings({ settings })
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
  white-space: pre-wrap;
}
</style>
