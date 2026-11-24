<template>
  <BaseDialog :show="failedMethod !== null" :title="t('payment.channelUnavailable.title')" width="narrow" close-on-click-outside @close="emit('close')">
    <div class="recharge-alternatives">
      <p v-if="failedMethod" class="payment-state">{{ t('payment.channelUnavailable.status', { method: t(`payment.methods.${failedMethod}`) }) }}</p>
      <h2>{{ t('payment.channelUnavailable.title') }}</h2>
      <div class="payment-options">
        <a class="payment-option shop-option" href="/custom/62a5a747dabbb404">
          <Icon name="gift" size="md" aria-hidden="true" />
          <span><strong>{{ t('payment.channelUnavailable.shop') }}</strong><small>{{ t('payment.channelUnavailable.shopHint') }}</small></span>
        </a>
        <button v-if="alternative" type="button" class="payment-option switch-option" @click="emit('switch-method')">
          <img :src="alternative.method === 'wxpay' ? wxpayIcon : alipayIcon" alt="" />
          <span><strong>{{ t('payment.channelUnavailable.switchTo', { method: t(`payment.methods.${alternative.method}`) }) }}</strong><small>{{ t('payment.channelUnavailable.continuePayment', { amount: alternative.amount }) }}</small></span>
        </button>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import alipayIcon from '@/assets/icons/alipay.svg'
import wxpayIcon from '@/assets/icons/wxpay.svg'

type CheckoutMethod = 'wxpay' | 'alipay'
defineProps<{ failedMethod: CheckoutMethod | null; alternative: { method: CheckoutMethod; amount: string } | null }>()
const emit = defineEmits<{ close: []; 'switch-method': [] }>()
const { t } = useI18n()
</script>

<style scoped>
:global(.modal-overlay:has(.recharge-alternatives)) {
  background: rgb(26 30 32 / 32%);
  backdrop-filter: blur(3px);
}
:global(.modal-content:has(.recharge-alternatives)) {
  position: relative;
  max-width: 416px;
  overflow: auto;
  border: 1px solid #24292d14;
  border-radius: 21px;
  background: white;
  box-shadow: 0 20px 65px #1820262b, 0 4px 12px #1820260a;
}
:global(.modal-content:has(.recharge-alternatives) .modal-header) { padding: 0; border: 0; background: transparent; }
:global(.modal-content:has(.recharge-alternatives) .modal-title) {
  position: absolute;
  clip-path: inset(50%);
  width: 1px;
  height: 1px;
  overflow: hidden;
}
:global(.modal-content:has(.recharge-alternatives) .modal-header button) {
  position: absolute;
  z-index: 1;
  right: 9px;
  top: 9px;
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  margin: 0;
  border-radius: 50%;
  color: #7d8281;
}
:global(.modal-content:has(.recharge-alternatives) .modal-body) { padding: 0; }
.recharge-alternatives { padding: 27px 24px 23px; }
.payment-state { display: flex; align-items: center; gap: 7px; margin: 0 38px 9px 0; font-size: 13px; color: #695d4c; }
.payment-state::before { content: ''; width: 7px; height: 7px; border-radius: 50%; background: #c78a48; }
h2 { color: #202526; font-size: 23px; line-height: 1.35; letter-spacing: -.035em; font-weight: 680; }
.payment-options { display: grid; gap: 8px; margin-top: 19px; }
.payment-option { display: flex; align-items: center; gap: 18px; width: 100%; padding: 12px 17px; border-radius: 12px; text-align: left; text-decoration: none; }
.payment-option :is(img, svg) { width: 23px; height: 23px; flex: none; }
.payment-option strong { display: block; color: #202a26; font-size: 15px; font-weight: 650; }
.payment-option small { display: block; margin-top: 2px; color: #5c6661; font-size: 12px; line-height: 1.5; }
.shop-option { border: 1px solid #c6d9c9; background: #eef5ef; color: #3b674d; }
.switch-option { border: 1px solid #dfe3e2; background: white; }
@media (hover: hover) and (pointer: fine) {
  .shop-option:hover { background: #e4efe5; }
  .switch-option:hover { background: #f6f8f7; }
}
@media (max-width: 365px) { .recharge-alternatives { padding-inline: 19px; } }
</style>
