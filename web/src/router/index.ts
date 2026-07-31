import { createRouter, createWebHistory } from 'vue-router'
import { getToken } from '@/composables/useApi'

const LoginPage = () => import('@/pages/LoginPage.vue')
const FilesPage = () => import('@/pages/FilesPage.vue')
const GalleryPage = () => import('@/pages/GalleryPage.vue')
const SystemPage = () => import('@/pages/SystemPage.vue')
const LogsPage = () => import('@/pages/LogsPage.vue')
const DuplicatesPage = () => import('@/pages/DuplicatesPage.vue')

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginPage, meta: { public: true } },
    { path: '/', redirect: '/files' },
    { path: '/files', name: 'files', component: FilesPage },
    { path: '/gallery', name: 'gallery', component: GalleryPage },
    { path: '/system', name: 'system', component: SystemPage },
    { path: '/duplicates', name: 'duplicates', component: DuplicatesPage },
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
