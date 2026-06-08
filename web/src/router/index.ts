import { createRouter, createWebHistory } from 'vue-router'
import LoginPage from '@/pages/LoginPage.vue'
import DashboardPage from '@/pages/DashboardPage.vue'
import DdnsPage from '@/pages/DdnsPage.vue'
import LogsPage from '@/pages/LogsPage.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginPage, meta: { public: true } },
    { path: '/', name: 'dashboard', component: DashboardPage },
    { path: '/ddns', name: 'ddns', component: DdnsPage },
    { path: '/logs', name: 'logs', component: LogsPage },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  if (to.meta.public) return true
  try {
    const res = await fetch('/api/status')
    const data = await res.json()
    if (data.code !== 0) return { name: 'login' }
    return true
  } catch {
    return { name: 'login' }
  }
})

export default router
