<template>
  <div>
    <a-page-header :title="t('settings.mail.title')" :sub-title="t('settings.mail.subtitle')" />

    <a-spin :spinning="loading">
      <a-card :title="t('settings.mail.connection')" style="margin-bottom: 16px">
        <a-form :model="form" layout="vertical" style="max-width: 840px">
          <a-row :gutter="16">
            <a-col :span="16">
              <a-form-item :label="t('settings.mail.host')" required>
                <a-input v-model:value="form.host" placeholder="smtp.example.com" />
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('settings.mail.port')" required>
                <a-input-number v-model:value="form.port" :min="1" style="width: 100%" />
              </a-form-item>
            </a-col>
          </a-row>

          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('settings.mail.username')">
                <a-input v-model:value="form.username" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('settings.mail.password')">
                <a-input-password v-model:value="form.password" :placeholder="t('settings.mail.passwordPlaceholder')" />
              </a-form-item>
            </a-col>
          </a-row>

          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('settings.mail.from')" required>
                <a-input v-model:value="form.from" placeholder="autotest@example.com" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('settings.mail.useSsl')">
                <a-switch v-model:checked="form.use_ssl" />
              </a-form-item>
            </a-col>
          </a-row>

          <a-form-item :label="t('settings.mail.defaultRecipients')">
            <a-textarea v-model:value="form.default_recipients" :rows="4" :placeholder="t('settings.mail.defaultRecipientsPlaceholder')" />
            <div class="field-hint">{{ t('settings.mail.defaultRecipientsHint') }}</div>
          </a-form-item>
        </a-form>

        <a-space>
          <a-button type="primary" ghost @click="handleTestConnection" :loading="testing">
            {{ t('settings.mail.testConnection') }}
          </a-button>
        </a-space>

        <a-alert
          v-if="connectionResult"
          :type="connectionResult.success ? 'success' : 'error'"
          :message="connectionResult.message"
          style="margin-top: 12px"
          show-icon
          closable
        />
      </a-card>

      <a-card :title="t('settings.mail.templates')" style="margin-bottom: 16px">
        <div class="field-hint" style="margin-bottom: 12px">{{ templateHelpText }}</div>
        <a-form :model="testMailForm" layout="vertical" style="max-width: 520px; margin-bottom: 8px">
          <a-form-item :label="t('settings.mail.testRecipient')" required>
            <a-input v-model:value="testMailForm.recipient" placeholder="qa@example.com" />
          </a-form-item>
        </a-form>

        <a-form :model="form" layout="vertical">
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('settings.mail.reviewSubject')">
                <a-input v-model:value="form.review_result_subject_template" />
              </a-form-item>
              <a-button type="primary" ghost @click="handleSendTemplateMail('review_result')" :loading="sendingTemplate === 'review_result'">
                {{ t('settings.mail.sendReviewTestMail') }}
              </a-button>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('settings.mail.reportSubject')">
                <a-input v-model:value="form.test_report_subject_template" />
              </a-form-item>
              <a-button type="primary" ghost @click="handleSendTemplateMail('test_report')" :loading="sendingTemplate === 'test_report'">
                {{ t('settings.mail.sendReportTestMail') }}
              </a-button>
            </a-col>
          </a-row>

          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('settings.mail.reviewBody')">
                <a-textarea v-model:value="form.review_result_body_template" :rows="12" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('settings.mail.reportBody')">
                <a-textarea v-model:value="form.test_report_body_template" :rows="12" />
              </a-form-item>
            </a-col>
          </a-row>
        </a-form>

        <a-alert
          v-if="sendResult"
          :type="sendResult.success ? 'success' : 'error'"
          :message="sendResult.message"
          style="margin-top: 12px"
          show-icon
          closable
        />
      </a-card>

      <a-button type="primary" size="large" @click="handleSave" :loading="saving">
        {{ t('settings.mail.save') }}
      </a-button>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { getMailSettings, saveMailSettings, sendTestMail, testMailConnection } from '@/api/setting'
import { useI18n } from 'vue-i18n'

const { t, locale } = useI18n()
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const sendingTemplate = ref<'review_result' | 'test_report' | ''>('')
const connectionResult = ref<{ success: boolean; message: string } | null>(null)
const sendResult = ref<{ success: boolean; message: string } | null>(null)

const form = reactive({
  host: '',
  port: 465,
  username: '',
  password: '',
  from: '',
  use_ssl: true,
  default_recipients: '',
  review_result_subject_template: '[AutoTestFlow] Review结果 - {{title}}',
  review_result_body_template:
    '<h2>Review结果通知</h2>\n<p><strong>标题:</strong> {{title}}</p>\n<p><strong>状态:</strong> {{status}}</p>\n<p><strong>审核意见:</strong> {{review_note}}</p>\n<p><strong>Git推送:</strong> {{git_summary}}</p>\n<p><strong>Bug:</strong> {{issue_title}}</p>\n<p><strong>项目:</strong> {{project_name}}</p>',
  test_report_subject_template: '[AutoTestFlow] 测试报告 - {{title}}',
  test_report_body_template:
    '<h2>测试报告: {{title}}</h2>\n<p><strong>总用例数:</strong> {{total_cases}}</p>\n<p><strong>通过:</strong> {{passed_cases}} | <strong>失败:</strong> {{failed_cases}}</p>\n<p><strong>通过率:</strong> {{pass_rate}}%</p>\n<p><strong>是否经过人工介入:</strong> {{has_intervention}}</p>\n<hr>\n<p>{{summary}}</p>\n<p><a href="{{report_url}}">查看完整报告</a></p>',
})

const testMailForm = reactive({
  recipient: '',
})

const templateHelpText =
  locale.value === 'en-US'
    ? 'Available variables: {{title}} {{status}} {{review_note}} {{git_summary}} {{issue_title}} {{project_name}} {{summary}} {{total_cases}} {{passed_cases}} {{failed_cases}} {{pass_rate}} {{has_intervention}} {{report_url}}'
    : '可用变量：{{title}} {{status}} {{review_note}} {{git_summary}} {{issue_title}} {{project_name}} {{summary}} {{total_cases}} {{passed_cases}} {{failed_cases}} {{pass_rate}} {{has_intervention}} {{report_url}}'

onMounted(fetchSettings)

async function fetchSettings() {
  loading.value = true
  try {
    const res = await getMailSettings()
    const list = res.data.data as Array<{ key: string; value: string }>
    for (const item of list) {
      if (!(item.key in form)) continue
      if (item.key === 'port') {
        form.port = Number(item.value || 465)
      } else if (item.key === 'use_ssl') {
        form.use_ssl = item.value === '1' || item.value === 'true'
      } else {
        ;(form as Record<string, any>)[item.key] = item.value
      }
    }
  } finally {
    loading.value = false
  }
}

async function handleTestConnection() {
  if (!form.host || !form.port || !form.from) {
    message.warning(t('settings.mail.messages.required'))
    return
  }
  testing.value = true
  connectionResult.value = null
  try {
    const res = await testMailConnection({
      host: form.host,
      port: form.port,
      username: form.username,
      password: form.password === '******' ? '' : form.password,
      from: form.from,
      use_ssl: form.use_ssl,
    })
    connectionResult.value = res.data.data
  } catch {
    connectionResult.value = { success: false, message: t('common.requestFailed') }
  } finally {
    testing.value = false
  }
}

async function handleSendTemplateMail(templateType: 'review_result' | 'test_report') {
  if (!testMailForm.recipient) {
    message.warning(t('settings.mail.messages.recipientRequired'))
    return
  }
  sendingTemplate.value = templateType
  sendResult.value = null
  try {
    const subjectTemplate =
      templateType === 'review_result' ? form.review_result_subject_template : form.test_report_subject_template
    const bodyTemplate =
      templateType === 'review_result' ? form.review_result_body_template : form.test_report_body_template
    const res = await sendTestMail({
      recipient: testMailForm.recipient,
      template_type: templateType,
      subject_template: subjectTemplate,
      body_template: bodyTemplate,
    })
    sendResult.value = res.data.data
  } catch {
    sendResult.value = { success: false, message: t('common.requestFailed') }
  } finally {
    sendingTemplate.value = ''
  }
}

async function handleSave() {
  saving.value = true
  try {
    const settings = [
      { key: 'host', value: form.host, encrypted: 0, description: 'SMTP服务器地址' },
      { key: 'port', value: String(form.port), encrypted: 0, description: 'SMTP端口' },
      { key: 'username', value: form.username, encrypted: 0, description: 'SMTP用户名' },
      { key: 'password', value: form.password, encrypted: 1, description: 'SMTP密码' },
      { key: 'from', value: form.from, encrypted: 0, description: '发件人邮箱' },
      { key: 'use_ssl', value: form.use_ssl ? '1' : '0', encrypted: 0, description: '是否启用SSL' },
      { key: 'default_recipients', value: form.default_recipients, encrypted: 0, description: '默认收件人列表' },
      { key: 'review_result_subject_template', value: form.review_result_subject_template, encrypted: 0, description: 'Review结果邮件主题模板' },
      { key: 'review_result_body_template', value: form.review_result_body_template, encrypted: 0, description: 'Review结果邮件正文模板' },
      { key: 'test_report_subject_template', value: form.test_report_subject_template, encrypted: 0, description: '测试报告邮件主题模板' },
      { key: 'test_report_body_template', value: form.test_report_body_template, encrypted: 0, description: '测试报告邮件正文模板' },
    ]
    await saveMailSettings({ settings })
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
