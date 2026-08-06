## ADDED Requirements

### Requirement: Codex 生图必须分为规划和执行两个账号阶段
系统 SHALL 让原 `general` 文本账号处理用户轮次和 `image_planning`，让 `image_only` 账号只处理 `image_execution`。系统 MUST NOT 把完整 Codex 请求或会话直接转发到只支持图片模型的沧元账号。

#### Scenario: 长对话后请求思维导图
- **WHEN** 用户在已有长上下文中要求把已讨论知识生成图片
- **THEN** 原文本账号 MUST 基于完整上下文生成自包含图片计划
- **THEN** 沧元 MUST 只接收经验证的图片参数与完整 visual prompt

#### Scenario: 沧元账号不支持 Responses
- **WHEN** image-only 账号模型列表只有图片模型
- **THEN** 系统 MUST 通过内部适配器执行图片任务
- **THEN** 系统 MUST NOT 向该账号发送 Responses 对话请求

### Requirement: 规划桥不得依赖客户端预先声明原生图片工具
当功能开关已开启且请求来自官方 Codex Responses 客户端、分组允许生图时，网关 MUST 能在客户端未携带 `image_generation` 工具声明的普通轮次进入规划桥，并向 `general` 账号注入内部 `sub2api_generate_image` 工具。普通文本轮次 MUST 仍由 `general` 账号处理；只有经模型产生且服务端校验通过的私有工具调用才能创建图片任务。

#### Scenario: Codex CLI 未声明 image_generation
- **WHEN** 官方 Codex 客户端发送没有原生 `image_generation` 工具声明的普通 Responses 轮次
- **THEN** 网关 MUST 保持该会话在规划桥和 `general` 账号上
- **THEN** 网关 MUST NOT 因缺少声明而把后续生图轮次直接转给 `image_only` 或漏掉私有工具规划

### Requirement: 文本会话与图片任务必须拥有独立粘性
Codex 的 `previous_response_id` 和文本会话 MUST 继续绑定原 general 账号。每个已提交图片任务 MUST 独立绑定执行它的 image-only 账号。图片完成、失败、重试或后续修改请求 MUST NOT 把文本会话迁移到沧元。

#### Scenario: 图片完成后继续聊天
- **WHEN** 图片结果已返回且用户继续提问
- **THEN** 后续文本轮次 MUST 回到原 general 账号和会话链
- **THEN** 上游沧元 task ID MUST NOT 被用作文本 previous response ID

#### Scenario: 用户要求修改刚才图片
- **WHEN** 用户在后续轮次要求基于上一张图修改
- **THEN** 原文本账号 MUST 理解修改要求并形成新的 edit 计划
- **THEN** 新图片任务 MAY 选择合格 image-only 账号，但旧任务记录保持不变

### Requirement: 执行意图不得仅靠关键词决定
系统 MUST 区分讨论、说明、否定、被动 namespace 与真实执行意图。只有可验证的内部图片工具调用或等价显式编排信号才能创建任务。现有显式意图检测 MAY 决定是否提供工具，但 MUST NOT 单独承担付费执行授权。

#### Scenario: 被动工具 namespace 出现在上下文
- **WHEN** 请求仅携带 `image_gen` namespace 或工具说明而用户没有要求执行
- **THEN** 系统 MUST NOT创建图片任务

#### Scenario: 用户明确要求原图直出
- **WHEN** 用户明确要求立即生成指定图片
- **THEN** 文本模型 MAY 产生内部图片工具调用
- **THEN** 网关只有在 schema、权限、额度和参数验证通过后才能执行

### Requirement: 图片计划必须结构化、自包含且可验证
内部工具参数 SHALL 包含 title、summary、sections、relationships、must_include、must_not_invent、layout、language、resolution、aspect_ratio 和 visual_prompt 中适用字段。`visual_prompt` MUST 在不读取原聊天记录的情况下足以执行。系统 MUST 对字段大小、枚举、分辨率、提示词长度和禁止字段做服务端校验。

#### Scenario: 模型只返回“按之前讨论生成”
- **WHEN** tool call 的 visual prompt 依赖“之前”“上述”等未展开引用且缺少必要摘要
- **THEN** 系统 MUST 判定 `image_plan_invalid`
- **THEN** 系统 MAY 要求原文本模型修复一次，但 MUST NOT 把残缺计划交给沧元

#### Scenario: 计划包含未讨论的事实
- **WHEN** `must_not_invent` 或校验结果表明计划引入未经确认的关键数据
- **THEN** 编排器 MUST 要求文本模型修订或向用户说明
- **THEN** 系统 MUST NOT 静默把臆造内容当作历史事实

### Requirement: Codex 协议实现必须经过真实 POC 门禁
系统 MUST 在实现正式编排前确认真实 HTTP、SSE、Responses WebSocket 和 Responses Lite 的工具调用与图片结果事件格式。每种对外形态 MUST 有脱敏 fixture 和端到端客户端验证；未经验证的形态 MUST 由独立 feature flag 保持关闭。

#### Scenario: SSE 图片事件未被客户端识别
- **WHEN** POC 中合成事件无法由 Codex 显示为图片或破坏流终止语义
- **THEN** SSE 编排 MUST NOT 上线
- **THEN** 直接 Images API 和工作台 MAY 独立继续开发

#### Scenario: 工具调用被重复投递
- **WHEN** 客户端断线重试导致同一 tool call ID 再次出现
- **THEN** 系统 MUST 返回同一内部图片任务或结果
- **THEN** 系统 MUST NOT 创建第二次上游生成或扣费

### Requirement: 工具结果必须按原协议回填且不伪造会话
编排器 SHALL 将图片任务状态/结果转换为 POC 确认的 Responses 工具输出或图片事件，保持事件顺序、关联 ID、终止原因、错误和 usage 语义。系统 MUST NOT 伪造上游 `previous_response_id` 或把沧元响应冒充文本账号的原生响应 ID。

#### Scenario: 图片生成失败
- **WHEN** 图片任务进入失败终态
- **THEN** Codex MUST 收到可理解的工具失败事件或文本错误
- **THEN** 错误 MUST 不泄露账号、上游 task ID 或 Key

#### Scenario: 客户端取消等待
- **WHEN** Codex 连接在上游已接受任务后取消
- **THEN** 后端 MUST 保留任务并继续安全收尾
- **THEN** 重连或查询 MUST 能通过幂等 ID 获取已有结果

### Requirement: 知识图生成限制必须对用户透明
当用户要求大量精确文字、严格关系或可编辑图表时，系统或工作台 SHALL 提示生成模型可能出现错字、漏字和布局偏差。第一版 MUST NOT 声称普通生图等价于 Mermaid/SVG 精确渲染。

#### Scenario: 用户要求逐字准确的复杂中文图
- **WHEN** 请求包含大量必须逐字准确的中文节点
- **THEN** 系统 SHOULD 提示能力限制并仍可在用户接受后生成
- **THEN** 失败验收 MUST 区分协议故障与模型视觉质量限制
