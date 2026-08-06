## Why

当前 Images API 和 Codex 原生 `image_generation` 路径会把图片请求发往普通 OpenAI 上游。部分普通上游限制输出档位，无法稳定透传 2K/4K；而沧元专用生图账号只提供图片模型，无法独立处理 Codex 的自然语言对话和长上下文。简单地在检测到“生图”关键词后切换整个账号，会丢失上下文、破坏 `previous_response_id` 粘性，并让普通请求误入只能生图的上游。

因此需要在现有分组和账号池内增加最小的账号用途标记，并增加一条仅面向沧元的图片执行支路。对于直接 Images API，请求可以直接进入图片执行；对于 Codex 对话，必须先由原普通文本账号理解上下文并生成自包含图片计划，再由沧元执行，最后把结果包装回原 Codex 会话。

## What Changes

- 在现有 OpenAI API Key 账号 `extra` 中增加 `account_purpose`，只允许 `general` 与 `image_only`；旧账号缺省为 `general`。
- 第一版将 `image_only` 固定解释为“沧元专用生图账号”，继续复用 `credentials.api_key`、`base_url`、`model_mapping`、分组、优先级、并发与状态字段。
- 扩展图片账号选择：图片执行优先使用同组可用 `image_only`，可配置是否回退普通账号；所有非图片请求强制排除 `image_only`。
- 增加沧元适配器，支持 generation/edit 的同步和异步提交/查询、1K/2K/4K 模型档位、尺寸与参考图校验、错误归一化和结果转存。
- 将 Codex 图片行为分为 `text`、`image_planning`、`image_execution` 三阶段；原普通账号负责前两阶段，专用账号只负责执行。
- 用内部工具编排承接 Codex 长上下文：文本模型输出结构化、自包含图片计划，网关拦截工具调用、执行生图并按真实 Responses 协议回填图片结果。
- 新增 PostgreSQL 持久化图片任务和加密临时 payload，Redis 仅负责可选的队列唤醒/锁/短期限流，对象存储保存完成结果；支持多实例租约、fencing、幂等、轮询粘性和单次结算。
- 新增用户生图工作台及其 JWT API；用户选择自己的 Sub2API API Key，不接触也不填写上游密钥。
- 保持现有 OpenAI 兼容 Images API 路径和无 `/v1` 别名；不泄露上游 task ID、账号 ID、Key 或原始错误。
- 在开发 Codex 编排前完成真实 HTTP、SSE、WebSocket、Responses Lite 和图片工具事件抓包 POC。

## Capabilities

### New Capabilities

- `dedicated-image-routing`：账号用途、同组过滤、优先级、回退、粘性和普通流量隔离。
- `cangyuan-image-adapter`：沧元协议、模型档位、参数校验、提交/轮询、结果归一化和安全边界。
- `codex-image-orchestration`：意图判定、长上下文规划、内部工具执行、协议回填及会话粘性。
- `image-workbench`：用户工作台任务提交、查询、结果访问、权限和配额。

### Modified Capabilities

- 现有 Images API、异步图片任务、对象存储和账号调度语义将被扩展，但现有普通账号路径必须保持向后兼容。

## Non-Goals

- 不建设任意供应商适配平台；第一版只支持沧元专用生图账号。
- 不增加更多账号分类，也不新增认证类型。
- 不把自然语言关键词分类器作为最终执行依据。
- 不让沧元理解 Codex 完整对话，也不向沧元发送 `previous_response_id`。
- 不保证生成模型能准确绘制大量中文文字、严谨知识图或可编辑流程图；精确 Mermaid/SVG 渲染属于后续能力。
- 不在第一版实现视频生成、图片搜索、图片理解、通用多模态代理或跨供应商成本优化。
- 不允许用户在工作台保存或提交上游 Key。

## Impact

- **数据库**：账号 `extra` 兼容读取；新增 `image_generation_jobs` 及索引/约束。
- **后端**：扩展账号过滤，新增图片编排、沧元适配、持久 Worker 和工作台 Handler。
- **前端**：账号表单增加用途字段，新增用户生图工作台。
- **外部 API**：兼容 Images API 不 breaking；新增用户工作台 API；管理员账号 CRUD 只扩展字段。
- **计费**：任务预占/结算必须幂等；失败释放预占；实际成功仅结算一次。
- **运维**：需要 PostgreSQL、Redis 和已启用对象存储；新增队列、轮询、上游与存储指标。
- **风险**：Codex 工具协议尚需真实 POC；4K 内存、上游提交不确定、重复生成、URL 过期和多实例重复消费为首要风险。
