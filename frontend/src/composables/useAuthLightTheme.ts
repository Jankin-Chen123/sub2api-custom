import { onBeforeMount } from 'vue'

export function useAuthLightTheme(): void {
  onBeforeMount(() => document.documentElement.classList.remove('dark'))
}
