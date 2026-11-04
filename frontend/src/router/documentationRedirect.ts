import type { RouteLocationNormalized } from 'vue-router'

// Documentation has its own deployment and authentication. A full navigation
// lets the edge proxy serve it instead of rendering a page in the main SPA.
export function redirectToDocumentation(to: Pick<RouteLocationNormalized, 'name' | 'path' | 'fullPath'>): false {
  const path = to.name === 'AdminDocumentation'
    ? `/docs/admin${to.fullPath.slice(to.path.length)}`
    : to.fullPath
  window.location.replace(`https://aibaipiao.top${path}`)
  return false
}
