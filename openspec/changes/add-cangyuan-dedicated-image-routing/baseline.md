# Baseline and verification record

Recorded locally on 2026-08-04 (Asia/Shanghai). No server, production
database, production container, or real provider credential was used.

## Repository baseline

- Branch: `custom/frontend`
- HEAD at the start of this verification pass: `b7d5c87c3baee5ab367b698fcb2eaf5ff9ef1d0b`
- Cangyuan documentation source: `https://ai.cangyuansuanli.cn/docs/api`
- Documentation version: the web documentation does not expose a version; the
  local contract records the observed endpoint and field shapes.

## Local verification evidence

The following checks have passed against the current worktree:

- `go test ./internal/service ./internal/repository -count=1`
- Codex-focused `go test -race ./internal/service ... -count=1`
- `go test ./internal/config ./internal/handler -count=1`
- `go test ./... -run '^$'`
- local Cangyuan adapter/worker smoke tests using an in-process fake HTTPS
  upstream (no external request)
- `git diff --check`
- frontend `pnpm test:run`, `pnpm typecheck`, `pnpm lint:check`, and `pnpm build`

Additional local tests cover the Cangyuan generation/edit adapter, async
polling, task fencing/idempotency, object-result handling, account-purpose
isolation, Codex planning/replay, and HTTP/SSE/WebSocket error mapping.

## Explicitly not verified here

- real paid Cangyuan generation/edit requests and their live response shape;
- real Codex HTTP/SSE/Responses WebSocket/Responses Lite client behavior;
- production deployment or multi-instance failure injection.

Those checks remain gated by the feature flags and must be run only after a
new, non-exposed credential is entered locally and the actual response fixtures
are reviewed and redacted.
