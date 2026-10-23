# 准确远程终端连接与历史恢复

本切片只连接列表中已有、正在运行的 Claude 会话。它不创建、停止或删除远程会话，不代表这些 CLI 操作已完成 GUI 覆盖。原生终端、项目、分屏和关闭本地连接的行为继续复用现有实现。

## 已冻结的 CLI 合同

`terminal_operations` 必须声明 `sessions.attach` 协议版本 1。界面发起 `fclaude attach <完整非零规范 UUID> --expected-server <URL> --expected-device <UUID>`，通过 `AGENT_REMOTE_HOME` 传递冻结的 CLI 数据目录。没有该能力时说明需要支持此协议的 CLI，禁止降级为不带身份条件的 attach。

列表使用既有 `sessions.list` 返回的实际 Server/device 上下文，不新增 preview。Rust CLI 负责启动时复验配置、准确授权及原任务观察，Server/Node 负责真正 SSH 身份验证。Go 不构建服务端命令、不传入凭据，也不把界面条件视为服务端授权。desktop 只在 CLI capabilities 同时声明 `sessions.attach` v1 与 `attachment_transfer` v1 时打开文件/图片拖放和原生剪贴板 payload。mygo 将文件/图片作为 Kitty clipboard offer 送给 fclaude；fclaude exact runtime 仍通过独立 SSH 通道向 Server/Node 申请并校验 session、device、实际 SSH key 和 lease，未声明能力时 desktop 保持 text-only，绝不回退 legacy receiver。终端文字复制/粘贴、选择复制及登录链接复制保持可用。普通无 expected flags CLI 行为不变。

## 附件状态合同

桌面将 exact 连接的附件状态投影为本地 `AttachmentStatus` v1。缺少任一能力声明时为 `mode=text_only`、文字剪贴板可用、`files=false`、`images=false`；同时声明 `sessions.attach` v1 与 `attachment_transfer` v1 且限制合法时为 `mode=exact_transfer`，允许 mygo 文件/图片 payload。该状态是 fail-closed 的 UI 门禁，不是上传授权；fclaude 每次上传仍由 Server/Node 验证绑定身份与 lease，状态校验拒绝未来版本、未知模式和越界限制。

因此，exact 远程终端确认页会准确显示当前模式。text-only 会拒绝文件拖放；exact_transfer 会把拖放路径转成 `transfer.FileData` 的 Kitty offer，不把本地路径作为命令插入远端终端，也不启动 legacy receiver。普通本地 Agent 终端仍安全插入拖放路径。CLI、Server、Node 的专用合同、派生目标账号和清理回执见 [exact session attachment transfer](../../agent-remote-cli/docs/exact-session-attachment-transfer.md)。

## 选择、确认与复用

从成功完成的当前列表选择会话，冻结 home、Server、device、会话 UUID、本地 project ID 和 cwd。确认界面显示这些范围及当前账户、节点、后端；本地目录只是终端归属与 cwd，不宣称它就是远程 workspace。保留 tmux 接管其他客户端的提示。

刷新、查询失败及登录/退出登录期间不能使用旧确认。认证上下文失效时丢弃确认；确认执行前再次检查所选项目及列表上下文。CLI 子进程仍须复验外部进程可能改变的配置。

复用存活终端必须同时匹配 home、Server、device、会话和本地项目，不能仅按 UUID 跨服务器或跨项目复用。

## 历史与存储

历史只新增公开远程上下文（home、Server、device、会话 UUID）以及受限的运行时展示元数据（账户、节点、backend）。这些账户/节点/backend 字段只用于重启后的历史展示，不是授权依据，也不保存 argv、环境、授权 token 或输出。存储写入版本升级为 3，读取兼容版本 1/2，拒绝覆盖未来版本；旧应用的版本检查因此不能静默丢弃新增上下文后写回。后台持久快照须深复制上下文和展示元数据，并验证它们只出现在 fclaude 会话且与工具和 Reference 一致。

带上下文的历史恢复先重新查询当前配置和准确会话，再进入确认；上下文不同、目标不存在或不再运行时禁止直接连接。无上下文旧历史只作提示，进入刷新后的远程列表，由用户重新选择并确认准确目标，不能把旧 UUID 自动绑定到当前服务器。恢复成功仍替换原历史 pane，保留原项目、cwd、名称和分屏位置；取消、失败及查询不改变原历史。

## 验证

全部测试、夹具及原生探针位于根 `tests/`。验证规范 UUID/argv、旧能力拒绝、上下文漂移、历史版本升级与深复制、旧历史重新选择、准确目标缺失/停机、刷新失败/认证期间无旧确认、存活终端去重及 pane 替换。界面新增文案进入官方 en/zh-Hans catalog，验证 900×600 的英文、中文和 en-XA，身份详情放入滚动区，固定页头不增加按钮。原生 PTY 探针验证实际 argv 和本地连接生命周期；本机验证不替代 Windows/Linux 原生验收。
