# 沧元专用生图路由变更索引

本 change 是后续开发的权威需求基线；当前工作区已按其中契约实现本地代码、接口和测试。真实上游 POC、灰度与发布门禁仍以 `tasks.md` 和 `verification.md` 中未完成项为准。

## 阅读顺序

1. `proposal.md`：为什么做、做什么、不做什么。
2. `design.md`：账号模型、路由、Codex 上下文编排、任务状态机、存储、计费与安全设计。
3. `specs/*/spec.md`：必须满足的可验收行为。
4. `tasks.md`：按风险排序的实施清单。
5. `verification.md`：POC 门槛、测试矩阵、灰度和回滚。
6. `docs/CANGYUAN_IMAGE_INTEGRATION.md`：开发者实施手册。
7. `docs/CANGYUAN_DEDICATED_IMAGE_API.md`：对外兼容 API、工作台 API 和管理员 API 契约。
8. `docs/CANGYUAN_REAL_POC_RESULTS.md`：脱敏的真实上游 POC 结果和复现方式。

## 决策摘要

- 账号用途只分 `general` 和 `image_only`；字段缺失视为 `general`。
- 第一版 `image_only` 只适配沧元，不建设通用生图供应商管理系统。
- 普通请求永远不能调度到 `image_only`；执行图片任务时优先选择 `image_only`。
- Codex 长上下文生图采用“原文本账号规划 → 沧元执行 → 结果回填原会话”，不能把整个对话切给沧元。
- `previous_response_id` 始终绑定原文本账号；图片任务拥有独立粘性。
- PostgreSQL 是异步任务事实源，并承载加密临时 payload；Redis 仅用于可选的唤醒、锁和短期限流，对象存储承载最终图片。
- 本地已实现 HTTP/SSE/WebSocket 两阶段桥接，并已用本机 Codex CLI 0.116.0 验证两种传输下的图片呈现；Responses Lite、完整断线/usage 语义和生产灰度仍未通过，因此正式启用继续保持关闭。

## 变更控制

实现中如发现真实协议、沧元行为或现有代码与本文档不同，必须先更新本 change 并评审，再改代码。不得让代码成为未记录的新事实源。
