<template>
  <div class="space-y-6">
    <!-- Async image object storage -->
    <div class="card p-6">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t("admin.backup.imageStorage.title") }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t("admin.backup.imageStorage.description") }}
          </p>
        </div>
        <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="imageStorageForm.enabled" type="checkbox" />
          <span>{{ t("admin.backup.imageStorage.enabled") }}</span>
        </label>
      </div>

      <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input v-model="imageStorageForm.reuse_backup_s3" type="checkbox" />
        <span>{{ t("admin.backup.imageStorage.reuseBackupS3") }}</span>
      </label>

      <label class="mt-3 inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input v-model="imageStorageForm.allow_base64_responses" type="checkbox" />
        <span>{{ t("admin.backup.imageStorage.allowBase64Responses") }}</span>
      </label>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t("admin.backup.imageStorage.allowBase64ResponsesHint") }}
      </p>

      <div class="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2">
        <div>
          <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
            {{ t("admin.backup.imageStorage.bucket") }}
          </label>
          <input
            v-model="imageStorageForm.bucket"
            class="input w-full"
            :placeholder="imageStorageForm.reuse_backup_s3 ? t('admin.backup.imageStorage.bucketInherited') : ''"
          />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
            {{ t("admin.backup.imageStorage.prefix") }}
          </label>
          <input v-model="imageStorageForm.prefix" class="input w-full" placeholder="images/" />
        </div>

        <template v-if="!imageStorageForm.reuse_backup_s3">
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t("admin.backup.s3.endpoint") }}
            </label>
            <input
              v-model="imageStorageForm.endpoint"
              class="input w-full"
              placeholder="https://<account_id>.r2.cloudflarestorage.com"
            />
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t("admin.backup.s3.region") }}
            </label>
            <input v-model="imageStorageForm.region" class="input w-full" placeholder="auto" />
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t("admin.backup.s3.accessKeyId") }}
            </label>
            <input v-model="imageStorageForm.access_key_id" class="input w-full" />
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t("admin.backup.s3.secretAccessKey") }}
            </label>
            <input
              v-model="imageStorageForm.secret_access_key"
              type="password"
              class="input w-full"
              :placeholder="imageStorageSecretConfigured ? t('admin.backup.s3.secretConfigured') : ''"
            />
          </div>
          <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 md:col-span-2">
            <input v-model="imageStorageForm.force_path_style" type="checkbox" />
            <span>{{ t("admin.backup.s3.forcePathStyle") }}</span>
          </label>
        </template>

        <div>
          <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
            {{ t("admin.backup.imageStorage.publicBaseUrl") }}
          </label>
          <input
            v-model="imageStorageForm.public_base_url"
            class="input w-full"
            :placeholder="t('admin.backup.imageStorage.publicBaseUrlPlaceholder')"
          />
        </div>
        <div>
          <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
            {{ t("admin.backup.imageStorage.presignExpiryHours") }}
          </label>
          <input
            v-model.number="imageStorageForm.presign_expiry_hours"
            type="number"
            min="1"
            class="input w-full"
          />
        </div>
      </div>

      <div class="mt-4 flex flex-wrap gap-2">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="testingImageStorage"
          @click="testImageStorage"
        >
          {{ testingImageStorage ? t("common.loading") : t("admin.backup.s3.testConnection") }}
        </button>
        <button
          type="button"
          class="btn btn-primary btn-sm"
          :disabled="savingImageStorage"
          @click="saveImageStorageConfig"
        >
          {{ savingImageStorage ? t("common.loading") : t("common.save") }}
        </button>
      </div>
    </div>

    <div class="card p-6">
      <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t("admin.settings.imageWorkbenchAnnouncements.title") }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t("admin.settings.imageWorkbenchAnnouncements.description") }}
          </p>
        </div>
        <button type="button" class="btn btn-secondary btn-sm" @click="addAnnouncement">
          {{ t("admin.settings.imageWorkbenchAnnouncements.add") }}
        </button>
      </div>

      <div class="mb-5 max-w-xs">
        <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t("admin.settings.imageWorkbenchAnnouncements.interval") }}
        </label>
        <input
          v-model.number="intervalDraft"
          type="number"
          min="1"
          max="3600"
          class="input w-full"
          @change="commitInterval"
        />
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t("admin.settings.imageWorkbenchAnnouncements.intervalHint") }}
        </p>
      </div>

      <div v-if="announcements.length === 0" class="rounded-lg border border-dashed border-gray-300 px-4 py-6 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
        {{ t("admin.settings.imageWorkbenchAnnouncements.empty") }}
      </div>
      <div v-else class="space-y-3">
        <div
          v-for="(announcement, index) in announcements"
          :key="announcement.id"
          class="flex items-start gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-700"
        >
          <span class="mt-2 w-6 shrink-0 text-center text-xs font-semibold text-gray-400">{{ index + 1 }}</span>
          <textarea
            :value="announcement.content"
            class="input min-h-20 flex-1 resize-y"
            maxlength="1000"
            :placeholder="t('admin.settings.imageWorkbenchAnnouncements.contentPlaceholder')"
            @input="updateAnnouncement(index, ($event.target as HTMLTextAreaElement).value)"
          />
          <button type="button" class="btn btn-secondary btn-sm shrink-0 text-red-600 hover:text-red-700 dark:text-red-400" @click="removeAnnouncement(index)">
            {{ t("admin.settings.imageWorkbenchAnnouncements.remove") }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

import { adminAPI } from "@/api";
import type { ImageStorageConfig } from "@/api/admin/backup";
import { useAppStore } from "@/stores";
import { isStepUpCancelled, useStepUp } from "@/composables/useStepUp";
import type { ImageWorkbenchAnnouncement } from "@/types";

const props = withDefaults(defineProps<{
  announcements?: ImageWorkbenchAnnouncement[];
  intervalSeconds?: number;
}>(), {
  announcements: () => [],
  intervalSeconds: 5,
});
const emit = defineEmits<{
  "update:announcements": [value: ImageWorkbenchAnnouncement[]];
  "update:intervalSeconds": [value: number];
}>();

const { t } = useI18n();
const appStore = useAppStore();
const imageStorageStepUp = useStepUp();

const imageStorageForm = ref<ImageStorageConfig>({
  enabled: false,
  reuse_backup_s3: true,
  allow_base64_responses: false,
  bucket: "",
  prefix: "images/",
  public_base_url: "",
  presign_expiry_hours: 24,
  max_download_bytes: 33554432,
  endpoint: "",
  region: "auto",
  access_key_id: "",
  secret_access_key: "",
  force_path_style: false,
});
const imageStorageSecretConfigured = ref(false);
const savingImageStorage = ref(false);
const testingImageStorage = ref(false);
const intervalDraft = ref(props.intervalSeconds);

watch(() => props.intervalSeconds, (value) => {
  intervalDraft.value = value;
});

function updateAnnouncement(index: number, content: string) {
  emit("update:announcements", props.announcements.map((item, itemIndex) => (
    itemIndex === index ? { ...item, content } : item
  )));
}

function addAnnouncement() {
  const id = typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `announcement-${Date.now()}`;
  emit("update:announcements", [...props.announcements, { id, content: "" }]);
}

function removeAnnouncement(index: number) {
  emit("update:announcements", props.announcements.filter((_, itemIndex) => itemIndex !== index));
}

function commitInterval() {
  const value = Math.min(3600, Math.max(1, Math.floor(Number(intervalDraft.value) || 5)));
  intervalDraft.value = value;
  emit("update:intervalSeconds", value);
}

async function loadImageStorageConfig() {
  try {
    const { config, secret_configured } = await adminAPI.backup.getImageStorageConfig();
    imageStorageForm.value = {
      ...config,
      prefix: config.prefix || "images/",
      region: config.region || "auto",
      secret_access_key: "",
    };
    imageStorageSecretConfigured.value = secret_configured;
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t("errors.networkError"));
  }
}

async function saveImageStorageConfig() {
  savingImageStorage.value = true;
  try {
    await imageStorageStepUp.run(() =>
      adminAPI.backup.updateImageStorageConfig(imageStorageForm.value),
    );
    appStore.showSuccess(t("admin.backup.imageStorage.saved"));
    await loadImageStorageConfig();
  } catch (error) {
    if (isStepUpCancelled(error)) return;
    appStore.showError((error as { message?: string })?.message || t("errors.networkError"));
  } finally {
    savingImageStorage.value = false;
  }
}

async function testImageStorage() {
  testingImageStorage.value = true;
  try {
    const result = await adminAPI.backup.testImageStorageConnection(imageStorageForm.value);
    if (result.ok) {
      appStore.showSuccess(result.message || t("admin.backup.s3.testSuccess"));
    } else {
      appStore.showError(result.message || t("admin.backup.s3.testFailed"));
    }
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t("errors.networkError"));
  } finally {
    testingImageStorage.value = false;
  }
}

onMounted(() => {
  void loadImageStorageConfig();
});
</script>
