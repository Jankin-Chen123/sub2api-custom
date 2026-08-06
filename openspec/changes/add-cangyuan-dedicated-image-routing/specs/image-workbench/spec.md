## ADDED Requirements

### Requirement: 用户工作台必须使用用户自己的 Sub2API API Key
系统 SHALL 提供生图工作台，但用户只能选择属于自己的、启用且有图片访问权限的 Sub2API `api_key_id`。前端和 API MUST NOT 提供填写、保存或读取沧元等上游 API Key 的能力。

#### Scenario: 用户创建工作台任务
- **WHEN** 登录用户选择自己的有效 API Key 并提交合法参数
- **THEN** 系统 MUST 按该 Key 的组、权限、额度和限流创建任务
- **THEN** 系统 MUST 在内部调度账号池而不是使用客户端上游凭据

#### Scenario: 用户提交他人的 API Key ID
- **WHEN** `api_key_id` 不属于当前用户
- **THEN** 系统 MUST 返回 404 或统一不可见错误
- **THEN** 系统 MUST NOT 泄露该 Key 是否存在

### Requirement: 工作台必须提供持久任务创建和查询 API
系统 SHALL 提供 `POST /api/v1/user/image-workbench/jobs`、列表、详情和 content 端点。创建成功 MUST 返回 Sub2API 公开 job ID、状态和建议轮询间隔；列表和详情 MUST 支持 generation/edit 的稳定状态、脱敏错误和结果元数据。

#### Scenario: 成功创建异步任务
- **WHEN** 请求通过鉴权、参数、额度和队列准入
- **THEN** API MUST 返回 202 和公开任务 ID
- **THEN** 响应 MUST NOT 包含 account ID 或上游 task ID

#### Scenario: 查询不存在或越权任务
- **WHEN** 用户查询不存在或不属于自己的任务
- **THEN** API MUST 统一返回 404

### Requirement: 工作台输入必须覆盖沧元第一版能力并在客户端和服务端校验
表单 SHALL 支持 generation/edit、prompt、1K/2K/4K、合法尺寸/比例、响应格式、参考图和单个 mask。服务端 MUST 作为最终校验者；前端提示 MUST 显示预计按张费用、4K 资源成本和复杂文字图限制。

#### Scenario: 前端绕过尺寸校验
- **WHEN** 客户端直接向 API 提交非法尺寸
- **THEN** 服务端 MUST 返回稳定 400 且不创建上游任务

#### Scenario: 上传超过限制的参考图
- **WHEN** 任一图片超过 10 MB 或去重后超过 9 张
- **THEN** 前端 SHOULD 提前提示
- **THEN** 服务端 MUST 拒绝并清理临时上传

### Requirement: 任务执行必须持久、幂等且只结算一次
工作台任务 MUST 使用 PostgreSQL 事实源和统一图片 Worker。创建请求 MUST 支持 `Idempotency-Key`；相同用户、API Key 和幂等键的重放 MUST 返回已有任务。成功任务 MUST 只结算一次，失败 MUST 释放预占，客户端断开 MUST NOT 导致重复生成。

#### Scenario: 创建响应丢失后用户重试
- **WHEN** 首次创建已成功但客户端未收到响应并使用相同幂等键重试
- **THEN** API MUST 返回原任务
- **THEN** 上游提交数和结算数 MUST 保持一

### Requirement: 图片内容访问必须鉴权且可续期
完成图片 SHALL 存储于现有对象存储。详情 MAY 返回短期签名 URL；`content` 端点 MUST 在每次访问时验证任务所有权并返回或重定向到有效内容。响应 MUST 使用安全 MIME、文件名和缓存策略。

#### Scenario: 签名 URL 已过期
- **WHEN** 用户仍有任务访问权但详情中的旧 URL 已过期
- **THEN** 重新获取详情或 content MUST 提供新的有效访问方式
- **THEN** 系统 MUST NOT 因 URL 过期重新生成图片

### Requirement: 工作台不得持久泄漏敏感图片输入
完整 prompt、参考图、mask、base64 和签名 URL query MUST NOT 写入浏览器 localStorage/sessionStorage、普通遥测、应用日志或非加密数据库列。跨进程所需原始载荷 MUST 使用有 TTL 的加密存储或受控临时对象。

#### Scenario: 用户刷新工作台
- **WHEN** 页面刷新或浏览器崩溃恢复
- **THEN** 页面 MUST 从受鉴权任务 API 恢复状态
- **THEN** 页面 MUST NOT 从浏览器持久存储恢复上游凭据或 base64 图片

### Requirement: 工作台关闭后必须保留已接受任务的收尾能力
管理员关闭新任务入口时，系统 MUST 拒绝新的工作台创建，但 MUST 允许任务所有者查询已接受任务。Worker MUST 继续轮询、转存和结算已获得上游 task ID 的任务。

#### Scenario: 灰度回滚时有任务正在生成
- **WHEN** 工作台开关被关闭且已有任务处于 polling
- **THEN** 新创建 MUST 被拒绝
- **THEN** 既有任务 MUST 继续到 completed 或 failed 终态
