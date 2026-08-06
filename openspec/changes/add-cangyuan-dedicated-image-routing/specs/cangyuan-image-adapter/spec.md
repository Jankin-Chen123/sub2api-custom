## ADDED Requirements

### Requirement: 第一版专用适配器必须只实现沧元协议
系统 SHALL 为 `image_only` 账号使用固定的沧元适配器，支持 `POST /v1/images/generations`、`POST /v1/images/edits`、`GET /v1/images/generations/{task_id}` 和 `GET /v1/images/edits/{task_id}`。Base URL MUST 安全归一，不能产生重复 `/v1/v1`。系统 MUST NOT 在第一版增加通用 provider 类型或让管理员自定义任意端点模板。

#### Scenario: Base URL 已包含 v1
- **WHEN** 账号 Base URL 以 `/v1` 结尾
- **THEN** 适配器 MUST 生成单一 `/v1/images/...` 路径
- **THEN** 请求 MUST NOT 发往 `/v1/v1/images/...`

#### Scenario: 查询 edit 任务
- **WHEN** 一个 edit 任务已获得上游 task ID
- **THEN** 适配器 MUST 使用 edits 查询端点
- **THEN** 系统 MUST NOT 使用 generations 查询端点猜测

### Requirement: 分辨率档位必须由模型名决定
系统 MUST 使用 `gpt-image-2-1k`、`gpt-image-2-2k`、`gpt-image-2-4k` 表达档位。`quality` MUST NOT 用于切换档位。公共模型、映射模型、`image_size` 和 `output_resolution` MUST 一致；冲突或无法唯一解析 MUST 在上游调用前返回 `image_model_not_allowed` 或 `image_invalid_size`。

#### Scenario: 请求 4K 16:9
- **WHEN** 请求模型映射为 `gpt-image-2-4k` 且尺寸为 `3840x2160`
- **THEN** 适配器 MUST 保留该模型和尺寸
- **THEN** 系统 MUST NOT 改写为 4096 边长或依赖 high quality

#### Scenario: 模型与 output_resolution 冲突
- **WHEN** 上游模型为 2K 但请求显式声明 4K 输出档位
- **THEN** 系统 MUST 在提交前返回 400
- **THEN** 系统 MUST NOT 自动升降档并产生不同费用

### Requirement: 尺寸必须在提交前完整校验
宽高 MUST 为 16 的倍数，最长边 MUST 不超过 3840，长宽比 MUST 不超过 3:1，总像素 MUST 不少于 655360。1K、2K、4K 模型的总像素上限 MUST 分别为 1048576、4194304、8294400。tier 模型的 `n` MUST 等于 1。

#### Scenario: 尺寸违反多个条件
- **WHEN** 输入尺寸同时违反倍数和像素上限
- **THEN** 系统 MUST 返回稳定 400 错误并指出客户端可修正的字段
- **THEN** 系统 MUST NOT调用上游或预结算成功费用

#### Scenario: 用户请求多张图片
- **WHEN** 公开 API 或工作台请求图片数量大于 1
- **THEN** 编排层 MUST 明确拆成多个各自 `n=1` 的任务或拒绝
- **THEN** 单次沧元 tier 请求 MUST NOT 发送 `n>1`

### Requirement: 参考图和 mask 必须受限且安全处理
系统 SHALL 支持文档规定的 JSON 参考图别名和 multipart 重复 `image`，去重后最多 9 张且单张不超过 10 MB。远程参考图和 mask MUST 仅接受 HTTPS 并通过受控下载器；下载器 MUST 限制解析地址、重定向、超时、响应大小、MIME 和图片解码资源。mask MUST 是单个带 alpha 的 PNG/HTTPS 资源，与第一张输入图格式和尺寸一致且不超过 10 MB。

JSON 图生图允许继续使用 `POST /v1/images/generations`；multipart 文件编辑使用 `POST /v1/images/edits`。`size` 可使用精确 `WIDTHxHEIGHT` 或沧元支持的 `W:H` 比例，也可单独使用 `aspect_ratio`，两者不得同时提供。

#### Scenario: JSON 使用 reference_images
- **WHEN** 请求以受支持别名传入不超过 9 张合法参考图
- **THEN** 适配器 MUST 归一为同一内部表示
- **THEN** 重复内容 MUST 只计一次数量

#### Scenario: 远程 URL 重定向到内网
- **WHEN** HTTPS 参考图在解析或重定向后指向私网、回环、link-local 或元数据地址
- **THEN** 下载器 MUST 拒绝请求并返回 `image_invalid_reference`
- **THEN** 系统 MUST NOT 向该地址发出内容请求

#### Scenario: mask 与第一张图不匹配
- **WHEN** mask 缺少 alpha 或尺寸/格式与第一张输入图不同
- **THEN** 系统 MUST 在上游提交前拒绝

### Requirement: 同步和异步结果必须归一并持久转存
适配器 SHALL 接受同步 `url` 或 `b64_json` 结果以及异步完成 URL。所有成功结果 MUST 转存至配置的对象存储后才进入 `completed`；公开结果 MUST 使用 Sub2API 管理的 URL 或鉴权内容端点，不依赖上游临时 URL。

#### Scenario: 上游返回 base64
- **WHEN** 上游同步返回 `b64_json`
- **THEN** 系统 MUST 在大小限制内解码并流式上传对象存储
- **THEN** 数据库、Redis 和日志 MUST NOT 长期保存 base64

#### Scenario: 上游完成 URL 即将过期
- **WHEN** 异步查询返回完成 URL
- **THEN** Worker MUST 立即受控下载并转存
- **THEN** 上游 URL MUST NOT 作为持久公开结果保存

### Requirement: 上游错误必须稳定归一且不可泄密
系统 MUST 将认证、额度、限流、参数、超时、服务错误、无效响应和未知提交分别归一为稳定内部错误。公开错误 MUST NOT 包含上游 Key、账号 ID、Base URL、task ID、完整响应正文、签名 URL 或内部堆栈。

#### Scenario: 沧元返回认证失败
- **WHEN** 上游返回 401/403
- **THEN** 账号健康状态 MAY 按现有机制记录异常
- **THEN** 客户端 MUST 只收到脱敏稳定错误

#### Scenario: 提交连接在发送后断开
- **WHEN** 系统不能确认上游是否已创建任务
- **THEN** 任务 MUST 进入 `submission_unknown` 或等价人工可查状态
- **THEN** 系统 MUST NOT 自动换账号重提

### Requirement: 适配器必须以真实契约测试为准
开发前 MUST 对沧元实际协议完成脱敏 POC，并将响应 fixture 纳入契约测试。文档与实际响应不一致时 MUST 先修订 OpenSpec，再实现；不得通过宽松解析把未知响应当作成功。

#### Scenario: 上游响应字段发生变化
- **WHEN** 契约测试发现状态或结果结构不符合已确认 fixture
- **THEN** 适配器 MUST 返回无效上游响应
- **THEN** 任务 MUST NOT 被标记完成或结算成功
