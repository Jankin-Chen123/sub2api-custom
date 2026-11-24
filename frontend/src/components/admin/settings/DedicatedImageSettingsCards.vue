<template>
  <div class="space-y-6">
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t("admin.settings.dedicatedImage.title") }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.dedicatedImage.description") }}
        </p>
      </div>
      <div class="divide-y divide-gray-100 px-6 dark:divide-dark-700">
        <div class="flex items-center justify-between gap-6 py-4">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">{{ t("admin.settings.dedicatedImage.enabled") }}</label>
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t("admin.settings.dedicatedImage.enabledHint") }}</p>
          </div>
          <Toggle v-model="dedicatedImageEnabled" data-testid="dedicated-image-enabled" />
        </div>
        <div class="flex items-center justify-between gap-6 py-4">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">{{ t("admin.settings.dedicatedImage.workerEnabled") }}</label>
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t("admin.settings.dedicatedImage.workerEnabledHint") }}</p>
          </div>
          <Toggle v-model="dedicatedImageWorkerEnabled" :disabled="!dedicatedImageEnabled" data-testid="dedicated-image-worker-enabled" />
        </div>
        <div class="flex items-center justify-between gap-6 py-4">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">{{ t("admin.settings.dedicatedImage.codexBridgeEnabled") }}</label>
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t("admin.settings.dedicatedImage.codexBridgeEnabledHint") }}</p>
          </div>
          <Toggle
            v-model="dedicatedImageCodexBridgeEnabled"
            :disabled="!dedicatedImageEnabled || !dedicatedImageWorkerEnabled"
            data-testid="dedicated-image-codex-bridge-enabled"
          />
        </div>
        <div class="flex items-center justify-between gap-6 py-4">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">{{ t("admin.settings.dedicatedImage.fallbackToGeneral") }}</label>
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t("admin.settings.dedicatedImage.fallbackToGeneralHint") }}</p>
          </div>
          <Toggle
            v-model="dedicatedImageFallbackToGeneral"
            :disabled="!dedicatedImageEnabled || !dedicatedImageWorkerEnabled"
            data-testid="dedicated-image-fallback-to-general"
          />
        </div>
      </div>
    </div>

    <!-- Image generation queue protection -->
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t("admin.settings.imageGenerationQueue.title") }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.imageGenerationQueue.description") }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div class="flex items-center justify-between">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">
              {{ t("admin.settings.imageGenerationQueue.enabled") }}
            </label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.imageGenerationQueue.enabledHint") }}
            </p>
          </div>
          <Toggle v-model="imageGenerationQueueEnabled" />
        </div>
        <div
          class="grid gap-4 border-t border-gray-100 pt-4 dark:border-dark-700 sm:grid-cols-2"
        >
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t("admin.settings.imageGenerationQueue.maxActiveJobs") }}
            </label>
            <input
              v-model.number="imageGenerationMaxActiveJobs"
              type="number"
              min="1"
              max="1000"
              class="input w-32"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.imageGenerationQueue.maxActiveJobsHint") }}
            </p>
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t("admin.settings.imageGenerationQueue.maxQueuedJobs") }}
            </label>
            <input
              v-model.number="imageGenerationMaxQueuedJobs"
              type="number"
              min="0"
              max="100000"
              class="input w-32"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t("admin.settings.imageGenerationQueue.maxQueuedJobsHint") }}
            </p>
          </div>
        </div>
        <p class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300">
          {{ t("admin.settings.imageGenerationQueue.safetyHint") }}
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";
import Toggle from "@/components/common/Toggle.vue";

const { t } = useI18n();

const dedicatedImageEnabled = defineModel<boolean>("dedicatedImageEnabled", { required: true });
const dedicatedImageWorkerEnabled = defineModel<boolean>("dedicatedImageWorkerEnabled", { required: true });
const dedicatedImageCodexBridgeEnabled = defineModel<boolean>("dedicatedImageCodexBridgeEnabled", { required: true });
const dedicatedImageFallbackToGeneral = defineModel<boolean>("dedicatedImageFallbackToGeneral", { required: true });
const imageGenerationQueueEnabled = defineModel<boolean>("imageGenerationQueueEnabled", { required: true });
const imageGenerationMaxActiveJobs = defineModel<number>("imageGenerationMaxActiveJobs", { required: true });
const imageGenerationMaxQueuedJobs = defineModel<number>("imageGenerationMaxQueuedJobs", { required: true });
</script>
