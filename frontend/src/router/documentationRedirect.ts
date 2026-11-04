import type { RouteLocationNormalized } from 'vue-router'

// Public documentation has its own deployment. A full navigation lets the
// edge proxy serve it. Admin documentation uses an authenticated embed route.
export function redirectToDocumentation(to: Pick<RouteLocationNormalized, 'name' | 'path' | 'fullPath'>): false {
  window.location.replace(`https://aibaipiao.top${to.fullPath}`)
  return false
}
