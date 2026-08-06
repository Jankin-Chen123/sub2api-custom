## ADDED Requirements

### Requirement: 账号用途必须只有普通与生图专用两种
系统 SHALL 使用账号 `extra.account_purpose` 区分 `general` 与 `image_only`。字段缺失、空值和升级前账号 MUST 视为 `general`；写入其他值 MUST 返回校验错误。第一版 `image_only` MUST 只表示使用 OpenAI API Key 凭据接入的沧元生图账号，不得新增认证类型或更多用途枚举。

#### Scenario: 读取升级前账号
- **WHEN** 一个现有账号没有 `extra.account_purpose`
- **THEN** 调度和管理 API MUST 将其视为 `general`
- **THEN** 该账号升级后的普通请求行为 MUST 与升级前一致

#### Scenario: 管理员创建沧元账号
- **WHEN** 管理员提交 `account_purpose=image_only`、API Key、Base URL 和模型映射
- **THEN** 系统 MUST 保存为现有 OpenAI API Key 账号
- **THEN** 系统 MUST 继续使用现有分组、优先级、状态和并发字段

#### Scenario: 写入未知用途
- **WHEN** 管理员提交除 `general`、`image_only` 外的用途
- **THEN** 系统 MUST 返回 400 且不修改账号

### Requirement: 非图片执行流量必须与生图专用账号强隔离
系统 MUST 在所有普通文本、代码、Responses、Chat Completions、WebSocket、工具协调和图片规划账号选择中排除 `image_only`。该隔离 MUST 在调度器能力过滤层实现，而不能依赖 Handler 关键词或调用方自觉。

#### Scenario: 混合分组处理普通对话
- **WHEN** 同一分组同时包含可用 general 和 image-only 账号且收到普通对话
- **THEN** 系统 MUST 只在 general 账号中调度
- **THEN** image-only 账号并发计数 MUST NOT 因该请求增加

#### Scenario: 分组只有 image-only 账号却收到文本请求
- **WHEN** 文本请求所在组没有可用 general 账号
- **THEN** 系统 MUST 按普通账号不可用返回错误
- **THEN** 系统 MUST NOT 尝试把文本请求发给沧元

### Requirement: 图片执行必须按用途、模型和健康状态选择账号
当且仅当内部阶段为 `image_execution` 时，系统 SHALL 优先选择同组 `image_only` 账号。候选账号 MUST 同时满足启用、分组归属、并发、模型映射、用途和健康条件。系统 MUST NOT 仅因为提示词包含“图片”“2K”“4K”而直接进入该阶段。

#### Scenario: 同组有合格的 image-only 账号
- **WHEN** 图片执行请求的公共模型可由一个或多个 image-only 账号映射
- **THEN** 系统 MUST 在这些账号内使用现有优先级和并发规则选择
- **THEN** 系统 MUST 把映射后的沧元模型传给适配器

#### Scenario: 模型不在账号白名单
- **WHEN** image-only 账号的 `model_mapping` 不包含请求公共模型或目标档位
- **THEN** 该账号 MUST 被排除
- **THEN** 系统 MUST NOT 猜测或绕过模型白名单

#### Scenario: 用户只是讨论图片 API
- **WHEN** 用户询问如何编写图片 API、否定生成、引用 namespace 或分析图片能力但未要求实际执行
- **THEN** 系统 MUST 保持 `text` 阶段
- **THEN** 系统 MUST NOT 创建图片任务或占用 image-only 账号

### Requirement: 回退到普通图片账号必须显式且单向
专用图片路由和 general fallback MUST 分别受默认关闭的配置控制。没有合格 image-only 账号且 fallback 关闭时，系统 MUST 返回 `image_provider_unavailable`；管理员显式开启 fallback 后，图片执行 MAY 使用现有普通图片账号。任何普通流量 MUST NOT 反向回退到 image-only。

#### Scenario: 专用账号不可用且 fallback 关闭
- **WHEN** 图片执行找不到合格 image-only 账号
- **THEN** 系统 MUST 返回稳定的不可用错误
- **THEN** 系统 MUST NOT 消耗普通账号额度

#### Scenario: fallback 已开启
- **WHEN** 图片执行找不到合格 image-only 账号但 fallback 已显式开启
- **THEN** 系统 MAY 使用现有普通 Images 调度链路
- **THEN** 响应 MUST 保持公开 Images 契约

### Requirement: 上游图片任务必须保持账号粘性
在上游接受提交并返回 task ID 后，系统 MUST 将任务绑定到原 `account_id`。后续查询、下载和错误归因 MUST 使用该账号；不得因账号优先级、并发、健康状态或服务重启而换账号重新提交。

#### Scenario: 轮询期间原账号达到并发上限
- **WHEN** 已提交任务进入轮询且原账号当前无新请求并发额度
- **THEN** 系统 MUST 继续使用原账号查询该 task ID
- **THEN** 系统 MUST NOT 在其他账号创建新任务

#### Scenario: 原账号被管理员禁用
- **WHEN** 已提交任务的账号在轮询期间被禁用
- **THEN** Worker MUST 允许只读收尾该既有任务或进入可恢复错误状态
- **THEN** 新任务 MUST NOT 再选择该账号

### Requirement: 普通账号兼容性必须可证明
功能开关关闭时，所有旧账号和现有普通图片路径 MUST 保持升级前行为。实现 MUST 建立黄金请求，覆盖 HTTP、SSE、WebSocket、Responses、Chat、工具和普通 Images API，并断言账号选择、请求体、响应 envelope 与计费没有非预期变化。

#### Scenario: 升级但未启用专用路由
- **WHEN** 部署包含本变更且管理员未开启功能
- **THEN** 系统 MUST 按现有链路处理所有请求
- **THEN** 系统 MUST NOT 创建新型持久图片任务或调用沧元
