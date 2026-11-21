<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950" :class="{ 'sticker-dashboard': stickerDashboard }">
    <!-- Background Decoration -->
    <div v-if="!stickerDashboard" class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader :sticker-dashboard="stickerDashboard" />

      <!-- Main Content -->
      <main
        class="p-4 md:p-6 lg:p-8"
        :class="{ 'sticker-dashboard-main': stickerDashboard }"
        data-app-content
      >
        <div v-if="stickerDashboard" class="notebook-rings" aria-hidden="true"><i v-for="ring in 5" :key="ring"></i></div>
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

defineProps<{ stickerDashboard?: boolean }>()

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>
