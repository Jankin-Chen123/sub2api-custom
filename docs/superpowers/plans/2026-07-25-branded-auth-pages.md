# Branded Authentication Pages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将登录、注册、忘记密码和重置密码四个认证页面改造成已确认的 C 方案“品牌画册”浅色界面，同时保留全部现有认证逻辑、真实 Cloudflare Turnstile 行为和移动端可用性。

**Architecture:** 认证页继续复用 `AuthLayout`，但把视觉职责拆成品牌面板、登录/注册模式切换和固定浅色主题作用域三个小模块。四个业务 View 只负责表单、状态和现有 API 行为；布局组件统一负责双栏结构、抽象装饰、响应式、动效、页脚与表单视觉规范。Turnstile 仍由现有 `TurnstileWidget` 按后端配置条件渲染，不创建任何伪造的人机验证界面。

**Tech Stack:** Vue 3 Composition API、TypeScript、Vue Router、Vue I18n、Pinia、Tailwind 现有原子类、Vue scoped CSS、Vitest、Vue Test Utils、Playwright 浏览器验收。

---

## 实施基线

- 工作目录：`D:\HuaweiMoveData\Users\23909\Desktop\mySub2api\sub2api-custom\frontend`
- 实施分支：`custom/frontend`
- 设计规格：`docs/superpowers/specs/2026-07-25-auth-pages-brand-design.md`
- 当前设计规格提交：`ac76e1415 docs: design branded authentication pages`
- 已知基线测试问题：完整 Vitest 套件中 `src/api/__tests__/admin.system.rollback.spec.ts` 有 2 个上游旧断言失败，原因是实现新增了 `{ timeout: 900000 }`。本任务不得新增其他失败，也不顺手修改该无关测试。
- 网站图标来源：继续使用后台公共设置中的 `site_logo`，经 `sanitizeUrl` 处理；页面只显示一处品牌图标，不把该图标复制为装饰元素。

## 文件职责

**新增文件**

- `src/composables/useAuthLightTheme.ts`：认证页面挂载期间移除根节点 `.dark`，离开最后一个认证页面后恢复原主题。
- `src/composables/__tests__/useAuthLightTheme.spec.ts`：验证浅色主题进入、认证路由切换和离开恢复。
- `src/components/auth/AuthModeTabs.vue`：登录/注册分段切换。
- `src/components/auth/__tests__/AuthModeTabs.spec.ts`：验证路由、激活态和无障碍属性。
- `src/components/auth/AuthBrandPanel.vue`：左侧品牌画册面板，仅包含一张网站图标和抽象装饰。
- `src/components/auth/__tests__/AuthBrandPanel.spec.ts`：验证单图标约束和品牌文案。
- `src/i18n/__tests__/authBrandLocales.spec.ts`：验证中英文品牌文案键完整。
- `src/components/layout/__tests__/AuthLayoutPresentation.spec.ts`：验证新布局的结构契约、浅色主题和动效降级。
- `src/views/auth/__tests__/authPagePresentation.spec.ts`：验证四页接入方式、真实 Turnstile 条件和无伪造 Cloudflare 标记。

**修改文件**

- `src/i18n/locales/zh/common.ts`
- `src/i18n/locales/en/common.ts`
- `src/components/layout/AuthLayout.vue`
- `src/components/layout/__tests__/siteLogoSanitization.spec.ts`
- `src/views/auth/LoginView.vue`
- `src/views/auth/RegisterView.vue`
- `src/views/auth/ForgotPasswordView.vue`
- `src/views/auth/ResetPasswordView.vue`

## Task 1: 增加品牌认证页文案

**Files:**
- Create: `src/i18n/__tests__/authBrandLocales.spec.ts`
- Modify: `src/i18n/locales/zh/common.ts:203`
- Modify: `src/i18n/locales/en/common.ts:203`

- [ ] **Step 1: 先写失败的中英文文案测试**

```ts
import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

const brandKeys = [
  'kicker',
  'headline',
  'description',
  'billingTitle',
  'billingDescription',
  'routingTitle',
  'routingDescription'
] as const

describe('auth brand locale copy', () => {
  it('defines the complete brand story in both locales', () => {
    for (const key of brandKeys) {
      expect(zh.auth.brand[key]).toBeTruthy()
      expect(en.auth.brand[key]).toBeTruthy()
    }
  })

  it('defines navigation labels used by the shared auth shell', () => {
    expect(zh.auth.modeTabsLabel).toBe('登录与注册')
    expect(zh.auth.backHome).toBe('返回首页')
    expect(en.auth.modeTabsLabel).toBe('Sign in or register')
    expect(en.auth.backHome).toBe('Back home')
  })
})
```

- [ ] **Step 2: 运行测试并确认因缺少键失败**

Run: `corepack pnpm test:run -- src/i18n/__tests__/authBrandLocales.spec.ts`

Expected: FAIL，错误指向 `auth.brand`、`auth.modeTabsLabel` 或 `auth.backHome` 不存在。

- [ ] **Step 3: 在中英文 `auth` 对象开头加入完整文案**

中文：

```ts
brand: {
  kicker: 'WELCOME TO AI GATEWAY',
  headline: '认真维护每一条\n通往 AI 的线路',
  description: '清晰、稳定、透明。登录后所有密钥、用量和渠道状态一目了然。',
  billingTitle: '清楚计费',
  billingDescription: '每一笔消耗都有迹可循',
  routingTitle: '灵活调度',
  routingDescription: '异常时自动选择可用线路'
},
modeTabsLabel: '登录与注册',
backHome: '返回首页',
```

英文：

```ts
brand: {
  kicker: 'WELCOME TO AI GATEWAY',
  headline: 'Every route to AI,\ncarefully maintained',
  description: 'Clear, stable and transparent. Keys, usage and channel health stay visible after sign-in.',
  billingTitle: 'Traceable billing',
  billingDescription: 'Every unit of usage can be reviewed',
  routingTitle: 'Flexible routing',
  routingDescription: 'Automatically selects an available route'
},
modeTabsLabel: 'Sign in or register',
backHome: 'Back home',
```

- [ ] **Step 4: 运行文案测试与消息编译测试**

Run: `corepack pnpm test:run -- src/i18n/__tests__/authBrandLocales.spec.ts src/i18n/__tests__/localesMessageCompile.spec.ts`

Expected: PASS。

- [ ] **Step 5: 提交文案改动**

```powershell
git add frontend/src/i18n/locales/zh/common.ts frontend/src/i18n/locales/en/common.ts frontend/src/i18n/__tests__/authBrandLocales.spec.ts
git commit -m "feat(auth): add branded page copy"
```

## Task 2: 建立认证页固定浅色主题作用域

**Files:**
- Create: `src/composables/useAuthLightTheme.ts`
- Create: `src/composables/__tests__/useAuthLightTheme.spec.ts`

认证页内部现有子组件大量使用 `dark:` 类。只在外层增加浅色背景无法覆盖这些类，因此需要在认证页生命周期内临时移除 `<html class="dark">`。实现必须保留用户原本的主题选择，且登录页切换注册页时不能闪回深色。

- [ ] **Step 1: 写失败测试，覆盖进入、连续认证路由和离开恢复**

```ts
import { mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { afterEach, describe, expect, it } from 'vitest'

import { useAuthLightTheme } from '../useAuthLightTheme'

const Harness = defineComponent({
  setup() {
    useAuthLightTheme()
    return () => null
  }
})

async function flushThemeRestore(): Promise<void> {
  await Promise.resolve()
  await nextTick()
}

describe('useAuthLightTheme', () => {
  afterEach(async () => {
    document.documentElement.classList.remove('dark')
    await flushThemeRestore()
  })

  it('forces light mode while mounted and restores dark mode after leaving auth', async () => {
    document.documentElement.classList.add('dark')
    const wrapper = mount(Harness)

    expect(document.documentElement.classList.contains('dark')).toBe(false)

    wrapper.unmount()
    await flushThemeRestore()

    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('does not flash dark mode during an auth-to-auth route transition', async () => {
    document.documentElement.classList.add('dark')
    const login = mount(Harness)

    login.unmount()
    const register = mount(Harness)
    await flushThemeRestore()

    expect(document.documentElement.classList.contains('dark')).toBe(false)

    register.unmount()
    await flushThemeRestore()

    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('leaves an originally light document light after unmount', async () => {
    const wrapper = mount(Harness)
    wrapper.unmount()
    await flushThemeRestore()

    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })
})
```

- [ ] **Step 2: 运行测试并确认模块缺失**

Run: `corepack pnpm test:run -- src/composables/__tests__/useAuthLightTheme.spec.ts`

Expected: FAIL，无法解析 `../useAuthLightTheme`。

- [ ] **Step 3: 实现带延迟恢复和引用计数的主题作用域**

```ts
import { onBeforeMount, onUnmounted } from 'vue'

let activeScopes = 0
let shouldRestoreDark = false
let restorePending = false
let restoreVersion = 0

export function useAuthLightTheme(): void {
  onBeforeMount(() => {
    restoreVersion += 1

    if (activeScopes === 0) {
      if (!restorePending) {
        shouldRestoreDark = document.documentElement.classList.contains('dark')
      }
      restorePending = false
      document.documentElement.classList.remove('dark')
    }

    activeScopes += 1
  })

  onUnmounted(() => {
    activeScopes = Math.max(0, activeScopes - 1)
    if (activeScopes !== 0) {
      return
    }

    restorePending = true
    const version = ++restoreVersion

    queueMicrotask(() => {
      if (version !== restoreVersion || activeScopes !== 0 || !restorePending) {
        return
      }

      document.documentElement.classList.toggle('dark', shouldRestoreDark)
      shouldRestoreDark = false
      restorePending = false
    })
  })
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `corepack pnpm test:run -- src/composables/__tests__/useAuthLightTheme.spec.ts`

Expected: PASS，3 tests。

- [ ] **Step 5: 提交主题作用域**

```powershell
git add frontend/src/composables/useAuthLightTheme.ts frontend/src/composables/__tests__/useAuthLightTheme.spec.ts
git commit -m "feat(auth): force light theme within auth routes"
```

## Task 3: 增加登录/注册模式切换组件

**Files:**
- Create: `src/components/auth/AuthModeTabs.vue`
- Create: `src/components/auth/__tests__/AuthModeTabs.spec.ts`

- [ ] **Step 1: 写失败的组件测试**

```ts
import { mount, RouterLinkStub } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'

import AuthModeTabs from '../AuthModeTabs.vue'

function mountTabs(active: 'login' | 'register') {
  const i18n = createI18n({
    legacy: false,
    locale: 'zh',
    messages: {
      zh: {
        auth: {
          modeTabsLabel: '登录与注册',
          signIn: '登录',
          signUp: '注册'
        }
      }
    }
  })

  return mount(AuthModeTabs, {
    props: { active },
    global: {
      plugins: [i18n],
      stubs: { RouterLink: RouterLinkStub }
    }
  })
}

describe('AuthModeTabs', () => {
  it('links to the login and register routes', () => {
    const wrapper = mountTabs('login')
    const links = wrapper.findAllComponents(RouterLinkStub)

    expect(links[0].props('to')).toBe('/login')
    expect(links[1].props('to')).toBe('/register')
  })

  it('marks only the active route as current', () => {
    const wrapper = mountTabs('register')
    const links = wrapper.findAllComponents(RouterLinkStub)

    expect(links[0].attributes('aria-current')).toBeUndefined()
    expect(links[1].attributes('aria-current')).toBe('page')
    expect(links[1].classes()).toContain('auth-mode-tab--active')
  })
})
```

- [ ] **Step 2: 运行并确认组件缺失**

Run: `corepack pnpm test:run -- src/components/auth/__tests__/AuthModeTabs.spec.ts`

Expected: FAIL，无法解析 `AuthModeTabs.vue`。

- [ ] **Step 3: 创建分段切换组件**

```vue
<template>
  <nav class="auth-mode-tabs" :aria-label="t('auth.modeTabsLabel')">
    <RouterLink
      to="/login"
      class="auth-mode-tab"
      :class="{ 'auth-mode-tab--active': active === 'login' }"
      :aria-current="active === 'login' ? 'page' : undefined"
    >
      {{ t('auth.signIn') }}
    </RouterLink>
    <RouterLink
      to="/register"
      class="auth-mode-tab"
      :class="{ 'auth-mode-tab--active': active === 'register' }"
      :aria-current="active === 'register' ? 'page' : undefined"
    >
      {{ t('auth.signUp') }}
    </RouterLink>
  </nav>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  active: 'login' | 'register'
}>()

const { t } = useI18n()
</script>

<style scoped>
.auth-mode-tabs {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  width: min(220px, 100%);
  padding: 5px;
  border: 1px solid #dcebed;
  border-radius: 8px;
  background: #edf5f5;
}

.auth-mode-tab {
  display: inline-flex;
  min-height: 40px;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  color: #71868a;
  font-size: 0.95rem;
  font-weight: 700;
  transition: color 180ms ease, background-color 180ms ease, box-shadow 180ms ease;
}

.auth-mode-tab:hover {
  color: #0d8f89;
}

.auth-mode-tab--active {
  color: #087b76;
  background: #ffffff;
  box-shadow: 0 5px 16px rgba(24, 111, 108, 0.1);
}
</style>
```

- [ ] **Step 4: 运行组件测试**

Run: `corepack pnpm test:run -- src/components/auth/__tests__/AuthModeTabs.spec.ts`

Expected: PASS。

- [ ] **Step 5: 提交模式切换组件**

```powershell
git add frontend/src/components/auth/AuthModeTabs.vue frontend/src/components/auth/__tests__/AuthModeTabs.spec.ts
git commit -m "feat(auth): add login register mode tabs"
```

## Task 4: 创建左侧品牌画册面板

**Files:**
- Create: `src/components/auth/AuthBrandPanel.vue`
- Create: `src/components/auth/__tests__/AuthBrandPanel.spec.ts`

- [ ] **Step 1: 写失败测试，锁定单图标和品牌信息结构**

```ts
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'

import AuthBrandPanel from '../AuthBrandPanel.vue'

const messages = {
  zh: {
    auth: {
      brand: {
        kicker: 'WELCOME TO AI GATEWAY',
        headline: '认真维护每一条\n通往 AI 的线路',
        description: '清晰、稳定、透明。',
        billingTitle: '清楚计费',
        billingDescription: '每一笔消耗都有迹可循',
        routingTitle: '灵活调度',
        routingDescription: '异常时自动选择可用线路'
      }
    }
  }
}

function mountPanel() {
  return mount(AuthBrandPanel, {
    props: {
      siteName: '爱白嫖公益站',
      siteLogo: '/brand.png'
    },
    global: {
      plugins: [createI18n({ legacy: false, locale: 'zh', messages })]
    }
  })
}

describe('AuthBrandPanel', () => {
  it('renders exactly one site logo', () => {
    const wrapper = mountPanel()
    const images = wrapper.findAll('img')

    expect(images).toHaveLength(1)
    expect(images[0].attributes('src')).toBe('/brand.png')
    expect(images[0].attributes('alt')).toBe('爱白嫖公益站')
  })

  it('renders the brand story and two service facts', () => {
    const text = mountPanel().text()

    expect(text).toContain('爱白嫖公益站')
    expect(text).toContain('认真维护每一条')
    expect(text).toContain('清楚计费')
    expect(text).toContain('灵活调度')
  })
})
```

- [ ] **Step 2: 运行并确认组件缺失**

Run: `corepack pnpm test:run -- src/components/auth/__tests__/AuthBrandPanel.spec.ts`

Expected: FAIL。

- [ ] **Step 3: 创建品牌面板模板和脚本**

```vue
<template>
  <aside class="auth-brand-panel">
    <div class="auth-brand-shape auth-brand-shape--top" aria-hidden="true"></div>
    <div class="auth-brand-shape auth-brand-shape--bottom" aria-hidden="true"></div>

    <header class="auth-brand-identity">
      <img class="auth-brand-logo" :src="siteLogo" :alt="siteName" />
      <strong class="auth-brand-name">{{ siteName }}</strong>
    </header>

    <div class="auth-brand-copy">
      <p class="auth-brand-kicker">{{ t('auth.brand.kicker') }}</p>
      <h1>{{ t('auth.brand.headline') }}</h1>
      <p class="auth-brand-description">{{ t('auth.brand.description') }}</p>
    </div>

    <dl class="auth-brand-facts">
      <div class="auth-brand-fact">
        <dt><span>01</span>{{ t('auth.brand.billingTitle') }}</dt>
        <dd>{{ t('auth.brand.billingDescription') }}</dd>
      </div>
      <div class="auth-brand-fact">
        <dt><span>02</span>{{ t('auth.brand.routingTitle') }}</dt>
        <dd>{{ t('auth.brand.routingDescription') }}</dd>
      </div>
    </dl>

    <div class="auth-brand-nodes" aria-hidden="true">
      <span class="auth-brand-node auth-brand-node--a"></span>
      <span class="auth-brand-node auth-brand-node--b"></span>
      <span class="auth-brand-node auth-brand-node--c"></span>
      <span class="auth-brand-route"></span>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  siteName: string
  siteLogo: string
}>()

const { t } = useI18n()
</script>
```

- [ ] **Step 4: 加入品牌面板样式与低强度动效**

在同一文件添加 scoped CSS，使用以下确定值：

```css
.auth-brand-panel {
  position: relative;
  min-width: 0;
  overflow: hidden;
  padding: clamp(36px, 4vw, 72px);
  color: #0b3438;
  background: #ddf4f5;
  isolation: isolate;
}

.auth-brand-identity {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 14px;
}

.auth-brand-logo {
  width: 64px;
  height: 64px;
  border-radius: 8px;
  object-fit: cover;
  box-shadow: 0 14px 30px rgba(16, 129, 126, 0.18);
  animation: auth-logo-float 5s ease-in-out infinite;
}

.auth-brand-name {
  font-size: 1.5rem;
}

.auth-brand-copy {
  position: relative;
  z-index: 2;
  max-width: 650px;
  margin-top: clamp(68px, 9vh, 128px);
}

.auth-brand-kicker {
  margin: 0 0 22px;
  color: #0b8f89;
  font-size: 0.78rem;
  font-weight: 800;
  letter-spacing: 0;
}

.auth-brand-copy h1 {
  margin: 0;
  white-space: pre-line;
  font-size: 4.25rem;
  font-weight: 800;
  line-height: 1.03;
  letter-spacing: 0;
}

.auth-brand-description {
  max-width: 620px;
  margin: 28px 0 0;
  color: #537a7e;
  font-size: 1.1rem;
  line-height: 1.8;
}

.auth-brand-facts {
  position: relative;
  z-index: 2;
  display: grid;
  gap: 22px;
  margin: clamp(50px, 8vh, 96px) 0 0;
}

.auth-brand-fact dt {
  display: flex;
  align-items: center;
  gap: 16px;
  font-weight: 800;
}

.auth-brand-fact dt span {
  display: inline-flex;
  width: 48px;
  height: 48px;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  color: #236c70;
  background: rgba(255, 255, 255, 0.8);
}

.auth-brand-fact dd {
  margin: -17px 0 0 64px;
  color: #66878a;
  font-size: 0.9rem;
}

.auth-brand-shape {
  position: absolute;
  z-index: 0;
  animation: auth-shape-drift 13s ease-in-out infinite alternate;
}

.auth-brand-shape--top {
  width: 540px;
  height: 420px;
  top: -210px;
  right: -100px;
  border-radius: 0 0 0 160px;
  background: #a7e4e5;
}

.auth-brand-shape--bottom {
  width: 310px;
  height: 250px;
  left: -80px;
  bottom: -110px;
  border-radius: 0 120px 0 0;
  background: #f2e3d6;
  animation-delay: -4s;
}

.auth-brand-nodes {
  position: absolute;
  right: 9%;
  bottom: 8%;
  width: 210px;
  height: 130px;
}

.auth-brand-node {
  position: absolute;
  z-index: 2;
  width: 34px;
  height: 34px;
  border: 6px solid rgba(255, 255, 255, 0.78);
  border-radius: 50%;
  box-shadow: 0 8px 20px rgba(36, 111, 108, 0.14);
  animation: auth-node-pulse 3.8s ease-in-out infinite;
}

.auth-brand-node--a { left: 0; bottom: 10px; background: #60d2cf; }
.auth-brand-node--b { right: 28px; top: 0; background: #ffd15c; animation-delay: -1.3s; }
.auth-brand-node--c { right: 0; bottom: 0; background: #ffab93; animation-delay: -2.4s; }

.auth-brand-route {
  position: absolute;
  inset: 28px 18px 18px 20px;
  border: 2px dashed rgba(24, 151, 145, 0.38);
  border-radius: 50%;
  transform: rotate(-10deg);
}

@keyframes auth-logo-float {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-5px); }
}

@keyframes auth-shape-drift {
  from { transform: translate3d(0, 0, 0); }
  to { transform: translate3d(14px, 10px, 0); }
}

@keyframes auth-node-pulse {
  0%, 100% { transform: scale(1); }
  50% { transform: scale(1.08); }
}

@media (max-width: 900px) {
  .auth-brand-panel { min-height: 280px; padding: 28px; }
  .auth-brand-name { font-size: 1.2rem; }
  .auth-brand-copy { margin-top: 36px; }
  .auth-brand-copy h1 { font-size: 2.8rem; }
  .auth-brand-description { margin-top: 16px; }
  .auth-brand-facts, .auth-brand-nodes { display: none; }
  .auth-brand-shape--top { width: 380px; height: 300px; }
}

@media (prefers-reduced-motion: reduce) {
  .auth-brand-logo,
  .auth-brand-shape,
  .auth-brand-node { animation: none; }
}
```

- [ ] **Step 5: 运行测试并提交**

Run: `corepack pnpm test:run -- src/components/auth/__tests__/AuthBrandPanel.spec.ts`

Expected: PASS。

```powershell
git add frontend/src/components/auth/AuthBrandPanel.vue frontend/src/components/auth/__tests__/AuthBrandPanel.spec.ts
git commit -m "feat(auth): add branded auth story panel"
```

## Task 5: 重构共享认证布局

**Files:**
- Modify: `src/components/layout/AuthLayout.vue`
- Create: `src/components/layout/__tests__/AuthLayoutPresentation.spec.ts`
- Modify: `src/components/layout/__tests__/siteLogoSanitization.spec.ts`

- [ ] **Step 1: 写结构契约测试**

```ts
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(dir, '../AuthLayout.vue'), 'utf8')

describe('AuthLayout presentation contract', () => {
  it('uses the shared brand panel and fixed-light theme scope', () => {
    expect(source).toContain("import AuthBrandPanel from '@/components/auth/AuthBrandPanel.vue'")
    expect(source).toContain("import { useAuthLightTheme } from '@/composables/useAuthLightTheme'")
    expect(source).toContain('useAuthLightTheme()')
    expect(source).toContain('<AuthBrandPanel')
  })

  it('provides one responsive shell with home link, content and footer slots', () => {
    expect(source).toContain('class="auth-shell"')
    expect(source).toContain('to="/"')
    expect(source).toContain('<slot />')
    expect(source).toContain('<slot name="footer" />')
  })

  it('supports reduced motion', () => {
    expect(source).toContain('@media (prefers-reduced-motion: reduce)')
  })
})
```

- [ ] **Step 2: 扩展网站图标安全测试**

在 `siteLogoSanitization.spec.ts` 加载 `AuthLayout.vue`：

```ts
const authLayoutSource = readFileSync(resolve(dir, '../AuthLayout.vue'), 'utf8')
```

新增断言：

```ts
it('AuthLayout sanitizes the public site logo', () => {
  expect(authLayoutSource).toContain("import { sanitizeUrl } from '@/utils/url'")
  expect(authLayoutSource).toContain('sanitizeUrl(appStore.siteLogo')
  expect(authLayoutSource).toContain('allowRelative: true')
  expect(authLayoutSource).toContain('allowDataUrl: true')
})
```

- [ ] **Step 3: 运行测试并确认旧布局不满足契约**

Run: `corepack pnpm test:run -- src/components/layout/__tests__/AuthLayoutPresentation.spec.ts src/components/layout/__tests__/siteLogoSanitization.spec.ts`

Expected: `AuthLayoutPresentation.spec.ts` FAIL；现有安全测试仍 PASS。

- [ ] **Step 4: 用双栏结构替换 `AuthLayout.vue` 模板和脚本**

```vue
<template>
  <div class="auth-page">
    <div class="auth-shell">
      <AuthBrandPanel :site-name="siteName" :site-logo="siteLogo || '/logo.svg'" />

      <main class="auth-form-panel">
        <RouterLink to="/" class="auth-home-link">
          <span aria-hidden="true">&larr;</span>
          {{ t('auth.backHome') }}
        </RouterLink>

        <div class="auth-form-content">
          <slot />

          <div class="auth-layout-footer">
            <slot name="footer" />
          </div>

          <p class="auth-copyright">
            &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
          </p>
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AuthBrandPanel from '@/components/auth/AuthBrandPanel.vue'
import { useAuthLightTheme } from '@/composables/useAuthLightTheme'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const appStore = useAppStore()

useAuthLightTheme()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() =>
  sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true })
)
const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>
```

- [ ] **Step 5: 添加共享布局和深层表单样式**

在 `AuthLayout.vue` 添加以下 scoped CSS。不要修改全局 `style.css`，避免影响控制台其他表单。

```css
.auth-page {
  position: relative;
  min-height: 100vh;
  display: grid;
  place-items: center;
  overflow: hidden;
  padding: clamp(16px, 3vw, 48px);
  color: #15383b;
  background-color: #f6fbfa;
  background-image:
    linear-gradient(rgba(36, 147, 143, 0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(36, 147, 143, 0.035) 1px, transparent 1px);
  background-size: 72px 72px;
}

.auth-shell {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: minmax(0, 1.06fr) minmax(480px, 0.94fr);
  width: min(1760px, 100%);
  min-height: min(820px, calc(100vh - 48px));
  overflow: hidden;
  border: 1px solid rgba(94, 151, 151, 0.14);
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 24px 70px rgba(38, 95, 92, 0.16);
  animation: auth-shell-enter 420ms cubic-bezier(0.2, 0.7, 0.2, 1) both;
}

.auth-form-panel {
  position: relative;
  min-width: 0;
  max-height: calc(100vh - 48px);
  overflow-y: auto;
  padding: clamp(72px, 7vh, 104px) clamp(40px, 5vw, 92px) 42px;
  background: #ffffff;
}

.auth-home-link {
  position: absolute;
  top: 32px;
  right: 36px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: #6d888a;
  font-size: 0.88rem;
  font-weight: 700;
  transition: color 180ms ease, transform 180ms ease;
}

.auth-home-link:hover {
  color: #0d8f89;
  transform: translateX(-2px);
}

.auth-form-content {
  width: min(590px, 100%);
  margin-inline: auto;
}

.auth-layout-footer {
  margin-top: 26px;
  text-align: center;
}

.auth-copyright {
  margin: 44px 0 0;
  color: #9aadaf;
  font-size: 0.76rem;
  text-align: center;
}

:deep(.auth-form-stack) {
  display: grid;
  gap: 24px;
}

:deep(.auth-view-heading) {
  margin: 0;
  color: #12373a;
  font-size: 2.5rem;
  font-weight: 800;
  line-height: 1.12;
  letter-spacing: 0;
}

:deep(.auth-view-description) {
  margin: 10px 0 0;
  color: #789093;
  font-size: 1rem;
  line-height: 1.65;
}

:deep(.input) {
  min-height: 56px;
  border-color: #d7e6e7;
  border-radius: 8px;
  color: #173b3e;
  background: #fbfdfd;
  transition: border-color 180ms ease, box-shadow 180ms ease, background-color 180ms ease;
}

:deep(.input:hover) {
  border-color: #b9d7d8;
  background: #ffffff;
}

:deep(.input:focus) {
  border-color: #22aaa4;
  box-shadow: 0 0 0 3px rgba(34, 170, 164, 0.12);
}

:deep(.input-label) {
  color: #315659;
  font-weight: 750;
}

:deep(.auth-turnstile-slot) {
  width: 100%;
  overflow: hidden;
  border-radius: 8px;
}

:deep(.auth-primary-action) {
  position: relative;
  display: inline-flex;
  min-height: 56px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 8px;
  text-align: center;
  box-shadow: 0 13px 28px rgba(14, 161, 154, 0.2);
  transition: transform 180ms ease, box-shadow 180ms ease, filter 180ms ease;
}

:deep(.auth-primary-action::after) {
  position: absolute;
  inset: 0;
  content: '';
  transform: translateX(-130%);
  background: linear-gradient(100deg, transparent 25%, rgba(255, 255, 255, 0.25), transparent 75%);
  transition: transform 520ms ease;
}

:deep(.auth-primary-action:hover:not(:disabled)) {
  transform: translateY(-1px);
  box-shadow: 0 17px 34px rgba(14, 161, 154, 0.25);
}

:deep(.auth-primary-action:hover:not(:disabled)::after) {
  transform: translateX(130%);
}

:deep(.auth-primary-action > *) {
  position: relative;
  z-index: 1;
}

:deep(.auth-status-card) {
  border: 1px solid;
  border-radius: 8px;
  padding: 24px;
}

:deep(.auth-status-card--success) {
  border-color: #bce5d2;
  color: #176a50;
  background: #f0fbf5;
}

:deep(.auth-status-card--warning) {
  border-color: #f1d9a8;
  color: #825b17;
  background: #fff9ec;
}

:deep(.auth-inline-link),
:deep(.auth-footer-copy a) {
  color: #0b9891;
  font-weight: 700;
  transition: color 180ms ease;
}

:deep(.auth-inline-link:hover),
:deep(.auth-footer-copy a:hover) {
  color: #076d69;
}

:deep(.auth-footer-copy) {
  color: #7d9496;
}

@keyframes auth-shell-enter {
  from { opacity: 0; transform: translateY(14px) scale(0.99); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}

@media (max-width: 1100px) {
  .auth-shell { grid-template-columns: minmax(0, 0.9fr) minmax(450px, 1.1fr); }
}

@media (max-width: 900px) {
  .auth-page { display: block; overflow: auto; }
  .auth-shell { grid-template-columns: minmax(0, 1fr); min-height: 0; }
  .auth-form-panel { max-height: none; overflow: visible; padding: 76px 28px 36px; }
  .auth-home-link { top: 26px; right: 28px; }
}

@media (max-width: 560px) {
  .auth-page { padding: 0; background: #ffffff; }
  .auth-shell { border: 0; border-radius: 0; box-shadow: none; }
  .auth-form-panel { padding: 68px 20px 30px; }
  .auth-home-link { top: 22px; right: 20px; }
  :deep(.auth-view-heading) { font-size: 2rem; }
}

@media (prefers-reduced-motion: reduce) {
  .auth-shell,
  .auth-home-link,
  :deep(.auth-primary-action),
  :deep(.auth-primary-action::after) {
    animation: none;
    transition: none;
  }
}
```

- [ ] **Step 6: 运行布局、安全和类型测试**

Run: `corepack pnpm test:run -- src/components/layout/__tests__/AuthLayoutPresentation.spec.ts src/components/layout/__tests__/siteLogoSanitization.spec.ts src/components/auth/__tests__/AuthBrandPanel.spec.ts`

Expected: PASS。

Run: `corepack pnpm typecheck`

Expected: PASS。

- [ ] **Step 7: 提交共享布局**

```powershell
git add frontend/src/components/layout/AuthLayout.vue frontend/src/components/layout/__tests__/AuthLayoutPresentation.spec.ts frontend/src/components/layout/__tests__/siteLogoSanitization.spec.ts
git commit -m "feat(auth): rebuild shared authentication layout"
```

## Task 6: 接入登录页和注册页

**Files:**
- Modify: `src/views/auth/LoginView.vue`
- Modify: `src/views/auth/RegisterView.vue`
- Create: `src/views/auth/__tests__/authPagePresentation.spec.ts`

- [ ] **Step 1: 先写登录/注册接入契约测试**

```ts
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const readView = (name: string) => readFileSync(resolve(dir, `../${name}.vue`), 'utf8')

const login = readView('LoginView')
const register = readView('RegisterView')
const forgot = readView('ForgotPasswordView')
const reset = readView('ResetPasswordView')

describe('auth page presentation', () => {
  it('connects login and register to the shared mode tabs', () => {
    expect(login).toContain('<AuthModeTabs active="login" />')
    expect(register).toContain('<AuthModeTabs active="register" />')
  })

  it('uses centered branded primary actions on login and register', () => {
    expect(login).toContain('auth-primary-action')
    expect(register).toContain('auth-primary-action')
    expect(login).toContain('auth-view-heading')
    expect(register).toContain('auth-view-heading')
  })

  it('keeps Turnstile conditional on real backend configuration', () => {
    for (const source of [login, register, forgot]) {
      expect(source).toContain('v-if="turnstileEnabled && turnstileSiteKey"')
      expect(source).toContain('<TurnstileWidget')
    }
    expect(reset).not.toContain('<TurnstileWidget')
  })

  it('does not hard-code a fake Cloudflare verification panel', () => {
    for (const source of [login, register, forgot, reset]) {
      expect(source).not.toMatch(/CLOUDFLARE|Cloudflare/)
    }
  })
})
```

- [ ] **Step 2: 运行并确认接入测试失败**

Run: `corepack pnpm test:run -- src/views/auth/__tests__/authPagePresentation.spec.ts`

Expected: FAIL，缺少 `AuthModeTabs` 和新的视觉类。

- [ ] **Step 3: 修改 `LoginView.vue` 的展示层，不改业务脚本**

在默认 slot 顶部加入：

```vue
<AuthModeTabs active="login" />
```

把最外层表单内容容器改为：

```vue
<div class="auth-form-stack">
```

标题改为：

```vue
<div>
  <h2 class="auth-view-heading">{{ t('auth.welcomeBack') }}</h2>
  <p class="auth-view-description">{{ t('auth.signInToAccount') }}</p>
</div>
```

Turnstile 外层改为：

```vue
<div v-if="turnstileEnabled && turnstileSiteKey" class="auth-turnstile-slot">
```

提交按钮类改为：

```vue
class="btn btn-primary auth-primary-action w-full"
```

页脚改为：

```vue
<p class="auth-footer-copy">
  {{ t('auth.dontHaveAccount') }}
  <router-link to="/register">{{ t('auth.signUp') }}</router-link>
</p>
```

脚本新增：

```ts
import AuthModeTabs from '@/components/auth/AuthModeTabs.vue'
```

保留登录协议、OAuth、2FA、Turnstile 回调、表单校验、API 调用和重定向代码原样。

- [ ] **Step 4: 修改 `RegisterView.vue` 的展示层，不改注册逻辑**

在默认 slot 顶部加入：

```vue
<AuthModeTabs active="register" />
```

与登录页一致，将外层改为 `auth-form-stack`，标题使用 `auth-view-heading` / `auth-view-description`，Turnstile 外层使用 `auth-turnstile-slot`，提交按钮使用：

```vue
class="btn btn-primary auth-primary-action w-full"
```

注册关闭提示改为：

```vue
<div v-if="!registrationEnabled && settingsLoaded" class="auth-status-card auth-status-card--warning">
```

页脚改为：

```vue
<p class="auth-footer-copy">
  {{ t('auth.alreadyHaveAccount') }}
  <router-link to="/login">{{ t('auth.signIn') }}</router-link>
</p>
```

脚本新增：

```ts
import AuthModeTabs from '@/components/auth/AuthModeTabs.vue'
```

保留邀请码、优惠码、邮箱验证、登录协议、Turnstile 和注册 API 逻辑原样。

- [ ] **Step 5: 运行接入测试、类型检查和相关已有测试**

Run: `corepack pnpm test:run -- src/views/auth/__tests__/authPagePresentation.spec.ts src/components/auth/__tests__/AuthModeTabs.spec.ts src/components/__tests__/LoginForm.spec.ts`

Expected: PASS。

Run: `corepack pnpm typecheck`

Expected: PASS。

- [ ] **Step 6: 提交登录/注册页面**

```powershell
git add frontend/src/views/auth/LoginView.vue frontend/src/views/auth/RegisterView.vue frontend/src/views/auth/__tests__/authPagePresentation.spec.ts
git commit -m "feat(auth): apply branded login and registration pages"
```

## Task 7: 接入忘记密码和重置密码页面

**Files:**
- Modify: `src/views/auth/ForgotPasswordView.vue`
- Modify: `src/views/auth/ResetPasswordView.vue`
- Modify: `src/views/auth/__tests__/authPagePresentation.spec.ts`

- [ ] **Step 1: 扩展展示契约测试**

在 `authPagePresentation.spec.ts` 新增：

```ts
it('uses the shared heading, action and status styles in password recovery', () => {
  expect(forgot).toContain('auth-view-heading')
  expect(forgot).toContain('auth-primary-action')
  expect(forgot).toContain('auth-status-card auth-status-card--success')
  expect(reset).toContain('auth-view-heading')
  expect(reset).toContain('auth-primary-action')
  expect(reset).toContain('auth-status-card auth-status-card--warning')
  expect(reset).toContain('auth-status-card auth-status-card--success')
})
```

- [ ] **Step 2: 运行并确认恢复页面尚未接入**

Run: `corepack pnpm test:run -- src/views/auth/__tests__/authPagePresentation.spec.ts`

Expected: FAIL，新断言失败。

- [ ] **Step 3: 修改 `ForgotPasswordView.vue` 展示层**

- 外层使用 `auth-form-stack`。
- 标题使用 `auth-view-heading` 和 `auth-view-description`。
- 成功提示外层使用 `auth-status-card auth-status-card--success`，内部现有图标和文案继续保留。
- 返回登录链接增加 `auth-inline-link`。
- Turnstile 外层增加 `auth-turnstile-slot`，仍仅在 `turnstileEnabled && turnstileSiteKey` 时渲染。
- 发送重置链接按钮增加 `auth-primary-action`。
- 页脚段落改为 `auth-footer-copy`。
- 不增加登录/注册 tabs，密码恢复页保持单任务流程。

- [ ] **Step 4: 修改 `ResetPasswordView.vue` 展示层**

- 外层使用 `auth-form-stack`。
- 标题使用 `auth-view-heading` 和 `auth-view-description`。
- 无效链接提示使用 `auth-status-card auth-status-card--warning`。
- 成功提示使用 `auth-status-card auth-status-card--success`。
- 请求新链接和返回登录链接增加 `auth-inline-link`。
- 重置密码按钮及成功状态登录按钮增加 `auth-primary-action`。
- 页脚段落改为 `auth-footer-copy`。
- 不引入 `TurnstileWidget`，因为重置链接本身依赖邮件 token；继续保持当前后端契约。

- [ ] **Step 5: 删除四个 View 中已被共享布局替代的局部淡入样式**

若 `ForgotPasswordView.vue` 或 `ResetPasswordView.vue` 中的 `.fade-*` 仍被邀请码或状态 transition 使用则保留；仅删除确认无引用的规则。运行 `rg 'name="fade"|fade-enter' src/views/auth` 后再决定，不能盲删。

- [ ] **Step 6: 运行展示、类型和构建测试**

Run: `corepack pnpm test:run -- src/views/auth/__tests__/authPagePresentation.spec.ts`

Expected: PASS。

Run: `corepack pnpm typecheck`

Expected: PASS。

Run: `corepack pnpm build`

Expected: PASS，Vite 正常生成 `dist`。

- [ ] **Step 7: 提交密码恢复页面**

```powershell
git add frontend/src/views/auth/ForgotPasswordView.vue frontend/src/views/auth/ResetPasswordView.vue frontend/src/views/auth/__tests__/authPagePresentation.spec.ts
git commit -m "feat(auth): restyle password recovery flows"
```

## Task 8: 完整验证与浏览器验收

**Files:**
- Verify only; no source changes unless a verified defect is found.

- [ ] **Step 1: 运行所有本任务新增和受影响测试**

```powershell
corepack pnpm test:run -- src/i18n/__tests__/authBrandLocales.spec.ts src/i18n/__tests__/localesMessageCompile.spec.ts src/composables/__tests__/useAuthLightTheme.spec.ts src/components/auth/__tests__/AuthModeTabs.spec.ts src/components/auth/__tests__/AuthBrandPanel.spec.ts src/components/layout/__tests__/AuthLayoutPresentation.spec.ts src/components/layout/__tests__/siteLogoSanitization.spec.ts src/views/auth/__tests__/authPagePresentation.spec.ts
```

Expected: 全部 PASS。

- [ ] **Step 2: 运行静态质量检查**

```powershell
corepack pnpm typecheck
corepack pnpm lint:check
corepack pnpm build
```

Expected: 三条命令全部 PASS。若 lint 仅有格式问题，先运行 `corepack pnpm lint`，审查 diff 后再重跑 `lint:check`。

- [ ] **Step 3: 运行完整 Vitest 套件并与基线比较**

Run: `corepack pnpm test:run`

Expected: 除已记录的 `src/api/__tests__/admin.system.rollback.spec.ts` 2 个上游旧断言外无其他失败。若出现任意新增失败，必须修复并重跑，不能把它归入基线。

- [ ] **Step 4: 启动本地开发服务器**

Run: `corepack pnpm dev -- --host 127.0.0.1`

Expected: Vite 输出本地 URL，默认 `http://127.0.0.1:5173/`；若端口被占用则使用 Vite 给出的新端口。

- [ ] **Step 5: 使用 Playwright 验收四个路由**

依次检查：

- `/login`
- `/register`
- `/forgot-password`
- `/reset-password?email=test%40example.com&token=demo-token`

视口至少覆盖：

- `1440x1000`：双栏画册布局，右侧表单不被裁切。
- `1024x900`：左右比例合理，长注册表单可滚动。
- `390x844`：单栏，品牌面板压缩，输入框、按钮和文字不溢出。

每个视口确认：

- 页面中只有一张网站图标。
- 登录/注册 tabs 激活态正确。
- “继续创建账户”等主按钮文字水平、垂直居中。
- Turnstile 关闭时不留伪造卡片或空白占位。
- Turnstile 开启时仅由真实 `TurnstileWidget` 产生区域，宽度不溢出。
- 忘记密码成功、重置链接无效、重置成功状态都使用新的浅色状态卡。
- 从认证页离开后，原本的深色主题会恢复。
- `prefers-reduced-motion: reduce` 下无持续漂浮、脉冲或闪光动画。

- [ ] **Step 6: 检查浏览器控制台和布局像素**

要求：

- 无 Vue warning、未处理 Promise rejection 或资源 404。
- 表单内容没有覆盖页脚或返回首页链接。
- Turnstile iframe 存在时宽度不超过表单容器。
- 右侧表单在注册字段较多时使用面板滚动，不使 CTA 脱离可达区域。

- [ ] **Step 7: 审查最终 diff**

```powershell
git status --short
git diff --check
git diff ac76e1415 -- frontend/src/components frontend/src/composables frontend/src/views/auth frontend/src/i18n
```

Expected:

- `git diff --check` 无尾随空格或冲突标记。
- 没有修改认证 API、store、路由契约或 Turnstile 核心实现。
- 没有添加用户提供图标的重复副本。
- 没有无关格式化或生成文件。

- [ ] **Step 8: 提交最终验收修正**

只有在浏览器验收发现并修复了实际问题时才创建该提交：

```powershell
git add frontend/src
git commit -m "fix(auth): polish responsive branded auth pages"
```

若没有额外修正，不创建空提交。

## 完成标准

- 四个认证页面均使用 C 方案浅色品牌画册布局。
- 登录和注册有一致但可区分的 tabs 激活态；密码恢复流程不显示 tabs。
- 网站图标每页仅出现一次，所有其他装饰均为 CSS 抽象图形。
- 登录、注册和忘记密码只在后端启用 Turnstile 且存在 site key 时显示真实组件。
- 所有主按钮内容稳定居中，禁用、加载、hover 状态不引发布局位移。
- 桌面、平板、手机和 reduced-motion 验收通过。
- 新增测试、typecheck、lint、build 全部通过；完整测试没有新增失败。
