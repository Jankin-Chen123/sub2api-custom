import { describe, expect, it } from 'vitest'
import {
  IMAGE_WORKBENCH_MAX_FILE_BYTES,
  validateMaskFile,
  validateReferenceFiles
} from '../imageWorkbenchValidation'

function file(name: string, type: string, size = 1) {
  return new File([new Uint8Array(size)], name, { type })
}

describe('imageWorkbenchValidation', () => {
  it('accepts up to nine image references under the size limit', () => {
    expect(validateReferenceFiles(Array.from({ length: 9 }, (_, index) => file(`${index}.png`, 'image/png')))).toBeNull()
  })

  it('rejects too many, non-image, and oversized references', () => {
    expect(validateReferenceFiles(Array.from({ length: 10 }, (_, index) => file(`${index}.png`, 'image/png')))).toBe('too_many_reference_files')
    expect(validateReferenceFiles([file('notes.txt', 'text/plain')])).toBe('reference_file_not_image')
    expect(validateReferenceFiles([file('large.png', 'image/png', IMAGE_WORKBENCH_MAX_FILE_BYTES + 1)])).toBe('reference_file_too_large')
  })

  it('requires a PNG mask and applies the same size limit', () => {
    expect(validateMaskFile(file('mask.jpg', 'image/jpeg'))).toBe('mask_file_not_png')
    expect(validateMaskFile(file('large.png', 'image/png', IMAGE_WORKBENCH_MAX_FILE_BYTES + 1))).toBe('mask_file_too_large')
    expect(validateMaskFile(file('mask.png', 'image/png'))).toBeNull()
  })
})
