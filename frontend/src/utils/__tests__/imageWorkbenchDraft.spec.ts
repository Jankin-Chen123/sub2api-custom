import { beforeEach, describe, expect, it } from 'vitest'
import {
  loadImageWorkbenchDraft,
  saveImageWorkbenchDraft
} from '../imageWorkbenchDraft'

const draft = {
  form: {
    apiKeyId: 7,
    model: 'gpt-image-2-2k',
    quality: 'high',
    size: '1536x1024',
    aspectRatio: '3:2',
    width: 1536,
    height: 1024,
    prompt: 'a red fox in snow',
    referenceUrls: 'https://example.com/reference.png'
  },
  referenceUrlInput: 'https://example.com/next.png',
  references: [{
    name: 'reference.png',
    type: 'image/png',
    lastModified: 123,
    dataURL: 'data:image/png;base64,aW1hZ2U=',
    isFile: true
  }]
}

describe('imageWorkbenchDraft', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  it('restores the draft within the same browser tab session', async () => {
    await saveImageWorkbenchDraft(42, draft)

    await expect(loadImageWorkbenchDraft(42)).resolves.toEqual(draft)
  })

  it('does not restore a previous draft after the tab session is removed', async () => {
    await saveImageWorkbenchDraft(42, draft)
    sessionStorage.removeItem('sub2api:image-workbench:draft-session')

    await expect(loadImageWorkbenchDraft(42)).resolves.toBeNull()
  })
})
