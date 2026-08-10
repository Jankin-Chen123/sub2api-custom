import type { ImageWorkbenchJob } from '@/api'

const IMAGE_CACHE_DB_NAME = 'sub2api-image-workbench-cache'
const IMAGE_CACHE_STORE_NAME = 'images'
const IMAGE_CACHE_MAX_AGE_MS = 90 * 24 * 60 * 60 * 1000
const IMAGE_CACHE_MAX_ENTRIES = 80
const IMAGE_CACHE_MAX_BYTES = 256 * 1024 * 1024

type ImageCacheRecord = {
  key: string
  userId: number
  jobId: string
  version: string
  job: ImageWorkbenchJob
  blob: Blob
  size: number
  createdAt: number
  lastAccessedAt: number
}

export type CachedImageWorkbenchEntry = {
  job: ImageWorkbenchJob
  blob: Blob
}

let imageCacheDBPromise: Promise<IDBDatabase | null> | null = null

function imageCacheSupported() {
  return typeof window !== 'undefined' && 'indexedDB' in window && Boolean(window.indexedDB)
}

function imageCacheKey(userId: number, jobId: string) {
  return `${userId}:${encodeURIComponent(jobId)}`
}

function imageCacheVersion(job: ImageWorkbenchJob) {
  return job.content_url || job.updated_at || job.id
}

function plainJob(job: ImageWorkbenchJob): ImageWorkbenchJob {
  return {
    ...job,
    ...(job.error ? { error: { ...job.error } } : {})
  }
}

function idbRequest<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error || new Error('IndexedDB request failed'))
  })
}

function idbTransaction(transaction: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    transaction.oncomplete = () => resolve()
    transaction.onerror = () => reject(transaction.error || new Error('IndexedDB transaction failed'))
    transaction.onabort = () => reject(transaction.error || new Error('IndexedDB transaction aborted'))
  })
}

function openImageCacheDB(): Promise<IDBDatabase | null> {
  if (!imageCacheSupported()) return Promise.resolve(null)
  if (imageCacheDBPromise) return imageCacheDBPromise

  imageCacheDBPromise = new Promise((resolve) => {
    let request: IDBOpenDBRequest
    try {
      request = window.indexedDB.open(IMAGE_CACHE_DB_NAME, 1)
    } catch {
      resolve(null)
      return
    }
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(IMAGE_CACHE_STORE_NAME)) {
        const store = db.createObjectStore(IMAGE_CACHE_STORE_NAME, { keyPath: 'key' })
        store.createIndex('userId', 'userId', { unique: false })
        store.createIndex('lastAccessedAt', 'lastAccessedAt', { unique: false })
      }
    }
    request.onsuccess = () => {
      const db = request.result
      db.onversionchange = () => {
        db.close()
        imageCacheDBPromise = null
      }
      resolve(db)
    }
    request.onerror = () => resolve(null)
    request.onblocked = () => resolve(null)
  })
  return imageCacheDBPromise
}

async function deleteRecords(db: IDBDatabase, keys: Iterable<string>) {
  const keyList = [...keys]
  if (!keyList.length) return
  const transaction = db.transaction(IMAGE_CACHE_STORE_NAME, 'readwrite')
  const completion = idbTransaction(transaction)
  const store = transaction.objectStore(IMAGE_CACHE_STORE_NAME)
  keyList.forEach(key => store.delete(key))
  await completion
}

async function touchRecords(db: IDBDatabase, records: ImageCacheRecord[]) {
  if (!records.length) return
  const transaction = db.transaction(IMAGE_CACHE_STORE_NAME, 'readwrite')
  const completion = idbTransaction(transaction)
  const store = transaction.objectStore(IMAGE_CACHE_STORE_NAME)
  const lastAccessedAt = Date.now()
  records.forEach(record => store.put({ ...record, lastAccessedAt }))
  await completion
}

async function cleanupImageCache(db: IDBDatabase, reserveBytes = 0, replacingKey = '') {
  const records = await idbRequest<ImageCacheRecord[]>(
    db.transaction(IMAGE_CACHE_STORE_NAME, 'readonly').objectStore(IMAGE_CACHE_STORE_NAME).getAll()
  ).catch(() => [])
  if (!records.length) return

  const now = Date.now()
  const sorted = [...records].sort((a, b) => a.lastAccessedAt - b.lastAccessedAt)
  const deleteKeys = new Set<string>()
  let totalBytes = 0
  let keptCount = 0

  for (const record of sorted) {
    if (record.key === replacingKey) {
      deleteKeys.add(record.key)
      continue
    }
    if (!record.blob || now - record.createdAt > IMAGE_CACHE_MAX_AGE_MS) {
      deleteKeys.add(record.key)
      continue
    }
    totalBytes += record.size || record.blob.size || 0
    keptCount += 1
  }

  const targetBytes = Math.max(0, IMAGE_CACHE_MAX_BYTES - reserveBytes)
  const targetEntries = Math.max(0, IMAGE_CACHE_MAX_ENTRIES - 1)
  for (const record of sorted) {
    if (deleteKeys.has(record.key)) continue
    if (keptCount <= targetEntries && totalBytes <= targetBytes) break
    deleteKeys.add(record.key)
    totalBytes -= record.size || record.blob?.size || 0
    keptCount -= 1
  }

  await deleteRecords(db, deleteKeys).catch(() => undefined)
}

export async function getCachedImageWorkbenchBlob(userId: number, job: ImageWorkbenchJob): Promise<Blob | null> {
  if (userId <= 0) return null
  const db = await openImageCacheDB()
  if (!db) return null
  const key = imageCacheKey(userId, job.id)
  const record = await idbRequest<ImageCacheRecord | undefined>(
    db.transaction(IMAGE_CACHE_STORE_NAME, 'readonly').objectStore(IMAGE_CACHE_STORE_NAME).get(key)
  ).catch(() => undefined)
  if (!record?.blob) return null

  const expired = Date.now() - record.createdAt > IMAGE_CACHE_MAX_AGE_MS
  if (expired || record.userId !== userId || record.jobId !== job.id || record.version !== imageCacheVersion(job)) {
    await deleteRecords(db, [key]).catch(() => undefined)
    return null
  }

  void touchRecords(db, [record]).catch(() => undefined)
  return record.blob
}

export async function listCachedImageWorkbenchEntries(userId: number): Promise<CachedImageWorkbenchEntry[]> {
  if (userId <= 0) return []
  const db = await openImageCacheDB()
  if (!db) return []
  const records = await idbRequest<ImageCacheRecord[]>(
    db.transaction(IMAGE_CACHE_STORE_NAME, 'readonly').objectStore(IMAGE_CACHE_STORE_NAME).index('userId').getAll(userId)
  ).catch(() => [])
  if (!records.length) return []

  const now = Date.now()
  const validRecords = records.filter(record => record.blob && record.job?.id === record.jobId && now - record.createdAt <= IMAGE_CACHE_MAX_AGE_MS)
  const invalidKeys = records.filter(record => !validRecords.includes(record)).map(record => record.key)
  if (invalidKeys.length) void deleteRecords(db, invalidKeys).catch(() => undefined)
  if (validRecords.length) void touchRecords(db, validRecords).catch(() => undefined)

  return validRecords
    .sort((a, b) => Date.parse(b.job.created_at) - Date.parse(a.job.created_at))
    .map(record => ({ job: plainJob(record.job), blob: record.blob }))
}

export async function putCachedImageWorkbenchBlob(userId: number, job: ImageWorkbenchJob, blob: Blob): Promise<void> {
  if (userId <= 0 || !blob.size || blob.size > IMAGE_CACHE_MAX_BYTES) return
  const db = await openImageCacheDB()
  if (!db) return
  const key = imageCacheKey(userId, job.id)
  await cleanupImageCache(db, blob.size, key).catch(() => undefined)

  const now = Date.now()
  const record: ImageCacheRecord = {
    key,
    userId,
    jobId: job.id,
    version: imageCacheVersion(job),
    job: plainJob(job),
    blob,
    size: blob.size,
    createdAt: now,
    lastAccessedAt: now
  }
  const transaction = db.transaction(IMAGE_CACHE_STORE_NAME, 'readwrite')
  const completion = idbTransaction(transaction)
  transaction.objectStore(IMAGE_CACHE_STORE_NAME).put(record)
  await completion
}
