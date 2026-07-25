# Auth Route Motion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make login, registration, forgot-password, and reset-password navigation use a smooth short-distance directional transition without replaying the whole authentication shell animation.

**Architecture:** A small pure utility resolves route direction and stores the current navigation motion before Vue renders the destination route. `AuthLayout` reads that one-shot state during setup and applies motion classes that keep the shell visually stable while animating only the right panel. `AuthModeTabs` uses a separate pseudo-element indicator with direction-aware keyframes.

**Tech Stack:** Vue 3, Vue Router 4, TypeScript, scoped CSS, Vitest, Vue Test Utils.

---

### Task 1: Resolve authentication navigation direction

**Files:**
- Create: `frontend/src/utils/authRouteMotion.ts`
- Create: `frontend/src/utils/__tests__/authRouteMotion.spec.ts`

- [ ] **Step 1: Write the failing direction tests**

```ts
import { describe, expect, it } from 'vitest'
import { resolveAuthRouteMotion } from '@/utils/authRouteMotion'

describe('resolveAuthRouteMotion', () => {
  it.each([
    ['/home', '/login', 'initial'],
    ['/login', '/register', 'forward'],
    ['/login', '/forgot-password', 'forward'],
    ['/forgot-password', '/login', 'backward'],
    ['/forgot-password', '/reset-password', 'forward'],
    ['/reset-password', '/forgot-password', 'backward'],
    ['/register', '/forgot-password', 'neutral'],
    ['/login', '/dashboard', 'neutral'],
  ])('maps %s -> %s to %s', (from, to, expected) => {
    expect(resolveAuthRouteMotion(from, to)).toBe(expected)
  })
})
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `frontend/node_modules/.bin/vitest.CMD run frontend/src/utils/__tests__/authRouteMotion.spec.ts`

Expected: FAIL because `@/utils/authRouteMotion` does not exist.

- [ ] **Step 3: Implement the pure resolver and one-shot state**

```ts
export type AuthRouteMotion = 'initial' | 'forward' | 'backward' | 'neutral'

const AUTH_ROUTE_ORDER: Readonly<Record<string, number>> = {
  '/login': 0,
  '/register': 1,
  '/forgot-password': 1,
  '/reset-password': 2,
}

let currentAuthRouteMotion: AuthRouteMotion = 'initial'

export function resolveAuthRouteMotion(fromPath: string, toPath: string): AuthRouteMotion {
  const toOrder = AUTH_ROUTE_ORDER[toPath]
  if (toOrder === undefined) return 'neutral'

  const fromOrder = AUTH_ROUTE_ORDER[fromPath]
  if (fromOrder === undefined) return 'initial'
  if (toOrder === fromOrder) return 'neutral'
  return toOrder > fromOrder ? 'forward' : 'backward'
}

export function setAuthRouteMotion(motion: AuthRouteMotion): void {
  currentAuthRouteMotion = motion
}

export function getAuthRouteMotion(): AuthRouteMotion {
  return currentAuthRouteMotion
}
```

- [ ] **Step 4: Run the direction tests**

Run: `frontend/node_modules/.bin/vitest.CMD run frontend/src/utils/__tests__/authRouteMotion.spec.ts`

Expected: 8 tests pass.

### Task 2: Apply direction state to router and authentication layout

**Files:**
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/layout/AuthLayout.vue`
- Modify: `frontend/src/components/layout/__tests__/AuthLayoutPresentation.spec.ts`

- [ ] **Step 1: Add failing presentation assertions**

Add assertions that `AuthLayout.vue` imports `getAuthRouteMotion`, binds `authMotionClass`, contains forward/backward content keyframes, disables shell animation during route switches, and handles reduced motion.

```ts
it('animates only auth content during directional route switches', () => {
  expect(authLayoutSource).toContain("import { getAuthRouteMotion } from '@/utils/authRouteMotion'")
  expect(authLayoutSource).toContain(':class="authMotionClass"')
  expect(authLayoutSource).toContain('.auth-motion--forward .auth-shell')
  expect(authLayoutSource).toContain('@keyframes auth-content-enter-forward')
  expect(authLayoutSource).toContain('@keyframes auth-content-enter-backward')
})
```

- [ ] **Step 2: Run the presentation test and verify it fails**

Run: `frontend/node_modules/.bin/vitest.CMD run frontend/src/components/layout/__tests__/AuthLayoutPresentation.spec.ts`

Expected: FAIL because the motion classes and keyframes are absent.

- [ ] **Step 3: Set route motion before navigation renders**

Import the resolver in `frontend/src/router/index.ts`:

```ts
import { resolveAuthRouteMotion, setAuthRouteMotion } from '@/utils/authRouteMotion'
```

Rename the existing `_from` argument to `from`, then add this at the beginning of the existing `router.beforeEach` callback:

```ts
setAuthRouteMotion(resolveAuthRouteMotion(from.path, to.path))
```

- [ ] **Step 4: Bind one-shot motion state in AuthLayout**

Add the utility import and setup value:

```ts
import { getAuthRouteMotion } from '@/utils/authRouteMotion'

const authMotionClass = `auth-motion--${getAuthRouteMotion()}`
```

Bind it to the root:

```vue
<div class="auth-page" :class="authMotionClass">
```

- [ ] **Step 5: Implement short-distance GPU-friendly motion**

Add styles that stop repeated shell entry for auth-to-auth switches and animate only `.auth-form-content`:

```css
.auth-motion--forward .auth-shell,
.auth-motion--backward .auth-shell,
.auth-motion--neutral .auth-shell {
  animation: none;
}

.auth-motion--forward .auth-form-content {
  animation: auth-content-enter-forward 340ms cubic-bezier(0.22, 1, 0.36, 1) both;
}

.auth-motion--backward .auth-form-content {
  animation: auth-content-enter-backward 340ms cubic-bezier(0.22, 1, 0.36, 1) both;
}

.auth-motion--neutral .auth-form-content {
  animation: auth-content-enter-neutral 240ms ease-out both;
}

@keyframes auth-content-enter-forward {
  from { opacity: 0; transform: translate3d(18px, 0, 0); }
  to { opacity: 1; transform: translate3d(0, 0, 0); }
}

@keyframes auth-content-enter-backward {
  from { opacity: 0; transform: translate3d(-18px, 0, 0); }
  to { opacity: 1; transform: translate3d(0, 0, 0); }
}

@keyframes auth-content-enter-neutral {
  from { opacity: 0; }
  to { opacity: 1; }
}
```

Use `12px` instead of `18px` inside the existing mobile media query. Extend the reduced-motion query so `.auth-form-content` receives `animation: none`.

- [ ] **Step 6: Run direction and layout tests**

Run: `frontend/node_modules/.bin/vitest.CMD run frontend/src/utils/__tests__/authRouteMotion.spec.ts frontend/src/components/layout/__tests__/AuthLayoutPresentation.spec.ts`

Expected: all tests pass.

### Task 3: Add a sliding login/register indicator and verify the flow

**Files:**
- Modify: `frontend/src/components/auth/AuthModeTabs.vue`
- Modify: `frontend/src/components/auth/__tests__/AuthModeTabs.spec.ts`

- [ ] **Step 1: Add failing class assertions**

Extend the component tests:

```ts
expect(wrapper.get('nav').classes()).toContain('auth-mode-tabs--register')
expect(wrapper.get('nav').classes()).toContain('auth-mode-tabs--motion-initial')
```

- [ ] **Step 2: Run the tab test and verify it fails**

Run: `frontend/node_modules/.bin/vitest.CMD run frontend/src/components/auth/__tests__/AuthModeTabs.spec.ts`

Expected: FAIL because the state classes are absent.

- [ ] **Step 3: Bind active and motion classes**

Import `getAuthRouteMotion`, capture it during setup, and bind the navigation classes:

```ts
import { getAuthRouteMotion } from '@/utils/authRouteMotion'
const authRouteMotion = getAuthRouteMotion()
```

```vue
<nav
  class="auth-mode-tabs"
  :class="[`auth-mode-tabs--${active}`, `auth-mode-tabs--motion-${authRouteMotion}`]"
  :aria-label="t('auth.modeTabsLabel')"
>
```

- [ ] **Step 4: Implement the indicator layer**

Make the navigation a two-column positioned grid. Draw the white active pill with `::before`; keep links above it with `z-index: 1`. Use `transform` for the static register position and direction keyframes for forward and backward navigation. Remove the active link background and shadow so only the indicator renders the pill. Disable all indicator animation for reduced motion.

- [ ] **Step 5: Run targeted auth tests**

Run:

```powershell
frontend/node_modules/.bin/vitest.CMD run frontend/src/utils/__tests__/authRouteMotion.spec.ts frontend/src/components/layout/__tests__/AuthLayoutPresentation.spec.ts frontend/src/components/auth/__tests__/AuthModeTabs.spec.ts frontend/src/views/auth/__tests__/authPagePresentation.spec.ts frontend/src/router/__tests__/guards.spec.ts
```

Expected: all selected test files pass.

- [ ] **Step 6: Run production verification**

Run from `frontend`:

```powershell
npm.cmd run build
```

Expected: `vue-tsc -b` and `vite build` exit with code 0.

- [ ] **Step 7: Verify browser navigation**

Open the local frontend and verify these paths without console-side animation errors:

- `/login` → `/register`: right-to-left progression with incoming content from the right.
- `/register` → `/login`: incoming content from the left.
- `/login` → `/forgot-password`: incoming content from the right.
- `/forgot-password` → `/login`: incoming content from the left.
- `/reset-password` → `/forgot-password`: incoming content from the left.

- [ ] **Step 8: Commit implementation**

```powershell
git add frontend/src/utils/authRouteMotion.ts frontend/src/utils/__tests__/authRouteMotion.spec.ts frontend/src/router/index.ts frontend/src/components/layout/AuthLayout.vue frontend/src/components/layout/__tests__/AuthLayoutPresentation.spec.ts frontend/src/components/auth/AuthModeTabs.vue frontend/src/components/auth/__tests__/AuthModeTabs.spec.ts
git commit -m "feat(auth): smooth directional page transitions"
```
