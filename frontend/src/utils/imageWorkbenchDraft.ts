const DRAFT_DB_NAME = 'sub2api-image-workbench-drafts'
const DRAFT_STORE_NAME = 'drafts'
const DRAFT_VERSION = 1
const DRAFT_SESSION_KEY = 'sub2api:image-workbench:draft-session'
const DRAFT_META_PREFIX = 'sub2api:image-workbench:draft-meta'
const DRAFT_FALLBACK_MAX_CHARS = 1_500_000

export type ImageWorkbenchDraftForm = {
  apiKeyId: number
  model: string
  quality: string
  size: string
  aspectRatio: string
  width: number
  height: number
  prompt: string
  referenceUrls: string
}

export type ImageWorkbenchDraftReference = {
  name: string
  type: string
  lastModified: number
  dataURL: string
  isFile: boolean
}

export type ImageWorkbenchDraft = {
  form: ImageWorkbenchDraftForm
  referenceUrlInput: string
  references: ImageWorkbenchDraftReference[]
}

type DraftMetaRecord = {
  version: number
  userId: number
  sessionId: string
  savedAt: number
  draft: Omit<ImageWorkbenchDraft, 'references'> & {
    references: ImageWorkbenchDraftReference[]
  }
}

type DraftImageRecord = {
  index: number
  name: string
  type: string
  lastModified: number
  isFile: boolean
  blob: Blob
}

type DraftDBRecord = {
  key: string
  userId: number
  sessionId: string
  savedAt: number
  references: DraftImageRecord[]
}

let draftDBPromise: Promise<IDBDatabase | null> | null = null

function storageAvailable() {
  return typeof window !== 'undefined' && typeof window.sessionStorage !== 'undefined'
}

function indexedDBAvailable() {
  return storageAvailable() && 'indexedDB' in window && Boolean(window.indexedDB)
}

function sessionId() {
  if (!storageAvailable()) return ''
  const existing = window.sessionStorage.getItem(DRAFT_SESSION_KEY)
  if (existing) return existing
  const generated = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2)}`
  try {
    window.sessionStorage.setItem(DRAFT_SESSION_KEY, generated)
  } catch {
    return ''
  }
  return generated
}

function metaKey(userId: number, currentSessionId: string) {
  return `${DRAFT_META_PREFIX}:${userId}:${currentSessionId}`
}

function dbKey(userId: number, currentSessionId: string) {
  return `${userId}:${currentSessionId}`
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

function openDraftDB(): Promise<IDBDatabase | null> {
  if (!indexedDBAvailable()) return Promise.resolve(null)
  if (draftDBPromise) return draftDBPromise

  draftDBPromise = new Promise((resolve) => {
    let request: IDBOpenDBRequest
    try {
      request = window.indexedDB.open(DRAFT_DB_NAME, DRAFT_VERSION)
    } catch {
      resolve(null)
      return
    }
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(DRAFT_STORE_NAME)) db.createObjectStore(DRAFT_STORE_NAME, { keyPath: 'key' })
    }
    request.onsuccess = () => {
      const db = request.result
      db.onversionchange = () => {
        db.close()
        draftDBPromise = null
      }
      resolve(db)
    }
    request.onerror = () => resolve(null)
    request.onblocked = () => resolve(null)
  })
  return draftDBPromise
}

function dataURLToBlob(dataURL: string): Blob | null {
  const separator = dataURL.indexOf(',')
  if (separator < 0) return null
  const header = dataURL.slice(0, separator)
  const payload = dataURL.slice(separator + 1)
  const type = header.match(/^data:([^;,]+)/i)?.[1] || 'application/octet-stream'
  try {
    if (/;base64/i.test(header)) {
      const binary = atob(payload)
      const bytes = new Uint8Array(binary.length)
      for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index)
      return new Blob([bytes], { type })
    }
    return new Blob([decodeURIComponent(payload)], { type })
  } catch {
    return null
  }
}

function blobToDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(reader.error || new Error('Unable to restore draft image'))
    reader.readAsDataURL(blob)
  })
}

function fallbackReferences(references: ImageWorkbenchDraftReference[]) {
  const totalChars = references.reduce((total, reference) => total + reference.dataURL.length, 0)
  if (totalChars <= DRAFT_FALLBACK_MAX_CHARS) return references
  return references.map(({ dataURL: _dataURL, ...reference }) => ({ ...reference, dataURL: '' }))
}

function readMeta(userId: number, currentSessionId: string): DraftMetaRecord | null {
  if (!storageAvailable() || !currentSessionId) return null
  try {
    const raw = window.sessionStorage.getItem(metaKey(userId, currentSessionId))
    if (!raw) return null
    const record = JSON.parse(raw) as DraftMetaRecord
    if (record.version !== DRAFT_VERSION || record.userId !== userId || record.sessionId !== currentSessionId) return null
    return record
  } catch {
    return null
  }
}

async function readDBRecord(userId: number, currentSessionId: string): Promise<DraftDBRecord | null> {
  const db = await openDraftDB()
  if (!db) return null
  return idbRequest<DraftDBRecord | undefined>(
    db.transaction(DRAFT_STORE_NAME, 'readonly').objectStore(DRAFT_STORE_NAME).get(dbKey(userId, currentSessionId))
  ).then(record => record || null).catch(() => null)
}

async function writeDBRecord(userId: number, currentSessionId: string, references: ImageWorkbenchDraftReference[], savedAt: number) {
  const db = await openDraftDB()
  if (!db) return
  const imageRecords = references.flatMap((reference, index) => {
    const blob = dataURLToBlob(reference.dataURL)
    return blob ? [{ index, name: reference.name, type: reference.type, lastModified: reference.lastModified, isFile: reference.isFile, blob }] : []
  })
  const transaction = db.transaction(DRAFT_STORE_NAME, 'readwrite')
  const completion = idbTransaction(transaction)
  const store = transaction.objectStore(DRAFT_STORE_NAME)
  if (imageRecords.length) {
    const record: DraftDBRecord = {
      key: dbKey(userId, currentSessionId),
      userId,
      sessionId: currentSessionId,
      savedAt,
      references: imageRecords
    }
    store.put(record)
  } else {
    store.delete(dbKey(userId, currentSessionId))
  }
  await completion
}

export async function loadImageWorkbenchDraft(userId: number): Promise<ImageWorkbenchDraft | null> {
  if (userId <= 0) return null
  const currentSessionId = sessionId()
  const meta = readMeta(userId, currentSessionId)
  if (!meta) return null

  const dbRecord = await readDBRecord(userId, currentSessionId)
  if (dbRecord?.references?.length) {
    const restoredReferences = await Promise.all(dbRecord.references
      .sort((a, b) => a.index - b.index)
      .map(async reference => {
        try {
          return {
            name: reference.name,
            type: reference.type,
            lastModified: reference.lastModified,
            isFile: reference.isFile,
            dataURL: await blobToDataURL(reference.blob)
          }
        } catch {
          return null
        }
      }))
    const references = restoredReferences.filter((reference): reference is ImageWorkbenchDraftReference => reference !== null)
    if (references.length) return { ...meta.draft, references }
  }
  return meta.draft
}

export async function saveImageWorkbenchDraft(userId: number, draft: ImageWorkbenchDraft): Promise<void> {
  if (userId <= 0) return
  const currentSessionId = sessionId()
  if (!currentSessionId || !storageAvailable()) return
  const savedAt = Date.now()
  const record: DraftMetaRecord = {
    version: DRAFT_VERSION,
    userId,
    sessionId: currentSessionId,
    savedAt,
    draft: { ...draft, references: fallbackReferences(draft.references) }
  }
  try {
    window.sessionStorage.setItem(metaKey(userId, currentSessionId), JSON.stringify(record))
  } catch {
    try {
      window.sessionStorage.setItem(metaKey(userId, currentSessionId), JSON.stringify({
        ...record,
        draft: { ...draft, references: [] }
      }))
    } catch {
      // Draft persistence is best effort and must never block the workbench.
    }
  }
  await writeDBRecord(userId, currentSessionId, draft.references, savedAt).catch(() => undefined)
}

export async function clearImageWorkbenchDraft(userId: number): Promise<void> {
  if (userId <= 0) return
  const currentSessionId = sessionId()
  if (!currentSessionId) return
  if (storageAvailable()) {
    try {
      window.sessionStorage.removeItem(metaKey(userId, currentSessionId))
    } catch {
      // Ignore storage cleanup failures.
    }
  }
  const db = await openDraftDB()
  if (!db) return
  const transaction = db.transaction(DRAFT_STORE_NAME, 'readwrite')
  const completion = idbTransaction(transaction)
  transaction.objectStore(DRAFT_STORE_NAME).delete(dbKey(userId, currentSessionId))
  await completion.catch(() => undefined)
}
