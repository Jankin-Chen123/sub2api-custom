<script setup lang="ts">
import { useI18n } from 'vue-i18n'

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
      <p class="auth-brand-kicker">{{ t('auth.brand.kicker') }}</p>
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
  isolation: isolate;
  display: flex;
  min-height: 100vh;
  flex-direction: column;
  overflow: hidden;
  padding: clamp(36px, 4vw, 72px);
  background: #ddf4f5;
  color: #073f46;
  letter-spacing: 0;
}

.auth-brand-shape {
  position: absolute;
  z-index: -1;
  pointer-events: none;
  animation: auth-brand-drift 13s ease-in-out infinite alternate;
}

.auth-brand-shape--top {
  top: 0;
  right: 0;
  width: 42%;
  height: 28%;
  border-radius: 0 0 0 160px;
  background: #b8e6e7;
}

.auth-brand-shape--bottom {
  right: 0;
  bottom: 0;
  width: 54%;
  height: 24%;
  border-radius: 0 120px 0 0;
  background: #b3e1dc;
  animation-direction: alternate-reverse;
}

.auth-brand-identity {
  display: inline-flex;
  align-items: center;
  gap: 14px;
}

.auth-brand-logo {
  width: 64px;
  height: 64px;
  flex: 0 0 64px;
  border-radius: 8px;
  object-fit: cover;
  animation: auth-brand-float 5s ease-in-out infinite;
}

.auth-brand-name {
  overflow-wrap: anywhere;
  font-size: 1.45rem;
  font-weight: 800;
  line-height: 1.2;
}

.auth-brand-copy {
  max-width: 680px;
  margin-top: clamp(68px, 9vh, 128px);
}

.auth-brand-kicker {
  margin: 0;
  color: #0d6870;
  font-size: 0.78rem;
  font-weight: 800;
  line-height: 1.35;
}

.auth-brand-headline {
  margin: 14px 0 0;
  white-space: pre-line;
  font-size: 4.25rem;
  font-weight: 800;
  line-height: 1.02;
}

.auth-brand-description {
  max-width: 560px;
  margin: 24px 0 0;
  color: #155960;
  font-size: 1.1rem;
  line-height: 1.6;
}

.auth-brand-facts {
  display: grid;
  max-width: 620px;
  gap: 22px;
  margin: clamp(50px, 8vh, 96px) 0 0;
}

.auth-brand-fact {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: start;
  gap: 14px;
}

.auth-brand-fact-number {
  display: grid;
  width: 42px;
  height: 32px;
  place-items: center;
  border-radius: 8px;
  background: #0b6670;
  color: #fff;
  font-size: 0.78rem;
  font-weight: 800;
  line-height: 1;
}

.auth-brand-fact dd {
  display: grid;
  gap: 4px;
  margin: 0;
}

.auth-brand-fact strong {
  font-size: 1rem;
  line-height: 1.35;
}

.auth-brand-fact span {
  color: #23636a;
  font-size: 0.92rem;
  line-height: 1.45;
}

.auth-brand-route {
  position: relative;
  display: flex;
  width: 186px;
  height: 34px;
  align-items: center;
  justify-content: space-between;
  margin-top: auto;
}

.auth-brand-route-line {
  position: absolute;
  right: 17px;
  left: 17px;
  border-top: 2px dashed #4e9698;
}

.auth-brand-route-node {
  z-index: 1;
  width: 34px;
  height: 34px;
  border: 6px solid #fff;
  border-radius: 50%;
  background: #0b6670;
  box-shadow: 0 0 0 1px #4e9698;
  animation: auth-brand-pulse 3.8s ease-in-out infinite;
}

.auth-brand-route-node--two {
  animation-delay: 0.35s;
}

.auth-brand-route-node--three {
  animation-delay: 0.7s;
}

@keyframes auth-brand-float {
  50% {
    transform: translateY(-8px);
  }
}

@keyframes auth-brand-drift {
  to {
    transform: translate(12px, -8px);
  }
}

@keyframes auth-brand-pulse {
  50% {
    transform: scale(1.1);
  }
}

@media (max-width: 900px) {
  .auth-brand-panel {
    min-height: 280px;
    padding: 28px;
  }

  .auth-brand-name {
    font-size: 1.2rem;
  }

  .auth-brand-copy {
    margin-top: 40px;
  }

  .auth-brand-headline {
    font-size: 2.8rem;
  }

  .auth-brand-facts,
  .auth-brand-route {
    display: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-brand-shape,
  .auth-brand-logo,
  .auth-brand-route-node {
    animation: none;
  }
}
</style>
