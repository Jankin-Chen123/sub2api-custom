<template>
  <div
    class="journal-standalone relative flex min-h-screen items-center justify-center overflow-hidden px-4 py-8"
  >
    <!-- Watercolor Paper Ambient Decoration -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div
        class="absolute -right-32 -top-32 h-96 w-96 rounded-full bg-[#92d4fb]/25 blur-3xl dark:bg-[#2c5b86]/25"
      ></div>
      <div
        class="absolute -bottom-32 -left-32 h-96 w-96 rounded-full bg-[#ff9466]/20 blur-3xl dark:bg-[#ff9a6e]/12"
      ></div>
    </div>

    <div class="card relative z-10 w-full max-w-md p-8 text-center sm:p-10">
      <!-- 404 Display -->
      <div class="mb-7">
        <div class="relative inline-block">
          <span class="text-[8.5rem] font-black leading-none text-[#efe4d0] dark:text-[#262c3d] sm:text-[10rem]"
            >404</span
          >
          <div class="absolute inset-0 flex items-center justify-center">
            <div
              class="flex h-20 w-20 items-center justify-center rounded-2xl border border-[#2574b3]/30 bg-gradient-to-b from-[#40a7ef] to-[#2682c8] shadow-lg shadow-[#2682c8]/25"
            >
              <svg
                class="h-10 w-10 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="1.8"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z"
                />
              </svg>
            </div>
          </div>
        </div>
      </div>

      <!-- Text Content -->
      <div class="mb-7">
        <h1 class="mb-2.5 text-2xl font-black text-[var(--craft-ink)]">
          {{ t('errors.pageNotFound') }}
        </h1>
        <p class="text-sm text-[var(--craft-muted)]">
          The page you are looking for doesn't exist or has been moved.
        </p>
      </div>

      <!-- Action Buttons -->
      <div class="flex flex-col justify-center gap-3 sm:flex-row">
        <button @click="goBack" class="btn btn-secondary">
          <Icon name="arrowLeft" size="md" class="mr-1.5" />
          {{ t('common.back') }}
        </button>
        <router-link to="/dashboard" class="btn btn-primary">
          <Icon name="home" size="md" class="mr-1.5" />
          {{ t('nav.dashboard') }}
        </router-link>
      </div>

      <!-- Home Link -->
      <p class="mt-7 text-sm text-[var(--craft-muted)]">
        <router-link
          to="/home"
          class="font-medium text-[var(--craft-sky-deep)] transition-colors hover:underline"
        >
          {{ t('auth.backHome') }}
        </router-link>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const router = useRouter()

function goBack(): void {
  router.back()
}
</script>
