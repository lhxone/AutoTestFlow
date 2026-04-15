<template>
  <a-config-provider :locale="antLocale">
    <router-view />
  </a-config-provider>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import enUS from 'ant-design-vue/es/locale/en_US'
import zhCN from 'ant-design-vue/es/locale/zh_CN'
import i18n from '@/locales'

const antLocale = computed(() => (
  i18n.global.locale.value === 'en-US' ? enUS : zhCN
))
</script>
