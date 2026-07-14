<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { apiPost, clearToken } from '@/composables/useApi'

const router = useRouter()
const route = useRoute()
const drawer = ref(false)
const changePwdDialog = ref(false)
const oldPwd = ref('')
const newPwd = ref('')
const confirmPwd = ref('')
const changePwdMsg = ref('')
const changePwdLoading = ref(false)

const isLoginPage = computed(() => route.name === 'login')

const navItems = [
  { icon: 'mdi-folder-outline', title: '文件', to: '/files' },
  { icon: 'mdi-image-multiple-outline', title: '相册', to: '/gallery' },
  { icon: 'mdi-chart-donut', title: '占用', to: '/system' },
  { icon: 'mdi-console-line', title: '日志', to: '/logs' },
]

async function logout() {
  try {
    await apiPost('/api/logout', {})
  } finally {
    clearToken()
  }
  router.push('/login')
}

async function changePassword() {
  changePwdMsg.value = ''
  if (!oldPwd.value || !newPwd.value || !confirmPwd.value) {
    changePwdMsg.value = '请填写所有字段'
    return
  }
  if (newPwd.value !== confirmPwd.value) {
    changePwdMsg.value = '两次输入的新密码不一致'
    return
  }
  if (newPwd.value.length < 4) {
    changePwdMsg.value = '新密码至少4个字符'
    return
  }
  changePwdLoading.value = true
  try {
    await apiPost('/api/change-password', {
      oldPassword: oldPwd.value,
      newPassword: newPwd.value,
    })
    changePwdDialog.value = false
    oldPwd.value = ''
    newPwd.value = ''
    confirmPwd.value = ''
    clearToken()
    router.push('/login')
  } catch (e: unknown) {
    changePwdMsg.value = e instanceof Error ? e.message : '修改失败'
  } finally {
    changePwdLoading.value = false
  }
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
            prepend-icon="mdi-lock-outline"
            title="修改密码"
            rounded="lg"
            base-color="on-surface-variant"
            @click="changePwdDialog = true"
            slim
          />
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
            prepend-icon="mdi-lock-outline"
            title="修改密码"
            rounded="lg"
            @click="changePwdDialog = true; drawer = false"
            slim
          />
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

    <!-- 修改密码对话框 -->
    <v-dialog v-model="changePwdDialog" max-width="400">
      <v-card>
        <v-card-title class="text-h6">修改密码</v-card-title>
        <v-card-text>
          <v-text-field v-model="oldPwd" label="旧密码" type="password" variant="outlined" density="compact" class="mb-2" />
          <v-text-field v-model="newPwd" label="新密码" type="password" variant="outlined" density="compact" class="mb-2" />
          <v-text-field v-model="confirmPwd" label="确认新密码" type="password" variant="outlined" density="compact" />
          <div v-if="changePwdMsg" class="text-error text-body-2 mt-2">{{ changePwdMsg }}</div>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="changePwdDialog = false">取消</v-btn>
          <v-btn color="primary" :loading="changePwdLoading" @click="changePassword">确认</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-app>
</template>

<style>
html { overflow-y: auto }
body { font-family: 'Google Sans', 'Noto Sans SC', -apple-system, BlinkMacSystemFont, sans-serif; }
</style>
