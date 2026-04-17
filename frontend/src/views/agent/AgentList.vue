<template>
  <div>
    <a-page-header :title="t('agent.list.title')" />

    <a-row :gutter="16" style="margin-bottom: 16px">
      <a-col :span="6">
        <a-input v-model:value="query.keyword" :placeholder="t('agent.list.searchPlaceholder')" allowClear @pressEnter="fetchData" />
      </a-col>
      <a-col>
        <a-button type="primary" @click="fetchData">{{ t('common.query') }}</a-button>
        <a-button style="margin-left: 8px" @click="openCreate">{{ t('agent.list.create') }}</a-button>
      </a-col>
    </a-row>

    <a-table :dataSource="list" :columns="columns" :loading="loading" :pagination="pagination"
             @change="handleTableChange" rowKey="id" size="middle">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'model'">
          {{ getProviderLabel(record.model_provider) }} / {{ record.model_name }}
        </template>
        <template v-if="column.key === 'is_default'">
          <a-tag v-if="record.is_default" color="gold">{{ t('agent.list.columns.defaultAgent') }}</a-tag>
          <span v-else>-</span>
        </template>
        <template v-if="column.key === 'workflows'">
          <a-tag v-for="s in record.workflows" :key="s.id" color="geekblue">{{ s.name }}</a-tag>
          <span v-if="!record.workflows?.length">-</span>
        </template>
        <template v-if="column.key === 'mcp_servers'">
          <a-tag v-for="m in record.mcp_servers" :key="m.id" color="purple">{{ m.name }}</a-tag>
          <span v-if="!record.mcp_servers?.length">-</span>
        </template>
        <template v-if="column.key === 'status'">
          <a-tag :color="record.status === 1 ? 'green' : 'red'">{{ record.status === 1 ? t('common.enabled') : t('common.disabled') }}</a-tag>
        </template>
        <template v-if="column.key === 'action'">
          <a-button type="link" size="small" @click="handleEdit(record)">{{ t('common.edit') }}</a-button>
          <a-popconfirm :title="t('common.confirmDelete')" @confirm="handleDelete(record.id)">
            <a-button type="link" size="small" danger>{{ t('common.delete') }}</a-button>
          </a-popconfirm>
        </template>
      </template>
    </a-table>

    <a-modal v-model:open="showModal" :title="editing ? t('agent.list.edit') : t('agent.list.create')"
             :confirmLoading="submitting" width="720px">
      <a-form layout="vertical">
        <!-- AI Model Configuration -->
        <a-divider orientation="left" style="margin-top: 0">{{ t('agent.list.section.aiModel') }}</a-divider>
        <a-form-item :label="t('agent.list.form.name')" required>
          <a-input v-model:value="form.name" />
        </a-form-item>
        <a-form-item :label="t('agent.list.form.description')">
          <a-input v-model:value="form.description" />
        </a-form-item>
        <a-form-item :label="t('agent.list.form.defaultAgent')">
          <a-switch v-model:checked="form.is_default" />
          <div class="ant-form-item-extra">{{ t('agent.list.form.defaultAgentHint') }}</div>
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item :label="t('agent.list.form.modelProvider')" required>
              <a-select
                v-model:value="form.model_provider"
                :options="providerOptions"
                :placeholder="t('agent.list.form.selectProvider')"
                @change="handleProviderChange"
              />
            </a-form-item>
          </a-col>
          <a-col :span="16">
            <a-form-item :label="t('agent.list.form.modelName')" required>
              <a-select
                v-if="!isCustomProvider"
                v-model:value="form.model_name"
                :options="modelOptions"
                :placeholder="t('agent.list.form.selectModel')"
                show-search
                :filter-option="filterOption"
              />
              <a-input v-else v-model:value="form.model_name" placeholder="claude-sonnet-4-20250514" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('agent.list.form.apiKeyRef')">
              <a-input-password v-model:value="form.api_key_ref" :placeholder="t('agent.list.form.apiKeyRefPlaceholder')" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('agent.list.form.baseUrl')">
              <a-input v-model:value="form.base_url" :placeholder="currentBaseUrlPlaceholder" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('agent.list.form.maxTokens')">
              <a-input-number v-model:value="form.max_tokens" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('agent.list.form.temperature')">
              <a-input-number v-model:value="form.temperature" :min="0" :max="2" :step="0.1" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item :label="t('agent.list.form.workflows')">
          <a-select
            v-model:value="form.workflow_ids"
            mode="multiple"
            :options="workflowOptions"
            :loading="loadingWorkflows"
            :placeholder="t('agent.list.form.workflowsPlaceholder')"
            option-filter-prop="label"
            show-search
            allow-clear
          />
        </a-form-item>
        <a-form-item label="MCP Servers">
          <a-space style="width: 100%">
            <a-select
              v-model:value="form.mcp_server_ids"
              mode="multiple"
              :options="mcpServerOptions"
              :loading="loadingMCPServers"
              placeholder="选择要绑定给 Agent 的 MCP Server"
              option-filter-prop="label"
              show-search
              allow-clear
              style="min-width: 420px"
            />
            <a-button @click="openMCPModal">管理 MCP Server</a-button>
          </a-space>
        </a-form-item>

        <!-- Runtime Workspace Configuration -->
        <a-divider orientation="left">{{ t('agent.list.section.cliRuntime') }}</a-divider>
        <a-alert type="info" show-icon style="margin-bottom: 12px"
          :message="t('agent.list.cliRuntime.hint')" />

        <a-form-item :label="t('agent.list.cliRuntime.workspaceRoot')">
          <a-input v-model:value="cliForm.workspace_root" :placeholder="t('agent.list.cliRuntime.workspaceRootPlaceholder')" />
        </a-form-item>

        <a-alert
          v-if="testResult"
          :type="testResult.success ? 'success' : 'error'"
          show-icon
          style="margin-top: 8px"
          :message="testResult.message"
          :description="formatTestResult(testResult)"
        />
      </a-form>
      <template #footer>
        <a-button @click="showModal = false">{{ t('common.cancel') }}</a-button>
        <a-button :loading="testingConnection" @click="handleTestConnection">{{ t('agent.list.form.testConnection') }}</a-button>
        <a-button type="primary" :loading="submitting" @click="handleSubmit">{{ t('common.save') }}</a-button>
      </template>
    </a-modal>

    <a-modal v-model:open="showMCPModal" title="MCP Server 管理" width="920px" :footer="null" destroy-on-close>
      <a-row :gutter="16">
        <a-col :span="14">
          <a-table :dataSource="mcpServerList" :loading="loadingMCPServers" rowKey="id" size="small" :pagination="false" :scroll="{ y: 360 }">
            <a-table-column title="名称" dataIndex="name" key="name" />
            <a-table-column title="类型" dataIndex="server_type" key="server_type" width="120" />
            <a-table-column title="状态" key="status" width="90">
              <template #default="{ record }">
                <a-tag :color="record.status === 1 ? 'green' : 'red'">{{ record.status === 1 ? '启用' : '禁用' }}</a-tag>
              </template>
            </a-table-column>
            <a-table-column title="操作" key="action" width="130">
              <template #default="{ record }">
                <a-space size="small">
                  <a-button type="link" size="small" @click="editMCPServer(record)">编辑</a-button>
                  <a-popconfirm title="确认删除该 MCP Server?" @confirm="removeMCPServer(record.id)">
                    <a-button type="link" size="small" danger>删除</a-button>
                  </a-popconfirm>
                </a-space>
              </template>
            </a-table-column>
          </a-table>
        </a-col>
        <a-col :span="10">
          <a-form layout="vertical">
            <a-form-item label="名称" required>
              <a-input v-model:value="mcpForm.name" placeholder="例如: chrome-devtools-mcp" />
            </a-form-item>
            <a-form-item label="描述">
              <a-input v-model:value="mcpForm.description" />
            </a-form-item>
            <a-form-item label="类型" required>
              <a-select v-model:value="mcpForm.server_type">
                <a-select-option value="stdio">stdio</a-select-option>
                <a-select-option value="sse">sse</a-select-option>
                <a-select-option value="streamable_http">streamable_http</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="Command (stdio 用)">
              <a-input v-model:value="mcpForm.command" placeholder="npx" />
            </a-form-item>
            <a-form-item label="Args JSON">
              <a-textarea v-model:value="mcpForm.args_json" :rows="2" placeholder='["-y", "@modelcontextprotocol/server-filesystem", "."]' />
            </a-form-item>
            <a-form-item label="URL (sse/http 用)">
              <a-input v-model:value="mcpForm.url" placeholder="http://localhost:9222/mcp" />
            </a-form-item>
            <a-form-item label="Env JSON">
              <a-textarea v-model:value="mcpForm.env_vars_json" :rows="2" placeholder='{"HTTP_PROXY":"http://127.0.0.1:7890"}' />
            </a-form-item>
            <a-form-item label="状态">
              <a-switch v-model:checked="mcpForm.enabled" />
            </a-form-item>
            <a-space>
              <a-button @click="resetMCPForm">重置</a-button>
              <a-button type="primary" :loading="savingMCPServer" @click="saveMCPServer">
                {{ mcpForm.id ? '更新' : '创建' }}
              </a-button>
            </a-space>
          </a-form>
        </a-col>
      </a-row>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import {
  getAgentList,
  createAgent,
  updateAgent,
  deleteAgent,
  testAgentConnection,
  getWorkflowList,
  getMCPServerList,
  createMCPServer,
  updateMCPServer,
  deleteMCPServer,
} from '@/api/agent'
import type { Agent, MCPServer, Workflow } from '@/types'
import { useI18n } from 'vue-i18n'

type ProviderKey = 'claude' | 'openai' | 'zhipu' | 'custom'

type ProviderPreset = {
  label: string
  baseUrl: string
  models: string[]
}

const MODEL_PRESETS: Record<Exclude<ProviderKey, 'custom'>, ProviderPreset> = {
  claude: {
    label: 'Anthropic',
    baseUrl: 'https://api.anthropic.com',
    models: [
      'claude-opus-4-6',
      'claude-sonnet-4-6',
      'claude-haiku-4-5',
    ],
  },
  openai: {
    label: 'OpenAI',
    baseUrl: 'https://api.openai.com',
    models: [
      'gpt-5.4',
      'gpt-5.4-mini',
      'gpt-5.4-nano',
    ],
  },
  zhipu: {
    label: '智谱 AI',
    baseUrl: 'https://open.bigmodel.cn/api/paas/v4',
    models: [
      'glm-5.1',
      'glm-5',
      'glm-5-turbo',
      'glm-4.7',
      'glm-4.6',
      'glm-4.5-air',
    ],
  },
}

const { t } = useI18n()
const list = ref<Agent[]>([])
const workflowOptions = ref<{ label: string; value: number }[]>([])
const mcpServerOptions = ref<{ label: string; value: number }[]>([])
const mcpServerList = ref<MCPServer[]>([])
const loading = ref(false)
const loadingWorkflows = ref(false)
const loadingMCPServers = ref(false)
const showModal = ref(false)
const showMCPModal = ref(false)
const submitting = ref(false)
const testingConnection = ref(false)
const savingMCPServer = ref(false)
const editing = ref<Agent | null>(null)
const editingMCPServer = ref<MCPServer | null>(null)
const testResult = ref<{ success: boolean; message: string; provider?: string; model?: string; base_url?: string; latency_ms?: number; sample_output?: string } | null>(null)
const query = reactive({ keyword: '' })
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })

const form = reactive({
  name: '', description: '', model_provider: 'claude' as ProviderKey, model_name: '',
  api_key_ref: '', base_url: '', max_tokens: 4096, temperature: 0.3, workflow_ids: [] as number[], mcp_server_ids: [] as number[], is_default: false,
})

const cliForm = reactive({
  workspace_root: '',
})

const mcpForm = reactive({
  id: 0,
  name: '',
  description: '',
  server_type: 'stdio',
  command: '',
  args_json: '',
  url: '',
  env_vars_json: '',
  enabled: true,
})

const providerOptions = computed(() => [
  { label: MODEL_PRESETS.claude.label, value: 'claude' },
  { label: MODEL_PRESETS.openai.label, value: 'openai' },
  { label: MODEL_PRESETS.zhipu.label, value: 'zhipu' },
  { label: t('agent.list.form.custom'), value: 'custom' },
])

const isCustomProvider = computed(() => form.model_provider === 'custom')

const currentPreset = computed(() => {
  if (form.model_provider === 'custom') {
    return null
  }
  return MODEL_PRESETS[form.model_provider]
})

const modelOptions = computed(() => {
  const preset = currentPreset.value
  if (!preset) {
    return []
  }

  const values = [...preset.models]
  if (form.model_name && !values.includes(form.model_name)) {
    values.unshift(form.model_name)
  }

  return values.map((value) => ({ label: value, value }))
})

const currentBaseUrlPlaceholder = computed(() => currentPreset.value?.baseUrl || 'https://api.example.com')

const columns = computed(() => [
  { title: t('agent.list.columns.id'), dataIndex: 'id', key: 'id', width: 60 },
  { title: t('agent.list.columns.name'), dataIndex: 'name', key: 'name' },
  { title: t('agent.list.columns.default'), key: 'is_default', width: 120 },
  { title: t('agent.list.columns.model'), key: 'model' },
  { title: t('agent.list.columns.workflows'), key: 'workflows' },
  { title: t('agent.list.columns.mcpServers'), key: 'mcp_servers' },
  { title: t('agent.list.columns.status'), key: 'status', width: 80 },
  { title: t('agent.list.columns.action'), key: 'action', width: 140 },
])

onMounted(async () => {
  await Promise.all([fetchData(), fetchWorkflows(), fetchMCPServers()])
})

async function fetchData() {
  loading.value = true
  try {
    const res = await getAgentList({ ...query, page: pagination.current, page_size: pagination.pageSize })
    const data = res.data.data
    list.value = data.list || []
    pagination.total = data.total
  } finally {
    loading.value = false
  }
}

async function fetchWorkflows() {
  loadingWorkflows.value = true
  try {
    const res = await getWorkflowList()
    const workflows = (res.data.data || []) as Workflow[]
    workflowOptions.value = workflows.map((workflow) => ({
      label: workflow.name,
      value: Number(workflow.id),
    }))
  } finally {
    loadingWorkflows.value = false
  }
}

async function fetchMCPServers() {
  loadingMCPServers.value = true
  try {
    const res = await getMCPServerList()
    const servers = (res.data.data || []) as MCPServer[]
    mcpServerList.value = servers
    mcpServerOptions.value = servers.map((server) => ({
      label: `${server.name} (${server.server_type})`,
      value: Number(server.id),
    }))
  } finally {
    loadingMCPServers.value = false
  }
}

function getProviderLabel(provider: string) {
  if (provider in MODEL_PRESETS) {
    return MODEL_PRESETS[provider as keyof typeof MODEL_PRESETS].label
  }
  return provider || t('agent.list.form.custom')
}

function applyProviderPreset(provider: Exclude<ProviderKey, 'custom'>) {
  const preset = MODEL_PRESETS[provider]
  form.base_url = preset.baseUrl
  if (!preset.models.includes(form.model_name)) {
    form.model_name = preset.models[0]
  }
}

function resetForm() {
  Object.assign(form, {
    name: '',
    description: '',
    model_provider: 'claude' as ProviderKey,
    model_name: '',
    api_key_ref: '',
    base_url: '',
    max_tokens: 4096,
    temperature: 0.3,
    workflow_ids: [],
    mcp_server_ids: [],
    is_default: false,
  })
  resetCliForm()
  testResult.value = null
  applyProviderPreset('claude')
}

function resetCliForm() {
  Object.assign(cliForm, {
    workspace_root: '',
  })
}

function loadCliFormFromConfigJSON(configJSON: any) {
  resetCliForm()
  if (!configJSON || typeof configJSON !== 'object') return
  const cli = configJSON.cli_runtime
  if (!cli || typeof cli !== 'object') return
  cliForm.workspace_root = cli.workspace_root || ''
}

function buildConfigJSON(): any {
  const workspaceRoot = cliForm.workspace_root.trim()
  if (!workspaceRoot) return null
  return { cli_runtime: { workspace_root: workspaceRoot } }
}

function handleProviderChange(value: ProviderKey) {
  form.model_provider = value
  if (value === 'custom') {
    form.model_name = ''
    form.base_url = ''
    return
  }
  applyProviderPreset(value)
}

function filterOption(input: string, option: { label?: string; value?: string }) {
  const label = String(option.label ?? option.value ?? '').toLowerCase()
  return label.includes(input.toLowerCase())
}

function handleTableChange(pag: any) {
  pagination.current = pag.current
  fetchData()
}

function openCreate() {
  editing.value = null
  resetForm()
  showModal.value = true
}

function handleEdit(record: Agent) {
  editing.value = record
  Object.assign(form, {
    name: record.name, description: record.description,
    model_provider: record.model_provider, model_name: record.model_name,
    api_key_ref: record.api_key_ref, base_url: record.base_url,
    max_tokens: record.max_tokens, temperature: record.temperature,
    workflow_ids: (record.workflows || []).map((workflow) => Number(workflow.id)),
    mcp_server_ids: (record.mcp_servers || []).map((server) => Number(server.id)),
    is_default: !!record.is_default,
  })
  loadCliFormFromConfigJSON(record.config_json)
  testResult.value = null
  showModal.value = true
}

function buildSubmitPayload() {
  const payload: any = {
    name: form.name,
    description: form.description,
    model_provider: form.model_provider,
    model_name: form.model_name,
    api_key_ref: form.api_key_ref,
    base_url: form.base_url,
    max_tokens: form.max_tokens,
    temperature: form.temperature,
    is_default: form.is_default,
    workflow_ids: form.workflow_ids,
    mcp_server_ids: form.mcp_server_ids,
  }
  const configJSON = buildConfigJSON()
  if (configJSON) {
    payload.config_json = configJSON
  } else {
    payload.config_json = {}
  }
  return payload
}

function formatTestResult(result: { provider?: string; model?: string; base_url?: string; latency_ms?: number; sample_output?: string }) {
  const parts = [
    result.provider ? `${t('agent.list.form.modelProvider')}: ${getProviderLabel(result.provider)}` : '',
    result.model ? `${t('agent.list.form.modelName')}: ${result.model}` : '',
    result.base_url ? `${t('agent.list.form.baseUrl')}: ${result.base_url}` : '',
    typeof result.latency_ms === 'number' ? `${t('agent.list.form.latency')}: ${result.latency_ms} ms` : '',
    result.sample_output ? `${t('agent.list.form.sampleOutput')}: ${result.sample_output}` : '',
  ]
  return parts.filter(Boolean).join(' | ')
}

async function handleTestConnection() {
  testingConnection.value = true
  testResult.value = null
  try {
    const res = await testAgentConnection({
      model_provider: form.model_provider,
      model_name: form.model_name,
      api_key_ref: form.api_key_ref,
      base_url: form.base_url,
      max_tokens: form.max_tokens,
      temperature: form.temperature,
    })
    const result = res.data.data
    testResult.value = result
    if (result.success) {
      message.success(t('agent.list.form.testSuccess'))
    } else {
      message.warning(result.message || t('agent.list.form.testFailed'))
    }
  } finally {
    testingConnection.value = false
  }
}

async function handleSubmit() {
  submitting.value = true
  try {
    const payload = buildSubmitPayload()
    console.log('[AgentList] submit payload:', JSON.stringify(payload))
    if (editing.value) {
      await updateAgent(editing.value.id, payload)
      message.success(t('common.updateSuccess'))
    } else {
      await createAgent(payload)
      message.success(t('common.createSuccess'))
    }
    showModal.value = false
    await fetchData()
  } catch (e: any) {
    console.error('[AgentList] submit error:', e)
    // 响应拦截器已经弹过一次错误提示，这里只在拦截器没处理的情况下提示
    if (e?.response) {
      message.error(e.response.data?.message || t('common.operationFailed'))
    }
  } finally {
    submitting.value = false
  }
}

function openMCPModal() {
  showMCPModal.value = true
  resetMCPForm()
  fetchMCPServers()
}

function resetMCPForm() {
  editingMCPServer.value = null
  Object.assign(mcpForm, {
    id: 0,
    name: '',
    description: '',
    server_type: 'stdio',
    command: '',
    args_json: '',
    url: '',
    env_vars_json: '',
    enabled: true,
  })
}

function editMCPServer(server: MCPServer) {
  editingMCPServer.value = server
  Object.assign(mcpForm, {
    id: server.id,
    name: server.name || '',
    description: server.description || '',
    server_type: server.server_type || 'stdio',
    command: server.command || '',
    args_json: server.args ? JSON.stringify(server.args) : '',
    url: server.url || '',
    env_vars_json: server.env_vars ? JSON.stringify(server.env_vars) : '',
    enabled: server.status !== 0,
  })
}

async function saveMCPServer() {
  if (!mcpForm.name.trim()) {
    message.warning('请填写 MCP Server 名称')
    return
  }
  savingMCPServer.value = true
  try {
    const payload: any = {
      name: mcpForm.name.trim(),
      description: mcpForm.description.trim(),
      server_type: mcpForm.server_type,
      command: mcpForm.command.trim(),
      url: mcpForm.url.trim(),
      status: mcpForm.enabled ? 1 : 0,
    }
    if (mcpForm.args_json.trim()) {
      payload.args = JSON.parse(mcpForm.args_json.trim())
    }
    if (mcpForm.env_vars_json.trim()) {
      payload.env_vars = JSON.parse(mcpForm.env_vars_json.trim())
    }

    if (mcpForm.id > 0) {
      await updateMCPServer(mcpForm.id, payload)
      message.success('MCP Server 已更新')
    } else {
      await createMCPServer(payload)
      message.success('MCP Server 已创建')
    }

    await fetchMCPServers()
    resetMCPForm()
  } catch (e: any) {
    message.error(e?.response?.data?.message || e?.message || '保存 MCP Server 失败')
  } finally {
    savingMCPServer.value = false
  }
}

async function removeMCPServer(id: number) {
  try {
    await deleteMCPServer(id)
    message.success('MCP Server 已删除')
    await fetchMCPServers()
    form.mcp_server_ids = form.mcp_server_ids.filter((item) => item !== id)
    if (mcpForm.id === id) {
      resetMCPForm()
    }
  } catch (e: any) {
    message.error(e?.response?.data?.message || e?.message || '删除 MCP Server 失败')
  }
}

async function handleDelete(id: number) {
  await deleteAgent(id)
  message.success(t('common.deleteSuccess'))
  fetchData()
}
</script>

<style scoped>
.text-muted {
  color: #999;
  font-size: 12px;
}
</style>
