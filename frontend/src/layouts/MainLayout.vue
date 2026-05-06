<template>
  <a-layout class="app-layout">
    <a-layout-sider v-model:collapsed="collapsed" collapsible theme="dark" :width="220" class="app-sider">
      <div class="logo">
        <span v-if="!collapsed">{{ t('layout.appName') }}</span>
        <span v-else>ATF</span>
      </div>
      <a-menu class="sider-menu" theme="dark" mode="inline" :selectedKeys="selectedKeys" :openKeys="openKeys"
              @click="onMenuClick" @openChange="onOpenChange">
        <a-menu-item key="/dashboard">
          <DashboardOutlined /><span>{{ t('layout.menu.dashboard') }}</span>
        </a-menu-item>
        <a-menu-item key="/projects" v-if="hasPermission('project:list')">
          <ProjectOutlined /><span>{{ t('layout.menu.projects') }}</span>
        </a-menu-item>
        <a-menu-item key="/issues" v-if="hasPermission('issue:list')">
          <BugOutlined /><span>{{ t('layout.menu.issues') }}</span>
        </a-menu-item>
        <a-menu-item key="/zentao-test-cases" v-if="hasPermission('test:list')">
          <CheckSquareOutlined /><span>{{ t('layout.menu.zentaoTestCases') }}</span>
        </a-menu-item>
        <a-menu-item key="/agents" v-if="hasPermission('agent:list')">
          <RobotOutlined /><span>{{ t('layout.menu.agents') }}</span>
        </a-menu-item>
        <a-menu-item key="/workflows" v-if="hasPermission('agent:list')">
          <AppstoreOutlined /><span>{{ t('layout.menu.workflows') }}</span>
        </a-menu-item>
        <a-menu-item key="/knowledge" v-if="hasPermission('knowledge:list')">
          <BookOutlined /><span>{{ t('layout.menu.knowledge') }}</span>
        </a-menu-item>
        <a-sub-menu key="system" v-if="showSystemMenu">
          <template #title>
            <span class="menu-group-title"><SettingOutlined /><span>{{ t('layout.menu.settings') }}</span></span>
          </template>
          <a-menu-item key="/users" v-if="hasPermission('user:list')">
            <span>{{ t('layout.menu.users') }}</span>
          </a-menu-item>
          <a-menu-item key="/settings/zentao" v-if="isAdmin">{{ t('layout.menu.zentao') }}</a-menu-item>
          <a-menu-item key="/settings/gitlab" v-if="isAdmin">{{ t('layout.menu.gitlab') }}</a-menu-item>
          <a-menu-item key="/settings/mail" v-if="isAdmin">{{ t('layout.menu.mail') }}</a-menu-item>
        </a-sub-menu>
      </a-menu>
    </a-layout-sider>

    <a-layout class="app-main">
      <a-layout-header class="header">
        <div class="header-left">
          <MenuFoldOutlined v-if="!collapsed" @click="collapsed = true" class="trigger" />
          <MenuUnfoldOutlined v-else @click="collapsed = false" class="trigger" />
          <a-breadcrumb style="margin-left: 16px">
            <a-breadcrumb-item>{{ currentRouteTitle }}</a-breadcrumb-item>
          </a-breadcrumb>
        </div>
        <div class="header-right">
          <div class="header-locale" :aria-label="t('common.language')">
            <GlobalOutlined class="header-locale-icon" />
            <a-segmented v-model:value="currentLocale" size="small" :options="localeOptions" />
          </div>
          <a-dropdown>
            <span class="user-info">
              <a-avatar size="small">{{ userStore.userInfo?.real_name?.[0] || 'U' }}</a-avatar>
              <span style="margin-left: 8px">{{ userStore.userInfo?.real_name || userStore.userInfo?.username }}</span>
            </span>
            <template #overlay>
              <a-menu>
                <a-menu-item @click="handleLogout">{{ t('layout.logout') }}</a-menu-item>
              </a-menu>
            </template>
          </a-dropdown>
        </div>
      </a-layout-header>

      <a-layout-content class="content-shell">
        <div class="content">
          <router-view :key="route.fullPath" />
        </div>
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useI18n } from 'vue-i18n'
import { setAppLocale } from '@/locales'
import {
  DashboardOutlined, ProjectOutlined, BugOutlined, RobotOutlined,
  MenuFoldOutlined, MenuUnfoldOutlined, SettingOutlined, GlobalOutlined,
  AppstoreOutlined, SyncOutlined,
  CheckSquareOutlined, BookOutlined,
} from '@ant-design/icons-vue'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()
const { t, locale } = useI18n()
const systemMenuPaths = ['/users', '/settings/zentao', '/settings/gitlab', '/settings/mail']

const collapsed = ref(false)
const selectedKeys = computed(() => [route.path])
const openKeys = ref<string[]>(systemMenuPaths.includes(route.path) ? ['system'] : [])
const currentRoute = computed(() => route)
const currentRouteTitle = computed(() => {
  const titleKey = currentRoute.value.meta?.titleKey as string | undefined
  return titleKey ? t(titleKey) : t('layout.menu.dashboard')
})
const currentLocale = computed({
  get: () => locale.value,
  set: (value: string) => setAppLocale(value as 'zh-CN' | 'en-US'),
})
const localeOptions = [
  { label: '中', value: 'zh-CN' },
  { label: 'EN', value: 'en-US' },
]

const isAdmin = computed(() => userStore.roles.includes('admin'))
const showSystemMenu = computed(() => isAdmin.value || hasPermission('user:list'))

function hasPermission(perm: string) {
  return userStore.hasPermission(perm)
}

function onOpenChange(keys: string[]) {
  openKeys.value = keys
}

function onMenuClick({ key }: { key: string }) {
  router.push(key)
  if (systemMenuPaths.includes(key)) {
    openKeys.value = ['system']
  }
}

function handleLogout() {
  userStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.app-layout {
  height: 100vh;
  overflow: hidden;
}
.app-sider {
  height: 100vh;
  overflow: hidden;
}
.logo {
  flex: 0 0 auto;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 18px;
  font-weight: bold;
  background: rgba(255, 255, 255, 0.1);
  margin: 8px;
  border-radius: 6px;
}
.app-sider :deep(.ant-layout-sider-children) {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.sider-menu {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
}
.sider-menu :deep(.ant-menu-submenu.ant-menu-submenu-inline > .ant-menu-submenu-title) {
  padding-left: 24px !important;
}
.sider-menu :deep(.ant-menu-sub .ant-menu-item) {
  padding-left: 48px !important;
}
.menu-group-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.app-main {
  min-width: 0;
  height: 100vh;
  overflow: hidden;
}
.header {
  flex: 0 0 auto;
  background: #fff;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
}
.header-left {
  display: flex;
  align-items: center;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}
.header-locale {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 6px;
  border-radius: 999px;
  border: 1px solid #d0d5dd;
  background: #f8fafc;
}
.header-locale-icon {
  color: #667085;
  margin: 0 2px 0 4px;
}
.header-locale :deep(.ant-segmented) {
  background: transparent;
  border-radius: 999px;
}
.header-locale :deep(.ant-segmented-group) {
  gap: 2px;
}
.header-locale :deep(.ant-segmented-item) {
  border-radius: 999px;
}
.header-locale :deep(.ant-segmented-item-label) {
  min-width: 34px;
  font-size: 12px;
  font-weight: 700;
  line-height: 24px;
  padding: 0 8px;
}
.header-locale :deep(.ant-segmented-thumb) {
  border-radius: 999px;
}
.header-locale :deep(.ant-segmented-item-selected) {
  border-radius: 999px;
  background: linear-gradient(135deg, #1d4ed8, #2563eb);
  color: #fff;
  box-shadow: 0 4px 10px rgba(37, 99, 235, 0.22);
}
.trigger {
  font-size: 18px;
  cursor: pointer;
  padding: 0 8px;
}
.user-info {
  cursor: pointer;
  display: flex;
  align-items: center;
}
.content-shell {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
}
.content {
  margin: 16px;
  padding: 24px;
  background: #fff;
  border-radius: 8px;
  min-height: 280px;
}
</style>
