export default {
  imageWorkbench: {
    title: 'Image Workbench',
    description: 'Use your Sub2API key with the dedicated Cangyuan image pool for native 1K, 2K, and 4K output.',
    notices: {
      originalResolution: 'Resolution is selected by model tier. 4K uses gpt-image-2-4k with a maximum edge of 3840; it is not treated as 4096×4096.',
      billable: 'Every submission incurs upstream image cost and is billed per image. Submitted jobs continue if you close this page.',
      textLimit: 'Dense small text, exact diagrams, and precise spelling can still be unreliable. Review generated images manually.',
      fourK: '4K images take longer, cost more, and use more download and object-storage capacity.'
    },
    form: {
      apiKey: 'Sub2API key', selectApiKey: 'Select an image-enabled key', noApiKey: 'No active OpenAI key with image permission is available.',
      quality: 'Quality', qualityOptions: { auto: 'Auto', low: 'Low', medium: 'Medium', high: 'High' },
      dimensionMode: 'Dimension mode', dimensionModes: { size: 'Exact size', aspectRatio: 'Aspect ratio' },
      aspectRatio: 'Aspect ratio', responseFormat: 'Upstream response format', responseFormats: { url: 'URL', b64Json: 'Base64 JSON' },
      loadingEstimate: 'Loading estimated cost...', estimateUnavailable: 'Cost estimate unavailable; the final price is checked before submission.',
      estimatedCost: 'Estimated cost: ${cost}',
      model: 'Resolution tier', size: 'Output size', prompt: 'Prompt', promptPlaceholder: 'Describe the subject, composition, style, lighting, text, and exclusions…',
      referenceUrls: 'Reference images (optional)', referenceUrlsPlaceholder: 'One HTTPS image URL per line, or upload below; 9 images maximum in total.',
      mask: 'Edit mask (optional)', maskPlaceholder: 'HTTPS PNG mask URL', maskHint: 'The mask must be an alpha PNG with the same dimensions as the first reference image.', promptSubmitHint: 'Press Ctrl/⌘ + Enter in the prompt to submit'
    },
    actions: { generate: 'Confirm cost and generate', submitting: 'Submitting…', loadPreview: 'Load preview', preview: 'Refresh preview', download: 'Download original' },
    jobs: { title: 'Recent jobs', autoRefresh: 'Active jobs refresh every 2 seconds', empty: 'No image jobs yet', size: 'Actual size', cost: 'Cost', createdAt: 'Created' },
    status: { queued: 'Queued', in_progress: 'Generating', completed: 'Completed', failed: 'Failed', submission_unknown: 'Submission needs review' },
    messages: { submitted: 'Job submitted. It will continue if you leave this page.' },
    errors: { loadKeys: 'Failed to load API keys', loadJobs: 'Failed to load image jobs', submit: 'Failed to submit image job', fileTooLarge: 'Each reference image must be 10 MB or smaller', invalidReferenceType: 'Reference files must be images', tooManyReferenceFiles: 'Select no more than 9 reference images', invalidMask: 'Mask must be a PNG no larger than 10 MB', preview: 'Failed to load preview', download: 'Failed to download image' }
  }
}
