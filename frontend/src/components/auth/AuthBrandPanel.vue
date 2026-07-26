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
  padding: 48px 56px;
  color: #102d32;
  background: #dff5f6;
}

.auth-brand-shape {
  position: absolute;
  z-index: 0;
  pointer-events: none;
}

.auth-brand-shape--top {
  top: -100px;
  right: -110px;
  width: 390px;
  height: 390px;
  border-radius: 45% 55% 62% 38%;
  background: rgba(111, 215, 217, 0.42);
  transform: rotate(18deg);
  animation: auth-brand-drift 13s ease-in-out infinite alternate;
}

.auth-brand-shape--bottom {
  left: -86px;
  bottom: -120px;
  width: 260px;
  height: 260px;
  border-radius: 50%;
  background: rgba(255, 192, 162, 0.38);
  animation: auth-brand-peach-drift 11s ease-in-out infinite alternate;
}

.auth-brand-identity {
  position: relative;
  z-index: 2;
  display: inline-flex;
  align-items: center;
  gap: 12px;
}

.auth-brand-logo {
  width: 48px;
  height: 48px;
  flex: 0 0 48px;
  border-radius: 15px;
  object-fit: cover;
  box-shadow: 0 9px 22px rgba(18, 139, 135, 0.2);
  animation: auth-brand-float 4.8s ease-in-out infinite;
  transition: transform 280ms ease, box-shadow 280ms ease;
}

.auth-brand-identity:hover .auth-brand-logo {
  transform: translateY(-3px) rotate(-2deg) scale(1.04);
  box-shadow: 0 14px 28px rgba(18, 139, 135, 0.25);
}

.auth-brand-name {
  overflow-wrap: anywhere;
  font-size: 20px;
  font-weight: 850;
}

.auth-brand-copy {
  position: relative;
  z-index: 2;
  max-width: 520px;
  margin-top: 88px;
}

.auth-brand-kicker {
  margin: 0;
  color: #108e87;
  font-size: 13px;
  font-weight: 900;
  letter-spacing: 0;
}

.auth-brand-headline {
  max-width: 480px;
  margin: 14px 0 16px;
  white-space: pre-line;
  font-size: 52px;
  font-weight: 800;
  line-height: 1.12;
  letter-spacing: 0;
}

.auth-brand-description {
  max-width: 500px;
  margin: 0;
  color: #5f7f85;
  font-size: 16px;
  line-height: 1.85;
}

.auth-brand-facts {
  position: relative;
  z-index: 2;
  display: grid;
  gap: 14px;
  margin: 30px 0 0;
}

.auth-brand-fact {
  display: grid;
  grid-template-columns: 38px minmax(0, 1fr);
  align-items: center;
  gap: 12px;
  transition: transform 250ms ease;
}

.auth-brand-fact:hover {
  transform: translateX(5px);
}

.auth-brand-fact-number {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border-radius: 12px;
  color: #315b60;
  background: rgba(255, 255, 255, 0.78);
  box-shadow: 0 6px 16px rgba(39, 105, 110, 0.07);
  font-size: 14px;
  font-style: normal;
}

.auth-brand-fact dd {
  display: grid;
  gap: 4px;
  margin: 0;
}

.auth-brand-fact strong {
  color: #294f55;
  font-size: 14px;
}

.auth-brand-fact span {
  color: #4f7076;
  font-size: 13px;
  line-height: 1.5;
}

.auth-brand-route {
  position: absolute;
  z-index: 2;
  right: 34px;
  bottom: 30px;
  width: 150px;
  height: 105px;
}

.auth-brand-route-line {
  position: absolute;
  inset: 18px 12px 12px 8px;
  border: 2px dashed rgba(17, 150, 142, 0.37);
  border-radius: 50%;
  transform: rotate(-14deg);
  animation: auth-brand-route-breathe 6s ease-in-out infinite;
}

.auth-brand-route-node {
  position: absolute;
  z-index: 1;
  display: block;
  border: 5px solid rgba(255, 255, 255, 0.76);
  border-radius: 50%;
  box-shadow: 0 8px 18px rgba(24, 120, 119, 0.13);
}

.auth-brand-route-node--one {
  top: 33px;
  left: 2px;
  width: 39px;
  height: 39px;
  background: #7bd9d5;
}

.auth-brand-route-node--two {
  top: 5px;
  right: 11px;
  width: 29px;
  height: 29px;
  background: #ffd45a;
  animation: auth-brand-node-float 3.6s ease-in-out infinite alternate;
}

.auth-brand-route-node--three {
  right: 1px;
  bottom: 5px;
  width: 25px;
  height: 25px;
  background: #ffb49f;
  animation: auth-brand-node-float 4.2s 0.7s ease-in-out infinite alternate-reverse;
}

@keyframes auth-brand-float {
  50% {
    transform: translateY(-8px);
  }
}

@keyframes auth-brand-drift {
  to {
    transform: translate(-24px, 26px) rotate(29deg) scale(1.04);
    border-radius: 58% 42% 40% 60%;
  }
}

@keyframes auth-brand-peach-drift {
  to {
    transform: translate(30px, -20px) scale(1.08);
  }
}

@keyframes auth-brand-route-breathe {
  0%,
  100% {
    opacity: 0.64;
    transform: rotate(-14deg) scale(0.96);
  }

  50% {
    opacity: 1;
    transform: rotate(-7deg) scale(1.04);
  }
}

@keyframes auth-brand-node-float {
  to {
    transform: translateY(-9px) translateX(3px);
  }
}

@media (max-height: 720px) and (min-width: 901px) {
  .auth-brand-copy {
    margin-top: 46px;
  }
}

@media (max-width: 900px) {
  .auth-brand-panel {
    min-height: 230px;
    padding: 25px 27px;
  }

  .auth-brand-logo {
    width: 42px;
    height: 42px;
    flex-basis: 42px;
  }

  .auth-brand-name {
    font-size: 15px;
  }

  .auth-brand-copy {
    margin-top: 31px;
  }

  .auth-brand-headline {
    margin: 9px 0;
    font-size: 29px;
  }

  .auth-brand-description {
    max-width: 88%;
    font-size: 12px;
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
