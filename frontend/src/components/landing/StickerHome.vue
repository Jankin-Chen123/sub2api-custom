<template>
  <div class="sticker-home" data-sticker-home>
    <div class="sticker-cover">
      <nav class="sticker-nav sticker-shell" :aria-label="t('home.sticker.navigation')">
        <RouterLink to="/home" class="sticker-brand">
          <img :src="logo" alt="" width="44" height="44" />
          <span>{{ siteName }}</span>
        </RouterLink>
        <div class="sticker-nav-actions">
          <RouterLink v-if="showModelPlaza" to="/model-plaza" class="sticker-nav-plaza" :aria-label="t('nav.modelPlaza')">
            <Icon name="grid" size="sm" />
            <span>{{ t('nav.modelPlaza') }}</span>
          </RouterLink>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="sticker-nav-doc">{{ t('home.sticker.docs') }}</a>
          <RouterLink :to="entryPath" class="sticker-button sticker-button-ink sticker-button-small">{{ entryLabel }}</RouterLink>
        </div>
      </nav>

      <main class="sticker-hero sticker-shell">
        <h1 class="sticker-title" :class="{ 'sticker-title-brand': isBrandTitle }">
          <template v-if="isBrandTitle"><span>爱白嫖</span><span>公益站</span></template>
          <template v-else>{{ siteName }}</template>
        </h1>
        <img class="sticker-mascot" src="/sticker/mascot.webp" width="560" height="560" :alt="t('home.sticker.mascot')" fetchpriority="high" />
        <div class="sticker-intro">
          <p>{{ t('home.sticker.tagline') }}</p>
          <div class="sticker-actions">
            <RouterLink :to="entryPath" class="sticker-button sticker-button-ink">
              {{ signedIn ? t('home.goToDashboard') : t('home.sticker.start') }}
              <Icon name="arrowRight" size="md" aria-hidden="true" />
            </RouterLink>
            <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="sticker-button">{{ t('home.sticker.guide') }}</a>
          </div>
        </div>
      </main>
    </div>

    <section class="sticker-benefits sticker-shell" aria-labelledby="sticker-benefits-title">
      <h2 id="sticker-benefits-title" class="sticker-section-title">{{ t('home.sticker.benefits') }}</h2>
      <div class="sticker-benefit-grid">
        <article v-for="(feature, index) in features" :key="feature" class="sticker-benefit">
          <div class="sticker-feature-art" :style="{ '--sticker-position': index * 50 + '% center' }" aria-hidden="true"></div>
          <div>
            <h3>{{ t(`home.sticker.${feature}.title`) }}</h3>
            <p>{{ t(`home.sticker.${feature}.description`) }}</p>
          </div>
        </article>
      </div>
    </section>

    <section class="sticker-setup sticker-shell" aria-labelledby="sticker-setup-title">
      <h2 id="sticker-setup-title">{{ t('home.sticker.setup') }}</h2>
      <ol>
        <li><Icon name="user" size="md" />{{ t('home.sticker.signIn') }}</li>
        <li><Icon name="key" size="md" />{{ t('dashboard.createApiKey') }}</li>
        <li><Icon name="bolt" size="md" />{{ t('home.sticker.request') }}</li>
      </ol>
      <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="sticker-button sticker-button-small">
        <Icon name="book" size="sm" /> {{ t('home.sticker.docs') }} <Icon name="chevronRight" size="sm" />
      </a>
    </section>

    <footer class="sticker-footer sticker-shell">
      <RouterLink to="/home" class="sticker-brand"><img :src="logo" alt="" width="34" height="34" />{{ siteName }}</RouterLink>
      <span>{{ t('home.sticker.footer') }}</span>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  siteName: string
  siteLogo: string
  docUrl: string
  signedIn: boolean
  dashboardPath: string
  showModelPlaza: boolean
}>()
const { t } = useI18n()
const logo = computed(() => props.siteLogo || '/brand-icon.png')
const entryPath = computed(() => props.signedIn ? props.dashboardPath : '/login')
const entryLabel = computed(() => t(props.signedIn ? 'home.dashboard' : 'home.login'))
const isBrandTitle = computed(() => props.siteName === '爱白嫖公益站')
const features = ['pricing', 'routing', 'verification'] as const
</script>

<style scoped src="./sticker-home.css"></style>
