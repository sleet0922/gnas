<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { apiGet, apiPost, DNS_PROVIDERS, type AppConfig, type DnsConfig } from '@/composables/useApi'

const config = ref<AppConfig | null>(null)
const activeTab = ref(0)
const saving = ref(false)
const snackbar = ref(false)
const snackbarText = ref('')

function showMsg(msg: string) {
  snackbarText.value = msg
  snackbar.value = true
}

onMounted(async () => {
  const data = await apiGet<AppConfig>('/api/config')
  if (data) {
    config.value = data
  }
})

function addDnsConf() {
  if (!config.value) return
  config.value.dnsConf.push({
    name: '', dnsName: 'alidns', dnsId: '', dnsSecret: '', dnsExtParam: '',
    ttl: '600', ipv4Enable: false, ipv4GetType: 'url', ipv4Url: '',
    ipv4NetInterface: '', ipv4Cmd: '', ipv4Domains: '',
    ipv6Enable: false, ipv6GetType: 'url', ipv6Url: '',
    ipv6NetInterface: '', ipv6Cmd: '', ipv6Reg: '', ipv6Domains: '',
    httpInterface: '',
  })
  activeTab.value = config.value.dnsConf.length - 1
}

function removeDnsConf(idx: number) {
  config.value?.dnsConf.splice(idx, 1)
  if (activeTab.value >= (config.value?.dnsConf.length || 0)) {
    activeTab.value = Math.max(0, (config.value?.dnsConf.length || 1) - 1)
  }
}

async function save() {
  if (!config.value) return
  saving.value = true
  try {
    await apiPost('/api/config/save', {
      username: config.value.username,
      password: '',
      notAllowWanAccess: config.value.notAllowWanAccess,
      webhookUrl: config.value.webhookUrl,
      webhookRequestBody: config.value.webhookRequestBody,
      webhookHeaders: config.value.webhookHeaders,
      dnsConf: config.value.dnsConf,
    })
    showMsg('配置已保存')
  } catch (e: unknown) {
    showMsg(e instanceof Error ? e.message : '保存失败')
  } finally {
    saving.value = false
  }
}

async function testWebhook() {
  if (!config.value) return
  try {
    await apiPost('/api/webhook/test', {
      url: config.value.webhookUrl,
      requestBody: config.value.webhookRequestBody,
      headers: config.value.webhookHeaders,
    })
    showMsg('Webhook 测试已发送')
  } catch (e: unknown) {
    showMsg(e instanceof Error ? e.message : '测试失败')
  }
}
</script>

<template>
  <div v-if="config">
    <h1 class="text-h5 font-weight-bold mb-6">DDNS 配置</h1>

    <!-- 账号设置 -->
    <v-card color="surface-variant" class="pa-5 mb-4">
      <div class="d-flex align-center ga-3 mb-4">
        <v-icon color="primary">mdi-account-outline</v-icon>
        <span class="text-subtitle-1 font-weight-medium">账号设置</span>
      </div>
      <v-row>
        <v-col cols="12" md="6">
          <v-text-field v-model="config.username" label="用户名" />
        </v-col>
        <v-col cols="12" md="6">
          <v-text-field v-model="config.password" label="新密码（留空不修改）" type="password" placeholder="留空则不修改" />
        </v-col>
      </v-row>
      <v-switch v-model="config.notAllowWanAccess" label="禁止公网访问" class="mt-2" />
    </v-card>

    <!-- DNS 配置 -->
    <v-card color="surface-variant" class="pa-5 mb-4">
      <div class="d-flex align-center ga-3 mb-4">
        <v-icon color="primary">mdi-dns-outline</v-icon>
        <span class="text-subtitle-1 font-weight-medium">DNS 配置</span>
        <v-spacer />
        <v-btn variant="tonal" size="small" prepend-icon="mdi-plus" @click="addDnsConf">添加</v-btn>
      </div>

      <v-tabs v-model="activeTab" density="comfortable" color="primary" slider-color="primary" class="mb-4">
        <v-tab v-for="(_, i) in config.dnsConf" :key="i" size="small">
          配置 {{ i + 1 }}
        </v-tab>
      </v-tabs>

      <v-window v-model="activeTab">
        <v-window-item v-for="(conf, i) in config.dnsConf" :key="i">
          <v-row>
            <v-col cols="12" md="6">
              <v-select
                v-model="conf.dnsName"
                label="DNS 服务商"
                :items="[...DNS_PROVIDERS]"
                item-title="label"
                item-value="value"
              />
            </v-col>
            <v-col cols="12" md="6">
              <v-text-field v-model="conf.ttl" label="TTL" />
            </v-col>
          </v-row>
          <v-row>
            <v-col cols="12" md="6">
              <v-text-field v-model="conf.dnsId" label="ID / Token" />
            </v-col>
            <v-col cols="12" md="6">
              <v-text-field v-model="conf.dnsSecret" label="Secret / Key" type="password" />
            </v-col>
          </v-row>

          <!-- IPv4 -->
          <div class="d-flex align-center ga-2 mt-4 mb-3">
            <v-chip size="small" color="primary" variant="tonal" label>IPv4</v-chip>
            <v-switch v-model="conf.ipv4Enable" label="启用" density="compact" class="ml-2" />
          </div>
          <v-row>
            <v-col cols="12" md="6">
              <v-select
                v-model="conf.ipv4GetType"
                label="获取方式"
                :items="[{title: '接口', value: 'url'}, {title: '网卡', value: 'netInterface'}, {title: '命令', value: 'cmd'}]"
              />
            </v-col>
            <v-col cols="12" md="6">
              <v-text-field v-model="conf.ipv4Url" label="URL" />
            </v-col>
          </v-row>
          <v-textarea v-model="conf.ipv4Domains" label="IPv4 域名（每行一个）" rows="3" class="mt-2" />

          <!-- IPv6 -->
          <div class="d-flex align-center ga-2 mt-6 mb-3">
            <v-chip size="small" color="primary" variant="tonal" label>IPv6</v-chip>
            <v-switch v-model="conf.ipv6Enable" label="启用" density="compact" class="ml-2" />
          </div>
          <v-row>
            <v-col cols="12" md="6">
              <v-select
                v-model="conf.ipv6GetType"
                label="获取方式"
                :items="[{title: '接口', value: 'url'}, {title: '网卡', value: 'netInterface'}, {title: '命令', value: 'cmd'}]"
              />
            </v-col>
            <v-col cols="12" md="6">
              <v-text-field v-model="conf.ipv6Reg" label="IPv6 正则" />
            </v-col>
          </v-row>
          <v-textarea v-model="conf.ipv6Domains" label="IPv6 域名（每行一个）" rows="3" class="mt-2" />

          <div class="d-flex justify-end mt-4">
            <v-btn variant="text" color="error" size="small" prepend-icon="mdi-delete-outline" @click="removeDnsConf(i)">
              删除此配置
            </v-btn>
          </div>
        </v-window-item>
      </v-window>
    </v-card>

    <!-- Webhook -->
    <v-card color="surface-variant" class="pa-5 mb-4">
      <div class="d-flex align-center ga-3 mb-4">
        <v-icon color="primary">mdi-webhook</v-icon>
        <span class="text-subtitle-1 font-weight-medium">Webhook 通知</span>
      </div>
      <v-text-field v-model="config.webhookUrl" label="Webhook URL" class="mb-2" />
      <v-textarea v-model="config.webhookRequestBody" label="Request Body" rows="3" class="mb-2" />
      <v-textarea v-model="config.webhookHeaders" label="Headers" rows="3" class="mb-2" />
      <v-btn variant="tonal" size="small" prepend-icon="mdi-send" @click="testWebhook">测试</v-btn>
    </v-card>

    <!-- 保存按钮 -->
    <div class="d-flex justify-end">
      <v-btn color="primary" size="large" prepend-icon="mdi-content-save-outline" :loading="saving" @click="save">
        保存配置
      </v-btn>
    </div>

    <v-snackbar v-model="snackbar" :timeout="3000" color="on-surface" rounded="lg">
      {{ snackbarText }}
    </v-snackbar>
  </div>

  <div v-else class="d-flex justify-center pa-8">
    <v-progress-circular indeterminate color="primary" />
  </div>
</template>
