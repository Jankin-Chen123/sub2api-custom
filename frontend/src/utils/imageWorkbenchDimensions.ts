export const IMAGE_DIMENSION_STEP = 16
export const IMAGE_DIMENSION_MIN_EDGE = 16
export const IMAGE_DIMENSION_MAX_EDGE = 3840
export const IMAGE_DIMENSION_MIN_PIXELS = 655_360
export const IMAGE_DIMENSION_MAX_PIXELS = 8_294_400
export const IMAGE_DIMENSION_EXPERIMENTAL_PIXELS = 2_560 * 1_440

export type ImageDimensionErrorCode =
  | 'not_positive_integer'
  | 'not_multiple_of_16'
  | 'edge_too_large'
  | 'aspect_ratio_too_wide'
  | 'pixels_too_few'
  | 'pixels_too_many'

export interface ImageDimensionValidation {
  code: ImageDimensionErrorCode | null
  width: number
  height: number
  pixels: number
  maxPixels: number
}

export function validateImageDimensions(width: number, height: number, maxPixels = IMAGE_DIMENSION_MAX_PIXELS): ImageDimensionValidation {
  const pixels = Number.isFinite(width) && Number.isFinite(height) ? width * height : 0
  const result = { code: null, width, height, pixels, maxPixels } as ImageDimensionValidation
  if (!Number.isInteger(width) || !Number.isInteger(height) || width <= 0 || height <= 0) {
    result.code = 'not_positive_integer'
  } else if (width % IMAGE_DIMENSION_STEP !== 0 || height % IMAGE_DIMENSION_STEP !== 0) {
    result.code = 'not_multiple_of_16'
  } else if (width > IMAGE_DIMENSION_MAX_EDGE || height > IMAGE_DIMENSION_MAX_EDGE) {
    result.code = 'edge_too_large'
  } else if (Math.max(width / height, height / width) > 3) {
    result.code = 'aspect_ratio_too_wide'
  } else if (pixels < IMAGE_DIMENSION_MIN_PIXELS) {
    result.code = 'pixels_too_few'
  } else if (pixels > Math.min(maxPixels, IMAGE_DIMENSION_MAX_PIXELS)) {
    result.code = 'pixels_too_many'
  }
  return result
}

export function isExperimentalImageDimensions(width: number, height: number) {
  return width * height > IMAGE_DIMENSION_EXPERIMENTAL_PIXELS
}
