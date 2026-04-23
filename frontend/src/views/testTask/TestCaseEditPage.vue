<template>
  <div>
    <a-page-header :title="pageTitle" @back="$router.back()">
      <template #extra>
        <a-tag :color="taskStatusColor(task?.status || '')">
          {{ translateTaskStatus(t, task?.status || '') || '-' }}
        </a-tag>
      </template>
    </a-page-header>

    <a-spin :spinning="loading">
      <a-card :title="t('testCase.detail.issueInfo')" size="small" style="margin-bottom: 16px">
        <a-descriptions :column="1" bordered size="small">
          <a-descriptions-item :label="t('testCase.detail.taskId')">{{ task?.id || '-' }}</a-descriptions-item>
          <a-descriptions-item :label="t('testCase.detail.issueTitle')">{{ task?.issue?.title || '-' }}</a-descriptions-item>
          <a-descriptions-item :label="t('testCase.detail.issueId')">{{ task?.issue_id || '-' }}</a-descriptions-item>
          <a-descriptions-item :label="t('common.status')">
            <a-tag :color="taskStatusColor(task?.status || '')">
              {{ translateTaskStatus(t, task?.status || '') || '-' }}
            </a-tag>
          </a-descriptions-item>
        </a-descriptions>
      </a-card>

      <a-card :title="t('testCase.detail.selfTestReportTitle')" size="small" style="margin-bottom: 16px">
        <a-alert
          v-if="selfTestReport"
          :type="selfTestReport.passed === false ? 'error' : 'success'"
          :message="selfTestReport.summary || t('testCase.detail.selfTestNoSummary')"
          show-icon
          style="margin-bottom: 12px"
        />
        <a-alert
          v-else
          type="info"
          :message="t('testCase.detail.selfTestNoReport')"
          show-icon
          style="margin-bottom: 12px"
        />

        <a-descriptions :column="1" size="small" bordered style="margin-bottom: 12px">
          <a-descriptions-item :label="t('testCase.detail.selfTestChecks')">
            <ul v-if="selfTestChecks.length" style="margin: 0; padding-left: 18px">
              <li v-for="(item, index) in selfTestChecks" :key="`check-${index}`">{{ item }}</li>
            </ul>
            <span v-else>-</span>
          </a-descriptions-item>
        </a-descriptions>
      </a-card>

      <a-collapse :defaultActiveKey="['cases', 'scripts']" style="margin-top: 16px">
        <a-collapse-panel key="cases" :header="t('testCase.detail.caseGroupTitle')">
          <template v-if="editableCases.length">
            <a-collapse v-model:activeKey="activeCasePanelKeys">
              <a-collapse-panel
                v-for="editableCase in editableCases"
                :key="editableCase.id"
                :header="`[${translateTestCaseCategory(t, editableCase.category)}] ${editableCase.title}`"
              >
                <a-form layout="vertical">
                  <a-row :gutter="16">
                    <a-col :span="24">
                      <a-form-item :label="t('testCase.detail.caseTitle')">
                        <a-input v-model:value="editableCase.title" :disabled="!canIntervene" />
                      </a-form-item>
                    </a-col>
                  </a-row>

                  <a-row :gutter="16">
                    <a-col :span="16">
                      <a-form-item :label="t('testCase.detail.precondition')">
                        <a-textarea v-model:value="editableCase.precondition" :rows="3" :disabled="!canIntervene" />
                      </a-form-item>
                    </a-col>
                    <a-col :span="8">
                      <a-form-item :label="t('review.detail.selfTestResult')">
                        <a-tag :color="editableCase.self_test_result === 'pass' ? 'green' : editableCase.self_test_result === 'fail' ? 'red' : 'default'">
                          {{ translateSelfTestResult(t, editableCase.self_test_result) }}
                        </a-tag>
                      </a-form-item>
                      <a-form-item :label="t('review.detail.source')">
                        <a-tag :color="editableCase.source === 'ai' ? 'blue' : 'orange'">
                          {{ translateCaseSource(t, editableCase.source) }}
                        </a-tag>
                      </a-form-item>
                    </a-col>
                  </a-row>

                  <a-form-item :label="t('testCase.detail.stepTableTitle')">
                    <a-table :dataSource="editableCase.stepRows" :columns="stepColumns" :pagination="false" size="small" rowKey="key">
                      <template #bodyCell="{ column, record }">
                        <template v-if="column.key === 'step'">
                          <a-textarea v-model:value="record.step" :rows="2" :disabled="!canIntervene" />
                        </template>
                        <template v-if="column.key === 'expected'">
                          <a-textarea v-model:value="record.expected" :rows="2" :disabled="!canIntervene" />
                        </template>
                        <template v-if="column.key === 'action'">
                          <a-button type="link" danger size="small" @click="removeStepRow(editableCase.id, record.key)" :disabled="!canIntervene">
                            {{ t('common.delete') }}
                          </a-button>
                        </template>
                      </template>
                    </a-table>
                    <a-button type="dashed" block style="margin-top: 12px" @click="appendStepRow(editableCase.id)" :disabled="!canIntervene">
                      {{ t('testCase.detail.addStep') }}
                    </a-button>
                  </a-form-item>

                  <a-form-item :label="t('testCase.detail.changeNote')" required v-if="canIntervene">
                    <a-input
                      v-model:value="editableCase.changeNote"
                      :placeholder="t('testCase.detail.changeNotePlaceholder')"
                    />
                  </a-form-item>

                  <a-space v-if="canIntervene">
                    <a-button type="primary" :loading="savingCaseId === editableCase.id" @click="saveCase(editableCase)">
                      {{ t('common.save') }}
                    </a-button>
                  </a-space>
                </a-form>
              </a-collapse-panel>
            </a-collapse>
          </template>
          <a-empty v-else :description="t('review.detail.noTestCases')" />
        </a-collapse-panel>

        <a-collapse-panel key="scripts" :header="t('testCase.detail.scriptGroupTitle')">
          <template v-if="editableScripts.length">
            <a-form v-for="editableScript in editableScripts" :key="editableScript.id" layout="vertical" style="margin-bottom: 16px">
              <a-form-item :label="t('testCase.detail.scriptContent')">
                <CodeEditor
                  v-model="editableScript.file_content"
                  :language="scriptEditorLanguage(editableScript)"
                  :height="380"
                  :readOnly="!canIntervene"
                />
              </a-form-item>
              <a-form-item :label="t('testCase.detail.changeNote')" required v-if="canIntervene">
                <a-input
                  v-model:value="editableScript.changeNote"
                  :placeholder="t('testCase.detail.changeNotePlaceholder')"
                />
              </a-form-item>
              <a-button type="primary" :loading="savingScriptId === editableScript.id" @click="saveScript(editableScript)" v-if="canIntervene">
                {{ t('common.save') }}
              </a-button>
            </a-form>
          </template>
          <a-empty v-else :description="t('review.detail.noTestScripts')" />
        </a-collapse-panel>

        <a-collapse-panel key="playwright" :header="t('testCase.detail.playwrightReport')">
          <template v-if="playwrightReportUrl">
            <iframe
              :key="playwrightReportFrameKey"
              class="playwright-report-frame"
              :src="playwrightReportUrl"
              sandbox="allow-scripts allow-same-origin allow-modals allow-popups allow-forms allow-downloads"
              @load="playwrightReportLoading = false"
            />
          </template>
          <div v-else-if="playwrightReportLoading" class="report-loading">
            <a-spin :tip="t('common.loading')" />
          </div>
          <a-empty v-else :description="t('taskRun.selfTest.noReportContent')" />
        </a-collapse-panel>
      </a-collapse>

    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import CodeEditor from '@/components/CodeEditor.vue'
import { getTestCases, getTestScripts, getTestTaskById, getSelfTestReport, getWorkspaceFileUrl, updateTestCase, updateTestScript } from '@/api/testTask'
import { useUserStore } from '@/stores/user'
import type { SelfTestReport, TestCaseVO, TestScriptVO, TestTask } from '@/types'
import { translateCaseSource, translateSelfTestResult, translateTaskStatus, translateTestCaseCategory } from '@/types'
import { useI18n } from 'vue-i18n'

type StepRow = {
  key: string
  step: string
  expected: string
}

type EditableCase = TestCaseVO & {
  stepRows: StepRow[]
  changeNote: string
}

type EditableScript = TestScriptVO & {
  changeNote: string
}

const route = useRoute()
const { t } = useI18n()
const userStore = useUserStore()

const taskId = Number(route.params.id)
const loading = ref(false)
const savingCaseId = ref<number | null>(null)
const savingScriptId = ref<number | null>(null)
const task = ref<TestTask | null>(null)
const editableCases = ref<EditableCase[]>([])
const editableScripts = ref<EditableScript[]>([])
const activeCaseKeys = ref<Array<number | string>>([])
const activeCasePanelKeys = ref<(number | string)[]>([])
const playwrightReportUrl = ref('')
const playwrightReportLoading = ref(false)
const playwrightReportFrameKey = ref(0)
const canIntervene = computed(() => userStore.hasPermission('test:intervene'))
const selfTestReport = computed<SelfTestReport | null>(() => {
  const report = task.value?.ai_output?.self_test
  if (!report || typeof report !== 'object') return null
  return report
})
const selfTestChecks = computed<string[]>(() => {
  if (!selfTestReport.value?.checks || !Array.isArray(selfTestReport.value.checks)) {
    return []
  }
  return selfTestReport.value.checks
})
const pageTitle = computed(() => task.value?.issue?.title || t('testCase.detail.title'))
const stepColumns = computed(() => [
  { title: t('testCase.detail.columns.step'), key: 'step' },
  { title: t('testCase.detail.columns.expected'), key: 'expected' },
  { title: t('common.action'), key: 'action', width: 90 },
])

onMounted(fetchDetail)
watch(
  () => route.query.caseId,
  () => syncActiveCase(),
)

async function fetchDetail() {
  loading.value = true
  try {
    const [taskRes, casesRes, scriptsRes] = await Promise.allSettled([
      getTestTaskById(taskId),
      getTestCases(taskId),
      getTestScripts(taskId),
    ])

    if (taskRes.status !== 'fulfilled' || casesRes.status !== 'fulfilled') {
      throw new Error('load_failed')
    }

    task.value = taskRes.value.data.data
    editableCases.value = (casesRes.value.data.data || []).map((item: TestCaseVO) => ({
      ...item,
      stepRows: parseStepRows(item.steps, item.expected),
      changeNote: '',
    }))
    syncActiveCase()
    activeCasePanelKeys.value = editableCases.value.map(c => c.id)
    if (scriptsRes.status === 'fulfilled') {
      editableScripts.value = (scriptsRes.value.data.data || []).map((item: TestScriptVO) => ({
        ...item,
        changeNote: '',
      }))
    } else {
      editableScripts.value = []
    }
    await loadPlaywrightReport()
  } catch {
    message.error(t('common.requestFailed'))
  } finally {
    loading.value = false
  }
}

async function loadPlaywrightReport() {
  playwrightReportUrl.value = ''
  playwrightReportLoading.value = true
  try {
    const res = await getSelfTestReport(taskId, 'playwright')
    const data = res.data.data || {}
    if (data.content_type?.includes('html') && data.report_path) {
      playwrightReportUrl.value = getWorkspaceFileUrl(taskId, data.report_path)
      playwrightReportFrameKey.value += 1
    } else {
      playwrightReportLoading.value = false
    }
  } catch (e) {
    console.error('loadPlaywrightReport failed:', e)
    playwrightReportLoading.value = false
  }
}

function syncActiveCase() {
  const rawCaseId = route.query.caseId
  const caseId = Number(Array.isArray(rawCaseId) ? rawCaseId[0] : rawCaseId)
  if (caseId && editableCases.value.some((item) => item.id === caseId)) {
    activeCaseKeys.value = [caseId]
    return
  }
  activeCaseKeys.value = editableCases.value.length ? [editableCases.value[0].id] : []
}

function parseStepRows(steps: string, expected: string) {
  const stepLines = normalizeLines(steps)
  const expectedLines = normalizeLines(expected)
  const maxLength = Math.max(stepLines.length, expectedLines.length, 1)
  const rows: StepRow[] = []
  for (let index = 0; index < maxLength; index += 1) {
    rows.push({
      key: `row-${index}-${Date.now()}`,
      step: stepLines[index] || '',
      expected: expectedLines[index] || '',
    })
  }
  return rows
}

function normalizeLines(value: string) {
  return value
    .split('\n')
    .map((item) => item.trim())
    .filter((item) => item.length > 0)
}

function appendStepRow(caseId: number) {
  const currentCase = editableCases.value.find((item) => item.id === caseId)
  if (!currentCase) {
    return
  }
  currentCase.stepRows.push({
    key: `row-${caseId}-${currentCase.stepRows.length}-${Date.now()}`,
    step: '',
    expected: '',
  })
}

function removeStepRow(caseId: number, rowKey: string) {
  const currentCase = editableCases.value.find((item) => item.id === caseId)
  if (!currentCase) {
    return
  }
  currentCase.stepRows = currentCase.stepRows.filter((item) => item.key !== rowKey)
  if (!currentCase.stepRows.length) {
    appendStepRow(caseId)
  }
}

async function saveCase(editableCase: EditableCase) {
  if (!editableCase.changeNote.trim()) {
    message.warning(t('testCase.detail.changeNoteRequired'))
    return
  }

  const filteredRows = editableCase.stepRows.filter((item) => item.step.trim() || item.expected.trim())
  if (!filteredRows.length) {
    message.warning(t('testCase.detail.stepRequired'))
    return
  }

  savingCaseId.value = editableCase.id
  try {
    await updateTestCase(editableCase.id, {
      title: editableCase.title,
      precondition: editableCase.precondition,
      steps: filteredRows.map((item) => item.step.trim()).join('\n'),
      expected: filteredRows.map((item) => item.expected.trim()).join('\n'),
      change_note: editableCase.changeNote,
    })
    message.success(t('testCase.detail.saveSuccess'))
    editableCase.changeNote = ''
    await fetchDetail()
  } finally {
    savingCaseId.value = null
  }
}

async function saveScript(editableScript: EditableScript) {
  if (!editableScript.changeNote.trim()) {
    message.warning(t('testCase.detail.changeNoteRequired'))
    return
  }

  savingScriptId.value = editableScript.id
  try {
    await updateTestScript(editableScript.id, {
      file_content: editableScript.file_content,
      change_note: editableScript.changeNote,
    })
    message.success(t('testCase.detail.saveSuccess'))
    editableScript.changeNote = ''
    await fetchDetail()
  } finally {
    savingScriptId.value = null
  }
}

function taskStatusColor(status: string) {
  const map: Record<string, string> = {
    pending: 'default',
    running: 'processing',
    completed: 'success',
    failed: 'error',
    warning: 'warning',
  }
  return map[status] || 'default'
}

function scriptEditorLanguage(script: EditableScript) {
  const filePath = script.file_path.toLowerCase()
  if (filePath.endsWith('.spec.ts') || filePath.endsWith('.ts')) {
    return 'typescript'
  }
  if (filePath.endsWith('.js')) {
    return 'javascript'
  }
  if (filePath.endsWith('.py')) {
    return 'python'
  }
  return script.language || 'typescript'
}

</script>

<style scoped>
.playwright-report-frame {
  width: 100%;
  height: 70vh;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  background: #fff;
}

.report-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 180px;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  background: #fff;
}
</style>
