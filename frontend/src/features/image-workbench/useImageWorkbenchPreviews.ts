import { reactive } from 'vue'
import type { ComputedRef, Ref } from 'vue'
import { imageWorkbenchAPI } from '@/api'
import type { ImageWorkbenchJob } from '@/api'
import {
  deleteCachedImageWorkbenchEntry,
  getCachedImageWorkbenchBlob,
  listCachedImageWorkbenchEntries,
  putCachedImageWorkbenchBlob,
} from '@/utils/imageWorkbenchCache'
import { extractErrorMessage } from './imageWorkbenchFormatters'

export interface UseImageWorkbenchPreviewsOptions {
  userId: Ref<number> | ComputedRef<number>
  createEditableReferenceDataURL: (blob: Blob) => Promise<string>
  getDownloadFileName: (job: ImageWorkbenchJob) => string
  onError: (message: string) => void
  previewErrorMessage: () => string
  downloadErrorMessage: () => string
}

function jobImageVersion(job: ImageWorkbenchJob): string {
  return job.content_url || job.updated_at || job.id
}

export function useImageWorkbenchPreviews(options: UseImageWorkbenchPreviewsOptions) {
  const {
    userId,
    createEditableReferenceDataURL,
    getDownloadFileName,
    onError,
    previewErrorMessage,
    downloadErrorMessage,
  } = options

  const previewURLs = reactive<Record<string, string>>({})
  const previewDataURLs = reactive<Record<string, string>>({})
  const previewBlobs = new Map<string, { version: string; blob: Blob }>()
  const pendingBlobLoads = new Map<string, Promise<Blob>>()

  function hasCurrentPreview(job: ImageWorkbenchJob): boolean {
    return previewBlobs.get(job.id)?.version === jobImageVersion(job) && Boolean(previewURLs[job.id])
  }

  function releasePreview(jobId: string) {
    const url = previewURLs[jobId]
    if (url) URL.revokeObjectURL(url)
    delete previewURLs[jobId]
    delete previewDataURLs[jobId]
    previewBlobs.delete(jobId)
  }

  function prunePreviews(activeJobIds: Set<string>) {
    Object.keys(previewURLs).forEach((jobId) => {
      if (!activeJobIds.has(jobId)) releasePreview(jobId)
    })
  }

  function setPreviewBlob(job: ImageWorkbenchJob, blob: Blob) {
    const version = jobImageVersion(job)
    const existing = previewBlobs.get(job.id)
    if (existing?.version === version && previewURLs[job.id]) return
    if (existing?.version !== version) delete previewDataURLs[job.id]
    if (previewURLs[job.id]) URL.revokeObjectURL(previewURLs[job.id])
    previewBlobs.set(job.id, { version, blob })
    previewURLs[job.id] = URL.createObjectURL(blob)
  }

  async function restoreCachedLibrary(): Promise<{ jobs: ImageWorkbenchJob[]; preferredJobId: string }> {
    const currentUserId = userId.value
    if (currentUserId <= 0) return { jobs: [], preferredJobId: '' }
    try {
      const entries = (await listCachedImageWorkbenchEntries(currentUserId)).slice(0, 30)
      if (!entries.length) return { jobs: [], preferredJobId: '' }
      const restoredJobs = entries.map(entry => entry.job)
      entries.forEach(entry => setPreviewBlob(entry.job, entry.blob))
      const preferred = restoredJobs.find(job => job.status === 'completed') || restoredJobs[0]
      return {
        jobs: restoredJobs,
        preferredJobId: preferred?.id || '',
      }
    } catch {
      // IndexedDB is an optional acceleration layer; the server remains the source of truth.
      return { jobs: [], preferredJobId: '' }
    }
  }

  async function hydrateCachedPreviews(imageJobs: ImageWorkbenchJob[]) {
    const currentUserId = userId.value
    if (currentUserId <= 0) return
    await Promise.all(imageJobs.map(async (job) => {
      if (hasCurrentPreview(job)) return
      try {
        const blob = await getCachedImageWorkbenchBlob(currentUserId, job)
        if (blob) setPreviewBlob(job, blob)
      } catch {
        // Cache read failures fall back to the authenticated content endpoint on demand.
      }
    }))
  }

  async function getImageBlob(job: ImageWorkbenchJob): Promise<Blob> {
    const version = jobImageVersion(job)
    const memoryEntry = previewBlobs.get(job.id)
    if (memoryEntry?.version === version) return memoryEntry.blob
    if (memoryEntry) releasePreview(job.id)

    const currentUserId = userId.value
    const loadKey = `${currentUserId}:${job.id}:${version}`
    const pending = pendingBlobLoads.get(loadKey)
    if (pending) return pending

    const load = (async () => {
      let blob: Blob | null = null
      if (currentUserId > 0) {
        try {
          blob = await getCachedImageWorkbenchBlob(currentUserId, job)
        } catch {
          blob = null
        }
      }
      if (!blob) {
        blob = await imageWorkbenchAPI.getContent(job.id)
        if (currentUserId > 0) {
          try {
            await putCachedImageWorkbenchBlob(currentUserId, job, blob)
          } catch {
            // Quota, privacy mode, and storage errors must not prevent normal previews/downloads.
          }
        }
      }
      previewBlobs.set(job.id, { version, blob })
      return blob
    })()

    pendingBlobLoads.set(loadKey, load)
    try {
      return await load
    } finally {
      pendingBlobLoads.delete(loadKey)
    }
  }

  async function loadPreview(job: ImageWorkbenchJob) {
    if (hasCurrentPreview(job)) return
    try {
      const blob = await getImageBlob(job)
      setPreviewBlob(job, blob)
    } catch (error: unknown) {
      onError(extractErrorMessage(error, previewErrorMessage()))
    }
  }

  async function downloadResult(job: ImageWorkbenchJob) {
    try {
      const blob = await getImageBlob(job)
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = getDownloadFileName(job)
      anchor.click()
      URL.revokeObjectURL(url)
    } catch (error: unknown) {
      onError(extractErrorMessage(error, downloadErrorMessage()))
    }
  }

  async function ensureEditableDataURL(job: ImageWorkbenchJob): Promise<string> {
    if (!previewDataURLs[job.id]) {
      try {
        previewDataURLs[job.id] = await createEditableReferenceDataURL(await getImageBlob(job))
      } catch (error: unknown) {
        onError(extractErrorMessage(error, previewErrorMessage()))
        return ''
      }
    }
    return previewDataURLs[job.id] || ''
  }

  async function refreshCachedJobMetadata(job: ImageWorkbenchJob) {
    const preview = previewBlobs.get(job.id)
    const currentUserId = userId.value
    if (!preview || currentUserId <= 0) return
    try {
      await putCachedImageWorkbenchBlob(currentUserId, job, preview.blob)
    } catch {
      // A cache metadata refresh is optional; the server name is authoritative.
    }
  }

  async function removeCachedJob(jobId: string) {
    releasePreview(jobId)
    const currentUserId = userId.value
    if (currentUserId <= 0) return
    try {
      await deleteCachedImageWorkbenchEntry(currentUserId, jobId)
    } catch {
      // The server deletion is authoritative; a stale optional cache entry must not block the UI.
    }
  }

  function disposePreviews() {
    Object.keys(previewURLs).forEach(releasePreview)
    previewBlobs.clear()
    pendingBlobLoads.clear()
  }

  return {
    previewURLs,
    previewDataURLs,
    prunePreviews,
    restoreCachedLibrary,
    hydrateCachedPreviews,
    loadPreview,
    downloadResult,
    ensureEditableDataURL,
    refreshCachedJobMetadata,
    removeCachedJob,
    disposePreviews,
  }
}
