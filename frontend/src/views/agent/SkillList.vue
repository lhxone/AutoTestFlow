<template>
  <div>
    <a-page-header :title="t('workflow.list.title')" />

    <a-row :gutter="16" style="margin-bottom: 16px">
      <a-col :span="6">
        <a-input v-model:value="keyword" :placeholder="t('workflow.list.searchPlaceholder')" allowClear />
      </a-col>
      <a-col>
        <a-button type="primary" @click="fetchData">{{ t('common.refresh') }}</a-button>
        <a-button v-if="canManage" style="margin-left: 8px" @click="openCreate">{{ t('workflow.list.create') }}</a-button>
      </a-col>
    </a-row>

    <a-table :dataSource="filteredList" :columns="columns" :loading="loading" rowKey="id" size="middle">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'workflow_type'">
          <a-tag :color="record.workflow_type === 'builtin' ? 'blue' : 'geekblue'">
            {{ translateWorkflowType(t, record.workflow_type) }}
          </a-tag>
        </template>
        <template v-if="column.key === 'status'">
          <a-tag :color="record.status === 1 ? 'green' : 'red'">{{ record.status === 1 ? t('common.enabled') : t('common.disabled') }}</a-tag>
        </template>
        <template v-if="column.key === 'prompt_template'">
          <div class="ellipsis-cell">{{ record.prompt_template || '-' }}</div>
        </template>
        <template v-if="column.key === 'action'">
          <a-button type="link" size="small" @click="handleEdit(record)" v-if="canManage">{{ t('common.edit') }}</a-button>
          <a-popconfirm v-if="canManage" :title="t('common.confirmDelete')" @confirm="handleDelete(record.id)">
            <a-button type="link" size="small" danger>{{ t('common.delete') }}</a-button>
          </a-popconfirm>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="showModal"
      :title="editingWorkflow ? t('workflow.list.edit') : t('workflow.list.create')"
      @ok="handleSubmit"
      :confirmLoading="submitting"
      width="1080px"
    >
      <a-form layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('workflow.list.form.name')" required>
              <a-input v-model:value="form.name" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('workflow.list.form.type')" required>
              <a-select v-model:value="form.workflow_type">
                <a-select-option value="builtin">builtin</a-select-option>
                <a-select-option value="custom">custom</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item :label="t('workflow.list.form.description')">
          <a-input v-model:value="form.description" />
        </a-form-item>
        <a-form-item :label="t('workflow.list.form.promptTemplate')">
          <MarkdownRichEditor
            v-model="form.prompt_template"
            :placeholder="t('workflow.list.form.promptPlaceholder')"
          />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('workflow.list.form.inputSchema')">
              <a-textarea v-model:value="form.input_schema_text" :rows="6" placeholder="{&#10;  &quot;type&quot;: &quot;object&quot;&#10;}" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('workflow.list.form.outputSchema')">
              <a-textarea v-model:value="form.output_schema_text" :rows="6" placeholder="{&#10;  &quot;type&quot;: &quot;object&quot;&#10;}" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item :label="t('workflow.list.form.configJson')">
          <a-textarea v-model:value="form.config_json_text" :rows="4" placeholder="{&#10;  &quot;timeout&quot;: 30&#10;}" />
        </a-form-item>
        <a-form-item :label="t('common.status')" v-if="editingWorkflow">
          <a-select v-model:value="form.status">
            <a-select-option :value="1">{{ t('common.enabled') }}</a-select-option>
            <a-select-option :value="0">{{ t('common.disabled') }}</a-select-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useUserStore } from '@/stores/user'
import { createWorkflow, deleteWorkflow, getWorkflowList, updateWorkflow } from '@/api/agent'
import MarkdownRichEditor from '@/components/MarkdownRichEditor.vue'
import type { Workflow } from '@/types'
import { translateWorkflowType } from '@/types'
import { useI18n } from 'vue-i18n'

const userStore = useUserStore()
const { t } = useI18n()

const list = ref<Workflow[]>([])
const loading = ref(false)
const submitting = ref(false)
const showModal = ref(false)
const editingWorkflow = ref<Workflow | null>(null)
const keyword = ref('')
const canManage = computed(() => userStore.hasPermission('agent:manage'))

const form = reactive({
  name: '',
  description: '',
  workflow_type: 'builtin',
  prompt_template: '',
  input_schema_text: '',
  output_schema_text: '',
  config_json_text: '',
  status: 1,
})

const columns = computed(() => [
  { title: t('workflow.list.columns.id'), dataIndex: 'id', key: 'id', width: 60 },
  { title: t('workflow.list.columns.name'), dataIndex: 'name', key: 'name', width: 180 },
  { title: t('workflow.list.columns.type'), key: 'workflow_type', width: 100 },
  { title: t('workflow.list.columns.description'), dataIndex: 'description', key: 'description', ellipsis: true },
  { title: t('workflow.list.columns.promptTemplate'), key: 'prompt_template', ellipsis: true },
  { title: t('workflow.list.columns.status'), key: 'status', width: 90 },
  { title: t('workflow.list.columns.action'), key: 'action', width: 140 },
])

const filteredList = computed(() => {
  const value = keyword.value.trim().toLowerCase()
  if (!value) return list.value
  return list.value.filter(item =>
    item.name.toLowerCase().includes(value) ||
    item.description.toLowerCase().includes(value),
  )
})

onMounted(fetchData)

async function fetchData() {
  loading.value = true
  try {
    const res = await getWorkflowList()
    list.value = res.data.data || []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingWorkflow.value = null
  Object.assign(form, {
    name: '',
    description: '',
    workflow_type: 'builtin',
    prompt_template: '',
    input_schema_text: '',
    output_schema_text: '',
    config_json_text: '',
    status: 1,
  })
  showModal.value = true
}

function handleEdit(record: Workflow) {
  editingWorkflow.value = record
  Object.assign(form, {
    name: record.name,
    description: record.description,
    workflow_type: record.workflow_type || 'builtin',
    prompt_template: record.prompt_template || '',
    input_schema_text: formatJSON(record.input_schema),
    output_schema_text: formatJSON(record.output_schema),
    config_json_text: formatJSON(record.config_json),
    status: record.status ?? 1,
  })
  showModal.value = true
}

async function handleSubmit() {
  if (!form.name.trim()) {
    message.warning(t('workflow.list.messages.nameRequired'))
    return
  }

  let payload: Record<string, unknown>
  try {
    payload = {
      name: form.name.trim(),
      description: form.description.trim(),
      workflow_type: form.workflow_type,
      prompt_template: form.prompt_template,
      input_schema: parseJSONText(form.input_schema_text),
      output_schema: parseJSONText(form.output_schema_text),
      config_json: parseJSONText(form.config_json_text),
      status: form.status,
    }
  } catch (error) {
    message.warning(error instanceof Error ? error.message : t('workflow.list.messages.invalidJson'))
    return
  }

  submitting.value = true
  try {
    if (editingWorkflow.value) {
      await updateWorkflow(editingWorkflow.value.id, payload)
      message.success(t('common.updateSuccess'))
    } else {
      await createWorkflow(payload)
      message.success(t('common.createSuccess'))
    }
    showModal.value = false
    fetchData()
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  await deleteWorkflow(id)
  message.success(t('common.deleteSuccess'))
  fetchData()
}

function parseJSONText(text: string) {
  const trimmed = text.trim()
  if (!trimmed) return null
  try {
    return JSON.parse(trimmed)
  } catch {
    throw new Error(t('workflow.list.messages.schemaParseFailed'))
  }
}

function formatJSON(value: unknown) {
  if (value == null) return ''
  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed || trimmed === 'null') return ''
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2)
    } catch {
      return trimmed
    }
  }
  return JSON.stringify(value, null, 2)
}
</script>

<style scoped>
.ellipsis-cell {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
