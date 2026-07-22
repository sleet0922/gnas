<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { apiGet, apiPost, setToken } from '@/composables/useApi'

const router = useRouter()
const username = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')
const needSetup = ref(false)

onMounted(async () => {
  const data = await apiGet<{ needSetup: boolean }>('/api/login')
  if (data) {
    needSetup.value = data.needSetup
  }
})

async function login() {
  error.value = ''
  loading.value = true
  try {
    const res = await apiPost<{ success: boolean, token: string }>('/api/login', {
      username: username.value,
      password: password.value,
    })
    if (res?.success && res.token) {
      setToken(res.token)
      router.push('/')
    } else {
      error.value = '登录失败'
    }
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <v-app style="background: #f8f9fa;">
    <v-container fluid class="fill-height d-flex justify-center align-center">
      <v-card width="420" class="pa-8" :elevation="2" :border="false" rounded="xl">
        <div class="d-flex justify-center mb-6">
          <v-avatar color="primary" size="56" class="text-white font-weight-bold text-h4">G</v-avatar>
        </div>
        <h1 class="text-h5 font-weight-bold text-center mb-1">GNAS</h1>
        <p class="text-body-2 text-on-surface-variant text-center mb-8">
          {{ needSetup ? '设置管理员账号以开始使用' : '登录以管理你的 NAS 服务' }}
        </p>

        <v-form @submit.prevent="login">
          <v-text-field
            v-model="username"
            label="用户名"
            prepend-inner-icon="mdi-account-outline"
            autocomplete="username"
          />
          <v-text-field
            v-model="password"
            label="密码"
            type="password"
            prepend-inner-icon="mdi-lock-outline"
            autocomplete="current-password"
            class="mt-1"
          />

          <v-alert v-if="error" type="error" variant="tonal" class="mt-4 mb-4" density="compact" rounded="lg">
            {{ error }}
          </v-alert>

          <v-btn
            type="submit"
            color="primary"
            size="large"
            block
            :loading="loading"
            class="mt-4 text-none font-weight-medium"
          >
            {{ needSetup ? '设置并登录' : '登录' }}
          </v-btn>
        </v-form>
      </v-card>
    </v-container>
  </v-app>
</template>
