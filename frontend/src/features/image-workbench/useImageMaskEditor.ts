import type { Ref } from 'vue'
import type { ImageWorkbenchJob } from '@/api'
import { IMAGE_DIMENSION_MIN_EDGE } from '@/utils/imageWorkbenchDimensions'
import { dataURLByteLength } from './imageWorkbenchFormatters'

export type BrushPoint = { x: number; y: number }
export type BrushStroke = { points: BrushPoint[]; size: number }

export interface ImageMaskEditorState {
  open: boolean
  job: ImageWorkbenchJob | null
  originalDataURL: string
  width: number
  height: number
  prompt: string
  brushSize: number
  mode: 'brush'
  drawing: boolean
  hasMarks: boolean
}

export interface UseImageMaskEditorOptions {
  editor: ImageMaskEditorState
  editorCanvasRef: Ref<HTMLCanvasElement | null>
  editorMaskCanvasRef: Ref<HTMLCanvasElement | null>
  editorImageRef: Ref<HTMLImageElement | null>
  maxReferenceBytes: number
  fileTooLargeMessage: () => string
}

export function useImageMaskEditor(options: UseImageMaskEditorOptions) {
  const {
    editor,
    editorCanvasRef,
    editorMaskCanvasRef,
    editorImageRef,
    maxReferenceBytes,
    fileTooLargeMessage,
  } = options

  const brushStrokes: BrushStroke[] = []

  function clearEditorMarks() {
    const canvas = editorCanvasRef.value
    const maskCanvas = editorMaskCanvasRef.value
    canvas?.getContext('2d')?.clearRect(0, 0, canvas.width, canvas.height)
    maskCanvas?.getContext('2d')?.clearRect(0, 0, maskCanvas.width, maskCanvas.height)
    brushStrokes.length = 0
    editor.hasMarks = false
    editor.drawing = false
  }

  function setupEditorCanvas() {
    const image = editorImageRef.value
    const canvas = editorCanvasRef.value
    const maskCanvas = editorMaskCanvasRef.value
    if (!image || !canvas || !maskCanvas || !image.naturalWidth || !image.naturalHeight) return
    editor.width = image.naturalWidth
    editor.height = image.naturalHeight
    canvas.width = image.naturalWidth
    canvas.height = image.naturalHeight
    maskCanvas.width = image.naturalWidth
    maskCanvas.height = image.naturalHeight
    clearEditorMarks()
  }

  function pointFromEvent(event: PointerEvent): BrushPoint | null {
    const canvas = editorCanvasRef.value
    if (!canvas) return null
    const rect = canvas.getBoundingClientRect()
    return {
      x: Math.max(0, Math.min(canvas.width, ((event.clientX - rect.left) * canvas.width) / rect.width)),
      y: Math.max(0, Math.min(canvas.height, ((event.clientY - rect.top) * canvas.height) / rect.height)),
    }
  }

  function drawSmoothStroke(ctx: CanvasRenderingContext2D, stroke: BrushStroke, color: string) {
    const points = stroke.points
    if (!points.length) return
    ctx.lineCap = 'round'
    ctx.lineJoin = 'round'
    ctx.strokeStyle = color
    ctx.lineWidth = stroke.size
    ctx.beginPath()
    ctx.moveTo(points[0].x, points[0].y)
    if (points.length === 1) {
      ctx.arc(points[0].x, points[0].y, stroke.size / 2, 0, Math.PI * 2)
      ctx.fillStyle = color
      ctx.fill()
      return
    }
    if (points.length === 2) {
      ctx.lineTo(points[1].x, points[1].y)
    } else {
      for (let index = 1; index < points.length - 1; index += 1) {
        const current = points[index]!
        const next = points[index + 1]!
        const midpointX = (current.x + next.x) / 2
        const midpointY = (current.y + next.y) / 2
        ctx.quadraticCurveTo(current.x, current.y, midpointX, midpointY)
      }
      const last = points[points.length - 1]!
      ctx.quadraticCurveTo(last.x, last.y, last.x, last.y)
    }
    ctx.stroke()
  }

  function redrawBrushStrokes() {
    const canvas = editorCanvasRef.value
    const maskCanvas = editorMaskCanvasRef.value
    if (!canvas || !maskCanvas) return
    const visual = canvas.getContext('2d')
    const mask = maskCanvas.getContext('2d')
    if (!visual || !mask) return
    visual.clearRect(0, 0, canvas.width, canvas.height)
    mask.clearRect(0, 0, maskCanvas.width, maskCanvas.height)
    mask.save()
    mask.globalCompositeOperation = 'source-over'
    mask.fillStyle = '#ffffff'
    mask.fillRect(0, 0, maskCanvas.width, maskCanvas.height)
    mask.globalCompositeOperation = 'destination-out'
    for (const stroke of brushStrokes) {
      drawSmoothStroke(visual, stroke, 'rgba(239, 68, 68, 0.72)')
      drawSmoothStroke(mask, stroke, '#000000')
    }
    mask.restore()
  }

  function startDrawing(event: PointerEvent) {
    if (editor.mode !== 'brush') return
    const point = pointFromEvent(event)
    if (!point) return
    editor.drawing = true
    brushStrokes.push({ points: [point], size: Math.max(1, Number(editor.brushSize) || 1) })
    editor.hasMarks = true
    redrawBrushStrokes()
    editorCanvasRef.value?.setPointerCapture(event.pointerId)
  }

  function draw(event: PointerEvent) {
    if (!editor.drawing) return
    const point = pointFromEvent(event)
    if (!point) return
    const stroke = brushStrokes[brushStrokes.length - 1]
    if (!stroke) return
    const previous = stroke.points[stroke.points.length - 1]
    if (!previous || Math.hypot(point.x - previous.x, point.y - previous.y) >= 0.5) {
      stroke.points.push(point)
      redrawBrushStrokes()
    }
  }

  function stopDrawing() {
    editor.drawing = false
  }

  async function createEditableReferenceDataURL(blob: Blob): Promise<string> {
    const bitmap = await createImageBitmap(blob)
    let canvas = document.createElement('canvas')
    canvas.width = bitmap.width
    canvas.height = bitmap.height
    const context = canvas.getContext('2d')
    if (!context) {
      bitmap.close()
      throw new Error('Canvas 2D context is unavailable')
    }
    context.drawImage(bitmap, 0, 0)
    bitmap.close()

    let dataURL = canvas.toDataURL('image/png')
    let byteLength = dataURLByteLength(dataURL)
    while (byteLength > maxReferenceBytes) {
      const scale = Math.min(0.9, Math.sqrt(maxReferenceBytes / byteLength) * 0.95)
      const width = Math.max(IMAGE_DIMENSION_MIN_EDGE, Math.floor(canvas.width * scale))
      const height = Math.max(IMAGE_DIMENSION_MIN_EDGE, Math.floor(canvas.height * scale))
      if (width === canvas.width && height === canvas.height) break

      const resized = document.createElement('canvas')
      resized.width = width
      resized.height = height
      const resizedContext = resized.getContext('2d')
      if (!resizedContext) throw new Error('Canvas 2D context is unavailable')
      resizedContext.drawImage(canvas, 0, 0, width, height)
      canvas = resized
      dataURL = canvas.toDataURL('image/png')
      byteLength = dataURLByteLength(dataURL)
    }
    if (byteLength > maxReferenceBytes) throw new Error(fileTooLargeMessage())
    return dataURL
  }

  return {
    setupEditorCanvas,
    pointFromEvent,
    drawSmoothStroke,
    redrawBrushStrokes,
    startDrawing,
    draw,
    stopDrawing,
    clearEditorMarks,
    createEditableReferenceDataURL,
  }
}
