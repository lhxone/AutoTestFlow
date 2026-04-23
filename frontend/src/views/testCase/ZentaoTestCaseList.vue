<template>
  <div>
    <a-page-header :title="$t('zentaoTestCase.list.title')" />

    <a-row :gutter="16" style="margin-bottom: 16px">
      <a-col :span="5">
        <a-input v-model:value="query.keyword" :placeholder="$t('zentaoTestCase.list.search')" allowClear @pressEnter="fetchData" />
      </a-col>
      <a-col :span="4">
        <a-select v-model:value="query.test_status" :placeholder="$t('zentaoTestCase.list.testStatus')" allowClear style="width: 100%">
          <a-select-option value="pending">{{ $t('zentaoTestCase.list.status.pending') }}</a-select-option>
          <a-select-option value="generating">{{ $t('zentaoTestCase.list.status.generating') }}</a-select-option>
          <a-select-option value="generated">{{ $t('zentaoTestCase.list.status.generated') }}</a-select-option>
          <a-select-option value="failed">{{ $t('zentaoTestCase.list.status.failed') }}</a-select-option>
        </a-select>
      </a-col>
      <a-col :span="4">
        <a-select v-model:value="query.project_id" :placeholder="$t('zentaoTestCase.list.project')" allowClear style="width: 100%">
          <a-select-option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</a-select-option>
        </a-select>
      </a-col>
      <a-col :span="4">
        <a-select v-model:value="query.type" :placeholder="$t('zentaoTestCase.list.type')" allowClear style="width: 100%">
          <a-select-option value="feature">{{ $t('zentaoTestCase.list.typeFeature') }}</a-select-option>
          <a-select-option value="performance">{{ $t('zentaoTestCase.list.typePerformance') }}</a-select-option>
          <a-select-option value="config">{{ $t('zentaoTestCase.list.typeConfig') }}</a-select-option>
          <a-select-option value="security">{{ $t('zentaoTestCase.list.typeSecurity') }}</a-select-option>
          <a-select-option value="interface">{{ $t('zentaoTestCase.list.typeInterface') }}</a-select-option>
          <a-select-option value="unit">{{ $t('zentaoTestCase.list.typeUnit') }}</a-select-option>
          <a-select-option value="other">{{ $t('zentaoTestCase.list.typeOther') }}</a-select-option>
        </a-select>
      </a-col>
      <a-col>
        <a-button type="primary" @click="fetchData">{{ $t('common.query') }}</a-button>
        <a-button style="margin-left: 8px" @click="handleSync">{{ $t('zentaoTestCase.list.sync') }}</a-button>
      </a-col>
    </a-row>

    <a-table :dataSource="list" :columns="columns" :loading="loading" :pagination="pagination"
             @change="handleTableChange" rowKey="id" size="middle" :scroll="{ x: 1200 }">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'test_status'">
          <a-tag :color="testStatusMap[record.test_status]?.color || 'default'">
            {{ testStatusMap[record.test_status]?.label || record.test_status }}
          </a-tag>
        </template>
        <template v-if="column.key === 'type'">
          <a-tag>{{ formatType(record.type) }}</a-tag>
        </template>
        <template v-if="column.key === 'priority'">
          <a-tag :color="priorityColor(record.priority)">P{{ record.priority }}</a-tag>
        </template>
        <template v-if="column.key === 'action'">
          <a-button type="link" size="small" @click="handleViewDetail(record)">
            {{ $t('zentaoTestCase.list.action.viewDetail') }}
          </a-button>
          <a-button type="link" size="small" @click="handleGenScript(record)"
                    :disabled="!canGenerateScript(record)">
            {{ $t('zentaoTestCase.list.action.generateScript') }}
          </a-button>
        </template>
      </template>
    </a-table>

    <!-- Sync modal -->
    <a-modal v-model:open="syncModalOpen" :title="$t('zentaoTestCase.list.syncModal.title')" @ok="doSync" :confirmLoading="syncLoading">
      <a-form layout="vertical">
        <a-form-item :label="$t('zentaoTestCase.list.syncModal.selectProject')" required>
          <a-select v-model:value="syncForm.project_id" :placeholder="$t('zentaoTestCase.list.syncModal.selectProject')" :loading="projectListLoading" show-search :filter-option="filterOption">
            <a-select-option v-for="p in projectList" :key="p.id" :value="p.id">
              {{ p.name }}<span v-if="p.zentao_project_name" style="color: #999; margin-left: 8px">（{{ p.zentao_project_name }}）</span>
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item>
          <a-checkbox v-model:checked="syncForm.full_sync">{{ $t('zentaoTestCase.list.syncModal.fullSync') }}</a-checkbox>
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- Detail modal -->
    <a-modal v-model:open="detailModalOpen" :title="$t('zentaoTestCase.list.detailModal.title')" width="800px" :footer="null">
      <template v-if="detailCase">
        <a-descriptions :column="2" bordered size="small">
          <a-descriptions-item :label="$t('zentaoTestCase.list.detailModal.id')">{{ detailCase.zentao_id }}</a-descriptions-item>
          <a-descriptions-item :label="$t('zentaoTestCase.list.detailModal.priority')">P{{ detailCase.priority }}</a-descriptions-item>
          <a-descriptions-item :label="$t('zentaoTestCase.list.detailModal.title')" :span="2">{{ detailCase.title }}</a-descriptions-item>
          <a-descriptions-item :label="$t('zentaoTestCase.list.detailModal.type')">{{ formatType(detailCase.type) }}</a-descriptions-item>
          <a-descriptions-item :label="$t('zentaoTestCase.list.detailModal.status')">{{ detailCase.status }}</a-descriptions-item>
          <a-descriptions-item :label="$t('zentaoTestCase.list.detailModal.precondition')" :span="2">{{ detailCase.precondition || '-' }}</a-descriptions-item>
          <a-descriptions-item :label="$t('zentaoTestCase.list.detailModal.steps')" :span="2">
            <div style="white-space: pre-wrap">{{ detailCase.steps || '-' }}</div>
          </a-descriptions-item>
          <a-descriptions-item :label="$t('zentaoTestCase.list.detailModal.expected')" :span="2">
            <div style="white-space: pre-wrap">{{ detailCase.expected || '-' }}</div>
          </a-descriptions-item>
          <a-descriptions-item :label="$t('zentaoTestCase.list.detailModal.testStatus')">{{ formatTestStatus(detailCase.test_status) }}</a-descriptions-item>
          <a-descriptions-item :label="$t('zentaoTestCase.list.detailModal.openedBy')">{{ detailCase.opened_by || '-' }}</a-descriptions-item>
        </a-descriptions>
      </template>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { getZentaoTestCaseList, syncZentaoTestCases, generateTestScript } from '@/api/zentao'
import { getProjectList } from '@/api/project'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const list = ref<any[]>([])
const projects = ref<{ id: number; name: string }[]>([])
const projectList = ref<{ id: number; name: string; zentao_project_name?: string }[]>([])
const projectListLoading = ref(false)
const loading = ref(false)
const syncModalOpen = ref(false)
const syncLoading = ref(false)
const detailModalOpen = ref(false)
const detailCase = ref<any>(null)

const syncForm = reactive({
  project_id: undefined as number | undefined,
  full_sync: false,
})

const query = reactive({
  keyword: '',
  test_status: '',
  project_id: undefined as number | undefined,
  type: '',
  page: 1,
  page_size: 20,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showSizeChanger: true,
  showTotal: (t: number) => `共 ${t} 条`,
})

const testStatusMap: Record<string, { label: string; color: string }> = {
  pending: { label: '待生成', color: 'default' },
  generating: { label: '生成中', color: 'processing' },
  generated: { label: '已生成', color: 'success' },
  failed: { label: '失败', color: 'error' },
}

const typeMap: Record<string, string> = {
  feature: '功能测试',
  performance: '性能测试',
  config: '配置相关',
  install: '安装部署',
  security: '安全相关',
  interface: '接口测试',
  unit: '单元测试',
  other: '其他',
}

const columns = [
  { title: 'ID', dataIndex: 'zentao_id', key: 'zentao_id', width: 80 },
  { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
  { title: '类型', dataIndex: 'type', key: 'type', width: 100 },
  { title: '优先级', dataIndex: 'priority', key: 'priority', width: 80 },
  { title: '状态', dataIndex: 'test_status', key: 'test_status', width: 100 },
  { title: '创建人', dataIndex: 'opened_by', key: 'opened_by', width: 100 },
  { key: 'action', title: '操作', width: 180, fixed: 'right' },
]

function formatType(type: string) {
  return typeMap[type] || type
}

function priorityColor(priority: number) {
  if (priority === 1) return 'red'
  if (priority === 2) return 'orange'
  return ''
}

function canGenerateScript(record: any) {
  return record.test_status === 'pending' || record.test_status === 'failed'
}

function formatTestStatus(status: string) {
  return testStatusMap[status]?.label || status
}

onMounted(() => {
  fetchProjects()
  fetchData()
})

async function fetchProjects() {
  try {
    const res = await getProjectList({ page: 1, page_size: 100 })
    projects.value = res.data.data?.list?.map((project: any) => ({ id: project.id, name: project.name })) || []
  } catch {
  }
}

async function fetchData() {
  loading.value = true
  try {
    const res = await getZentaoTestCaseList({ ...query, page: pagination.current, page_size: pagination.pageSize })
    const data = res.data.data
    list.value = data.list || []
    pagination.total = data.total || 0
  } catch (e: any) {
    message.error(e.response?.data?.message || t('common.requestFailed'))
  } finally {
    loading.value = false
  }
}

function handleTableChange(pag: any) {
  pagination.current = pag.current
  pagination.pageSize = pag.pageSize
  fetchData()
}

function handleSync() {
  syncModalOpen.value = true
  fetchProjectList()
}

function filterOption(input: string, option: any) {
  const label = option.children?.[0]?.children?.[0]?.children || ''
  return String(label).toLowerCase().includes(input.toLowerCase())
}

async function fetchProjectList() {
  if (projectList.value.length > 0) return
  projectListLoading.value = true
  try {
    const res = await getProjectList({ page: 1, page_size: 100 })
    projectList.value = res.data.data?.list || []
  } finally {
    projectListLoading.value = false
  }
}

async function doSync() {
  if (!syncForm.project_id) {
    message.error(t('zentaoTestCase.list.syncModal.selectProject'))
    return
  }
  syncLoading.value = true
  try {
    await syncZentaoTestCases({
      project_id: syncForm.project_id,
      full_sync: syncForm.full_sync,
    })
    message.success(t('zentaoTestCase.list.syncModal.fullSync') ? '全量同步完成' : '增量同步完成')
    syncModalOpen.value = false
    fetchData()
  } catch (e: any) {
    message.error(e.response?.data?.message || t('common.requestFailed'))
  } finally {
    syncLoading.value = false
  }
}

function handleViewDetail(record: any) {
  detailCase.value = record
  detailModalOpen.value = true
}

async function handleGenScript(record: any) {
  try {
    await generateTestScript({ test_case_id: record.id })
    message.success('脚本生成任务已创建')
    fetchData()
  } catch (e: any) {
    message.error(e.response?.data?.message || t('common.requestFailed'))
  }
}
</script>
