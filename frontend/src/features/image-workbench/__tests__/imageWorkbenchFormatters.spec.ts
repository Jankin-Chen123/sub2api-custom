import { describe, expect, it } from 'vitest'
import {
  clampDimension,
  computeAspectRatioSize,
  dataURLByteLength,
  dimensionErrorKey,
  formatCost,
  formatJobWaitTime,
  isTerminalJob,
  jobQualityOption,
  jobResolution,
  parseSize,
  sanitizeDownloadFileName,
  snapDimension,
  statusClass,
} from '../imageWorkbenchFormatters'

describe('imageWorkbenchFormatters', () => {
  it('parses size strings and falls back to 1024x1024', () => {
    expect(parseSize('1536x1024')).toEqual([1536, 1024])
    expect(parseSize(' 3840X2160 ')).toEqual([3840, 2160])
    expect(parseSize('invalid')).toEqual([1024, 1024])
  })

  it('clamps and snaps dimensions to model step and bounds', () => {
    expect(snapDimension(1025, 'nearest')).toBe(1024)
    expect(snapDimension(1025, 'up')).toBe(1040)
    expect(snapDimension(1039, 'down')).toBe(1024)

    expect(clampDimension(8, 1760)).toBe(16)
    expect(clampDimension(100, 1760)).toBe(96)
    expect(clampDimension(2400, 1760)).toBe(1760)
    expect(clampDimension(Number.NaN, 1760)).toBe(1024)
  })

  it('computes base64 data URL byte lengths accurately', () => {
    expect(dataURLByteLength('data:image/png;base64,cG5n')).toBe(3) // "png"
    expect(dataURLByteLength('data:image/png;base64,aW1hZ2U=')).toBe(5) // "image"
    expect(dataURLByteLength('data:image/png;base64,YQ==')).toBe(1) // "a"
  })

  it('formats status badge classes, resolution, quality, cost, and filenames', () => {
    expect(statusClass('completed')).toContain('green')
    expect(statusClass('failed')).toContain('red')
    expect(statusClass('queued')).toContain('blue')

    expect(jobResolution({ model: 'gpt-image-2.5-sunburst-4k' })).toBe('4K')
    expect(jobResolution({ model: 'custom-model' })).toBe('—')

    expect(jobQualityOption({ model: 'gpt-image-2-1k', quality: 'high' })).toBe('high')
    expect(jobQualityOption({ model: 'gpt-image-2-1k', quality: 'auto' })).toBe('auto')

    expect(sanitizeDownloadFileName('sunset/lake:2026*', 'job_1')).toBe('sunset_lake_2026_.png')
    expect(sanitizeDownloadFileName('   ', 'job_1')).toBe('job_1.png')

    expect(formatCost(0.12)).toBe('0.1200')
    expect(formatCost(0)).toBe('0.0000')
  })

  it('formats wait time using updated_at for terminal jobs and live clock otherwise', () => {
    expect(isTerminalJob('completed')).toBe(true)
    expect(isTerminalJob('failed')).toBe(true)
    expect(isTerminalJob('in_progress')).toBe(false)

    const createdAt = '2026-08-04T00:00:00.000Z'
    const updatedAt = '2026-08-04T00:01:05.000Z'
    const liveClock = Date.parse('2026-08-04T00:02:10.000Z')

    expect(formatJobWaitTime({ status: 'completed', created_at: createdAt, updated_at: updatedAt }, liveClock)).toBe('01:05')
    expect(formatJobWaitTime({ status: 'in_progress', created_at: createdAt, updated_at: updatedAt }, liveClock)).toBe('02:10')
    expect(formatJobWaitTime({ status: 'queued', created_at: 'bad-date', updated_at: updatedAt }, liveClock)).toBe('00:00')
  })

  it('computes bounded aspect ratio dimensions and resolves dimension error keys', () => {
    expect(computeAspectRatioSize(1024, '', 1760, 1_048_576)).toBeNull()
    expect(computeAspectRatioSize(1536, '16:9', 1760, 1_048_576)).toEqual([1360, 768])
    expect(computeAspectRatioSize(3840, '16:9', 3840, 8_294_400)).toEqual([3840, 2160])

    expect(dimensionErrorKey('edge_too_large')).toBe('imageWorkbench.form.dimensionErrors.maxEdge')
    expect(dimensionErrorKey('pixels_too_many')).toBe('imageWorkbench.form.dimensionErrors.maxPixels')
    expect(dimensionErrorKey('unknown_code')).toBe('imageWorkbench.form.dimensionErrors.invalid')
  })
})
