<template>
  <div>
    <a-page-header :title="t('settings.cliRuntime.title')" :sub-title="t('settings.cliRuntime.subtitle')" />

    <a-spin :spinning="loading">
      <a-alert
        type="info"
        show-icon
        style="margin-bottom: 16px"
        :message="t('settings.cliRuntime.helpTitle')"
        :description="t('settings.cliRuntime.helpContent')"
      />

      <a-card :title="t('settings.cliRuntime.configuration')" style="margin-bottom: 16px">
        <a-form :model="form" layout="vertical" style="max-width: 900px">
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('settings.cliRuntime.command')" required>
                <a-input v-model:value="form.command" :placeholder="t('settings.cliRuntime.commandPlaceholder')" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('settings.cliRuntime.timeout')">
                <a-input v-model:value="form.timeout" placeholder="20m" />
              </a-form-item>
            </a-col>
          </a-row>

          <a-form-item :label="t('settings.cliRuntime.argsJson')">
            <a-textarea v-model:value="form.args_json" :rows="4" placeholder='["exec", "--cd", "{{repo_dir}}"]' />
            <div class="field-hint">{{ t('settings.cliRuntime.argsJsonHint') }}</div>
          </a-form-item>

          <a-form-item :label="t('settings.cliRuntime.envJson')">
            <a-textarea v-model:value="form.env_json" :rows="4" placeholder='{"HTTP_PROXY":"http://127.0.0.1:7890"}' />
          </a-form-item>

          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item 
                :label="t('settings.cliRuntime.workspaceRoot')"
                :help="t('settings.cliRuntime.workspaceRootDescription')"
              >
                <a-input 
                  v-model:value="form.workspace_root" 
                  :placeholder="t('settings.cliRuntime.workspaceRootPlaceholder')"
                />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('settings.cliRuntime.preserveWorkspace')">
                <a-switch v-model:checked="form.preserve_workspace" />
              </a-form-item>
            </a-col>
          </a-row>

          <a-row :gutter="16">
            <a-col :span="8">
              <a-form-item :label="t('settings.cliRuntime.repoDirName')">
                <a-input v-model:value="form.repo_dir_name" />
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('settings.cliRuntime.controlDirName')">
                <a-input v-model:value="form.control_dir_name" />
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('settings.cliRuntime.logFileName')">
                <a-input v-model:value="form.log_file_name" />
              </a-form-item>
            </a-col>
          </a-row>

          <a-row :gutter="16">
            <a-col :span="8">
              <a-form-item :label="t('settings.cliRuntime.inputFileName')">
                <a-input v-model:value="form.input_file_name" />
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('settings.cliRuntime.promptFileName')">
                <a-input v-model:value="form.prompt_file_name" />
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('settings.cliRuntime.resultFileName')">
                <a-input v-model:value="form.result_file_name" />
              </a-form-item>
            </a-col>
          </a-row>
        </a-form>
      </a-card>

      <a-button type="primary" size="large" @click="handleSave" :loading="saving">
        {{ t('settings.cliRuntime.save') }}
      </a-button>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { getCLIRuntimeSettings, saveCLIRuntimeSettings } from '@/api/setting'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)

const form = reactive({
  command: '',
  args_json: '[]',
  timeout: '20m',
  workspace_root: '',
  repo_dir_name: 'repo',
  control_dir_name: '.autotestflow',
  input_file_name: 'input.json',
  prompt_file_name: 'prompt.md',
  result_file_name: 'result.json',
  log_file_name: 'cli.log',
  preserve_workspace: true,
  env_json: '{}',
})

onMounted(fetchSettings)

async function fetchSettings() {
  loading.value = true
  try {
    const res = await getCLIRuntimeSettings()
    const list = res.data.data as Array<{ key: string; value: string }>
    for (const item of list) {
      if (!(item.key in form)) continue
      if (item.key === 'preserve_workspace') {
        form.preserve_workspace = ['1', 'true', 'yes', 'on'].includes(String(item.value).toLowerCase())
      } else {
        ;(form as Record<string, any>)[item.key] = item.value
      }
    }
  } finally {
    loading.value = false
  }
}

function safeJSON(raw: string, fallback: string) {
  const text = String(raw || '').trim()
  if (!text) return fallback
  try {
    const parsed = JSON.parse(text)
    return JSON.stringify(parsed)
  } catch {
    return ''
  }
}

async function handleSave() {
  if (!form.command.trim()) {
    message.warning(t('settings.cliRuntime.messages.commandRequired'))
    return
  }

  const argsJSON = safeJSON(form.args_json, '[]')
  if (!argsJSON || !Array.isArray(JSON.parse(argsJSON))) {
    message.warning(t('settings.cliRuntime.messages.argsInvalid'))
    return
  }

  const envJSON = safeJSON(form.env_json, '{}')
  if (!envJSON || Array.isArray(JSON.parse(envJSON)) || typeof JSON.parse(envJSON) !== 'object') {
    message.warning(t('settings.cliRuntime.messages.envInvalid'))
    return
  }

  saving.value = true
  try {
    const settings = [
      { key: 'command', value: form.command.trim(), encrypted: 0, description: 'CLI 可执行命令' },
      { key: 'args_json', value: argsJSON, encrypted: 0, description: 'CLI 参数(JSON数组)' },
      { key: 'timeout', value: form.timeout.trim(), encrypted: 0, description: 'CLI 超时时间' },
      { key: 'workspace_root', value: form.workspace_root.trim(), encrypted: 0, description: 'CLI 工作区根目录' },
      { key: 'repo_dir_name', value: form.repo_dir_name.trim(), encrypted: 0, description: '仓库目录名' },
      { key: 'control_dir_name', value: form.control_dir_name.trim(), encrypted: 0, description: '控制目录名' },
      { key: 'input_file_name', value: form.input_file_name.trim(), encrypted: 0, description: '输入文件名' },
      { key: 'prompt_file_name', value: form.prompt_file_name.trim(), encrypted: 0, description: 'Prompt 文件名' },
      { key: 'result_file_name', value: form.result_file_name.trim(), encrypted: 0, description: '结果文件名' },
      { key: 'log_file_name', value: form.log_file_name.trim(), encrypted: 0, description: '日志文件名' },
      { key: 'preserve_workspace', value: String(form.preserve_workspace), encrypted: 0, description: '是否保留工作区' },
      { key: 'env_json', value: envJSON, encrypted: 0, description: '额外环境变量(JSON对象)' },
    ]
    await saveCLIRuntimeSettings({ settings })
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
