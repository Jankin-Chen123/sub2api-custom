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
