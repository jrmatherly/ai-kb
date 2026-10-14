import { createApp, h } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory, RouterView } from 'vue-router'
import TDesign, { ConfigProvider, type GlobalConfigProvider } from 'tdesign-vue-next'
import enUSConfig from 'tdesign-vue-next/esm/locale/en_US'
import 'tdesign-vue-next/es/style/index.css'
import '@/assets/theme/theme.css'
import { installTDesignIconOfflineGuard } from '@/utils/tdesign-icon-offline'
import i18n from './i18n/embed'
import EmbedPage from '@/views/embed/EmbedPage.vue'
import ProtectedResourcePreview from '@/components/ProtectedResourcePreview.vue'

installTDesignIconOfflineGuard()

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/embed/:channelId',
      name: 'embed',
      component: EmbedPage,
    },
  ],
})

// Runtime-only Vue build cannot compile string templates - use a render fn.
// The locale module is typed with readonly tuples that GlobalConfigProvider declares as mutable.
const app = createApp({
  render: () => h(ConfigProvider, { globalConfig: enUSConfig as unknown as GlobalConfigProvider }, () => [h(RouterView), h(ProtectedResourcePreview)]),
})

app.use(TDesign)
app.use(createPinia())
app.use(router)
app.use(i18n)

router.isReady().finally(() => {
  app.mount('#embed-app')
})
