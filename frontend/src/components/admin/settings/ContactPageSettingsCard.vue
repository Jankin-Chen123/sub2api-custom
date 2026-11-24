<template>
  <!-- Contact Page Settings -->
  <div
    class="rounded-2xl border border-primary-200 bg-primary-50/40 p-5 dark:border-primary-900/50 dark:bg-primary-950/20"
    data-testid="contact-page-settings-card"
  >
    <div class="mb-4">
      <h3 class="font-semibold text-gray-900 dark:text-white">
        {{ t("admin.settings.site.contactPage.title") }}
      </h3>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t("admin.settings.site.contactPage.description") }}
      </p>
      <p
        class="mt-3 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300"
      >
        {{ t("admin.settings.site.contactPage.qqExpiryWarning") }}
      </p>
    </div>

    <div class="grid grid-cols-1 gap-5 xl:grid-cols-2">
      <section class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
        <h4 class="mb-4 text-sm font-semibold text-gray-800 dark:text-gray-200">
          {{ t("admin.settings.site.contactPage.qqTitle") }}
        </h4>
        <div class="space-y-4">
          <div>
            <label class="input-label">
              {{ t("admin.settings.site.contactPage.qqGroupNumber") }}
            </label>
            <input
              v-model="qqGroupNumber"
              type="text"
              inputmode="numeric"
              class="input font-mono"
              data-testid="contact-page-qq-group-number"
              :placeholder="t('admin.settings.site.contactPage.qqGroupNumberPlaceholder')"
            />
          </div>
          <div>
            <label class="input-label">
              {{ t("admin.settings.site.contactPage.qqQrImage") }}
            </label>
            <ImageUpload
              v-model="qqQrImage"
              mode="image"
              size="lg"
              :upload-label="t('admin.settings.site.contactPage.replaceImage')"
              :remove-label="t('admin.settings.site.remove')"
              :hint="t('admin.settings.site.contactPage.imageHint')"
              :max-size="2 * 1024 * 1024"
            />
          </div>
        </div>
      </section>

      <section class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
        <h4 class="mb-4 text-sm font-semibold text-gray-800 dark:text-gray-200">
          {{ t("admin.settings.site.contactPage.telegramTitle") }}
        </h4>
        <div class="space-y-4">
          <div>
            <label class="input-label">
              {{ t("admin.settings.site.contactPage.telegramName") }}
            </label>
            <input
              v-model="telegramName"
              type="text"
              class="input"
              data-testid="contact-page-telegram-name"
              :placeholder="t('admin.settings.site.contactPage.telegramNamePlaceholder')"
            />
          </div>
          <div>
            <label class="input-label">
              {{ t("admin.settings.site.contactPage.telegramUrl") }}
            </label>
            <input
              v-model="telegramUrl"
              type="url"
              class="input font-mono text-sm"
              data-testid="contact-page-telegram-url"
              placeholder="https://t.me/ai_baipiao"
            />
          </div>
          <div>
            <label class="input-label">
              {{ t("admin.settings.site.contactPage.telegramQrImage") }}
            </label>
            <ImageUpload
              v-model="telegramQrImage"
              mode="image"
              size="lg"
              :upload-label="t('admin.settings.site.contactPage.replaceImage')"
              :remove-label="t('admin.settings.site.remove')"
              :hint="t('admin.settings.site.contactPage.imageHint')"
              :max-size="2 * 1024 * 1024"
            />
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";
import ImageUpload from "@/components/common/ImageUpload.vue";

const { t } = useI18n();

const qqGroupNumber = defineModel<string>("qqGroupNumber", { required: true });
const qqQrImage = defineModel<string>("qqQrImage", { required: true });
const telegramName = defineModel<string>("telegramName", { required: true });
const telegramUrl = defineModel<string>("telegramUrl", { required: true });
const telegramQrImage = defineModel<string>("telegramQrImage", { required: true });
</script>
