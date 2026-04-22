<template>
  <div>
    <a-page-header :title="t('dashboard.title')" :sub-title="t('dashboard.subtitle')" />
    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="6">
        <a-card :loading="loading">
          <a-statistic :title="t('dashboard.stats.projects')" :value="displayValue(stats.projects)" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card :loading="loading">
          <a-statistic :title="t('dashboard.stats.pendingReviews')" :value="displayValue(stats.pendingReviews)" :value-style="{ color: '#faad14' }" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card :loading="loading">
          <a-statistic :title="t('dashboard.stats.interventionNeeded')" :value="displayValue(stats.interventionNeeded)" :value-style="{ color: '#ff4d4f' }" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card :loading="loading">
          <a-statistic
            :title="t('dashboard.stats.passRate')"
            :value="displayValue(stats.passRate, true)"
            :suffix="stats.passRate == null ? '' : '%'"
            :value-style="{ color: '#52c41a' }"
          />
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="16">
      <a-col :span="12">
        <a-card :title="t('dashboard.quickActions.title')" class="dashboard-equal-height">
          <a-space wrap>
            <a-button type="primary" @click="$router.push('/issues')">{{ t('dashboard.quickActions.issues') }}</a-button>
            <a-button @click="$router.push('/reviews')">{{ t('dashboard.quickActions.reviews') }}</a-button>
            <a-button @click="$router.push('/test-tasks')">{{ t('dashboard.quickActions.testTasks') }}</a-button>
          </a-space>
        </a-card>
      </a-col>
      <a-col :span="12">
        <a-card :title="t('dashboard.recentActivities.title')" class="dashboard-equal-height">
          <div v-if="recentActivities.length === 0" style="padding: 12px 0">
            <a-empty :description="t('dashboard.recentActivities.empty')" />
          </div>
          <a-list v-else :data-source="recentActivities" :split="false" class="recent-activity-list">
            <template #renderItem="{ item }">
              <a-list-item>
                <a-list-item-meta>
                  <template #title>
                    <a-space>
                      <a-tag :color="activityColor(item.action)">{{ item.action_label }}</a-tag>
                      <span>{{ item.username }}</span>
                    </a-space>
                  </template>
                  <template #description>
                    <a-space>
                      <span>{{ item.ip }}</span>
                      <span>{{ formatTime(item.created_at) }}</span>
                    </a-space>
                  </template>
                </a-list-item-meta>
              </a-list-item>
            </template>
          </a-list>
        </a-card>
      </a-col>
    </a-row>

    <a-card :title="t('dashboard.issueSync.title')" style="margin-top: 16px" :loading="loading">
      <a-empty v-if="issueSyncProjects.length === 0" :description="t('dashboard.issueSync.empty')" />
      <a-list v-else :data-source="issueSyncProjects" item-layout="horizontal">
        <template #renderItem="{ item }">
          <a-list-item>
            <a-list-item-meta>
              <template #title>
                <a-space>
                  <span>{{ item.project_name }}</span>
                  <a-tag :color="syncStatusColor(item.status)">{{ syncStatusLabel(item.status) }}</a-tag>
                </a-space>
              </template>
              <template #description>
                <div>
                  {{ t('dashboard.issueSync.lastSyncCounts') }}：
                  {{ t('dashboard.issueSync.added') }}：{{ item.added_count }}，
                  {{ t('dashboard.issueSync.updated') }}：{{ item.updated_count }}，
                  {{ t('dashboard.issueSync.deleted') }}：{{ item.deleted_count }}
                </div>
                <div v-if="item.completed_at">{{ t('dashboard.issueSync.completedAt') }}：{{ item.completed_at }}</div>
                <div v-else-if="item.started_at">{{ t('dashboard.issueSync.startedAt') }}：{{ item.started_at }}</div>
                <div v-if="item.error_message" style="color: #ff4d4f">{{ t('dashboard.issueSync.errorMessage') }}：{{ item.error_message }}</div>
              </template>
            </a-list-item-meta>
          </a-list-item>
        </template>
      </a-list>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { getDashboardStats, getRecentActivities } from '@/api/dashboard'
import type { DashboardProjectSyncStatus, DashboardStats } from '@/types'
import { useI18n } from 'vue-i18n'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

const { t } = useI18n()
const loading = ref(false)
const issueSyncProjects = ref<DashboardProjectSyncStatus[]>([])
const recentActivities = ref<RecentActivity[]>([])
const stats = reactive<{
  projects: number | null
  pendingReviews: number | null
  interventionNeeded: number | null
  passRate: number | null
}>({
  projects: null,
  pendingReviews: null,
  interventionNeeded: null,
  passRate: null,
})

interface RecentActivity {
  id: number
  username: string
  action: string
  action_label: string
  ip: string
  created_at: string
}

onMounted(() => {
  fetchStats()
  fetchRecentActivities()
})

async function fetchStats() {
  loading.value = true
  try {
    const res = await getDashboardStats()
    const data = (res.data.data || {}) as DashboardStats
    stats.projects = data.projects
    stats.pendingReviews = data.pending_reviews
    stats.interventionNeeded = data.intervention_needed
    stats.passRate = data.pass_rate == null ? null : Number(data.pass_rate.toFixed(2))
    issueSyncProjects.value = data.issue_sync_projects || []
  } finally {
    loading.value = false
  }
}

async function fetchRecentActivities() {
  try {
    const res = await getRecentActivities()
    recentActivities.value = (res.data.data || []) as RecentActivity[]
  } catch {
    recentActivities.value = []
  }
}

function displayValue(value: number | null, decimal = false) {
  if (value == null) {
    return '--'
  }
  return decimal ? Number(value.toFixed(2)) : value
}

function syncStatusColor(status: string) {
  const map: Record<string, string> = {
    success: 'success',
    failed: 'error',
    running: 'processing',
    unknown: 'default',
  }
  return map[status] || 'default'
}

function syncStatusLabel(status: string) {
  const keyMap: Record<string, string> = {
    success: 'dashboard.issueSync.status.success',
    failed: 'dashboard.issueSync.status.failed',
    running: 'dashboard.issueSync.status.running',
    unknown: 'dashboard.issueSync.status.unknown',
  }
  return t(keyMap[status] || keyMap.unknown)
}

function activityColor(action: string) {
  switch (action) {
    case 'login_success':
      return 'success'
    case 'login_failed':
      return 'error'
    case 'logout':
      return 'default'
    default:
      return 'default'
  }
}

function formatTime(timeStr: string) {
  return dayjs(timeStr).fromNow()
}
</script>

<style scoped>
.dashboard-equal-height {
  height: 100%;
}
.dashboard-equal-height :deep(.ant-card-body) {
  flex: 1;
}
.recent-activity-list {
  max-height: 350px;
  overflow-y: auto;
}
.recent-activity-list :deep(.ant-list-item) {
  padding: 8px 0;
}
.recent-activity-list :deep(.ant-list-item-meta) {
  align-items: center;
}
</style>
