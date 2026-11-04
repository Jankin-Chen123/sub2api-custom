<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiClient } from '@/api/client'
import AppLayout from '@/components/layout/AppLayout.vue'

const { t } = useI18n()
const frame = ref<HTMLIFrameElement | null>(null)

async function handleTicketRequest(event: MessageEvent) {
  if (event.origin !== window.location.origin || event.source !== frame.value?.contentWindow) return
  if (event.data?.type !== 'aibaipiao-docs:ticket-request' || typeof event.data.id !== 'string' || event.data.id.length > 100) return
  const reply: Record<string, unknown> = { type: 'aibaipiao-docs:ticket-response', id: event.data.id }
  try {
    const { data } = await apiClient.post<{ ticket: string }>('/admin/integrations/docs-session')
    reply.ticket = data.ticket
  } catch (error) {
    const failure = error as { status?: number; message?: string }
    reply.error = failure.message || '暂时无法验证主站登录，请重试'
    reply.status = failure.status || 503
  }
  // Navigation may have removed/replaced the frame while the request ran.
  if (event.source === frame.value?.contentWindow) {
    frame.value.contentWindow?.postMessage(reply, window.location.origin)
  }
}

onMounted(() => window.addEventListener('message', handleTicketRequest))
onBeforeUnmount(() => window.removeEventListener('message', handleTicketRequest))
</script>

<template>
  <AppLayout>
    <iframe
      ref="frame"
      src="/docs/admin?embedded=1"
      :title="t('nav.documentation')"
      class="documentation-frame"
    ></iframe>
  </AppLayout>
</template>

<style scoped>
.documentation-frame {
  display: block;
  width: 100%;
  height: calc(100dvh - 8rem);
  min-height: 420px;
  border: 0;
  border-radius: 1rem;
  background: transparent;
}
</style>
