# 沧元真实上游 POC 记录

本文只记录脱敏后的联调证据，不保存 API key、完整 prompt、图片 URL、签名参数或上游任务 ID。

## 2026-08-04 结果

测试入口为：

```text
POST https://ai.cangyuansuanli.cn/v1/images/generations
Authorization: Bearer <local key, not recorded>
Content-Type: application/json
```

已完成三次真实 generation 请求：

| 项目 | 1K | 2K | 4K |
| --- | --- | --- | --- |
| 模型 | `gpt-image-2-1k` | `gpt-image-2-2k` | `gpt-image-2-4k` |
| 请求尺寸 | `1024x1024` | `2048x2048` | `3840x2160` |
| `n` | `1` | `1` | `1` |
| 返回状态 | HTTP 200 | HTTP 200 | HTTP 200 |
| 图片数量 | 1 | 1 | 1 |
| 返回形式 | URL | URL | URL |

同时使用同一 key 请求 `GET /v1/models`，返回 HTTP 200，并确认账号可见 1K、2K、4K 三个 `gpt-image-2-*` 模型。

另外完成了一次 1K 异步 generation：提交返回 HTTP 200、初始状态为 `queued`，随后使用同一个上游任务绑定轮询，最终状态为 `completed`，返回 1 个图片结果。该响应没有依赖名为 `completed` 的布尔字段；适配器应以状态和结果数据归一判断完成。

还发送了一次明确的非法模型请求作为无费用错误测试：沧元返回 HTTP 400，错误对象存在，`type` 为 `new_api_error`，没有稳定的 `code`。因此服务端不能依赖沧元错误 code，必须继续映射为本项目稳定的 `image_*` 错误分类。

另外完成了以下 1K edit 实测，输入源图和 mask 均为 1024×1024 的脱敏 PNG，响应只记录数量和状态：

- JSON `POST /v1/images/edits`，`images` 使用 data URL、不带 mask：HTTP 200，返回 1 个图片 URL。
- 同一 JSON edit 增加带 alpha 的 `mask`：HTTP 502，错误类型为上游错误，适配器归一为 `image_upstream_unavailable`。
- multipart `POST /v1/images/edits`，重复提交两个 `image` 文件字段、不带 mask：HTTP 200，返回 1 个图片 URL。
- multipart edit 增加一个带 alpha 的 `mask` 文件字段：HTTP 200，返回 1 个图片 URL。

因此服务端在资产解析完成后，会把带 mask 的公开 JSON/工作台请求内部提升为 multipart，再交给沧元；公开 API 形状不变，避免把已知的 JSON-mask 502 暴露为不透明的上游差异。JSON 参考图 edit 的基本路径、multipart 重复 `image`、multipart mask 已有真实证据；JSON-mask 直传仍明确标记为不兼容，不能自动重试成第二个生成任务。

这证明了当前端点、Authorization 方式、模型名和 1K/2K/4K generation 请求形状可以到达沧元并返回图片结果。同步完成响应中 `model`/`status` 不是必需字段；异步流程则使用 `queued`/`completed` 状态。适配器以 `data`、任务 ID 和可选状态字段归一判断，因此不会因可选字段缺失而失败。

## 尚未由真实上游证明的边界

- generation 异步超时、轮询错误和重试退避行为；（异步提交、`queued` 到 `completed` 的基础路径已验证）
- JSON 参考图的其他别名；（`images` data URL 已验证，其他别名仍未逐一实测）
- 4K 返回图片的实际像素元数据、完整耗时和费用；
- URL 有效期和下载鉴权；（本轮已验证 1K `b64_json` 返回，但尚未验证 URL 有效期和下载鉴权）
- 真实 Codex HTTP、SSE、Responses WebSocket 客户端事件、断线重连和 usage 语义；
- 真实多实例、进程重启、Redis 丢失和结算故障注入。

在这些边界得到真实证据前，`dedicated_image.codex_bridge_enabled` 必须保持关闭；直接 Images API、工作台和 worker 的本地协议测试可以继续运行。

本地测试已用内存 fake 覆盖 Codex HTTP 非流式、HTTP SSE、Responses WebSocket 事件回填、synthetic response replay、取消保留 durable job，以及 replay 持久化失败时不提前写响应；这些证据不能替代真实 Codex 客户端验收。

edit smoke 可在同一容器命令中将测试名替换为 `TestRealCangyuanJSONEditSmoke` 或 `TestRealCangyuanMultipartEditSmoke`，并设置对应的 `CANGYUAN_REAL_EDIT_SMOKE=1` 或 `CANGYUAN_REAL_MULTIPART_SMOKE=1`。带 mask 复测分别额外设置 `CANGYUAN_REAL_EDIT_WITH_MASK=1` 或 `CANGYUAN_REAL_MULTIPART_WITH_MASK=1`。这些开关默认关闭；每次成功提交都会产生上游费用。

## 可复现的本地 opt-in smoke

仓库中的 `TestRealCangyuanGenerationSmoke` 默认跳过。需要明确承担上游费用时，在本地临时注入 key 文件路径：

```powershell
$env:CANGYUAN_REAL_SMOKE = "1"
$keyFile = "<local key file path>"
$env:CANGYUAN_API_KEY_FILE = "/run/secrets/cangyuan-key"
$env:CANGYUAN_BASE_URL = "https://ai.cangyuansuanli.cn/v1"
# 可选：设为 1 时测试同一任务的异步提交和轮询；默认同步 1K。
# $env:CANGYUAN_REAL_ASYNC = "1"
docker run --rm -e CANGYUAN_REAL_SMOKE -e CANGYUAN_API_KEY_FILE -e CANGYUAN_BASE_URL -e CANGYUAN_REAL_ASYNC `
  -v "${keyFile}:/run/secrets/cangyuan-key:ro" `
  -v "${PWD}\backend:/src" -v sub2api-go-mod:/go/pkg/mod `
  -v sub2api-go-build:/root/.cache/go-build -w /src golang:1.26.5 `
  go test ./internal/service -run '^TestRealCangyuanGenerationSmoke$' -count=1 -timeout=300s
Remove-Item Env:CANGYUAN_REAL_SMOKE,Env:CANGYUAN_API_KEY_FILE,Env:CANGYUAN_BASE_URL,Env:CANGYUAN_REAL_ASYNC
```

## 2026-08-05 verification note

The recorded POC and the subsequent verification attempt were made from a
Hong Kong node. The later live generation attempt did not complete within the
client wait window and was stopped; this is treated as a network/provider
availability observation, not as evidence that the API contract or adapter
logic is invalid. No key, provider task ID, signed URL, or raw response was
recorded. These results do not establish reachability from a Singapore node.
An earlier base64 attempt ended with a submission-unknown network error; the
successful retry below supersedes that observation for the tested 1K request.
Repeat the live edit/multipart smoke only after the route is known to be
reachable and keep the dedicated Codex bridge disabled until the client-level
POC is complete.

## 2026-08-05 local public Responses WebSocket E2E

This is local integration evidence, not a live-provider result. The isolated
app was temporarily started with the Responses WebSocket ingress switches
enabled. A Python `websockets` client connected to
`ws://127.0.0.1:18080/v1/responses` and completed two turns on one connection:

1. an ordinary text turn completed with the normal Responses event sequence;
2. a 2K image turn supplied the first response ID as `previous_response_id` and
   completed with the image-generation in-progress, generating, and completed
   events.

The fake-upstream audit showed the planner requests using the general account
and the durable image execution using the Cangyuan image-only account with
`gpt-image-2-2k`, `2048x2048`, and asynchronous submission. No provider key,
prompt, upstream task ID, image URL, or raw response was recorded. The Python
client result is protocol evidence; the official Codex CLI result is recorded
below. Responses Lite and full disconnect/usage semantics remain separate
open gates. This does not change the rule that all live-provider checks in
this document used a Hong Kong node; no Singapore reachability is inferred.

The patched local image was additionally exercised with Codex CLI `0.116.0`
and a custom provider configured with `supports_websockets = true`. In normal
(non-`--json`) exec mode, the CLI connected to the public WebSocket, the app
logged the HTTP-bridge ingress, the isolated PostgreSQL database showed a
completed 2K `codex` job on account 2, and the CLI printed `generated image`.
The `--json` projection omits that image item even though the interactive
surface renders it; this is a CLI output-format difference, not a bridge
failure. Official CLI HTTP/SSE and WebSocket image rendering are proven
against the local fake-upstream. Responses Lite and full disconnect/usage
acceptance remain open gates.

## 2026-08-05 local implementation evidence

The local-only verification added after the Hong Kong-node POC includes:

- a fresh-worker handoff after upstream submission, proving that the persisted upstream task binding is reused after a process-restart-style boundary;
- active-lease double-claim prevention, expired submission lease recovery, Redis/payload-loss re-polling, and injected billing-ledger retry tests;
- a successful authenticated workbench HTTP POST test that persists the job metadata separately from the sensitive request payload;
- a runtime guard that normalizes zero-value worker options before creating the recovery ticker.

These checks run against local fakes and do not prove live provider reachability. Official Codex CLI HTTP/SSE and WebSocket image rendering are recorded above; Responses Lite and full disconnect/usage semantics remain unverified. All recorded live-provider checks in this document were made from a Hong Kong node; no Singapore reachability is inferred. The Codex bridge remains disabled until the remaining client gates are separately verified.

## 2026-08-05 Hong Kong-node live rerun

按用户确认，本轮真实上游复测使用的是香港节点网络环境。每次测试都从本地只读 key 文件注入凭据，测试日志只记录模型、状态和结果数量，不记录 key、上游任务 ID、图片 URL 或原始响应。

| 测试 | 结果 |
| --- | --- |
| `gpt-image-2-1k` synchronous generation，`1024x1024` | HTTP 200，1 个 URL 结果 |
| `gpt-image-2-2k` synchronous generation，`2048x2048` | HTTP 200，1 个 URL 结果 |
| `gpt-image-2-4k` synchronous generation，`3840x2160` | HTTP 200，1 个 URL 结果 |
| 1K `response_format=b64_json` generation | HTTP 200，1 个非空 `b64_json` 结果 |
| JSON edit，data URL 参考图，不带 mask | HTTP 200，1 个 URL 结果 |
| multipart edit，重复 `image` 字段并带 alpha PNG mask | HTTP 200，1 个 URL 结果 |

这组结果补足了 1K/2K/4K 实际请求、base64 响应、JSON 基础 edit、multipart 重复图片和 multipart mask 的当前实证。它仍不能证明 4K 返回内容的像素元数据、URL 生命周期、所有别名的上游接受方式，或真实 Codex 客户端协议闭环；`dedicated_image.codex_bridge_enabled` 继续保持关闭。

不要把 key 内容替换进命令、源码、测试输出或提交记录。

## 2026-08-05 本机 Codex CLI 协议 fixture

使用本机 `codex-cli 0.116.0`，将 Responses provider 指向本地 fake SSE 服务，未注入任何上游 key。fake 服务返回了脱敏的 Responses 图片事件序列：`response.created`、`response.in_progress`、图片 output item 事件、`response.image_generation_call.*` 和 `response.completed`。CLI rollout 中确认收到 `image_generation_call` 并完成 turn。

同一实验还验证了客户端请求形态：关闭实验性图片能力时，Codex CLI 可以完全不发送 `image_generation` 工具声明；开启该实验性能力时才会发送。由此，桥接检测不能把客户端声明作为必要条件，必须由普通账号规划器判断当前请求是否真的需要图片。

这只是客户端协议消费者与本地 fake 的兼容性证据，不是沧元真实上游或 Responses Lite、完整断线/usage 语义的验收。真实 Codex HTTP/SSE/WebSocket 桥接已经在本地服务端 E2E 中验证；正式 feature flag 仍保持关闭，直到剩余门槛完成。
