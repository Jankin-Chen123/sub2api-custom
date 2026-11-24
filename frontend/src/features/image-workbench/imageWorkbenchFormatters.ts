import type { ImageWorkbenchJob, ImageWorkbenchStatus } from '@/api'
import { IMAGE_DIMENSION_MIN_EDGE, IMAGE_DIMENSION_STEP } from '@/utils/imageWorkbenchDimensions'
import { isImageWorkbenchQualityAllowed } from '@/utils/imageWorkbenchModelCapabilities'

export function parseSize(value: string): [number, number] {
  const match = value.trim().match(/^(\d+)x(\d+)$/i)
  if (!match) return [1024, 1024]
  return [Number(match[1]), Number(match[2])]
}

export function clampDimension(value: number, maxEdge: number): number {
  const numeric = Number.isFinite(value)
    ? Math.round(value / IMAGE_DIMENSION_STEP) * IMAGE_DIMENSION_STEP
    : 1024
  return Math.min(maxEdge, Math.max(IMAGE_DIMENSION_MIN_EDGE, numeric))
}

export function snapDimension(
  value: number,
  direction: 'up' | 'down' | 'nearest' = 'nearest'
): number {
  const raw = Number.isFinite(value)
    ? value / IMAGE_DIMENSION_STEP
    : 1024 / IMAGE_DIMENSION_STEP
  const snapped =
    direction === 'up'
      ? Math.ceil(raw)
      : direction === 'down'
        ? Math.floor(raw)
        : Math.round(raw)
  return snapped * IMAGE_DIMENSION_STEP
}

export function dataURLByteLength(dataURL: string): number {
  const separator = dataURL.indexOf(',')
  const payload = separator >= 0 ? dataURL.slice(separator + 1) : dataURL
  const padding = payload.endsWith('==') ? 2 : payload.endsWith('=') ? 1 : 0
  return Math.max(0, Math.floor((payload.length * 3) / 4) - padding)
}

export function statusClass(status: ImageWorkbenchStatus): string {
  if (status === 'completed') {
    return 'bg-green-100 text-green-700 dark:bg-green-950/40 dark:text-green-300'
  }
  if (status === 'failed' || status === 'submission_unknown') {
    return 'bg-red-100 text-red-700 dark:bg-red-950/40 dark:text-red-300'
  }
  return 'bg-blue-100 text-blue-700 dark:bg-blue-950/40 dark:text-blue-300'
}

export function jobResolution(job: Pick<ImageWorkbenchJob, 'model'>): string {
  return job.model.match(/-(\d+k)$/i)?.[1]?.toUpperCase() || '—'
}

export function jobQualityOption(job: Pick<ImageWorkbenchJob, 'model' | 'quality'>): string {
  return job.quality && isImageWorkbenchQualityAllowed(job.model, job.quality)
    ? job.quality
    : 'auto'
}

export function sanitizeDownloadFileName(displayName: string, fallbackId: string): string {
  const safeName = [...displayName]
    .map((character) =>
      character.charCodeAt(0) < 32 || /[\\/:*?"<>|]/.test(character) ? '_' : character
    )
    .join('')
    .trim()
  return `${safeName || fallbackId}.png`
}

export function formatCost(value: number): string {
  return Number(value || 0).toFixed(4)
}

export function isTerminalJob(status: ImageWorkbenchStatus): boolean {
  return status === 'completed' || status === 'failed' || status === 'submission_unknown'
}

export function formatJobWaitTime(
  job: Pick<ImageWorkbenchJob, 'status' | 'created_at' | 'updated_at'>,
  nowMs: number
): string {
  const createdAt = Date.parse(job.created_at)
  if (!Number.isFinite(createdAt)) return '00:00'
  const completedAt = isTerminalJob(job.status) ? Date.parse(job.updated_at) : Number.NaN
  const endAt = Number.isFinite(completedAt) ? completedAt : nowMs
  const elapsedSeconds = Math.max(0, Math.floor((endAt - createdAt) / 1000))
  const minutes = Math.floor(elapsedSeconds / 60)
  const seconds = elapsedSeconds % 60
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

export function extractErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message
  if (
    typeof error === 'object' &&
    error !== null &&
    'message' in error &&
    typeof (error as { message?: unknown }).message === 'string' &&
    (error as { message: string }).message
  ) {
    return (error as { message: string }).message
  }
  if (typeof error === 'string' && error) return error
  return fallback
}
