<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { apiPost } from '@/composables/useApi'

const router = useRouter()
const route = useRoute()
const drawer = ref(false)

const isLoginPage = computed(() => route.name === 'login')

const navItems = [
  { icon: 'mdi-view-dashboard-outline', title: '概览', to: '/' },
  { icon: 'mdi-dns-outline', title: 'DDNS', to: '/ddns' },
  { icon: 'mdi-console-line', title: '日志', to: '/logs' },
]

async function logout() {
  await apiPost('/api/logout', {})
  router.push('/login')
}
</script>

<template>
  <v-app v-if="isLoginPage">
    <router-view />
  </v-app>

  <v-app v-else>
    <!-- 桌面端导航栏 -->
    <v-navigation-drawer :width="220" permanent class="d-none d-md-flex" color="background">
      <div class="pa-4 pb-2">
        <div class="d-flex align-center ga-3">
          <v-avatar color="primary" size="36" class="text-white font-weight-bold text-h6">G</v-avatar>
          <span class="text-h6 font-weight-bold text-primary">GNAS</span>
        </div>
      </div>

      <v-list density="comfortable" nav class="px-2">
        <v-list-item
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          :prepend-icon="item.icon"
          :title="item.title"
          rounded="lg"
          color="primary"
          base-color="on-surface-variant"
          slim
        />
      </v-list>

      <template #append>
        <v-divider />
        <v-list density="comfortable" nav class="px-2 pb-4">
          <v-list-item
            prepend-icon="mdi-logout"
            title="登出"
            rounded="lg"
            base-color="on-surface-variant"
            @click="logout"
            slim
          />
        </v-list>
      </template>
    </v-navigation-drawer>

    <!-- 移动端导航栏 -->
    <v-bottom-navigation class="d-flex d-md-none" color="primary" grow>
      <v-btn v-for="item in navItems" :key="item.to" :to="item.to">
        <v-icon>{{ item.icon }}</v-icon>
        <span>{{ item.title }}</span>
      </v-btn>
    </v-bottom-navigation>

    <!-- 顶部应用栏 -->
    <v-app-bar flat color="background" class="d-md-none">
      <v-app-bar-nav-icon @click="drawer = !drawer" />
      <v-toolbar-title class="font-weight-bold text-primary">GNAS</v-toolbar-title>
    </v-app-bar>

    <v-main>
      <v-container class="pa-4 pa-md-8" style="max-width: 960px;">
        <router-view />
      </v-container>
    </v-main>

    <!-- 移动端抽屉 -->
    <v-navigation-drawer v-model="drawer" temporary class="d-md-none">
      <div class="pa-4 pb-2">
        <div class="d-flex align-center ga-3">
          <v-avatar color="primary" size="36" class="text-white font-weight-bold text-h6">G</v-avatar>
          <span class="text-h6 font-weight-bold text-primary">GNAS</span>
        </div>
      </div>
      <v-list density="comfortable" nav class="px-2">
        <v-list-item
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          :prepend-icon="item.icon"
          :title="item.title"
          rounded="lg"
          color="primary"
          @click="drawer = false"
          slim
        />
      </v-list>
      <template #append>
        <v-divider />
        <v-list density="comfortable" nav class="px-2 pb-4">
          <v-list-item
            prepend-icon="mdi-logout"
            title="登出"
            rounded="lg"
            @click="logout"
            slim
          />
        </v-list>
      </template>
    </v-navigation-drawer>
  </v-app>
</template>

<style>
html { overflow-y: auto }
body { font-family: 'Google Sans', 'Noto Sans SC', -apple-system, BlinkMacSystemFont, sans-serif; }
</style>
