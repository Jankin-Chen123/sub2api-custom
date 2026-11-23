<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import CraftSparks from '@/components/craft/CraftSparks.vue'

defineProps<{
  siteName: string
  siteLogo: string
}>()

const { t } = useI18n()
</script>

<template>
  <aside class="auth-brand-panel">
    <div class="auth-brand-shape auth-brand-shape--top" aria-hidden="true" />
    <div class="auth-brand-shape auth-brand-shape--bottom" aria-hidden="true" />

    <header class="auth-brand-identity">
      <img class="auth-brand-logo" :src="siteLogo" :alt="siteName" />
      <span class="auth-brand-name">{{ siteName }}</span>
    </header>

    <section class="auth-brand-copy">
      <p class="auth-brand-kicker">
        <span>{{ t('auth.brand.kicker') }}</span>
        <CraftSparks variant="sticker" size="sm" class="auth-brand-sparks" />
      </p>
      <h1 class="auth-brand-headline">{{ t('auth.brand.headline') }}</h1>
      <p class="auth-brand-description">{{ t('auth.brand.description') }}</p>
    </section>

    <dl class="auth-brand-facts">
      <div class="auth-brand-fact">
        <dt class="auth-brand-fact-number">01</dt>
        <dd>
          <strong>{{ t('auth.brand.billingTitle') }}</strong>
          <span>{{ t('auth.brand.billingDescription') }}</span>
        </dd>
      </div>
      <div class="auth-brand-fact">
        <dt class="auth-brand-fact-number">02</dt>
        <dd>
          <strong>{{ t('auth.brand.routingTitle') }}</strong>
          <span>{{ t('auth.brand.routingDescription') }}</span>
        </dd>
      </div>
      <div class="auth-brand-fact">
        <dt class="auth-brand-fact-number">03</dt>
        <dd>
          <strong>{{ t('auth.brand.validationTitle') }}</strong>
          <span>{{ t('auth.brand.validationDescription') }}</span>
        </dd>
      </div>
    </dl>

    <div class="auth-brand-route" aria-hidden="true">
      <span class="auth-brand-route-line" />
      <span class="auth-brand-route-node auth-brand-route-node--one" />
      <span class="auth-brand-route-node auth-brand-route-node--two" />
      <span class="auth-brand-route-node auth-brand-route-node--three" />
    </div>
  </aside>
</template>

<style scoped>
.auth-brand-panel {
  position: relative;
  display: flex;
  min-height: 100%;
  flex-direction: column;
  overflow: hidden;
  padding: 46px 52px;
  color: #27221d;
  background-color: #88d2fb;
  background-image:
    radial-gradient(circle at 22% 18%, rgba(255, 255, 255, 0.34) 0%, transparent 45%),
    radial-gradient(circle at 82% 84%, rgba(255, 255, 255, 0.25) 0%, transparent 42%),
    url('/sticker/paper.webp');
  background-size: 100% 100%, 100% 100%, 160px;
  box-shadow: inset -10px 0 22px -14px rgba(68, 50, 28, 0.14);
}

.auth-brand-shape {
  position: absolute;
  z-index: 0;
  pointer-events: none;
}

.auth-brand-shape--top {
  top: -110px;
  right: -110px;
  width: 360px;
  height: 360px;
  border-radius: 45% 55% 62% 38%;
  background: rgba(255, 250, 240, 0.26);
  transform: rotate(18deg);
  animation: auth-brand-drift 13s ease-in-out infinite alternate;
}

.auth-brand-shape--bottom {
  left: -80px;
  bottom: -110px;
  width: 260px;
  height: 260px;
  border-radius: 50%;
  background: rgba(255, 198, 166, 0.46);
  animation: auth-brand-peach-drift 11s ease-in-out infinite alternate;
}

.auth-brand-identity {
  position: relative;
  z-index: 2;
  display: inline-flex;
  width: fit-content;
  max-width: 100%;
  align-items: center;
  gap: 11px;
  padding: 7px 16px 7px 8px;
  border-radius: 999px;
  border: 1.5px solid rgba(255, 255, 255, 0.9);
  background: rgba(255, 250, 240, 0.92);
  box-shadow: 0 6px 16px rgba(42, 86, 122, 0.13);
}

.auth-brand-logo {
  width: 40px;
  height: 40px;
  flex: 0 0 40px;
  border-radius: 12px;
  object-fit: cover;
  box-shadow: 0 4px 10px rgba(39, 34, 29, 0.14);
  animation: auth-brand-float 4.8s ease-in-out infinite;
  transition: transform 280ms ease, box-shadow 280ms ease;
}

.auth-brand-identity:hover .auth-brand-logo {
  transform: translateY(-2px) rotate(-3deg) scale(1.04);
}

.auth-brand-name {
  overflow-wrap: anywhere;
  color: #27221d;
  font-size: 18px;
  font-weight: 900;
}

.auth-brand-copy {
  position: relative;
  z-index: 2;
  max-width: 520px;
  margin-top: 44px;
}

.auth-brand-kicker {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  padding: 4px 12px;
  border-radius: 999px;
  background: rgba(255, 250, 240, 0.88);
  color: #266c9e;
  font-size: 12px;
  font-weight: 850;
  letter-spacing: 0.02em;
  box-shadow: 0 3px 8px rgba(42, 86, 122, 0.1);
}

.auth-brand-sparks {
  margin-top: -2px;
}

.auth-brand-headline {
  max-width: 480px;
  margin: 16px 0 14px;
  white-space: pre-line;
  color: #241f1b;
  font-size: 46px;
  font-weight: 900;
  line-height: 1.14;
  letter-spacing: -0.02em;
  text-shadow:
    0 2px 0 rgba(255, 253, 246, 0.92),
    0 0 14px rgba(255, 253, 246, 0.65);
}

.auth-brand-description {
  max-width: 480px;
  margin: 0;
  color: #293c4a;
  font-size: 15px;
  font-weight: 600;
  line-height: 1.75;
}

.auth-brand-facts {
  position: relative;
  z-index: 2;
  display: grid;
  gap: 12px;
  margin: 24px 0 46px;
}

.auth-brand-fact {
  display: grid;
  grid-template-columns: 38px minmax(0, 1fr);
  align-items: center;
  gap: 12px;
  padding: 11px 14px;
  border-radius: 18px;
  border: 1.5px solid rgba(255, 255, 255, 0.85);
  background: rgba(255, 250, 240, 0.88);
  box-shadow: 0 5px 14px rgba(45, 92, 130, 0.11);
  transition: transform 220ms ease, box-shadow 220ms ease;
}

.auth-brand-fact:hover {
  transform: translateX(4px);
  box-shadow: 0 8px 18px rgba(45, 92, 130, 0.16);
}

.auth-brand-fact-number {
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border-radius: 12px;
  color: #27221d;
  background: #ffe3d3;
  border: 1px solid #f7c4a9;
  font-size: 13px;
  font-weight: 900;
  font-style: normal;
}

.auth-brand-fact dd {
  display: grid;
  gap: 2px;
  margin: 0;
}

.auth-brand-fact strong {
  color: #27221d;
  font-size: 14px;
  font-weight: 850;
}

.auth-brand-fact span {
  color: #655748;
  font-size: 12.5px;
  line-height: 1.45;
}

.auth-brand-route {
  position: absolute;
  z-index: 1;
  right: 24px;
  bottom: 12px;
  width: 140px;
  height: 98px;
  opacity: 0.9;
  transform: scale(0.86);
  transform-origin: bottom right;
}

.auth-brand-route-line {
  position: absolute;
  inset: 16px 12px 12px 8px;
  border: 2px dashed rgba(255, 250, 240, 0.85);
  border-radius: 50%;
  transform: rotate(-14deg);
  animation: auth-brand-route-breathe 6s ease-in-out infinite;
}

.auth-brand-route-node {
  position: absolute;
  z-index: 1;
  display: block;
  border: 4px solid #fffdf7;
  border-radius: 50%;
  box-shadow: 0 6px 14px rgba(42, 86, 122, 0.16);
}

.auth-brand-route-node--one {
  top: 32px;
  left: 4px;
  width: 34px;
  height: 34px;
  background: #5cb9fa;
}

.auth-brand-route-node--two {
  top: 6px;
  right: 12px;
  width: 26px;
  height: 26px;
  background: #ffd45a;
  animation: auth-brand-node-float 3.6s ease-in-out infinite alternate;
}

.auth-brand-route-node--three {
  right: 4px;
  bottom: 6px;
  width: 22px;
  height: 22px;
  background: #ff9c70;
  animation: auth-brand-node-float 4.2s 0.7s ease-in-out infinite alternate-reverse;
}

@keyframes auth-brand-float {
  50% {
    transform: translateY(-5px);
  }
}

@keyframes auth-brand-drift {
  to {
    transform: translate(-20px, 22px) rotate(28deg) scale(1.04);
    border-radius: 58% 42% 40% 60%;
  }
}

@keyframes auth-brand-peach-drift {
  to {
    transform: translate(26px, -18px) scale(1.06);
  }
}

@keyframes auth-brand-route-breathe {
  0%,
  100% {
    opacity: 0.72;
    transform: rotate(-14deg) scale(0.96);
  }

  50% {
    opacity: 1;
    transform: rotate(-8deg) scale(1.04);
  }
}

@keyframes auth-brand-node-float {
  to {
    transform: translateY(-7px) translateX(2px);
  }
}

@media (max-height: 720px) and (min-width: 901px) {
  .auth-brand-copy {
    margin-top: 38px;
  }
}

@media (max-width: 900px) {
  .auth-brand-panel {
    min-height: 230px;
    padding: 24px 24px;
    box-shadow: none;
  }

  .auth-brand-logo {
    width: 36px;
    height: 36px;
    flex-basis: 36px;
  }

  .auth-brand-name {
    font-size: 15px;
  }

  .auth-brand-copy {
    margin-top: 24px;
  }

  .auth-brand-headline {
    margin: 10px 0 8px;
    font-size: 29px;
  }

  .auth-brand-description {
    max-width: 90%;
    font-size: 12.5px;
    line-height: 1.65;
  }

  .auth-brand-facts,
  .auth-brand-route {
    display: none;
  }
}

@media (max-width: 520px) {
  .auth-brand-headline {
    font-size: 25px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-brand-shape,
  .auth-brand-logo,
  .auth-brand-route-line,
  .auth-brand-route-node {
    animation: none;
  }
}
</style>
