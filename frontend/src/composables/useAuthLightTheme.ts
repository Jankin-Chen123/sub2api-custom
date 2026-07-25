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
