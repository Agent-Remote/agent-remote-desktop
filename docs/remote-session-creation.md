# 远程会话创建 GUI 草案

desktop 提供准确远程会话连接与本地终端管理；本页定义创建 GUI 如何复用 Rust CLI 的调用方
原键。创建弹窗只负责输入、确认和展示，调用方原键、journal、凭据及远程编排仍由 CLI 负责，
Go 不维护第二套远程执行状态机。

跨仓字段、锁序和 Server 原子受理见 [Server 草案](../../agent-remote-server/docs/session-creation-caller-key.md)，
本地 journal 与 POST/GET 恢复见 [CLI 草案](../../agent-remote-cli/docs/session-creation-caller-key.md)。

## 所有权和用户流程

GUI 生成请求表单和确认视图，实际 `request_id`、journal、凭据、远程 POST、重启恢复和
观察全部交给与人工命令相同的 Rust CLI。Go 只保存本地 project、pane、cwd 及 CLI 返回的
公开 session UUID/上下文；不保存 argv、凭据、环境或创建任务正文。创建向导的阶段为：

1. 选择项目、工具、account 和 `new`/interrupted replacement；显示 account 来源但不顺便
   修改默认 account。
2. 调用 CLI 独立读取/ensure 已存在的 workspace/sync，展示完整目标、Node、backend、冲突
   和准备证据；verified flush 只作为瞬时 observation。
3. 用户确认后 CLI 生成 request UUID 并持久化 digest/上下文，再提交一次 caller-key create。
4. 丢响应时显示“查询原请求”，由 CLI GET 原 key；禁止用当前项目 latest、默认账户或新
   `fclaude run` 猜测。拿到明确 session UUID 后，连接仍走已实现的 exact attach 确认。

## 状态呈现

`accepted`、`admission_pending`、`running`、`failed`、`unknown`、`deleted` 是原请求/任务
观察，不等于终端物理状态。界面分别展示进程终止、Skill 保存/发布、同步 flush 和连接状态；
任何一个状态不能合并成“全部完成”。关闭窗口后的后台运行和显式退出确认沿用生命周期
设计，创建未知时退出不得假定远程会话已停止。

所有新增 UI 文案进入 mygo `en`/`zh-Hans` catalog，错误使用稳定错误码，Agent 输出、用户
输入、路径、命令参数和协议标识保持原文。创建请求详情仅显示摘要、UUID、Node/backend、
workspace/project 和可读的阶段，不显示 raw argv。

## 已知兼容边界

- current-project 列表只是提交前选择；同一 project 可能存在不同 account 的会话，不能以
  默认账户覆盖用户已选会话。
- replacement 必须绑定原 account/workspace/project/node/backend 和 interrupted session；
  target 漂移、session 删除、refresh 失败均回到明确选择，而不是自动替代。
- Server account affinity 与实际调度 Node 可能不一致。receipt 的实际 Node 是唯一权威；
  与 prepared Sync node 不相容时显示需要重新审阅，不由 GUI 传 node 强制调度。
- 旧 CLI/Server 不支持 caller-key 时，GUI 只显示“需要支持创建原键的 CLI”，不得降级到
  无法恢复的 POST。

## desktop 验收清单

1. 中英文及 en-XA 下创建确认、pending、unknown、conflict 和 deleted 布局在 900×600 可读；
   长 UUID/错误摘要进入滚动区域。
2. 进程重启、窗口关闭、登录切换、项目切换和 pane 替换不会丢失或跨项目复用 request 上下文。
3. CLI transport timeout 后只能查询原 key；同 key receipt 与 session/task UUID 在恢复后保持不变。
4. sync target/prepare evidence 漂移、flush unknown、Node mismatch、replacement invalid 均
   阻止提交并给出可操作的重新审阅路径。
5. session 被删除后，原请求仍可观察；GUI 不新建 session、不从历史 UUID 猜测当前 session。
6. exact attach 只在用户独立确认并获得完整 session UUID 后执行；创建成功不自动宣称 attach
   成功，终端和后台生命周期分别验证。
7. Go tests、fixtures、helper 和截图全部放 `tests/`，不在 `internal/` 添加 `_test.go`。

当前 GUI 已接入 `sessions.create` 和 owner-scoped `sessions.create.request` desktop-control 方法。
能力页中的 `fclaude new` 直接打开该结构化创建审核；`fclaude` 默认入口、`run` 和 `list`
进入同一份远程会话库存，用户先选择准确会话再 attach 或点击新建。入口按当前 CLI
声明的 `sessions.list`/`sessions.create`、home、Server 和 device 状态显示；缺少写能力时
仍可查看清单，但不会打开可提交的创建表单。工作台独立启动的 fclaude 原生终端继续承载
Claude 参数透传和首次远程授权，不把终端文本当成创建回执。
创建参数仅提交 schema 版本、准确上下文、账户/工作区/项目、参数摘要、replacement 和同步目标
证据；request UUID 由 CLI 在提交前生成并写入 journal。窗口重启后的恢复应由 CLI journal 提供原
request UUID，GUI 只能查询该 UUID；未知、冲突和已删除状态始终保留原回执并禁止猜测最新会话。

当 create POST 已发出但响应传输丢失时，CLI 从本地 journal 投影不含秘密的 `unknown`
回执和原 `request_id`；Go 仅将该 UUID 填入“查询原请求”，随后使用
`sessions.create.request` 的 GET 观察。journal 读取失败或身份不匹配时仍返回未知错误，
不会为了让按钮可用而生成新 key。

replacement 审阅现在分别要求原中断会话的 session UUID、account、workspace、project、node
和 runtime backend；这些字段与新会话的选择分开保存并原样提交，缺少任一字段都会在 Go 边界
拒绝。桌面能力评估也将 `sessions.create` 声明为需要已配置 Server/device 的写操作，只有
CLI 宣布 `exact_request` 确认能力时才显示可提交入口；不支持创建原键的 CLI 仍只能显示查询
或升级提示。
