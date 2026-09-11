import type { ImageWorkbenchModel, ImageWorkbenchQuality } from '@/api'
import { IMAGE_DIMENSION_MAX_PIXELS } from './imageWorkbenchDimensions'

export type SelectableImageWorkbenchQuality = Exclude<ImageWorkbenchQuality, 'auto'>

export interface ImageWorkbenchModelCapabilities {
  value: ImageWorkbenchModel
  label: string
  tier: '1K' | '2K' | '4K'
  defaultSize: string
  defaultQuality: SelectableImageWorkbenchQuality
  maxPixels: number
  qualities: readonly SelectableImageWorkbenchQuality[]
  aspectRatios: readonly string[]
}

// Cangyuan's fixed-resolution GPT Image models currently share the same
// documented quality and ratio sets. Keep these capabilities attached to each
// model so the workbench remains correct if a model family diverges later.
const FIXED_MODEL_QUALITIES = ['low', 'medium', 'high', 'xhigh', 'max'] as const
const FIXED_MODEL_ASPECT_RATIOS = ['1:1', '16:9', '9:16', '3:2', '2:3', '4:3', '3:4', '21:9'] as const

export const IMAGE_WORKBENCH_MODELS: readonly ImageWorkbenchModelCapabilities[] = [
  imageModel('gpt-image-2-1k', '1K · gpt-image-2-1k', '1K', '1024x1024', 1_048_576),
  imageModel('gpt-image-2-2k', '2K · gpt-image-2-2k', '2K', '2048x2048', 4_194_304),
  imageModel('gpt-image-2-4k', '4K · gpt-image-2-4k', '4K', '3840x2160', IMAGE_DIMENSION_MAX_PIXELS),
  imageModel('gpt-image-2.5-flare-1k', '1K · GPT Image 2.5 Flare', '1K', '1024x1024', 1_048_576),
  imageModel('gpt-image-2.5-flare-2k', '2K · GPT Image 2.5 Flare', '2K', '2048x2048', 4_194_304),
  imageModel('gpt-image-2.5-flare-4k', '4K · GPT Image 2.5 Flare', '4K', '3840x2160', IMAGE_DIMENSION_MAX_PIXELS),
  imageModel('gpt-image-2.5-sunburst-1k', '1K · GPT Image 2.5 Sunburst', '1K', '1024x1024', 1_048_576),
  imageModel('gpt-image-2.5-sunburst-2k', '2K · GPT Image 2.5 Sunburst', '2K', '2048x2048', 4_194_304),
  imageModel('gpt-image-2.5-sunburst-4k', '4K · GPT Image 2.5 Sunburst', '4K', '3840x2160', IMAGE_DIMENSION_MAX_PIXELS)
]

const MODEL_CAPABILITIES = new Map(IMAGE_WORKBENCH_MODELS.map(model => [model.value, model]))

function imageModel(
  value: ImageWorkbenchModel,
  label: string,
  tier: ImageWorkbenchModelCapabilities['tier'],
  defaultSize: string,
  maxPixels: number
): ImageWorkbenchModelCapabilities {
  return {
    value,
    label,
    tier,
    defaultSize,
    defaultQuality: 'medium',
    maxPixels,
    qualities: FIXED_MODEL_QUALITIES,
    aspectRatios: FIXED_MODEL_ASPECT_RATIOS
  }
}

export function imageWorkbenchModelCapabilities(model: ImageWorkbenchModel): ImageWorkbenchModelCapabilities {
  return MODEL_CAPABILITIES.get(model) || IMAGE_WORKBENCH_MODELS[0]!
}

export function isImageWorkbenchQualityAllowed(model: ImageWorkbenchModel, quality: unknown): quality is SelectableImageWorkbenchQuality {
  return imageWorkbenchModelCapabilities(model).qualities.includes(quality as SelectableImageWorkbenchQuality)
}

export function isImageWorkbenchAspectRatioAllowed(model: ImageWorkbenchModel, ratio: unknown): ratio is string {
  return typeof ratio === 'string' && imageWorkbenchModelCapabilities(model).aspectRatios.includes(ratio)
}
