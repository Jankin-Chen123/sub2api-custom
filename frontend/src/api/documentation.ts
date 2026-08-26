import { apiClient, buildApiUrl } from './client'

export interface DocumentationHeading {
  level: number
  title: string
  id: string
}

export interface DocumentationAsset {
  path: string
  sha256: string
  bytes: number
  width: number
  height: number
}

export interface DocumentationManifest {
  id: string
  title: string
  source_file: string
  created_at: string
  published_at?: string
  content_sha256: string
  content_bytes: number
  assets: DocumentationAsset[]
  outline: DocumentationHeading[]
  warnings: string[]
}

export interface DocumentationChanges {
  has_active: boolean
  content_changed: boolean
  assets_added: number
  assets_removed: number
  assets_changed: number
}

export interface DocumentationPreview {
  draft_id: string
  manifest: DocumentationManifest
  markdown: string
  changes: DocumentationChanges
}

export interface DocumentationVersion {
  manifest: DocumentationManifest
  active: boolean
}

export interface DocumentationState {
  active?: DocumentationManifest
  versions: DocumentationVersion[]
}

export async function getActiveDocumentation(): Promise<DocumentationManifest> {
  const { data } = await apiClient.get<DocumentationManifest>('/docs')
  return data
}

export async function getDocumentationContent(versionID: string): Promise<string> {
  const { data } = await apiClient.get<string>(`/docs/versions/${encodeURIComponent(versionID)}/content`, {
    responseType: 'text'
  })
  return data
}

export function documentationAssetBase(versionID: string): string {
  return buildApiUrl(`/docs/versions/${encodeURIComponent(versionID)}`)
}

export function documentationPreviewAssetBase(draftID: string): string {
  return buildApiUrl(`/docs/previews/${encodeURIComponent(draftID)}`)
}

export async function getDocumentationState(): Promise<DocumentationState> {
  const { data } = await apiClient.get<DocumentationState>('/admin/docs')
  return data
}

export async function importDocumentation(
  file: File,
  onProgress?: (percent: number) => void
): Promise<DocumentationPreview> {
  const form = new FormData()
  form.append('file', file)
  const { data } = await apiClient.post<DocumentationPreview>('/admin/docs/import', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 120000,
    onUploadProgress: (event) => {
      if (event.total && onProgress) {
        onProgress(Math.min(100, Math.round((event.loaded / event.total) * 100)))
      }
    }
  })
  return data
}

export async function publishDocumentationDraft(draftID: string): Promise<DocumentationManifest> {
  const { data } = await apiClient.post<DocumentationManifest>(
    `/admin/docs/drafts/${encodeURIComponent(draftID)}/publish`
  )
  return data
}

export async function activateDocumentationVersion(versionID: string): Promise<DocumentationManifest> {
  const { data } = await apiClient.post<DocumentationManifest>(
    `/admin/docs/versions/${encodeURIComponent(versionID)}/activate`
  )
  return data
}
