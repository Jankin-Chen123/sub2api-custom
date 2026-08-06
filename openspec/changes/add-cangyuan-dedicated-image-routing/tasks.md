# Implementation Tasks

## 0. 冻结基线与 POC（Go/No-Go）

- [x] 0.1 记录当前分支、commit、现有 Images/Codex 相关测试基线和沧元文档版本/抓取日期
- [x] 0.2 用最小 1K 请求验证沧元 generation 同步、异步提交与查询的真实请求/响应/错误格式
- [x] 0.3 验证 edit、JSON 参考图别名、multipart 重复 `image`、mask、URL 与 base64 返回
- [x] 0.4 验证 1K/2K/4K 模型、尺寸边界、`n=1`、冲突 `image_size/output_resolution` 的真实行为
- [ ] 0.5 对真实 Codex HTTP、SSE、WebSocket、Responses Lite 抓包，固定工具声明、调用、图片结果、usage、错误和断线语义；HTTP/SSE/WebSocket 的本机 Codex CLI 图片呈现已通过，Responses Lite 与完整断线/usage 仍是正式启用门槛
- [x] 0.6 验证被动 `image_gen` namespace 不触发执行，明确 tool call ID 可否作为幂等键
- [x] 0.7 把 POC fixture 脱敏后保存为测试数据；任何关键闭环不成立时只推进直接 API/工作台

## 1. 账号用途与管理员配置

- [x] 1.1 为账号 extra 增加 `account_purpose` 解析、校验、缺省兼容和单元测试
- [x] 1.2 扩展管理员账号 CRUD DTO，拒绝未知用途且不改变旧响应兼容性
- [x] 1.3 在账号新增/编辑页增加“普通账号 / 生图专用账号（沧元）”字段和说明
- [x] 1.4 限制 `image_only` 第一版只能用于 OpenAI API Key 凭据；验证 base URL 和模型映射完整性
- [x] 1.5 增加付费测试端点及二次确认（如本轮实现），默认使用 1K

## 2. 调度隔离

- [x] 2.1 给调度请求增加 `text/image_planning/image_execution` 阶段，并让 Codex 规划使用独立阶段
- [x] 2.2 普通选择器强制排除 `image_only`，覆盖 Responses/Chat/WS/工具/重试路径
- [x] 2.3 图片选择器优先同组 `image_only`，检查状态、并发、模型映射和账户用途
- [x] 2.4 实现默认关闭的专用路由开关和默认关闭的 general fallback
- [x] 2.5 获得上游 task ID 后将轮询绑定原 account ID；测试禁用/并发变化时不重提
- [x] 2.6 建立旧账号、普通账号、混合分组和无专用账号的黄金回归矩阵

## 3. 沧元适配器

- [x] 3.1 定义独立 adapter 接口与 request/result/error DTO，不让上游字段泄漏到 Handler
- [x] 3.2 实现 base URL 归一，避免 `/v1/v1`，并实现 generation/edit 同步调用
- [x] 3.3 实现异步提交、generation/edit 查询、状态归一与轮询退避
- [x] 3.4 实现 tier/model/size/ratio/pixel/`n` 一致性校验
- [x] 3.5 实现 JSON/multipart 参考图、去重、9 张/10 MB 限制和 mask 校验
- [x] 3.6 实现受控 HTTPS 下载、MIME 嗅探、重定向与 DNS/IP 防护
- [x] 3.7 实现 url/base64 结果流式转存和响应归一；增加 4K 内存压力测试
- [x] 3.8 用 POC fixture 建立 adapter contract tests，并覆盖 401/429/5xx/timeout/invalid JSON

## 4. 持久图片任务

- [x] 4.1 新增 `image_generation_jobs` SQL migration、索引、枚举约束和清理策略
- [x] 4.2 实现 repository：幂等创建、原子 claim、claim version、租约、状态转换和终态更新
- [x] 4.3 将 Redis 降为唤醒/锁/短期限流；将新 payload 持久化到加密 PostgreSQL 临时表并实现 PostgreSQL 定时扫描恢复
- [x] 4.4 设计加密临时 payload 或受控临时对象，禁止 prompt/图片二进制进入普通表和日志
- [x] 4.5 实现 Worker 生命周期、优雅关闭、轮询退避、租约续期和滞留任务回收
- [x] 4.6 实现 submission_unknown，禁止不明确提交自动重试
- [x] 4.7 实现对象存储转存、URL 续签/content 代理和过期清理
- [x] 4.8 实现预占、释放、成功单次结算和全链路幂等测试

## 5. 兼容 Images API

- [x] 5.1 保持 `/v1` 和无前缀同步/异步/查询路由及现有 envelope
- [x] 5.2 将直接图片请求接入新阶段调度与任务基础设施
- [x] 5.3 对外只暴露 Sub2API task ID 和稳定错误，不返回 account/upstream task/key
- [x] 5.4 明确同步超时后任务是否转为可查询；保持客户端兼容并写入 API 文档
- [x] 5.5 回归现有 OpenAI/Grok 普通图片账号、审核、限流、并发、模型映射和计费

## 6. Codex 编排

- [x] 6.1 定义独立的 `sub2api_generate_image` 内部工具 schema 和 HTTP/SSE/WS 协议转换器；本机 Codex CLI 的 HTTP/SSE 与 WebSocket 图片呈现已通过，Responses Lite 不进入该桥接
- [x] 6.2 不依赖客户端预先声明图片工具，向官方 Codex 规划请求注入私有工具并交给普通模型判断；服务端只执行私有工具调用；普通讨论、长上下文、重复 tool call、否定/非生图路径已有本地回归
- [x] 6.3 保持原文本 account 与 `previous_response_id` 粘性，建立独立 image job 粘性
- [x] 6.4 验证并归一文本模型产生的结构化计划，强制生成自包含 `prompt`
- [x] 6.5 截获 tool call、创建幂等 job、等待并合成 HTTP/SSE/WS 图片结果；Responses Lite 按安全策略保持旁路，待其独立客户端契约验收后再决定是否接入
- [x] 6.6 处理取消、断线、重连、重复 tool call、工具失败、部分文本和 usage 聚合
- [x] 6.7 增加长上下文思维导图端到端测试，验证发往沧元的 prompt 包含关键知识点但不含无关隐私
- [x] 6.8 Codex 编排使用独立 `dedicated_image.codex_bridge_enabled` feature flag；默认关闭

## 7. 生图工作台

- [x] 7.1 实现创建/列表/详情/content 用户 API 和同用户同 API Key 所有权校验
- [x] 7.2 实现 API Key、模型档位、尺寸、提示词、参考图、mask 和格式表单
- [x] 7.3 实现任务进度轮询、失败重试提示、结果预览与下载
- [x] 7.4 前端不接收/保存上游 Key，不把 prompt/base64/签名 URL 写入持久浏览器状态或遥测
- [x] 7.5 加入额度预估、按张费用提示、4K 耗时/成本提示和中文文字局限说明
- [x] 7.6 覆盖桌面/移动端、键盘、上传校验、404 所有权隐藏和过期 URL 刷新测试

## 8. 可观测性、安全与运维

- [x] 8.1 增加 job 状态、队列深度、claim、轮询、上游延迟/错误、存储、结算和用途过滤指标
- [x] 8.2 建立日志字段 allowlist 和 canary 泄漏测试
- [x] 8.3 对 prompt、参考图、mask、base64、Authorization、签名 URL query 和上游响应做泄漏扫描
- [x] 8.4 增加每用户/API Key/group/account 并发和速率限制，单独限制 4K
- [x] 8.5 定义任务保留、临时载荷、对象存储、失败记录和审计日志清理周期
- [x] 8.6 编写队列积压、沧元故障、对象存储故障、结算异常和回滚运行手册

## 9. 验收与发布

- [ ] 9.1 严格验证 OpenSpec，并运行 Markdown 链接检查和 `git diff --check`
- [x] 9.2 运行相关 Go 单测、集成测试、race、迁移测试和前端 Vitest/typecheck/lint/build
- [x] 9.3 运行普通请求黄金回归，证明旧账号默认 general 且无行为变化
- [x] 9.4 运行 1K/2K/4K、generation/edit、同步/异步、工作台、Codex HTTP/SSE/WS E2E
- [x] 9.5 在本地 disposable PostgreSQL/Redis 环境运行多实例抢占、进程重启、Redis 丢失、租约过期、重复请求和单次扣费故障注入；生产故障注入与灰度观察仍属于外部发布门禁
- [ ] 9.6 先 schema/暗部署，再测试组 direct API/workbench 灰度，最后单独灰度 Codex 编排
- [ ] 9.7 观察至少一个完整业务周期后扩大流量；异常时关闭入口但继续收尾已提交任务
- [x] 9.8 已将 JSON mask 的上游 502 差异、内部 multipart 归一、Redis 唤醒非事实源、Responses Lite 旁路等实现差异回写 OpenSpec 与开发/API 文档；严格 CLI 校验仍受工具缺失限制

## Current verification status (2026-08-05)

- Local implementation evidence: full Go tests pass; the focused frontend image/account/workbench suite, typecheck, lint, and production build pass. The full 214-file frontend Vitest run was not used as a release claim because unrelated baseline tests fail in this workspace.
- The Cangyuan generation/model-list, async polling, JSON basic edit, multipart repeated-image, and multipart-mask evidence recorded in `docs/CANGYUAN_REAL_POC_RESULTS.md` remains valid. JSON-mask direct submission returned HTTP 502 and is internally promoted to multipart after asset normalization.
- Cangyuan base64 response behavior, JSON basic edit, and multipart repeated-image/mask behavior are now proven in the 2026-08-05 Hong Kong-node rerun. JSON alias handling is covered by the local public-entry contract tests, but acceptance of every alias by the live provider and real Codex client transport behavior are not yet fully proven; keep `codex_bridge_enabled` disabled until those POCs are complete.
- The relevant Cangyuan/ImageGeneration/Codex race-test subset passes. The full repository race run still has failures outside this feature and must not be reported as a clean full-race result.
- A local fake-upstream flow now covers orchestrator creation, worker preparation, asynchronous submission, a transient polling failure, sticky re-polling, result storage, settlement, and terminal completion. Disposable-PostgreSQL/Redis acceptance also covers two-instance idempotency, claim fencing, expired submission leases, process-handoff polling, Redis-loss payload recovery, and independent Worker wake-up/claim fencing. These checks do not replace live-provider, Responses Lite, or staged-production acceptance.
- Release-gate overrides: the `9.1` strict OpenSpec command could not be run because the `openspec` CLI is not installed in this workspace. The feature-scoped race subset in `9.2` passes; the full repository race run is not clean and must not be reported as a clean full-race result.

## 2026-08-05 verification update

Task 9.4 is complete for the local acceptance environment: Hong Kong-node live
Cangyuan checks cover 1K/2K/4K generation, base64 generation, JSON edit, and
multipart edit/mask; the isolated app covers synchronous/asynchronous jobs,
the workbench, and public HTTP/SSE/Responses WebSocket gateway flows. The
WebSocket client-level check used one connection with an ordinary turn followed
by a 2K image turn carrying `previous_response_id`.

Task 0.5 remains open only because Responses Lite and full disconnect/usage
semantics have not been accepted; official Codex CLI HTTP/SSE and WebSocket
image rendering are now verified locally. Tasks 9.5--9.7
also remain open because Redis-loss/fault-injection coverage and any staged or
production rollout observation are outside this local-only turn.

## 2026-08-05 durable payload update

- Migration `196_image_generation_payloads.sql` now persists new encrypted temporary payloads in PostgreSQL; Redis is no longer required for new payload writes.
- The durable-store Redis-loss regression passes locally, with a read-only legacy Redis fallback for rolling upgrades. Full multi-process fault injection and production rollout gates remain open.
- The current worktree also builds as `codex-sub2api-e2e:current`; a disposable app container from that image passed `/health` without replacing the existing local e2e app.
- The opt-in PostgreSQL integration suite passed against the isolated local e2e database for the durable payload and repository claim/idempotency paths.
- The new opt-in durable payload + Worker integration also passed: fresh
  repository/payload-store instances resumed the same bound upstream task
  after a transient poll failure and completed it without a second submission.
- The same integration now injects an unreachable Redis client while the
  PostgreSQL durable path is active; new payload reads/writes and cleanup still
  pass. Multi-process Redis recovery and staged rollout evidence remain open.

## 2026-08-06 local continuation

- [x] 9.5a local durable-path Redis-loss rerun: the complete repository/migration
  integration set passed with an unreachable Redis client and no leftover
  disposable rows.
- [x] 9.5a also passes under race with two independent Workers racing for the
  same PostgreSQL submission lease; only one reaches the fake upstream.
- [ ] 9.5b multi-process Redis recovery/wake-up and staged deployment evidence
  remains an external release gate.

### 2026-08-06 local continuation

- [x] Added per-frame Responses Lite detection for the WebSocket HTTP bridge;
  Lite frames do not enter the dedicated image planner.
- [x] Added a fail-closed guard for synthetic `resp_img_*` continuation IDs in
  a Lite frame, plus a focused service regression test.
- [ ] Official Codex Responses Lite client acceptance, multi-process Redis
  recovery/wake-up, and staged deployment evidence remain external gates.

## 2026-08-06 wake-up implementation

- [x] Added an optional Redis Pub/Sub wake-up adapter carrying only the
  durable image `job_id`; prompts, credentials, image bytes, upstream task IDs,
  and result URLs are never published.
- [x] The orchestrator publishes only after PostgreSQL job creation. Publish
  failure does not fail the request; the PostgreSQL worker scan remains the
  correctness and recovery path.
- [x] The Worker runtime interrupts idle/retry waits on wake-up, reconnects a
  dropped subscription with bounded backoff, and continues operating when
  Redis is unavailable.
- [x] Added miniredis cross-subscriber coverage, optional-Redis coverage,
  orchestrator publish-failure fallback coverage, and wake-interrupted wait
  coverage, a runtime-level immediate-rescan test, and independent-client
  broadcast coverage. This does not close the staged multi-process/production
  gate.

## 2026-08-06 multi-process local acceptance

- [x] Added an opt-in disposable-PostgreSQL/Redis integration test with two
  independent Worker runtimes, repositories, encrypted payload stores, Redis
  clients, and a shared fake Cangyuan upstream.
- [x] The test proves both runtimes reach idle before job creation, Redis
  wake-up causes discovery, PostgreSQL claim fencing permits one submission,
  and the final lifecycle performs one hold, one settlement, and payload
  cleanup. It passes normally and under `go test -race` in the isolated local
  e2e network; no leftover job, payload, or account rows remain.
- [ ] Staged deployment observation and production multi-process evidence
  remain intentionally open because this task is local-only.

## 2026-08-06 ambiguous submission hardening

- [x] Cangyuan POST 5xx responses now enter `submission_unknown` and cannot be
  automatically resubmitted; adapter and Worker regression tests cover the
  no-duplicate-generation/no-duplicate-hold invariant.

## 2026-08-06 local verification refresh

- [x] The complete backend test suite passed in the local Docker Go environment
  with `CGO_ENABLED=0`: `go test ./... -count=1 -timeout=900s`.
- [x] The feature-scoped race suites passed in `golang:1.26.5` (which provides
  CGO): Cangyuan/ImageGeneration/Codex image service tests, dedicated-image and
  workbench handler tests, and image-generation repository tests.
- [x] Frontend `typecheck`, `lint:check`, production `build`, and the focused
  image/account/workbench Vitest set passed locally.
- [ ] The full frontend Vitest run remains a baseline-environment issue:
  14/214 files failed, 69/1465 tests failed, and 3 unhandled errors. The
  failures are in unrelated admin/payment/navigation/storage/media tests and
  are not used as evidence for this feature.
- [ ] The `openspec` CLI is not installed locally, so strict OpenSpec validation
  remains pending. No production switch is enabled by this local refresh.

## 2026-08-06 continuation verification

- [x] `CGO_ENABLED=0 go test ./... -count=1 -timeout=900s` passed again in the
  local Docker Go environment.
- [x] The focused frontend image/account/workbench suite passed with 5 files and
  54 tests; `corepack pnpm typecheck`, `corepack pnpm lint:check`, and
  `corepack pnpm build` passed serially.
- [x] The Cangyuan/ImageGeneration/Codex service and affected handler/repository
  race subset passed in `golang:1.26.5` with CGO.
- [x] The disposable PostgreSQL/Redis image-generation integration suite passed
  normally and under race, including the two-runtime wake-up test; no production
  service or server was contacted or modified.
