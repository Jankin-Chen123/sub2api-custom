import { apiClient } from './client'

export type ImageWorkbenchModel = 'gpt-image-2-1k' | 'gpt-image-2-2k' | 'gpt-image-2-4k'
export type ImageWorkbenchQuality = 'auto' | 'low' | 'medium' | 'high'
export type ImageWorkbenchStatus = 'queued' | 'in_progress' | 'completed' | 'failed' | 'submission_unknown'

export interface ImageWorkbenchJob {
  id: string
  name?: string
  status: ImageWorkbenchStatus
  operation: 'generation' | 'edit'
  model: ImageWorkbenchModel
  requested_size: string
  actual_size: string
  quality?: ImageWorkbenchQuality
  estimated_cost: number
  settled_cost: number
  created_at: string
  updated_at: string
  content_url?: string
  error?: { code: string; message: string }
}

export interface CreateImageWorkbenchJobRequest {
  api_key_id: number
  operation: 'generation' | 'edit'
  model: ImageWorkbenchModel
  prompt: string
  size?: string
  aspect_ratio?: string
  quality?: ImageWorkbenchQuality
  response_format?: 'url' | 'b64_json'
  images?: string[]
  mask?: string
}

export interface ImageWorkbenchCostEstimate {
  model: ImageWorkbenchModel
  size_tier: '1K' | '2K' | '4K'
  base_cost: number
  rate_multiplier: number
  estimated_cost: number
}

export interface ImageWorkbenchJobList {
  data: ImageWorkbenchJob[]
  limit: number
  offset: number
}

export async function createJob(payload: CreateImageWorkbenchJobRequest, idempotencyKey = crypto.randomUUID()): Promise<ImageWorkbenchJob> {
  const { data } = await apiClient.post<ImageWorkbenchJob>('/user/image-workbench/jobs', payload, {
    headers: { 'Idempotency-Key': idempotencyKey }
  })
  return data
}

export async function estimateCost(payload: { api_key_id: number; model: ImageWorkbenchModel }): Promise<ImageWorkbenchCostEstimate> {
  const { data } = await apiClient.post<ImageWorkbenchCostEstimate>('/user/image-workbench/estimate', payload)
  return data
}

export async function listJobs(limit = 30, offset = 0): Promise<ImageWorkbenchJobList> {
  const { data } = await apiClient.get<ImageWorkbenchJobList>('/user/image-workbench/jobs', {
    params: { limit, offset }
  })
  return data
}

export async function getJob(id: string): Promise<ImageWorkbenchJob> {
  const { data } = await apiClient.get<ImageWorkbenchJob>(`/user/image-workbench/jobs/${encodeURIComponent(id)}`)
  return data
}

export async function getContent(id: string): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/user/image-workbench/jobs/${encodeURIComponent(id)}/content`, {
    responseType: 'blob'
  })
  return data
}

export async function renameJob(id: string, name: string): Promise<ImageWorkbenchJob> {
  const { data } = await apiClient.patch<ImageWorkbenchJob>(`/user/image-workbench/jobs/${encodeURIComponent(id)}`, { name })
  return data
}

export const imageWorkbenchAPI = { createJob, estimateCost, listJobs, getJob, getContent, renameJob }
