import { createRouter, createWebHistory } from 'vue-router'
import LoginPage from '@/pages/LoginPage.vue'
import FilesPage from '@/pages/FilesPage.vue'
import GalleryPage from '@/pages/GalleryPage.vue'
import SystemPage from '@/pages/SystemPage.vue'
import LogsPage from '@/pages/LogsPage.vue'
import { getToken } from '@/composables/useApi'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginPage, meta: { public: true } },
    { path: '/', redirect: '/files' },
    { path: '/files', name: 'files', component: FilesPage },
    { path: '/gallery', name: 'gallery', component: GalleryPage },
    { path: '/system', name: 'system', component: SystemPage },
    { path: '/logs', name: 'logs', component: LogsPage },
    { path: '/:pathMatch(.*)*', redirect: '/files' },
  ],
})

router.beforeEach((to) => {
  if (to.meta.public) return true
  if (!getToken()) return { name: 'login' }
  return true
})

export default router
