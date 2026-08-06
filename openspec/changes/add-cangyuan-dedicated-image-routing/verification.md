# Verification Plan

## Current audit (2026-08-05)

- Full non-race Go tests pass with `go test ./... -count=1`.
- The CI critical frontend Vitest set plus all new image-workbench/account-purpose tests pass; typecheck, lint, and production build pass. The full frontend Vitest run is not claimed because the host's default worker configuration exceeded the local timeout.
- Race tests for Cangyuan, ImageGeneration, and Codex dedicated-image paths pass. The full repository race run is not clean because unrelated existing tests fail under race; this remains an open release-gate item.
- The live provider evidence now includes generation/model-list, 1K/2K/4K generation, 1K base64 output, async polling, JSON edit without mask, multipart edit with repeated `image`, and multipart edit with an alpha mask. A JSON edit with a mask returned HTTP 502 and is now internally promoted to multipart after asset normalization. The recorded checks used a Hong Kong node; no Singapore reachability is inferred.
- `dedicated_image.codex_bridge_enabled` must remain `false` until the remaining Responses Lite and full disconnect/usage gates are completed.
- The local fake-upstream flow `TestImageGenerationOrchestratorWorkerFakeUpstreamCompletesDurableWorkbenchJob` passes through the real adapter and result store, covering created/queued/submitted/polling/storing/settling/completed without an external request. It is local execution evidence only, not live-provider or real-client acceptance.
- The fake flow now recreates the worker instance after upstream submission, so the polling/retry half also exercises a process-restart-style handoff while preserving the bound upstream task.
- Local worker fault-injection coverage now includes an active-lease double-claim attempt, expired submitting-lease recovery to `submission_unknown`, repeated idempotent billing lifecycle calls, and settlement retry after an injected usage-ledger failure.
- `TestWorkbenchCreateHTTPHandlerPersistsDedicatedJobAndPayload` exercises the authenticated workbench POST handler locally and proves that the public request becomes a durable job plus an out-of-row payload; it does not contact a provider.
- `ImageGenerationWorkerRuntime.Start` normalizes zero-value worker options before creating its recovery ticker; the zero-value runtime test prevents a non-positive-ticker panic.
- An opt-in disposable-PostgreSQL/Redis acceptance now passes with two independent repository/Worker instances: concurrent idempotent creation converges to one row, concurrent claim permits one worker, an expired `submitting` lease becomes `submission_unknown`, the old claim is fenced from renewal, process-handoff polling does not resubmit, and Redis wake-up discovery still leaves PostgreSQL claim fencing authoritative. Staged deployment and production observation remain separate open gates.

## 2026-08-05 local public Responses WebSocket E2E

The isolated local app was temporarily started with both Responses WebSocket
ingress switches enabled. A Python `websockets` client connected to the public
gateway at `ws://127.0.0.1:18080/v1/responses` without using a provider key.
One connection completed two turns:

- turn 1 was ordinary text and completed with `response.created`, output-item,
  output-text, and `response.completed` events;
- turn 2 supplied the first response ID as `previous_response_id` and requested
  a new 2K puppy image; it completed with `response.created`,
  `response.in_progress`, `response.image_generation_call.in_progress`,
  `response.image_generation_call.generating`,
  `response.image_generation_call.completed`, and `response.completed` events.

The isolated fake-upstream audit confirmed that both planner turns used the
general account while the durable image request used the Cangyuan image-only
account with `gpt-image-2-2k`, `2048x2048`, and asynchronous submission. This
proves the local public WebSocket bridge and same-session context handoff. The
official CLI transport/rendering result is recorded below; Responses Lite,
full disconnect/usage semantics, and live-provider reachability remain
separate gates. The normal configuration keeps the Codex bridge disabled until
those remaining gates are completed.

The patched local image was also exercised with Codex CLI `0.116.0` using a
custom provider with `supports_websockets = true`. In normal (non-`--json`)
exec mode, the CLI opened the public WebSocket, the app logged
`ingress_ws_http_bridge`, PostgreSQL recorded a completed `codex` 2K job bound
to the image-only account, and the CLI printed both `image generation started`
and `generated image`. The CLI `--json` stream intentionally reports a
different event projection and did not emit an `item.completed` image item;
that is not evidence of a rendering failure. Official Codex CLI HTTP/SSE and
WebSocket transport plus image rendering are now proven against the local
fake-upstream. Responses Lite and full disconnect/usage semantics remain open.

## 0. 当前真实 POC 进展

已在不保存凭据的前提下完成沧元真实 `GET /v1/models`、1K/2K/4K 同步 generation、1K `b64_json`、1K 异步提交和同任务轮询，以及 JSON data URL edit、multipart 重复 `image` 与 alpha mask 基本路径：成功请求返回 HTTP 200 和 1 个结果，异步请求从 `queued` 轮询到 `completed` 并返回 1 个结果。JSON-mask 直传仍为已知上游差异，服务端在资产归一化后改用 multipart。详细脱敏记录见 `docs/CANGYUAN_REAL_POC_RESULTS.md`。本机 Codex CLI HTTP/SSE/WebSocket 图片呈现已完成本地 fake-upstream 验收；Responses Lite、完整断线/usage 语义和生产灰度仍是未完成门槛，不得据此开启正式 Codex bridge。

本地内存 fake 已覆盖 Codex HTTP 非流式、HTTP SSE、Responses WebSocket 的图片事件回填、replay 恢复和 replay 持久化失败顺序；这些测试不等同于真实 Codex 客户端 POC。

## 1. 发布门槛

以下全部满足才允许启用直接专用生图路由：

- 沧元 generation/edit 与同步/异步协议 POC 已固定为脱敏 fixture。
- 1K、2K、4K 的模型、尺寸和费用归属得到真实验证。
- PostgreSQL 持久任务能跨进程重启和 Redis 清空恢复。
- 幂等、submission unknown、账号粘性和单次结算故障注入通过。
- 普通聊天、Responses、WebSocket、工具调用和普通 Images 黄金请求无回归。
- 对象存储、内存、并发和敏感信息门禁通过。

Codex 自动编排另有独立门槛：HTTP、SSE、WebSocket、Responses Lite 的真实客户端闭环全部通过；不通过的形态保持 feature flag 关闭，不阻挡工作台和直接 API 发布。

## 2. POC 证据

每个 POC 应保存：请求形态、脱敏响应、HTTP 状态、事件顺序、耗时、重试行为、模型/尺寸、服务日期和结论。不得保存真实 Key、完整用户 prompt、base64 或可长期访问的图片 URL。

| POC | 必须证明 | 失败处理 |
| --- | --- | --- |
| Cangyuan sync | generation/edit 最终 envelope 可归一 | 停止 adapter 开发 |
| Cangyuan async | task ID、状态、查询路径和终态明确 | 第一版仅同步或停止上线 |
| Tier/size | 1K/2K/4K 边界与冲突行为 | 按真实行为修订 spec |
| Reference/mask | 别名、multipart、数量/大小/mask 行为 | 关闭未验证入口 |
| Codex HTTP | tool call 可截获并回填图片 | 关闭 HTTP 编排 |
| Codex SSE | 事件顺序、终止、usage 正确 | 关闭 SSE 编排 |
| Codex WS | 每轮关联、重连、错误正确 | 关闭 WS 编排 |
| Responses Lite | 客户端能显示最终图片 | 关闭 Lite 编排 |

## 3. 账号与路由矩阵

| 场景 | 预期 |
| --- | --- |
| 旧账号无 purpose | 视为 general |
| 普通聊天，同组有 image-only | 永远只选 general |
| 直接生图，同组有可用 image-only | 选 image-only |
| image-only 模型映射不包含请求模型 | 排除该账号 |
| image-only 达到并发 | 选择其他合格 image-only 或返回不可用 |
| 无 image-only，fallback 关闭 | `image_provider_unavailable` |
| 无 image-only，fallback 开启 | 使用现有普通图片账号 |
| 已获 task ID 后账号变为高负载 | 仍由原账号轮询，不重提 |
| image-only 收到 Responses/chat | 调度层拒绝选择 |

## 4. 参数边界

- 宽/高非 16 倍数、最长边大于 3840、比例大于 3:1、像素小于 655360。
- 1K 超 1048576、2K 超 4194304、4K 超 8294400。
- 请求档位与模型、`image_size`、`output_resolution` 冲突。
- tier 模型 `n=0/1/2`。
- 0、1、9、10 张参考图；重复图；9.9/10/10.1 MB；错误 MIME 和图片炸弹。
- HTTP URL、HTTPS 重定向到私网、DNS rebinding、超时、过大响应。
- mask 无 alpha、非 PNG、尺寸/格式不匹配、多 mask。

## 5. 状态机与故障注入

- 每个合法状态转换及所有非法跨越都要测试。
- 多 Worker 同时 claim 同一 job，只允许一方成功。
- 旧 claim version 在租约回收后不能写终态或结算。
- 提交前连接失败可安全换账号；提交响应不明确不得自动重提。
- 收到 task ID 后轮询 429/5xx/timeout 只重试查询，不重复提交。
- 服务在 submitting/submitted/polling/storing/settling 各阶段退出后可恢复。
- Redis 清空后 PostgreSQL 扫描恢复；对象存储失败可重试存储但不重生图片。
- 结算服务超时/重复回调后最终只扣一次；失败只释放一次。

## 6. Codex 上下文验收

固定一段包含至少 20 轮、多个主题和明确结论的对话，最后要求生成知识思维导图。验证：

- 最后一轮仍由原文本账号及原 `previous_response_id` 处理。
- 结构化计划覆盖规定的关键知识点、关系和禁止臆造项。
- 发往沧元的是自包含 prompt，不依赖其看见历史聊天。
- 沧元请求不包含不必要的原始聊天、凭据、工具输出或隐藏系统指令。
- 结果在 Codex 中显示，后续“再简洁一点”仍能由原文本账号理解并发起新图片任务。
- 讨论“怎么调用 image_gen API”、否定生图、仅引用图片名称时不触发执行。

图片中文字准确率采用人工验收，不作为系统协议正确性的替代。若用户要求逐字准确，应明确提示使用后续 SVG/Mermaid 能力。

## 7. API 与权限

- 同步/异步 Images API 的 `/v1` 和无前缀别名都通过。
- 工作台 JWT 缺失/过期、API Key 不属于用户、Key 禁用、组无权、任务越权均正确拒绝。
- 越权查询与不存在任务统一 404。
- 公开响应不含 `account_id`、`upstream_task_id`、上游 base URL、Authorization 或原始上游错误。
- `Idempotency-Key` 重放返回同一 job；不同用户不能碰撞读取。
- content 下载需鉴权或有效短签名，且使用正确 MIME、文件名与缓存头。

## 8. 性能与容量

- 1K/2K/4K 分别测提交、生成、下载、转存和总耗时 P50/P95/P99。
- 4K url/base64 在设定并发下测峰值 RSS，证明没有整批多份复制。
- 参考图上传和远程下载执行大小/并发/超时限制。
- 队列达到容量后快速失败，不拖垮普通文本流量。
- 数据库索引覆盖待领取、待轮询、用户列表、幂等查询和清理扫描。

## 9. 安全泄漏门禁

为 Key、prompt、参考图、base64 和签名 query 植入唯一 canary，扫描：应用日志、错误、数据库非加密列、Redis key 名、指标 label、管理 API、用户 API、浏览器 local/session storage、Sentry/遥测和截图。任何 canary 出现在不允许位置都阻止发布。

## 10. 灰度与回滚

1. 部署 schema、读取兼容和指标，所有开关关闭。
2. 配置一个测试组和一只低额度沧元账号，仅管理员测试。
3. 开启直接 Images API，再开启工作台；观察错误率、重复率、任务龄和结算差异。
4. Codex POC 全通过后，仅对测试 API Key 开启编排。
5. 扩大前确认 24 小时内无重复提交/重复扣费、普通请求错误率无变化。

回滚时关闭新任务入口和 Codex 工具注入，停止 claim 新任务，但继续轮询、转存和结算已经获得上游 task ID 的任务。数据库表和 `account_purpose` 保留以便前滚，不执行破坏性 down migration。

## 11. 建议命令

```powershell
openspec validate add-cangyuan-dedicated-image-routing --type change --strict --no-interactive
git diff --check
git status --short
```

实现阶段还需运行受影响 Go 包测试、`go test -race`、迁移集成测试以及前端 `pnpm lint:check`、`pnpm typecheck`、相关 Vitest 和生产构建。

## 2026-08-05 durable payload update

- Migration `196_image_generation_payloads.sql` adds an encrypted, TTL-bound PostgreSQL payload table. New payload writes no longer require Redis.
- `TestDurableImageGenerationPayloadStoreSurvivesRedisLoss` passes with Redis stopped after the durable write; the payload is still read and deleted through PostgreSQL.
- A legacy Redis read-only fallback keeps payloads created before the migration readable during a rolling upgrade. Full two-process Redis outage/recovery and production fault injection remain release gates.

## 2026-08-05 current-source image boot

- The current worktree built successfully as local image `codex-sub2api-e2e:current`.
- A disposable app container from that image started on the isolated e2e network with the Worker and Codex bridge disabled and returned `{"status":"ok"}` from `/health`. The existing local e2e app and all production containers were left unchanged.
- The opt-in PostgreSQL integration suite passed against the isolated local e2e database for migration 196, encrypted payload round-trip/delete with no Redis dependency, concurrent idempotent creation, single-worker claim, expired submission recovery, and claim fencing.

## 2026-08-05 durable payload + worker process-handoff acceptance

- `TestDurableImageGenerationPayloadWorkerProcessRestartDoesNotResubmit` now
  joins the real PostgreSQL job repository, encrypted PostgreSQL payload store,
  production Cangyuan adapter, worker state machine, result staging, and
  settlement fakes behind one opt-in integration test.
- The test creates a temporary account in the disposable database, submits to
  a TLS fake upstream, recreates repository/payload-store/worker instances
  between submission and polling, injects a transient polling 502, and proves
  that the same upstream task is polled to completion with one submission,
  one result-store call, one hold, and one settlement. The payload is deleted
  after terminal completion.
- The test passed against the isolated local e2e PostgreSQL container with
  `-tags image_generation_integration`. It strengthens local evidence for
  process restart and durable payload recovery. Its production-shaped durable
  store receives a deliberately unreachable Redis client; PostgreSQL-backed
  new payload reads/writes and terminal cleanup still complete successfully.
  Full multi-process Redis recovery and staged rollout gates remain open.

## 2026-08-05 local regression rerun

- `go test ./... -count=1 -timeout=600s` passed in the Docker Go environment.
- The feature-scoped race expression covering Cangyuan, image-generation, and
  Codex dedicated-image tests passed with `CGO_ENABLED=1`. A broader service
  race expression still finds pre-existing concurrent `gin.SetMode` writes in
  unrelated OpenAI compatibility tests; this is not reported as a clean
  repository-wide race result.
- The complete opt-in repository/migration image integration set passed against
  the isolated local PostgreSQL container, including the new durable worker
  process-handoff test.
- The frontend critical set plus the new account-purpose/workbench tests
  passed: 9 files, 115 tests. Frontend typecheck, lint, production build,
  Markdown local-link validation, and `git diff --check` also passed.
- The strict OpenSpec command remains unverified because the `openspec` CLI is
  not installed in this workspace; the local-link and diff checks do pass.

## 2026-08-06 Redis-loss rerun

- The durable payload + Worker integration was rerun with a real Redis client
  configured to an unreachable local port rather than `nil`. The PostgreSQL
  payload path completed the full process-handoff flow, including result
  staging, one settlement, and cleanup; the disposable database contained no
  leftover test account, job, or payload rows afterward.
- The complete opt-in repository/migration integration set passed again. The
  remaining 9.5 gap is specifically multi-process Redis recovery/wake-up
  behavior under a staged deployment, not the durable PostgreSQL path for new
  jobs.
- The same durable process-handoff test passed under `go test -race`; its
  blocked-submit phase made two independent Worker instances race for one
  PostgreSQL submission lease, with exactly one winner and one upstream
  submission.

## 2026-08-06 Codex Responses Lite frame-boundary hardening

- The WebSocket HTTP-bridge ingress now re-evaluates the Responses Lite marker
  for every frame instead of relying only on the first frame's session-level
  bridge decision. A Lite frame therefore bypasses the dedicated image planner.
- A Lite frame carrying a synthetic `resp_img_*` continuation ID is rejected
  with a policy close rather than forwarding an opaque ID to the ordinary
  upstream bridge; the client must restart that conversation without switching
  transport modes mid-session.
- Added a focused regression test for non-Lite routing, Lite bypass, and the
  synthetic-response guard. The targeted service test command passed in the
  local Docker Go environment; the existing service/handler and
  repository/migration package suites also passed in the same local run.
- This is local protocol coverage only. It does not claim acceptance by an
  official Codex Responses Lite client or multi-process production rollout.

## 2026-08-06 durable image wake-up

- `ImageGenerationOrchestrator.Create` now publishes an optional Redis
  wake-up only after the PostgreSQL job row has been created. The message is
  limited to the durable Sub2API `job_id`; it contains no prompt, credential,
  image data, upstream task ID, or result URL.
- `ImageGenerationWorkerRuntime` uses the message to interrupt idle/retry
  delays. A disconnected subscription is retried with bounded backoff, while
  PostgreSQL polling/recovery remains authoritative when Redis is unavailable
  or a message is missed.
- The miniredis publisher/subscriber test, no-Redis behavior test,
  orchestrator publish-failure fallback test, and wake-interrupted delay test
  pass locally; the runtime-level wake-to-immediate-rescan test also passes
  under `go test -race`. Two independent Redis clients also both receive the
  same wake-up under `go test -race`. This is implementation-level evidence
  only; staged multi-process recovery and production rollout observation remain
  open gates.

### Post-change local regression evidence

- `go test ./internal/service ./internal/handler -count=1 -timeout=300s`
  passed after the change (`service` 100.244s; `handler` 35.656s).
- `go test ./internal/repository ./migrations -count=1 -timeout=300s` passed
  after the change.
- The focused frontend regression set passed: 9 files and 114 tests;
  `vue-tsc --noEmit`, ESLint, and the production Vite build also passed.
- An earlier feature-scoped race attempt could not start in the then-current
  local Go image because it had no C compiler (`gcc`); the standard Go
  container used in the current continuation provides CGO for the race run
  recorded below.

### Current local continuation regression

- `go test ./internal/service ./internal/handler ./internal/repository ./migrations ./cmd/server -count=1 -timeout=600s` passed in the local Go container.
- The feature-scoped `go test -race` run for image-generation, Codex bridge,
  runtime, and repository tests passed with `CGO_ENABLED=1` in the standard Go
  container.
- The affected frontend tests passed directly through the existing local
  `node_modules` binary: 6 files and 71 tests. Typecheck, ESLint, and the
  production Vite build also passed.
- The exact user-provided Cangyuan key was not found in workspace files. The
  live provider checks recorded by this change were performed from a Hong Kong
  node; no Singapore-node reachability is claimed.

## 2026-08-06 local routing-boundary hardening

- The legacy OpenAI Images passthrough selector is now general-only even when
  durable dedicated routing is enabled. Explicit Cangyuan tier requests are
  intercepted by the durable dedicated-image handler; unrelated image models
  cannot be forwarded directly to an `image_only` account that only speaks the
  Cangyuan protocol.
- Image execution account filtering now checks the requested public model's
  account mapping before selection and skips image-only accounts without a
  matching Cangyuan tier. General fallback accounts use the same Cangyuan
  mapping check. Codex job creation also preserves the API-key group column,
  falling back to the loaded group relation only when that column is absent.
- A network/transport failure on a POST is still marked
  `submission_unknown`, while an HTTP response with invalid JSON or an error
  envelope is a definite upstream protocol failure and is not marked
  `submission_unknown`.
- Targeted and affected-package local regressions passed after these changes:
  `go test ./internal/service ./internal/handler ./internal/repository
  ./migrations ./cmd/server -count=1 -timeout=600s`, plus the new account
  purpose, Cangyuan adapter, scheduler, and Codex group-scope assertions.
  The feature-scoped service/repository suite also passed with
  `go test -race` in a Go image with `gcc`.
  Frontend workbench/account-purpose tests (11 tests), typecheck, ESLint, and
  production build also passed.

## 2026-08-06 multi-process local acceptance

- An opt-in disposable-PostgreSQL/Redis integration test now starts two
  independent Worker runtimes with separate repositories, encrypted payload
  stores, Redis clients, and a shared fake Cangyuan upstream.
- Both runtimes are proven idle before the job is inserted; Redis wake-up then
  causes discovery, PostgreSQL claim fencing permits one submission, and the
  lifecycle performs one hold, one settlement, and payload cleanup.
- The test passed normally and under `go test -race` in the isolated local e2e
  network. Post-test checks found zero leftover matching job, payload, or
  account rows. This closes the local multi-process wake-up evidence only;
  staged deployment and production observation remain open.

## 2026-08-06 ambiguous POST failure hardening

- Cangyuan adapter POST responses with HTTP 5xx now carry
  `SubmissionUnknown`, preventing automatic resubmission when a provider or
  intermediary may have accepted the request before returning an error.
  HTTP 429 and explicit client errors retain their safe retry/rejection
  behavior; transport failures were already treated as ambiguous.
- Adapter contract tests and Worker tests prove that 502/500 submission
  outcomes do not cause a second upstream generation or a second hold, while
  invalid successful JSON remains a definite protocol failure.

## 2026-08-06 local verification refresh

- Full backend regression passed in the local Docker Go environment with
  `CGO_ENABLED=0`: `go test ./... -count=1 -timeout=900s`.
- Feature-scoped race regression passed in `golang:1.26.5` with CGO for the
  Cangyuan/ImageGeneration/Codex image service tests, dedicated-image and
  workbench handler tests, and image-generation repository tests.
- Frontend `corepack pnpm typecheck`, `corepack pnpm lint:check`, and
  `corepack pnpm build` passed. The focused image/account/workbench suite
  passed with 4 files and 51 tests.
- A full single-threaded frontend Vitest run is not clean in the current
  baseline: 200/214 files passed, 14 failed; 1,396/1,465 tests passed, 69
  failed; and Vitest reported 3 unhandled errors. The failures are in
  unrelated existing admin/payment/navigation/storage/media tests and are not
  attributed to the dedicated-image change.
- `git diff --check` passed, the exact user-provided Cangyuan key was absent
  from tracked workspace files, and all live provider evidence remains tied to
  the Hong Kong node. No Singapore-node reachability is claimed.
- A local Markdown-link audit covered 29 Markdown files and 11 local links with
  zero missing local targets. The source-file credential scan covered tracked
  and relevant untracked implementation/documentation files without finding
  the user-provided key or any long `sk-` credential.
- `openspec` is not installed in this workspace; strict OpenSpec CLI validation
  and staged/production rollout observation remain external gates. The
  dedicated-image feature flags remain disabled by default.
