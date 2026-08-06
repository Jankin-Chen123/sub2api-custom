export default {
  imageWorkbench: {
    title: '生图工作台',
    description: '使用你自己的 Sub2API 密钥调用组内沧元专用生图账号，支持原分辨率 1K、2K 和 4K 输出。',
    notices: {
      originalResolution: '分辨率由模型档位决定：4K 使用 gpt-image-2-4k，最长边上限为 3840，不会把 4K 猜成 4096×4096。',
      billable: '每次提交都会产生上游生图费用，并按图片张数结算；任务提交后即使关闭页面也会继续完成。',
      textLimit: '复杂中文小字、严谨关系图和精确拼写仍可能不稳定，请生成后人工核对。',
      fourK: '4K 图片耗时更长、费用更高，也会占用更多下载和对象存储资源。'
    },
    form: {
      quality: '质量', qualityOptions: { auto: '自动', low: '低', medium: '中', high: '高' },
      dimensionMode: '尺寸方式', dimensionModes: { size: '指定尺寸', aspectRatio: '宽高比' },
      aspectRatio: '宽高比', responseFormat: '上游响应格式', responseFormats: { url: 'URL', b64Json: 'Base64 JSON' },
      loadingEstimate: '正在获取费用预估...', estimateUnavailable: '暂时无法获取费用预估；提交前仍会再次校验最终价格。',
      estimatedCost: '预估费用：${cost}',
      apiKey: 'Sub2API 密钥', selectApiKey: '请选择可生图的密钥', noApiKey: '暂无 OpenAI 生图权限的可用密钥。',
      model: '分辨率档位', size: '输出尺寸', prompt: '提示词', promptPlaceholder: '描述主体、构图、风格、光线、文字和必须避免的内容……',
      referenceUrls: '参考图（可选）', referenceUrlsPlaceholder: '每行一个 HTTPS 图片地址，或在下方上传；合计最多 9 张。',
      mask: '编辑蒙版（可选）', maskPlaceholder: 'HTTPS PNG 蒙版地址', maskHint: '蒙版必须是带 Alpha 的 PNG，并与第一张参考图尺寸一致。', promptSubmitHint: '提示词输入框中按 Ctrl/⌘ + Enter 可提交'
    },
    actions: { generate: '确认费用并开始生图', submitting: '正在提交……', loadPreview: '加载预览', preview: '刷新预览', download: '下载原图' },
    jobs: { title: '最近任务', autoRefresh: '进行中的任务每 2 秒自动刷新', empty: '还没有生图任务', size: '实际尺寸', cost: '费用', createdAt: '创建时间' },
    status: { queued: '排队中', in_progress: '生成中', completed: '已完成', failed: '失败', submission_unknown: '提交结果待核查' },
    messages: { submitted: '任务已提交，离开页面后仍会继续执行。' },
    errors: { loadKeys: '加载 API 密钥失败', loadJobs: '加载生图任务失败', submit: '提交生图任务失败', fileTooLarge: '单张参考图不能超过 10 MB', invalidReferenceType: '参考图必须是图片文件', tooManyReferenceFiles: '最多只能选择 9 张参考图', invalidMask: '蒙版必须是 10 MB 以内的 PNG', preview: '加载预览失败', download: '下载图片失败' }
  }
}
