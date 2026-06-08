<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()
</script>

<template>
  <div>
    <h1 class="text-h5 font-weight-bold mb-6">概览</h1>

    <v-row>
      <v-col cols="12" sm="6">
        <v-card color="surface-variant" class="pa-5">
          <div class="d-flex align-center ga-3 mb-4">
            <v-icon color="primary">mdi-information-outline</v-icon>
            <span class="text-subtitle-1 font-weight-medium">系统状态</span>
          </div>
          <v-row dense>
            <v-col cols="6">
              <div class="text-caption text-on-surface-variant">DDNS 服务</div>
              <div class="d-flex align-center ga-2 mt-1">
                <v-icon size="10" color="success">mdi-circle</v-icon>
                <span class="text-body-2 font-weight-medium">运行中</span>
              </div>
            </v-col>
            <v-col cols="6">
              <div class="text-caption text-on-surface-variant">版本</div>
              <div class="text-body-2 font-weight-medium mt-1">1.0.0</div>
            </v-col>
          </v-row>
        </v-card>
      </v-col>
    </v-row>

    <v-row class="mt-2">
      <v-col cols="12" sm="6">
        <v-card
          color="surface-variant"
          class="pa-5 cursor-pointer"
          hover
          @click="router.push('/ddns')"
        >
          <div class="d-flex align-center ga-3 mb-2">
            <v-icon color="primary">mdi-dns-outline</v-icon>
            <span class="text-subtitle-1 font-weight-medium">DDNS 配置</span>
          </div>
          <p class="text-body-2 text-on-surface-variant">管理动态域名解析配置，支持多种 DNS 服务商</p>
        </v-card>
      </v-col>
      <v-col cols="12" sm="6">
        <v-card
          color="surface-variant"
          class="pa-5 cursor-pointer"
          hover
          @click="router.push('/logs')"
        >
          <div class="d-flex align-center ga-3 mb-2">
            <v-icon color="primary">mdi-console-line</v-icon>
            <span class="text-subtitle-1 font-weight-medium">运行日志</span>
          </div>
          <p class="text-body-2 text-on-surface-variant">查看 DDNS 运行日志和更新记录</p>
        </v-card>
      </v-col>
    </v-row>
  </div>
</template>
