import { describe, expect, it } from 'vitest'
import {
  IMAGE_WORKBENCH_MODELS,
  imageWorkbenchModelCapabilities,
  isImageWorkbenchAspectRatioAllowed,
  isImageWorkbenchQualityAllowed
} from './imageWorkbenchModelCapabilities'

describe('image workbench model capabilities', () => {
  it('registers all fixed GPT Image 2 and GPT Image 2.5 resolution models', () => {
    expect(IMAGE_WORKBENCH_MODELS.map(model => model.value)).toEqual([
      'gpt-image-2-1k',
      'gpt-image-2-2k',
      'gpt-image-2-4k',
      'gpt-image-2.5-flare-1k',
      'gpt-image-2.5-flare-2k',
      'gpt-image-2.5-flare-4k',
      'gpt-image-2.5-sunburst-1k',
      'gpt-image-2.5-sunburst-2k',
      'gpt-image-2.5-sunburst-4k'
    ])
  })

  it.each(IMAGE_WORKBENCH_MODELS)('uses the documented controls for $value', model => {
    expect(model.qualities).toEqual(['low', 'medium', 'high', 'xhigh', 'max'])
    expect(model.qualities).not.toContain('auto')
    expect(model.aspectRatios).toEqual(['1:1', '16:9', '9:16', '3:2', '2:3', '4:3', '3:4', '21:9'])
    expect(isImageWorkbenchQualityAllowed(model.value, 'max')).toBe(true)
    expect(isImageWorkbenchQualityAllowed(model.value, 'auto')).toBe(false)
    expect(isImageWorkbenchAspectRatioAllowed(model.value, '21:9')).toBe(true)
    expect(isImageWorkbenchAspectRatioAllowed(model.value, '5:4')).toBe(false)
  })

  it('changes the pixel budget with the selected resolution tier', () => {
    expect(imageWorkbenchModelCapabilities('gpt-image-2.5-flare-1k').maxPixels).toBe(1_048_576)
    expect(imageWorkbenchModelCapabilities('gpt-image-2.5-flare-2k').maxPixels).toBe(4_194_304)
    expect(imageWorkbenchModelCapabilities('gpt-image-2.5-flare-4k').maxPixels).toBe(8_294_400)
  })
})
