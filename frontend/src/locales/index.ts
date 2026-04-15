import { createI18n } from 'vue-i18n'
import enUS from './en-US.json'
import zhCN from './zh-CN.json'

export const LOCALE_STORAGE_KEY = 'atf-locale'
export const SUPPORTED_LOCALES = ['zh-CN', 'en-US'] as const
export type AppLocale = typeof SUPPORTED_LOCALES[number]

function resolveInitialLocale(): AppLocale {
  const storedLocale = localStorage.getItem(LOCALE_STORAGE_KEY)
  if (storedLocale && SUPPORTED_LOCALES.includes(storedLocale as AppLocale)) {
    return storedLocale as AppLocale
  }
  return 'zh-CN'
}

const i18n = createI18n({
  legacy: false,
  locale: resolveInitialLocale(),
  fallbackLocale: 'en-US',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS,
  },
})

export function setAppLocale(locale: AppLocale) {
  i18n.global.locale.value = locale
  localStorage.setItem(LOCALE_STORAGE_KEY, locale)
}

export default i18n

export type I18nMessages = typeof zhCN
