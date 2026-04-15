<template>
  <div>
    <a-page-header :title="t('testCase.list.title')" />
    <TestCaseList embedded />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import TestCaseList from '@/views/testTask/TestCaseList.vue'

const { t } = useI18n()
</script>
