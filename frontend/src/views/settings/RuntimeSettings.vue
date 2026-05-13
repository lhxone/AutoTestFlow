<template>
  <div>
    <a-page-header :title="t('settings.runtime.title')" :sub-title="t('settings.runtime.subtitle')" />

    <a-spin :spinning="loading">
      <a-card :title="t('settings.runtime.overview')" class="runtime-card">
        <a-form :model="form" layout="vertical" class="runtime-form">
          <a-row :gutter="16">
            <a-col :xs="24" :md="8">
              <a-form-item
                :label="t('settings.runtime.maxConcurrentTasks')"
                :help="t('settings.runtime.maxConcurrentTasksHint')"
                required
              >
                <a-input-number
                  v-model:value="form.max_concurrent_tasks"
                  :min="1"
                  :precision="0"
                  class="full-width"
                />
              </a-form-item>
            </a-col>

            <a-col :xs="24" :md="8">
              <a-form-item
                :label="t('settings.runtime.taskTimeoutMinutes')"
                :help="t('settings.runtime.taskTimeoutHint')"
                required
              >
                <a-input-number
                  v-model:value="form.task_timeout_minutes"
                  :min="1"
                  :precision="0"
                  class="full-width"
                />
              </a-form-item>
            </a-col>

            <a-col :xs="24" :md="8">
              <a-form-item
                :label="t('settings.runtime.pendingGenerateIntervalMinutes')"
                :help="t('settings.runtime.pendingGenerateIntervalHint')"
                required
              >
                <a-input-number
                  v-model:value="form.pending_generate_interval_minutes"
                  :min="1"
                  :precision="0"
                  class="full-width"
                />
              </a-form-item>
            </a-col>
          </a-row>
        </a-form>
      </a-card>

      <a-button type="primary" size="large" :loading="saving" @click="handleSave">
        {{ t('settings.runtime.save') }}
      </a-button>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { getRuntimeSettings, saveRuntimeSettings } from '@/api/setting'

const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)

const form = reactive({
  max_concurrent_tasks: 1,
  task_timeout_minutes: 30,
  pending_generate_interval_minutes: 1,
})

onMounted(fetchSettings)

async function fetchSettings() {
  loading.value = true
  try {
    const res = await getRuntimeSettings()
    const list = res.data.data as Array<{ key: keyof typeof form; value: string }>
    for (const item of list) {
      if (!(item.key in form)) continue
      const value = Number.parseInt(String(item.value || ''), 10)
      if (Number.isInteger(value) && value > 0) {
        form[item.key] = value
      }
    }
  } finally {
    loading.value = false
  }
}

function isPositiveInteger(value: number) {
  return Number.isInteger(value) && value > 0
}

async function handleSave() {
  if (
    !isPositiveInteger(form.max_concurrent_tasks) ||
    !isPositiveInteger(form.task_timeout_minutes) ||
    !isPositiveInteger(form.pending_generate_interval_minutes)
  ) {
    message.warning(t('settings.runtime.messages.positiveInteger'))
    return
  }

  saving.value = true
  try {
    await saveRuntimeSettings({
      settings: [
        {
          key: 'max_concurrent_tasks',
          value: String(form.max_concurrent_tasks),
          encrypted: 0,
          description: '待生成工单与流水线触发生成任务的最大并行数量',
        },
        {
          key: 'task_timeout_minutes',
          value: String(form.task_timeout_minutes),
          encrypted: 0,
          description: '生成类任务最大执行超时时间(分钟)',
        },
        {
          key: 'pending_generate_interval_minutes',
          value: String(form.pending_generate_interval_minutes),
          encrypted: 0,
          description: '待生成工单定时巡检间隔(分钟)',
        },
      ],
    })
    message.success(t('common.saveSuccess'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.runtime-card {
  margin-bottom: 16px;
}

.runtime-form {
  max-width: 1080px;
}

.full-width {
  width: 100%;
}
</style>
