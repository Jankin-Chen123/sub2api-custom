import { describe, expect, it } from 'vitest'
import {
  IMAGE_DIMENSION_MAX_PIXELS,
  isExperimentalImageDimensions,
  validateImageDimensions
} from './imageWorkbenchDimensions'

describe('image workbench dimensions', () => {
  it.each([
    [1024, 1024],
    [1792, 1024],
    [2048, 1152],
    [2560, 1440],
    [3840, 2160],
    [1248, 2496]
  ])('accepts %dx%d', (width, height) => {
    expect(validateImageDimensions(width, height).code).toBeNull()
  })

  it.each([
    [1000, 1000, 'not_multiple_of_16'],
    [4096, 2304, 'edge_too_large'],
    [3072, 512, 'aspect_ratio_too_wide'],
    [512, 512, 'pixels_too_few'],
    [3840, 2304, 'pixels_too_many']
  ] as const)('rejects %dx%d with %s', (width, height, code) => {
    expect(validateImageDimensions(width, height).code).toBe(code)
  })

  it('applies the selected model tier pixel cap', () => {
    expect(validateImageDimensions(1536, 1024, 1_048_576).code).toBe('pixels_too_many')
    expect(validateImageDimensions(1024, 1024, 1_048_576).code).toBeNull()
    expect(validateImageDimensions(3840, 2160, IMAGE_DIMENSION_MAX_PIXELS).code).toBeNull()
  })

  it('marks output above 2560x1440 as experimental', () => {
    expect(isExperimentalImageDimensions(2560, 1440)).toBe(false)
    expect(isExperimentalImageDimensions(2560, 1456)).toBe(true)
  })
})
