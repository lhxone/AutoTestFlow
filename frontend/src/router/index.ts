import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useUserStore } from '@/stores/user'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/login/LoginPage.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/dashboard/DashboardPage.vue'),
        meta: { titleKey: 'layout.menu.dashboard' },
      },
      {
        path: 'users',
        name: 'UserList',
        component: () => import('@/views/user/UserList.vue'),
        meta: { titleKey: 'layout.menu.users', permission: 'user:list' },
      },
      {
        path: 'projects',
        name: 'ProjectList',
        component: () => import('@/views/project/ProjectList.vue'),
        meta: { titleKey: 'layout.menu.projects', permission: 'project:list' },
      },
      {
        path: 'issues',
        name: 'IssueList',
        component: () => import('@/views/issue/IssueList.vue'),
        meta: { titleKey: 'layout.menu.issues', permission: 'issue:list' },
      },
      {
        path: 'agents',
        name: 'AgentList',
        component: () => import('@/views/agent/AgentList.vue'),
        meta: { titleKey: 'layout.menu.agents', permission: 'agent:list' },
      },
      {
        path: 'workflows',
        name: 'WorkflowList',
        component: () => import('@/views/agent/SkillList.vue'),
        meta: { titleKey: 'layout.menu.workflows', permission: 'agent:list' },
      },
      {
        path: 'knowledge/:projectId?',
        name: 'Knowledge',
        component: () => import('@/views/knowledge/index.vue'),
        meta: { titleKey: 'layout.menu.knowledge', permission: 'knowledge:list' },
      },
      {
        path: 'reviews',
        name: 'ReviewList',
        component: () => import('@/views/review/ReviewList.vue'),
        meta: { titleKey: 'review.list.title', permission: 'review:list' },
      },
      {
        path: 'reviews/:id',
        name: 'ReviewDetail',
        component: () => import('@/views/review/ReviewDetail.vue'),
        meta: { titleKey: 'review.detail.title', permission: 'review:list' },
      },
      {
        path: 'test-tasks',
        name: 'TestTaskList',
        component: () => import('@/views/testTask/TestTaskList.vue'),
        meta: { titleKey: 'testTask.list.title', permission: 'test:list' },
      },
      {
        path: 'test-cases/tasks/:id/edit',
        name: 'TestCaseEdit',
        component: () => import('@/views/testTask/TestCaseEditPage.vue'),
        meta: { titleKey: 'testCase.detail.title', permissionAny: ['test:list', 'review:list'] },
      },
      {
        path: 'test-tasks/:id/run',
        name: 'TaskRunDetail',
        component: () => import('@/views/testTask/TaskRunDetailPage.vue'),
        meta: { titleKey: 'taskRun.drawerTitle', permission: 'test:list' },
      },
      {
        path: 'zentao-test-cases',
        name: 'ZentaoTestCaseList',
        component: () => import('@/views/testCase/ZentaoTestCaseList.vue'),
        meta: { titleKey: 'layout.menu.zentaoTestCases', permission: 'test:list' },
      },
      {
        path: 'executions',
        name: 'ExecutionList',
        component: () => import('@/views/testTask/ExecutionList.vue'),
        meta: { titleKey: 'execution.list.title', permission: 'test:list' },
      },
      {
        path: 'settings/zentao',
        name: 'ZentaoSettings',
        component: () => import('@/views/settings/ZentaoSettings.vue'),
        meta: { titleKey: 'layout.menu.zentao', role: 'admin' },
      },
      {
        path: 'settings/gitlab',
        name: 'GitLabSettings',
        component: () => import('@/views/settings/GitLabSettings.vue'),
        meta: { titleKey: 'layout.menu.gitlab', role: 'admin' },
      },
      {
        path: 'settings/mail',
        name: 'MailSettings',
        component: () => import('@/views/settings/MailSettings.vue'),
        meta: { titleKey: 'layout.menu.mail', role: 'admin' },
      },
      {
        path: 'settings/cli-runtime',
        name: 'CLIRuntimeSettings',
        component: () => import('@/views/settings/CLIRuntimeSettings.vue'),
        meta: { titleKey: 'layout.menu.cliRuntime', role: 'admin' },
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 路由守卫: 登录检查 + 权限检查
router.beforeEach(async (to, _from, next) => {
  if (to.meta.public) {
    next()
    return
  }

  const userStore = useUserStore()

  if (!userStore.isLoggedIn) {
    next('/login')
    return
  }

  // 首次进入时拉取用户信息
  if (!userStore.userInfo) {
    await userStore.fetchUserInfo()
  }

  // 权限检查
  const perm = to.meta.permission as string | undefined
  const permAny = to.meta.permissionAny as string[] | undefined
  if (perm && !userStore.hasPermission(perm)) {
    next('/dashboard')
    return
  }
  if (permAny && !permAny.some((item) => userStore.hasPermission(item))) {
    next('/dashboard')
    return
  }

  next()
})

export default router
