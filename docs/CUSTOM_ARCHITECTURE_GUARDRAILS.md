# Custom Architecture Guardrails & Sync Discipline

> 本文档定义 `sub2api-custom`（Fork 自 `Wei-Shaw/sub2api`）在保持低冲突上游同步前提下的工程边界、静态门禁与分阶段重构路线。

---

## 一、核心原则：高内聚定制，低冲突同步

由于本仓库持续合并上游 `Wei-Shaw/sub2api` 发布版本，所有结构治理遵循以下约束：

1. **禁止打散上游原有目录与包布局**：不移动 `internal/middleware/`、`internal/util/`、`internal/model/`、上游 `auth_*_oauth.go` 或 `cmd/server/wire.go` 的既定路径，避免后续合并产生大规模文件冲突。
2. **在扩展点隔离定制代码**：
   - **后端 DI 装配**：纯定制服务的 Wire Provider 函数与集合统一放在 `backend/internal/service/wire_custom.go`（例如 `CustomImageGenerationProviderSet`、`ProvideNewcomerCampaignService`、`ProvideRedeemService`、`ProvideGatewayService`、`ProvideOpenAIGatewayService`、`ProvideChannelMonitorAccountProbe`），纯定制 Repository 装配集合放在 `backend/internal/repository/wire_custom.go`（`CustomProviderSet`），仅在 `backend/internal/service/wire.go` 与 `backend/internal/repository/wire.go` 保留最小化的集合引用。
   - **后端上游文件定制扩展**：直接扩展上游 Repository、Handler 或 Service 的定制方法与辅助函数，优先拆出到同包独立 `*_custom.go` 文件（如 `backend/internal/repository/channel_monitor_repo_custom.go`、`backend/internal/handler/openai_gateway_handler_custom.go`、`backend/internal/service/openai_account_scheduler_custom.go`），避免将大段定制逻辑直接塞进上游热冲突文件。
   - **后端新增独立业务域**：优先建立独立子包（参考 `internal/payment`、`internal/securityaudit`、`internal/managementmcp`），避免继续向平铺的 `package service` 根目录堆叠大文件。
   - **数据库迁移**：遵循 `backend/migrations/custom_migration_policy_test.go` 约束，新定制迁移统一采用 `custom_YYYYMMDD_<description>[_notx].sql` 命名，不占用上游数字序号且不修改已上线的历史迁移文件。
   - **前端新业务域**：统一采用 Feature Slice 目录 `frontend/src/features/<feature-name>/`（参考 `src/features/prompt-audit/`、`src/features/channel-monitor-v2/`、`src/features/image-workbench/`、`src/features/paper-transition/`），将 API、类型、纯格式化函数、Composable 与单测收拢到同一功能目录内。
   - **前端系统设置定制项**：禁止继续直接向 `frontend/src/views/admin/SettingsView.vue` 内联大段定制表单，统一抽离为 `frontend/src/components/admin/settings/` 下的独立子组件（如 `DedicatedImageSettingsCards.vue`、`ContactPageSettingsCard.vue`、`AdminAccessKeysSettings.vue`、`RechargeBonusTierEditor.vue`、`RegistrationEmailBlacklistEditor.vue`）。

---

## 二、已启用的静态门禁（`backend/.golangci.yml` 与 `frontend/.eslintrc.cjs`）

`depguard` 在保留上游 `service-no-repository` 与 `handler-no-repository` 规则的基础上，新增两条 Ent ORM 分层门禁；Phase 3 完成后，`service-no-ent` 与 `handler-no-ent` 白名单中已无任何纯定制业务文件（仅剩历史冻结的上游文件）：

| 规则名 | 作用范围 | 禁止依赖 | 治理策略 |
| --- | --- | --- | --- |
| `handler-no-ent` | `**/internal/handler/**`（排除 `*_test.go`） | `github.com/Wei-Shaw/sub2api/ent`、`entgo.io/ent` | 仅余上游历史文件列为**冻结白名单**；禁止新增 Handler 直接依赖 Ent |
| `service-no-ent` | `**/internal/service/**`（排除 `*_test.go`、`wire.go`、`wire_custom.go`） | `github.com/Wei-Shaw/sub2api/ent`、`entgo.io/ent` | 存量直接依赖 Ent 的上游文件列为**冻结白名单**；纯定制业务文件已清零，新 Service 必须通过 Repository 接口访问数据 |

> [!IMPORTANT]
> 两个白名单均为**只减不增（Shrink-Only）**策略。任何新文件不得加入白名单；若某个白名单文件被拆分或重构移除 `ent` 依赖，应同步从白名单中删去对应路径。

- **前端特征目录严格类型门禁（`frontend/.eslintrc.cjs`）**：通过 ESLint `overrides` 对 `src/features/**/*.ts` 与 `src/features/**/*.vue`（排除 `**/*.spec.ts`）强制开启 `"@typescript-eslint/no-explicit-any": "error"`，防止新特性切片回流 `any`。

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

### Phase 3（已完成于 `rebuild` 分支）
1. **移除迎新活动 Service 对 Ent 的直接依赖**：在 `backend/internal/service/newcomer_campaign_service.go` 定义 `NewcomerCampaignSQLClient` 与 `NewcomerCampaignRepository`，由 `backend/internal/repository/newcomer_campaign_repo.go` 与 `backend/internal/repository/wire_custom.go`（`CustomProviderSet`）承接 SQL 执行、事务创建与注册事务上下文适配。业务 SQL 仍由 Service 持有；后续可继续按业务操作下沉。同步清除 `newcomer_campaign_*.go` 全部 4 个 Service 文件的 `ent` 依赖，并将其从 `service-no-ent` 白名单移除。真实仓储与注册事务的提交、回滚由 `newcomer_campaign_repo_integration_test.go` 覆盖。
2. **抽离上游热冲突网关与调度扩展**：将 `backend/internal/handler/openai_gateway_handler.go` 与 `backend/internal/service/openai_account_scheduler.go` 中的定制辅助逻辑分别外置到 `openai_gateway_handler_custom.go`（上游差异从 `+300/-81` 降至 `+159/-92`）与 `openai_account_scheduler_custom.go`（上游差异从 `+246/-34` 降至 `+59/-41`）。注：`redeem_service.go` 与 `subscription_service.go` 的定制方法直接操作 `s.entClient`，为避免违反只减不增的 `service-no-ent` 门禁，暂保留原位等待后续连同 Ent 查询一起下沉至 Repository。
3. **完成 `ImageWorkbenchView.vue` 组件化拆解**：拆分出 `frontend/src/features/image-workbench/components/` 下的 5 个子组件（`ImageWorkbenchHeader.vue`、`ImageWorkbenchFormPanel.vue`、`ImageWorkbenchBatchList.vue`、`ImageWorkbenchLibrary.vue`、`ImageWorkbenchArtworkPreviewModal.vue`）以及 `useImageWorkbenchReferences.ts` 与 `imageWorkbenchFormatters.ts` 中的纯比例/文件读取辅助函数。主视图从 1,570 个物理行缩减至 821 行，其中 772 行为非空行。草稿、预览与遮罩编辑分别由 Composable 承接，模板结构和样式随组件迁移。
4. **清零定制前端 `: any` 并开启 `src/features/` 严格 Lint**：移除 `CheckinPrizeManager.vue`（2 处）与 `DailyCheckinWheel.vue`（1 处）最后 3 处 `catch (error: any)`（纯定制非测试前端文件 `: any` 清零），并在 `frontend/.eslintrc.cjs` 中对 `src/features/**/*.{ts,vue}`（排除 `**/*.spec.ts`）强制开启 `@typescript-eslint/no-explicit-any: "error"`。

### 后续长期演进（按需评估）
1. **`redeem_service.go` / `subscription_service.go` 定制 Ent 查询下沉**：在具备真实 Postgres 集成测试环境时，将返利审核与套餐购买流程中的 `s.entClient` 事务逻辑封装至独立 Repository，进一步收窄上游合入冲突面。
2. **`handler-no-ent` / `service-no-ent` 剩余上游白名单**：如 `payment_handler.go`、`admin/payment_handler.go` 等仅携带极小差异的定制代码，优先跟随上游结构演进，避免为极小改动引入合入冲突。
3. **`cmd/server/wire.go` 的 `provideCleanup` 参数列表收敛**：属于上游核心装配入口，待上游调整或明确多模块统一生命周期设计后再评估引入 `Lifecycle` / `ShutdownCloser` 注册集合。
4. **前端特征类型隔离纪律**：`frontend/src/types/index.ts` 仅保留上游共享类型及极小差异（`+19/-1`），后续所有新增定制业务类型严格收敛于 `frontend/src/features/<feature>/types.ts`。
