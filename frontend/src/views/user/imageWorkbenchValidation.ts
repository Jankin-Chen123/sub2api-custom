export const IMAGE_WORKBENCH_MAX_REFERENCE_FILES = 9
export const IMAGE_WORKBENCH_MAX_FILE_BYTES = 10 * 1024 * 1024

export type ImageWorkbenchFileError =
  | 'too_many_reference_files'
  | 'reference_file_not_image'
  | 'reference_file_too_large'
  | 'mask_file_not_png'
  | 'mask_file_too_large'

export function validateReferenceFiles(files: File[]): ImageWorkbenchFileError | null {
  if (files.length > IMAGE_WORKBENCH_MAX_REFERENCE_FILES) return 'too_many_reference_files'
  if (files.some(file => !file.type.toLowerCase().startsWith('image/'))) return 'reference_file_not_image'
  if (files.some(file => file.size > IMAGE_WORKBENCH_MAX_FILE_BYTES)) return 'reference_file_too_large'
  return null
}

export function validateMaskFile(file: File): ImageWorkbenchFileError | null {
  if (file.type.toLowerCase() !== 'image/png') return 'mask_file_not_png'
  if (file.size > IMAGE_WORKBENCH_MAX_FILE_BYTES) return 'mask_file_too_large'
  return null
}
