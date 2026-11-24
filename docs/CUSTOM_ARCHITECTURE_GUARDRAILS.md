# Custom Architecture Guardrails & Sync Discipline

> 本文档定义 `sub2api-custom`（Fork 自 `Wei-Shaw/sub2api`）在保持低冲突上游同步前提下的工程边界、静态门禁与分阶段重构路线。

---

## 一、核心原则：高内聚定制，低冲突同步

由于本仓库持续合并上游 `Wei-Shaw/sub2api` 发布版本，所有结构治理遵循以下约束：

1. **禁止打散上游原有目录与包布局**：不移动 `internal/middleware/`、`internal/util/`、`internal/model/`、上游 `auth_*_oauth.go` 或 `cmd/server/wire.go` 的既定路径，避免后续合并产生大规模文件冲突。
2. **在扩展点隔离定制代码**：
   - **后端 DI 装配**：纯定制服务的 Wire Provider 函数与集合统一放在 `backend/internal/service/wire_custom.go`（例如 `CustomImageGenerationProviderSet`、`ProvideRedeemService`、`ProvideGatewayService`、`ProvideOpenAIGatewayService`、`ProvideChannelMonitorAccountProbe`），仅在 `backend/internal/service/wire.go` 保留最小化的 `wire.NewSet` 引用。
   - **后端上游 Repository 定制扩展**：直接扩展上游 Repository 结构体的方法或辅助函数统一放在同包独立 `*_custom.go` 文件（如 `backend/internal/repository/channel_monitor_repo_custom.go`），避免把大段定制方法直接塞进上游 `channel_monitor_repo.go`。
   - **后端新增独立业务域**：优先建立独立子包（参考 `internal/payment`、`internal/securityaudit`、`internal/managementmcp`），避免继续向平铺的 `package service` 根目录堆叠大文件。
   - **数据库迁移**：遵循 `backend/migrations/custom_migration_policy_test.go` 约束，新定制迁移统一采用 `custom_YYYYMMDD_<description>[_notx].sql` 命名，不占用上游数字序号且不修改已上线的历史迁移文件。
   - **前端新业务域**：统一采用 Feature Slice 目录 `frontend/src/features/<feature-name>/`（参考 `src/features/prompt-audit/`、`src/features/channel-monitor-v2/`、`src/features/image-workbench/`、`src/features/paper-transition/`），将 API、类型、纯格式化函数、Composable 与单测收拢到同一功能目录内。
   - **前端系统设置定制项**：禁止继续直接向 `frontend/src/views/admin/SettingsView.vue` 内联大段定制表单，统一抽离为 `frontend/src/components/admin/settings/` 下的独立子组件（如 `DedicatedImageSettingsCards.vue`、`ContactPageSettingsCard.vue`、`AdminAccessKeysSettings.vue`、`RechargeBonusTierEditor.vue`、`RegistrationEmailBlacklistEditor.vue`）。

---

## 二、已启用的静态门禁（`backend/.golangci.yml`）

`depguard` 在保留上游 `service-no-repository` 与 `handler-no-repository` 规则的基础上，新增两条 Ent ORM 分层门禁：

| 规则名 | 作用范围 | 禁止依赖 | 治理策略 |
| --- | --- | --- | --- |
| `handler-no-ent` | `**/internal/handler/**`（排除 `*_test.go`） | `github.com/Wei-Shaw/sub2api/ent`、`entgo.io/ent` | 8 个历史文件列为**冻结白名单**；禁止新增 Handler 直接依赖 Ent |
| `service-no-ent` | `**/internal/service/**`（排除 `*_test.go`、`wire.go`、`wire_custom.go`） | `github.com/Wei-Shaw/sub2api/ent`、`entgo.io/ent` | 存量直接依赖 Ent 的文件列为**冻结白名单**；新 Service 必须通过 Repository 接口访问数据 |

> [!IMPORTANT]
> 两个白名单均为**只减不增（Shrink-Only）**策略。任何新文件不得加入白名单；若某个白名单文件被拆分或重构移除 `ent` 依赖，应同步从白名单中删去对应路径。

### 本地校验注意

- `backend/go.mod` 指定 Go `1.27.4`。若本地 `golangci-lint` 二进制由更低版本 Go（如 `1.27.0`）编译，运行带类型检查的 linter 可能出现 `export data version 5 is greater than maximum supported version 4`。
- 本地可先用以下命令验证依赖边界与类型安全，或用 Go 1.27.4 重新安装 `golangci-lint`（`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1`）：
  ```powershell
  golangci-lint run --enable-only=depguard ./...
  go vet ./cmd/server/... ./internal/service/... ./internal/handler/... ./migrations/...
  ```

---

## 三、分阶段演进路线

### Phase 1（已完成于 `rebuild` 分支）
- 清理本地未忽略缓存目录（`.playwright-cli/`、`frontend/.playwright-cli/`、`.pnpm-store/`）。
- 新增 `handler-no-ent` / `service-no-ent` `depguard` 门禁。
- 将纯定制 Wire Provider 抽离至 `backend/internal/service/wire_custom.go`。
- 拆解纯定制巨型后端文件：`codex_dedicated_image_bridge.go`（拆为 bridge / planner / events / replay 4 个文件）与 `newcomer_campaign_service.go`（拆为 service / reconcile / status / backfill 4 个文件）。
- 抽离 `SettingsView.vue` 中的定制区块至 `DedicatedImageSettingsCards.vue` / `ContactPageSettingsCard.vue`，拆解 `ImageWorkbenchView.vue` 纯函数与画布遮罩编辑器至 `src/features/image-workbench/`，外置 `ContactPageView.vue` 样式至 `contact-page.css`。

### Phase 2（已完成于 `rebuild` 分支）
- **抽离上游 Repository 中的定制扩展**：将 `backend/internal/repository/channel_monitor_repo.go` 中的定制探活/健康快照方法移至 `channel_monitor_repo_custom.go`（`channel_monitor_repo.go` 对 `upstream/main` 差异收敛为 `+22/-1`）。
- **按职责拆解纯定制 Handler**：将 `backend/internal/handler/dedicated_image_handler.go` 拆分为调度主干（`dedicated_image_handler.go`）、请求标准化/幂等解析（`dedicated_image_request.go`）与响应/错误输出（`dedicated_image_response.go`）。
- **拆解纯定制 Service 大文件**：
  - `image_generation_worker.go` 拆出 `image_generation_selector.go`（账号租约/适配器工厂）与 `image_generation_worker_runtime.go`（后台运行时生命周期）。
  - `cangyuan_image_adapter.go` 拆出 `cangyuan_image_validation.go`（模型能力映射、请求/资源校验与规范化）。
  - `channel_monitor_health.go` 拆出 `channel_monitor_health_selection.go`（调度加权与粘性旁路决策）。
- **迁移误放路由目录的定制模块**：将 `frontend/src/router/paperTurn.ts`、`paperSound.ts` 及单测移至 `frontend/src/features/paper-transition/`，保持 `router/` 仅承载路由表与标题解析。
- **深化 `ImageWorkbenchView.vue` 模块化并清零 `: any`**：将草稿持久化与预览缓存分别抽离为 `useImageWorkbenchDraft.ts` 与 `useImageWorkbenchPreviews.ts`（含单测 `useImageWorkbenchDraft.spec.ts`），补充 `extractErrorMessage`，将视图内 9 处 `catch (error: any)` 全部替换为 `unknown`；同时将 `JournalBannerSticker.vue` 的 `<style scoped>` 完整外置至 `journal-banner-sticker.css`（保持 DOM 与视觉样式 100% 不变）。

### Phase 3（后续演进建议）
1. **将 `newcomer_campaign` 原始 SQL 下沉为 `NewcomerCampaignRepository`**：把 `newcomer_campaign_*.go` 中的 `ent` / `sql` 访问收拢到 `internal/repository/`，从 `service-no-ent` 白名单中彻底移除迎新活动相关文件。
2. **继续精简 `ImageWorkbenchView.vue` 与定制类型**：将生图工作台 UI 模板拆分为参数面板、预览画布、作品库与遮罩编辑弹窗子组件，并将纯定制前端类型从 `frontend/src/types/index.ts` 拆入对应 `src/features/<feature>/types.ts`。
3. **继续消解 `handler-no-ent` 历史白名单**：优先将定制占比高的 `payment_handler.go` / `admin/payment_handler.go` 响应映射逻辑下沉为 service DTO。
4. **收拢生命周期注册与特征目录严格 Lint**：评估在 `cmd/server/` 引入上游兼容的 `Lifecycle` / `ShutdownCloser` 注册集合收敛 `provideCleanup` 的长参数列表，并在 `frontend/src/features/` 内试点启用更严格的 ESLint 规则（如限制新增 `no-explicit-any`）。
