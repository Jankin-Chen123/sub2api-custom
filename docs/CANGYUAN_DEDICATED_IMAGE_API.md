# 沧元专用生图与工作台 API 契约

> 状态：Images API、用户工作台与 Codex HTTP/SSE/WebSocket 两阶段桥接代码已实现但默认关闭；沧元 generation、异步轮询、1K/2K/4K、JSON 基础 edit、multipart 重复 `image`、multipart mask 和 1K `b64_json` 的真实脱敏 POC 已记录。沧元 JSON-mask 直传实测返回 502，服务端会在资产归一化后内部转为 multipart；本机 Codex CLI 0.116.0 的 HTTP/SSE 与 WebSocket 图片呈现已在本地 fake-upstream 验证，Responses Lite 和完整断线/usage 语义仍未完成。本文按当前代码行为记录。日期：2026-08-06。

本文定义三类接口：已有 OpenAI 兼容 Images API 的扩展语义、建议新增的用户生图工作台 API、现有管理员账号 API 的字段扩展。沧元上游协议仅在服务端内部使用。

## 1. 通用约定

### 鉴权

- OpenAI 兼容网关：`Authorization: Bearer <Sub2API API Key>`。
- 工作台/管理员：沿用现有站点 JWT/Session 鉴权与 CSRF 约定。
- 用户永远不提交沧元 Key。

### ID 与时间

- 对外任务 ID 由 Sub2API 生成，例如 `imgjob_...`。
- 不返回上游 task ID、账号 ID、Base URL 或上游模型凭据。
- OpenAI 兼容任务的 `created_at` 使用 Unix 秒；JWT 工作台的 `created_at/updated_at` 沿用 Go JSON 时间格式（RFC 3339）。

### 幂等

异步创建和工作台创建接受：

```http
Idempotency-Key: <1..128 printable characters>
```

同一用户、Sub2API API Key、操作和幂等键的重复请求返回原任务。键相同但请求体 Hash 不同返回 409 `idempotency_conflict`。

### 错误 envelope

OpenAI 兼容端点保持 OpenAI 风格：

```json
{
  "error": {
    "message": "Requested image size is not valid for the selected model",
    "type": "invalid_request_error",
    "param": "size",
    "code": "image_invalid_size"
  }
}
```

工作台使用项目现有 JSON envelope；业务错误中的稳定 `code` 与本文一致。公开 message 可本地化，但客户端不得依赖 message。

## 2. OpenAI 兼容 Images API

### 路由

```text
POST /v1/images/generations
POST /images/generations

POST /v1/images/edits
POST /images/edits

POST /v1/images/generations/async
POST /images/generations/async

POST /v1/images/edits/async
POST /images/edits/async

GET /v1/images/tasks/{task_id}
GET /images/tasks/{task_id}

GET /v1/images/tasks/{task_id}/content
GET /images/tasks/{task_id}/content
```

无 `/v1` 路径是兼容别名，语义必须一致。

### 2.1 创建图片

```http
POST /v1/images/generations
Authorization: Bearer sk-sub2api-...
Content-Type: application/json
```

```json
{
  "model": "gpt-image-2-4k",
  "prompt": "一只金毛犬在海边奔跑，电影感，自然光",
  "size": "3840x2160",
  "n": 1,
  "async": true,
  "response_format": "url"
}
```

支持的核心字段：

| 字段 | 类型 | 必填 | 规则 |
| --- | --- | --- | --- |
| `model` | string | 是 | 必须通过账号 model mapping 映射到 1K/2K/4K 模型 |
| `prompt` | string | 是 | 非空；长度上限由服务配置并在调用上游前校验 |
| `size` | string | 否 | 精确尺寸 `WIDTHxHEIGHT`，或沧元支持的比例 `W:H`；与 `aspect_ratio` 不能同时传 |
| `aspect_ratio` | string | 否 | 正整数比例，例如 `16:9`；未传精确 `size` 时由沧元按模型档位计算 |
| `n` | integer | 否 | tier 模型只能为 1；默认 1 |
| `async` | boolean | 否 | `true` 创建持久异步任务；也可使用 `/async` 路由别名 |
| `quality` | string | 否 | 透传允许值，但不决定分辨率档位 |
| `response_format` | string | 否 | `url` 或 `b64_json`；异步最终统一返回托管 URL |
| `image_size` | string | 否 | 如传入必须与模型档位一致 |
| `output_resolution` | string | 否 | 如传入必须与模型档位一致 |

同步成功保持现有 Images envelope：

```json
{
  "created": 1785800000,
  "data": [
    {
      "url": "https://storage.example.com/..."
    }
  ]
}
```

同步入口最多等待 `dedicated_image.sync_wait_timeout_seconds`。如果任务仍在执行，服务端不取消任务，而是返回与异步创建相同的 `202 Accepted` 任务对象，并设置 `Location` 和 `Retry-After: 2`；客户端随后查询 `poll_url`。

上游的 URL/base64 结果都会先转存并校验。同步请求可按 `response_format` 返回 URL 或 `b64_json`；异步任务完成后统一通过受鉴权的 `content` URL 取图，避免把临时上游 URL 暴露给用户。

### 2.2 JSON 图生图与编辑图片

沧元文档允许 JSON 图生图继续使用 `POST /v1/images/generations`；服务端会保留参考图字段并按 generation 端点提交。multipart 文件编辑使用 `POST /v1/images/edits`。

JSON 形式可接受以下任一参考图字段：

```text
image
images
imageUrls
image_urls
reference_images
referenceImages
image_refs
```

归一后请求示例：

```json
{
  "model": "gpt-image-2-2k",
  "prompt": "把天空改成日落，保持主体不变",
  "size": "2048x2048",
  "images": [
    "https://example.com/source.png"
  ],
  "mask": "https://example.com/mask.png",
  "n": 1
}
```

multipart：

```bash
curl https://api.example.com/v1/images/edits \
  -H "Authorization: Bearer sk-sub2api-..." \
  -F "model=gpt-image-2-2k" \
  -F "prompt=把背景改成雪山" \
  -F "size=2048x2048" \
  -F "image=@source-1.png" \
  -F "image=@source-2.png" \
  -F "mask=@mask.png"
```

参考图去重后最多 9 张，单张不超过 10 MB。mask 最多一个，必须是带 alpha 的 PNG，与第一张输入图同尺寸/格式且不超过 10 MB。远程内容仅允许 HTTPS，仍会经过服务端安全下载校验。由于沧元 JSON-mask 直传实测返回 HTTP 502，带 mask 的公开 JSON 请求会在服务端下载、校验并归一为 multipart 后再提交；用户不需要改变请求格式。

### 2.3 异步创建

异步端点请求体与对应同步端点相同。成功返回 202：

```json
{
  "id": "imgjob_01K...",
  "task_id": "imgjob_01K...",
  "object": "image.generation.task",
  "status": "queued",
  "created_at": 1785800000,
  "poll_url": "/v1/images/tasks/imgjob_01K..."
}
```

响应头：

```http
Location: /v1/images/tasks/imgjob_01K...
Retry-After: 2
Cache-Control: no-store
```

状态对外归一为：

```text
queued
in_progress
completed
failed
submission_unknown
```

内部 `planning/submitting/submitted/polling/storing/settling` 对客户端统一显示为 `in_progress`。`submission_unknown` 是独立终态，表示无法确认上游是否已接受请求；客户端只能查询或人工处理，不得自动重提。

### 2.4 查询异步任务

必须使用提交任务的同一 Sub2API API Key：

```http
GET /v1/images/tasks/imgjob_01K...
Authorization: Bearer sk-sub2api-...
```

处理中：

```json
{
  "id": "imgjob_01K...",
  "task_id": "imgjob_01K...",
  "object": "image.generation.task",
  "status": "in_progress",
  "created_at": 1785800000,
  "model": "gpt-image-2-4k",
  "size": "3840x2160",
  "poll_url": "/v1/images/tasks/imgjob_01K..."
}
```

完成：

```json
{
  "id": "imgjob_01K...",
  "task_id": "imgjob_01K...",
  "object": "image.generation.task",
  "status": "completed",
  "model": "gpt-image-2-4k",
  "size": "3840x2160",
  "poll_url": "/v1/images/tasks/imgjob_01K...",
  "data": [
    {"url": "/v1/images/tasks/imgjob_01K.../content"}
  ],
  "created_at": 1785800000
}
```

失败：

```json
{
  "id": "imgjob_01K...",
  "task_id": "imgjob_01K...",
  "object": "image.generation.task",
  "status": "failed",
  "error": {
    "code": "image_upstream_rejected",
    "message": "Image generation failed"
  },
  "model": "gpt-image-2-4k",
  "size": "3840x2160",
  "poll_url": "/v1/images/tasks/imgjob_01K...",
  "created_at": 1785800000
}
```

不存在、不同用户或不同 API Key 的任务统一返回 404 `image_task_not_found`。

任务完成后，`GET /v1/images/tasks/{task_id}/content` 使用同一 Sub2API API Key 鉴权并代理图片字节；不会暴露对象 key、沧元 task ID 或签名 URL。

## 3. 用户生图工作台 API

基础路径：`/api/v1/user/image-workbench`。以下 envelope 中只展示业务 `data`；实现时应套用项目现有成功/错误包装。

### 3.1 费用预估

工作台可在提交前获取当前价格快照。该接口只校验当前用户拥有的 Sub2API API Key 和图片权限，不创建任务、不冻结余额，也不会请求沧元上游。

```http
POST /api/v1/user/image-workbench/estimate
Content-Type: application/json
```

```json
{
  "api_key_id": 123,
  "model": "gpt-image-2-4k"
}
```

成功响应：

```json
{
  "model": "gpt-image-2-4k",
  "size_tier": "4K",
  "base_cost": 0.08,
  "rate_multiplier": 1,
  "estimated_cost": 0.08
}
```

费用是非绑定快照；提交任务时服务端会再次校验并以任务中保存的价格快照为准。

### 3.2 创建任务

```http
POST /api/v1/user/image-workbench/jobs
Content-Type: application/json
Idempotency-Key: workbench-20260804-001
```

generation 示例：

```json
{
  "api_key_id": 123,
  "operation": "generation",
  "model": "gpt-image-2-4k",
  "prompt": "一张极简科技海报，深蓝背景，柔和体积光",
  "size": "3840x2160",
  "response_format": "url"
}
```

edit 示例：

```json
{
  "api_key_id": 123,
  "operation": "edit",
  "model": "gpt-image-2-2k",
  "prompt": "保持人物不变，把背景替换为雨夜城市",
  "size": "2048x2048",
  "images": ["https://example.com/source.png"],
  "mask": "data:image/png;base64,..."
}
```

工作台前端会把本地参考图和 PNG mask 转为 data URL 放入本次请求，不写入 `localStorage`。也可提交 HTTPS 图片 URL；后端统一执行大小、MIME、重定向和 SSRF 校验。不允许提交浏览器本地路径或任意服务端路径。

202 响应：

```json
{
  "id": "imgjob_01K...",
  "status": "queued",
  "operation": "generation",
  "model": "gpt-image-2-4k",
  "requested_size": "3840x2160",
  "actual_size": "",
  "estimated_cost": 0.1,
  "settled_cost": 0,
  "created_at": "2026-08-04T00:00:00Z",
  "updated_at": "2026-08-04T00:00:00Z"
}
```

服务端校验 `api_key_id` 必须属于当前用户、处于启用状态且组允许图片请求。客户端不能传 `account_id`、`group_id`、`upstream_model`、`upstream_task_id` 或上游 Key。

### 3.3 任务列表

```http
GET /api/v1/user/image-workbench/jobs?status=completed&operation=generation&limit=20&offset=0
```

建议查询参数：

| 参数 | 规则 |
| --- | --- |
| `status` | 可选：`queued,in_progress,completed,failed,submission_unknown` |
| `operation` | 可选：`generation,edit` |
| `limit` | 默认 20，最大 100；非法值由 repository 归一为 20 |
| `offset` | 默认 0，负数归一为 0 |

响应：

```json
{
  "data": [
    {
      "id": "imgjob_01K...",
      "status": "completed",
      "operation": "generation",
      "model": "gpt-image-2-4k",
      "requested_size": "3840x2160",
      "actual_size": "3840x2160",
      "estimated_cost": 0.1,
      "settled_cost": 0.1,
      "content_url": "/api/v1/user/image-workbench/jobs/imgjob_01K.../content",
      "created_at": "2026-08-04T00:00:00Z",
      "updated_at": "2026-08-04T00:00:45Z"
    }
  ],
  "limit": 20,
  "offset": 0
}
```

列表不返回完整 prompt；可返回经过长度限制和敏感信息处理的 `prompt_preview`，是否启用应由隐私评审决定。

### 3.4 任务详情

```http
GET /api/v1/user/image-workbench/jobs/imgjob_01K...
```

```json
{
  "id": "imgjob_01K...",
  "status": "completed",
  "operation": "generation",
  "model": "gpt-image-2-4k",
  "requested_size": "3840x2160",
  "actual_size": "3840x2160",
  "content_url": "/api/v1/user/image-workbench/jobs/imgjob_01K.../content",
  "estimated_cost": 0.1,
  "settled_cost": 0.1,
  "created_at": "2026-08-04T00:00:00Z",
  "updated_at": "2026-08-04T00:00:45Z"
}
```

详情不返回 prompt、API Key ID、account ID、上游 task ID、请求 Hash 或对象 key。

### 3.5 获取图片内容

```http
GET /api/v1/user/image-workbench/jobs/imgjob_01K.../content
```

服务端重新验证 JWT 用户和任务来源后代理返回图片字节。

必须设置安全 `Content-Type`、`Content-Disposition` 和 `X-Content-Type-Options: nosniff`。任务未完成返回 409 `image_task_not_ready`，失败或越权按统一规则返回。

工作台使用这个受鉴权的 content 代理，不把沧元签名 URL交给浏览器；用户再次预览时会重新请求 content，因此不存在前端持久化过期签名 URL 的刷新问题。

## 4. 管理员账号 API 扩展

复用现有账号列表、创建、读取、更新、删除 API。请求/响应的 `extra` 扩展：

```json
{
  "account_purpose": "general"
}
```

枚举：

| 值 | 含义 |
| --- | --- |
| `general` | 默认；普通兼容账号 |
| `image_only` | 沧元专用生图账号；不得处理文本/Responses |

建议管理员读取响应增加派生字段：

```json
{
  "account_purpose": "image_only",
  "image_provider_label": "cangyuan"
}
```

`image_provider_label` 只用于 UI 展示，第一版不允许写入其他 provider 值。

### 可选：付费图片测试

```http
POST /api/v1/admin/accounts/{id}/test-image
```

```json
{
  "confirm": true,
  "model": "gpt-image-2-1k",
  "prompt": "A simple blue circle on a white background"
}
```

规则：

- 仅允许 `image_only` 账号。
- `confirm` 必须为 true；UI 二次确认“会产生上游费用”。
- `model` 省略时默认为 `gpt-image-2-1k`；只允许沧元的 1K/2K/4K 模型。
- `size`、参考图和 `mask` 不属于此管理测试接口；需要这些能力时使用 Images API 或工作台。
- 此接口只执行一次真实上游测试，不创建用户任务、不扣 Sub2API 用户余额。
- 成功响应只返回模型、状态和耗时；响应不含 Key、上游 task ID、签名 URL 或原始错误正文。

成功响应：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "success": true,
    "model": "gpt-image-2-1k",
    "status": "completed",
    "completed": true,
    "duration_ms": 12345
  }
}
```

未确认费用时返回 400；账号不是 `image_only` 时返回 400；沧元鉴权、网络、模型或生成失败统一返回 502，不透传上游原始错误。

## 5. 稳定错误码

| HTTP | code | 含义 | 是否建议自动重试 |
| ---: | --- | --- | --- |
| 400 | `image_prompt_too_long` | 提示词超过服务端 12,000 个 Unicode 字符限制 | 否 |
| 400 | `image_model_not_allowed` | 模型不在允许映射内 | 否 |
| 502 | `image_provider_config_invalid` | 选中的账号无法初始化沧元适配器 | 否，修正账号配置后重新提交 |
| 400 | `image_invalid_size` | 尺寸/档位/比例/像素不合法 | 否 |
| 400 | `image_invalid_quality` | `quality` 不是 `low/medium/high/auto` | 否 |
| 400 | `image_invalid_reference` | 参考图数量、大小、MIME 或 URL 不合法 | 否 |
| 400 | `image_invalid_mask` | mask 格式、alpha、尺寸不合法 | 否 |
| 400 | `image_plan_invalid` | Codex 图片计划不完整或不自包含 | 通常否，由编排器修复 |
| 401/403 | 项目既有鉴权码 | 用户/API Key 无效或无权限 | 否 |
| 404 | `image_task_not_found` | 不存在或不可见 | 否 |
| 409 | `idempotency_conflict` | 同一幂等键对应不同请求 | 否 |
| 409 | `image_task_not_ready` | content 尚未就绪 | 是，按 Retry-After |
| 429 | 项目既有限流码 | 用户/Key/组/账号并发或速率限制 | 是，退避 |
| 503 | `image_provider_unavailable` | 没有合格专用账号且未回退 | 是，退避 |
| 502 | `image_upstream_rejected` | 上游明确拒绝/异常 | 视错误细分 |
| 504 | `image_upstream_timeout` | 查询或请求超时 | 只重试查询，不盲目重提 |
| 502 | `image_storage_failed` | 结果转存失败 | 服务端重试存储，不重生图片 |
| 502/503 | `image_submission_unknown` | 无法确认提交是否被接受 | 客户端不得自动重提 |
| 503 | `image_orchestration_unavailable` | Codex 编排形态未启用/不可用 | 可改用直接 API |

响应中的 `Retry-After` 应用于可安全轮询/退避的状态。客户端对 `image_submission_unknown` 重试必须复用同一幂等键。

## 6. Codex HTTP/SSE/WebSocket 两阶段桥接

该能力不是公开图片 API，而是 `/responses` 的独立可选路由。只有同时满足以下条件才启用：

- `dedicated_image.enabled=true`、`worker_enabled=true`、`codex_bridge_enabled=true`；
- 官方 Codex HTTP `/responses` 或 Responses WebSocket 请求（非 Responses Lite）；
- 分组允许生图；客户端不必预先声明原生 `image_generation` 工具，因为真实 Codex CLI 可能在普通轮次和生图轮次都省略该声明；
- 调度器选中的账号仍是 `general`，绝不会把对话切给 `image_only`。

服务端把原始完整 `input` 交给具备 Responses 能力的 `general` 账号，并注入内部 `sub2api_generate_image` 函数。普通模型未调用该函数时，响应原样回放；调用时，服务端校验 `prompt/model/size/quality`，创建 `source=codex` 的持久任务，由 `image_only` 沧元账号执行，再合成 Responses `image_generation_call`。图片任务的计费独立于普通模型转发计费。合成响应 ID 使用专用 `resp_img_...` 前缀；后续普通文本轮次即使不再携带被动图片 namespace，也会由桥接层把该 ID 还原为原文本账号的真实 `previous_response_id`，不会把对话断在沧元任务上。

当前支持 HTTP 非流式、HTTP SSE 与 Responses WebSocket 回放；WebSocket 会话整段走 HTTP planner，普通文本仍回到 `general` 账号，图片任务才切到 `image_only`。桥接任务等待至 `sync_wait_timeout_seconds`，超时会保留可查询的持久任务并返回错误。Responses Lite 暂不进入该桥接。

当前本地 Codex CLI POC 使用 `codex-cli 0.116.0`：其 `image_generation` feature 默认关闭，普通请求也可能没有原生 `image_generation` 工具声明；服务端因此不把该声明作为进入规划桥的必要条件，而是注入私有规划函数。非 `--json` 交互模式已在 HTTP/SSE 与 WebSocket 两种传输下显示 `image generation started` 和 `generated image`；`--json` 是不同的事件投影，不作为图片呈现验收依据。Responses Lite 和完整断线/usage 语义仍保持独立门槛。

### 6.1 工具调用、断线与重连契约

同一轮 planner 可能在非流式响应、SSE 事件或 WebSocket 回放中重复发送同一个工具调用的快照/参数增量。网关按工具 `call_id` 和规范化后的参数去重：完全相同的重复事件只创建一个 `source=codex` 图片任务；同一响应出现不同 `call_id` 或不同图片参数时返回 `image_plan_invalid`，不会猜测要生成哪一张，也不会把 `n>1` 转发给沧元。

planner 在工具调用前已经产生的短文本会保留在合成 Responses 输出中，并排在 `image_generation_call` 之前。这些文本仅用于恢复客户端可见的输出顺序，不会被拼入发给沧元的自包含 `prompt`；沧元始终只接收通过校验的图片计划。

客户端断线、取消等待或桥接等待超时不会撤销已创建的持久任务。任务一旦拿到上游 `task_id`，后续查询固定原沧元账号，不能因为重连或轮询暂时失败而重新提交。客户端重连应复用同一个工具 `call_id`/幂等键；桥接会复用原任务，完成后再写入 `resp_img_...` 回放记录。HTTP/SSE/WebSocket 的 planner token usage 保留在合成响应中，图片任务费用独立结算，不把图片字节或上游 task ID 填入 usage。

## 7. 尺寸校验参考

设宽高为 `w`、`h`：

```text
w % 16 == 0
h % 16 == 0
max(w, h) <= 3840
max(w, h) / min(w, h) <= 3
w * h >= 655360
```

并满足模型像素上限。服务端应使用安全整数运算避免乘法溢出。

## 8. 缓存、保留和隐私

- 任务状态响应：`Cache-Control: no-store`。
- 用户内容：`Cache-Control: private, no-store`，除非对象存储私有签名策略另有安全定义。
- 上游临时 URL 不作为公开持久字段。
- 完整 prompt、图片、mask、base64、Authorization 和签名 URL query 不写普通日志。
- 任务/对象保留期必须由产品配置明确展示；删除策略不得早于计费和争议审计所需期限。

## 9. 尚待 POC 固定的字段

以下不能凭文档猜测，开发前必须用真实响应定稿并回写本文：

- 沧元同步与异步模式的明确选择参数和状态字段。
- 异步查询的所有中间/终态值及错误 envelope。
- 上游 URL 有效期和下载鉴权要求。
- Codex Responses Lite 的工具结果、完整断线/usage 语义与图片事件结构。
- 沧元对 1K/2K/4K、edit、URL/base64 的真实计费与边界行为。

## 10. 上游内部接口（不对用户公开）

服务端根据已选 image-only 账号调用：

```text
POST {normalized_base_url}/v1/images/generations
POST {normalized_base_url}/v1/images/edits
GET  {normalized_base_url}/v1/images/generations/{upstream_task_id}
GET  {normalized_base_url}/v1/images/edits/{upstream_task_id}
```

Authorization 由账号凭据注入。任何公开 API、日志、指标 label 或前端状态都不得包含完整 URL、Authorization、上游 task ID 和原始响应正文。

## 11. 相关资料

- [集成开发指南](./CANGYUAN_IMAGE_INTEGRATION.md)
- [当前异步图片任务行为](./ASYNC_IMAGE_TASKS.md)
- [沧元 API 文档](https://ai.cangyuansuanli.cn/docs/api)
- [管理员生图账号测试接口](./CANGYUAN_ADMIN_IMAGE_TEST_API.md)
- [生图运维指标](./CANGYUAN_IMAGE_OBSERVABILITY.md)
- [本功能 OpenSpec](../openspec/changes/add-cangyuan-dedicated-image-routing/README.md)
