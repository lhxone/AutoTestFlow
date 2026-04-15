<template>
  <div>
    <a-page-header :title="t('user.list.title')" />

    <a-row :gutter="16" style="margin-bottom: 16px">
      <a-col :span="6">
        <a-input v-model:value="query.keyword" :placeholder="t('user.list.searchPlaceholder')" allowClear @pressEnter="fetchData" />
      </a-col>
      <a-col :span="4">
        <a-select v-model:value="query.status" :placeholder="t('common.status')" allowClear style="width: 100%">
          <a-select-option :value="1">{{ t('common.enabled') }}</a-select-option>
          <a-select-option :value="0">{{ t('common.disabled') }}</a-select-option>
        </a-select>
      </a-col>
      <a-col>
        <a-button type="primary" @click="fetchData">{{ t('common.query') }}</a-button>
        <a-button style="margin-left: 8px" @click="showModal = true">{{ t('user.list.create') }}</a-button>
        <a-button style="margin-left: 8px" @click="openLoginLogModal">{{ t('user.loginLog.open') }}</a-button>
      </a-col>
    </a-row>

    <a-table :dataSource="list" :columns="columns" :loading="loading" :pagination="pagination"
             @change="handleTableChange" rowKey="id" size="middle">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'status'">
          <a-tag :color="record.status === 1 ? 'green' : 'red'">{{ record.status === 1 ? t('common.enabled') : t('common.disabled') }}</a-tag>
        </template>
        <template v-if="column.key === 'roles'">
          <a-tag v-for="r in record.roles" :key="r.id" color="blue">{{ formatRoleName(r.code, r.name) }}</a-tag>
        </template>
        <template v-if="column.key === 'action'">
          <a-button type="link" size="small" @click="handleEdit(record)">{{ t('common.edit') }}</a-button>
          <a-popconfirm :title="t('common.confirmDelete')" @confirm="handleDelete(record.id)">
            <a-button type="link" size="small" danger>{{ t('common.delete') }}</a-button>
          </a-popconfirm>
        </template>
      </template>
    </a-table>

    <a-modal v-model:open="showModal" :title="editingUser ? t('user.list.edit') : t('user.list.create')" @ok="handleSubmit" :confirmLoading="submitting">
      <a-form layout="vertical">
        <a-form-item :label="t('user.list.form.username')" required v-if="!editingUser">
          <a-input v-model:value="form.username" />
        </a-form-item>
        <a-form-item :label="t('user.list.form.password')" required v-if="!editingUser">
          <a-input-password v-model:value="form.password" />
        </a-form-item>
        <a-form-item :label="t('user.list.form.realName')">
          <a-input v-model:value="form.real_name" />
        </a-form-item>
        <a-form-item :label="t('user.list.form.email')">
          <a-input v-model:value="form.email" />
        </a-form-item>
        <a-form-item :label="t('user.list.form.phone')">
          <a-input v-model:value="form.phone" />
        </a-form-item>
        <a-form-item :label="t('user.list.form.roles')" required>
          <a-select v-model:value="form.role_ids" mode="multiple" :placeholder="t('user.list.form.rolesPlaceholder')">
            <a-select-option :value="1">{{ t('user.roles.admin') }}</a-select-option>
            <a-select-option :value="2">{{ t('user.roles.test_lead') }}</a-select-option>
            <a-select-option :value="3">{{ t('user.roles.tester') }}</a-select-option>
            <a-select-option :value="4">{{ t('user.roles.dev_lead') }}</a-select-option>
            <a-select-option :value="5">{{ t('user.roles.viewer') }}</a-select-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="showLoginLogModal" :title="t('user.loginLog.title')" :footer="null" :width="1100">
      <a-row :gutter="16" style="margin-bottom: 16px">
        <a-col :span="6">
          <a-input v-model:value="loginLogQuery.username" :placeholder="t('user.loginLog.username')" allowClear @pressEnter="fetchLoginLogs" />
        </a-col>
        <a-col :span="6">
          <a-select v-model:value="loginLogQuery.action" :placeholder="t('user.loginLog.action')" allowClear style="width: 100%">
            <a-select-option value="login_success">{{ t('user.loginLog.actions.login_success') }}</a-select-option>
            <a-select-option value="login_failed">{{ t('user.loginLog.actions.login_failed') }}</a-select-option>
            <a-select-option value="logout">{{ t('user.loginLog.actions.logout') }}</a-select-option>
          </a-select>
        </a-col>
        <a-col>
          <a-button type="primary" @click="fetchLoginLogs">{{ t('common.query') }}</a-button>
        </a-col>
      </a-row>

      <a-table :dataSource="loginLogs" :columns="loginLogColumns" :loading="loginLogLoading" :pagination="loginLogPagination"
               @change="handleLoginLogTableChange" rowKey="id" size="middle">
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'action'">
            {{ formatLoginLogAction(record.action) }}
          </template>
          <template v-if="column.key === 'result'">
            <a-tag :color="record.action === 'login_failed' ? 'error' : 'success'">
              {{ record.detail?.reason || t('user.loginLog.result.success') }}
            </a-tag>
          </template>
        </template>
      </a-table>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { getUserList, createUser, updateUser, deleteUser, getLoginLogList } from '@/api/user'
import type { LoginLog, User } from '@/types'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const list = ref<User[]>([])
const loading = ref(false)
const loginLogs = ref<LoginLog[]>([])
const loginLogLoading = ref(false)
const showModal = ref(false)
const showLoginLogModal = ref(false)
const submitting = ref(false)
const editingUser = ref<User | null>(null)
const query = reactive({ keyword: '', status: undefined as number | undefined, page: 1, page_size: 20 })
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })
const loginLogQuery = reactive({ username: '', action: undefined as string | undefined })
const loginLogPagination = reactive({ current: 1, pageSize: 10, total: 0 })

const form = reactive({
  username: '', password: '', real_name: '', email: '', phone: '', role_ids: [] as number[],
})

const columns = computed(() => [
  { title: t('common.id'), dataIndex: 'id', key: 'id', width: 60 },
  { title: t('user.list.columns.username'), dataIndex: 'username', key: 'username' },
  { title: t('user.list.columns.realName'), dataIndex: 'real_name', key: 'real_name' },
  { title: t('user.list.columns.email'), dataIndex: 'email', key: 'email' },
  { title: t('user.list.columns.roles'), key: 'roles' },
  { title: t('user.list.columns.status'), key: 'status', width: 80 },
  { title: t('user.list.columns.action'), key: 'action', width: 140 },
])

const loginLogColumns = computed(() => [
  { title: t('common.id'), dataIndex: 'id', key: 'id', width: 80 },
  { title: t('user.loginLog.columns.username'), dataIndex: 'username', key: 'username', width: 120 },
  { title: t('user.loginLog.columns.action'), key: 'action', width: 140 },
  { title: t('user.loginLog.columns.result'), key: 'result', width: 220 },
  { title: t('user.loginLog.columns.ip'), dataIndex: 'ip', key: 'ip', width: 140 },
  { title: t('user.loginLog.columns.userAgent'), dataIndex: 'user_agent', key: 'user_agent', ellipsis: true },
  { title: t('user.loginLog.columns.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 180 },
])

onMounted(fetchData)

async function fetchData() {
  loading.value = true
  try {
    const res = await getUserList({ ...query, page: pagination.current, page_size: pagination.pageSize })
    const data = res.data.data
    list.value = data.list || []
    pagination.total = data.total
  } finally {
    loading.value = false
  }
}

function handleTableChange(pag: any) {
  pagination.current = pag.current
  pagination.pageSize = pag.pageSize
  fetchData()
}

function handleEdit(user: User) {
  editingUser.value = user
  form.real_name = user.real_name
  form.email = user.email
  form.phone = user.phone
  form.role_ids = user.roles?.map(r => r.id) || []
  showModal.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (editingUser.value) {
      await updateUser(editingUser.value.id, {
        real_name: form.real_name, email: form.email, phone: form.phone, role_ids: form.role_ids,
      })
      message.success(t('common.updateSuccess'))
    } else {
      await createUser(form)
      message.success(t('common.createSuccess'))
    }
    showModal.value = false
    editingUser.value = null
    Object.assign(form, { username: '', password: '', real_name: '', email: '', phone: '', role_ids: [] })
    fetchData()
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  await deleteUser(id)
  message.success(t('common.deleteSuccess'))
  fetchData()
}

function formatRoleName(code: string, fallback: string) {
  if (!code) return fallback
  const key = `user.roles.${code}`
  const translated = t(key)
  return translated === key ? fallback : translated
}

function openLoginLogModal() {
  showLoginLogModal.value = true
  fetchLoginLogs()
}

async function fetchLoginLogs() {
  loginLogLoading.value = true
  try {
    const res = await getLoginLogList({
      ...loginLogQuery,
      page: loginLogPagination.current,
      page_size: loginLogPagination.pageSize,
    })
    const data = res.data.data
    loginLogs.value = data.list || []
    loginLogPagination.total = data.total
  } finally {
    loginLogLoading.value = false
  }
}

function handleLoginLogTableChange(pag: any) {
  loginLogPagination.current = pag.current
  loginLogPagination.pageSize = pag.pageSize
  fetchLoginLogs()
}

function formatLoginLogAction(action: string) {
  const key = `user.loginLog.actions.${action}`
  const translated = t(key)
  return translated === key ? action : translated
}
</script>
