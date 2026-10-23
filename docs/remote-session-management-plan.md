# 远程会话管理实施顺序

本文记录管理员运行时恢复之后的 P2 切片审计。准确连接、创建、停止、固定集合删除和端口转发原请求合同已经冻结并接入；生命周期停止回报已增加 Server 侧身份守卫。桌面端口转发现在由 Go 受管 Rust CLI 子进程持有真实 loopback listener，并提供 TTL、bounded reconnect 和准确停止；跨 Server/Node/CLI 联合门禁与三平台原生验收仍待完成。主程序与业务继续使用 Go/mygo，共享远程编排、凭据和原请求记录由 Rust CLI 负责；全部 desktop 测试、夹具及测试程序位于根 `tests/`，所有自有界面文案进入官方中英文 catalog。

## 现有能力与缺口

| 操作 | 可复用实现 | 必须补齐的边界 |
| --- | --- | --- |
| 创建 | Server `ToolSessionService.create_session`、Rust `session_creation` / `session_sync`、`sessions.create` 原请求查询、现有原生 PTY | 需要跨 Server/Node/CLI 联合恢复验收；丢失响应仍不能用项目最新会话替代原请求 |
| 连接 | 已实现 Server/device/完整 UUID 条件、原授权任务只读观察、Node 实际 SSH key 核验 | 一次 POST 后只 GET 原任务行，不重放未知受理；Node 使用 `tmux attach-session -d`，另一个客户端连接可能接管当前连接；精确连接暂禁旧图片/文件传输通道 |
| 停止 | `sessions.stop` caller key、固定 `stop_tool_session:{session_id}` 任务、Node legacy / managed stop、Server `SessionStopResultGuard` | 已有原键回执和 owner-scoped 查询；旧任务失败不能靠新键自动重试，跨 Node 联合终止证明仍待验收 |
| 保存观察 | `session_saving`、原 Skill finalization operation ID | stop-status 仅表示 managed Skill snapshot 保存与发布，不能声称项目文件或全部 Claude 历史已保存 |
| 删除 | Server 单项及批量删除、固定集合、Skill 保留检查、Rust/desktop 原请求回执 | 仍需跨端联合门禁和三平台原生验收；集合变化不能扩大删除范围 |
| 转发 | Rust `port_forward`、Server grant/connection、caller key 幂等创建、真实 loopback listener、TTL、bounded transport retry、Go 受管进程与精确停止 | Server/Node 实网联合门禁、三平台原生安装和跨重启回执仍需验收 |

普通用户不能使用管理员 `/nodes/tasks/{task_id}`。后续应提供 owner 范围的准确任务观察，返回原任务 ID、记录 UUID、受控状态和固定错误码，不泄漏 payload、原始任务结果或凭据。支持用户及设备的只读认证应单独设计，不能复用管理员恢复的 user-only 权限依赖。

## 先保证生命周期，再接操作界面

本轮 Server 已加入旧创建/停止原任务结果保护：原任务结果不可变，准确重放不重复写业务状态，冲突结果明确拒绝。创建结果只推进原来仍处于 starting 的同一会话，不能覆盖停止意图、已停止状态或替代会话。完整验证结果记录在[实施状态](implementation-status.md)，这一状态保护不等同于下面的运行时终止保证。

Desktop 的 exact session create/stop/delete 入口现在也与 Rust 的身份边界保持一致：Go 在调用 CLI 前拒绝非规范、非零 UUID 的 session、node、account、workspace 和 caller-key request ID（包括大写、花括号、短 ID 与全零 UUID）。这项校验只是本地 fail-closed 预检；设备上下文、Server 授权和实际运行时身份仍由 Rust、Server、Node 再次复核，不能把本地合同测试当作远程联调证据。

端口转发的 create/stop/reconnect 及受管进程启动事件也执行同一 UUID 规则：session、forward、node 以及 caller request ID 的可选身份字段必须是规范非零 UUID；列表或启动事件中的非规范运行时身份会被拒绝，不会把错误 forward 绑定到本地 listener。loopback 地址、端口和进程状态仍需独立验证，真实 Server/Node 隧道与权限验收不由这些 Go 检查替代。

`StartForward` 在启动 Rust 子进程前再次检查这些字段、client instance、runtime backend 和 TTL；错误输入不会先产生 listener 或等待启动事件。恢复模式可以附带原 session/节点用于启动回执匹配，但始终只把完整 `forward_id` 作为 CLI 恢复目标，不能退回 create。

仅增加 Server 状态比较不足以证明进程已停止。Node 在会话 spec 尚不存在时可能返回 stopped，随后迟到的创建仍可能启动运行时。应先审查并复用每会话生命周期锁、持久停止标记或有明确顺序保证的任务依赖，确保停止先于创建、创建在途和工作进程重启都不会晚启动。缺少该能力的 Node 不能被宣称已安全完成启动中停止。

最小可先验证范围是已稳定运行会话的准确停止；starting 阶段明确等待原创建结果。此阶段限制不能作为完整 CLI 覆盖验收，启动中停止保证仍需后续实现。legacy stop 失败也不证明进程已经终止，不能仅凭 failed 状态授权新删除操作。

界面分别展示进程终止、Skill 保存/发布、项目 Mutagen 同步。任一成功不能替代另外两项；取消等待不意味着远程任务撤销。

Desktop 远程列表已提供准确停止确认：确认快照包含完整会话 UUID、节点、运行时后端及配置上下文；Rust CLI 负责 caller key journal，Go 仅保存原 key 并调用 owner-scoped 查询。`accepted`、`stopping`、`pending`、`unknown`、`saved`、`failed` 和 `deleted` 保持独立显示，结果未知时不提交第二次停止。桌面在 CLI 返回 caller key 后还会持久化不含内容的恢复指针（key、home、Server、device、session、node、runtime backend 及可选 stop task ID），重启后须显式打开恢复入口再查询原 key，不自动查询或重放；回执、输出、argv、凭据和进程结果仍由 CLI journal 管理。传输中断未返回 key 时不生成新 key。该 UI 切片仍需 Server/Node/CLI 联合恢复及 Windows/Linux 原生验证。

会话创建现在同样持久化 CLI 返回的原 request UUID 及准确配置上下文。重启后桌面只显示显式的 query-only 恢复入口，要求当前 home、Server 和 device 与保存范围一致；恢复页隐藏创建字段并禁用新建按钮，只调用 `sessions.create.request` 查询 CLI journal。desktop 不复制 creation receipt、argv、同步计划或凭据，也不以 latest/default 会话替代原 key。该恢复切片已有 Go storage/UI 定向测试，仍需真实 Server/Node/SSH 联合恢复和 Windows/Linux 原生验证。

固定集合删除的 Desktop 审阅也已接入 `sessions.delete`、`sessions.delete.request/status`：用户先选择明确处于 `failed`、`interrupted` 或 `stopped` 的 Claude 会话，确认页冻结排序后的 session UUID、账户、工作区、项目、节点、运行时后端、选择摘要及与 CLI/Server 相同的摘要；列表刷新或上下文变化会禁用提交。Rust CLI 生成并持久化原请求键，Go 在未知结果时只查询该键。回执保留每个会话的上下文、删除进度、集合变化和 Skill/保留阻塞，停止完成不能被显示为删除完成。桌面收到原 key 后只持久化 home/Server/device、Claude、排序后的会话 UUID、selection digest 和 intent fingerprint；重启后需显式打开 query-only 恢复入口，查询回执必须再次匹配原上下文和完整集合摘要，绝不重发或扩大集合。该切片仍需跨端联合门禁和原生平台验收。

### 已实现但仍待联合验收的基础切片

以下合同已确定。Rust flush、创建/删除/转发 caller key、Server 防护和 Node 停止结果守卫已通过各自定向门禁；这些基础不能计入完整 GUI 覆盖：

- Node 使用绑定原节点和会话的持久停止意图，覆盖 Native、Docker Sandbox 及 managed spec/start/cancel/recovery。停止意图必须先于等待长运行操作落盘；同会话跨进程锁保证启动与停止顺序。历史成功回执保持不可变，不能再次授权启动或新建 broker 权限。
- 已安装 Helper 通过受认证本机 socket 独立提供 `session_lifecycle` v1，Worker 按健康后端报告，Server 严格规范化。该能力只表示可以阻止迟到启动，不代表某个进程已经终止。旧 Helper、缺失或损坏的能力均不能由版本号推断为支持。
- 无原 spec 或无法证明原运行时已停止时返回独立 pending，不能走普通 terminal failure 后被当作可删除。Server 同时保护单项、批量删除，检查原停止任务及其收据；旧失败任务不会自动重置或改写，后续仍需准确恢复合同。
- Rust 增加仅作用于已准备同步关系的内部 verified flush：核对共用上下文、目录身份、SQLite 关系、Server 完整目标与健康状态，按实际 Mutagen identifier 执行并复验。它不初始化或修复同步、不启动 daemon，也不提供可持久复用的会话启动授权。取消或超时须保留 flush 可能已执行的事实。

基础合同见[Server 生命周期能力](../../agent-remote-server/docs/session-lifecycle.md)、[Node 持久停止意图](../../agent-remote-node/docs/session-lifecycle.md)及 [Rust verified flush](../../agent-remote-cli/docs/verified-session-sync.md)。Server、Node 和 Rust 基础已分别通过各自门禁；跨端联合停止恢复、原创建键与 owner 范围观察协议仍待冻结。不把内部 flush 直接暴露为已完成的创建入口。flush 的执行未知状态仅描述同步命令，凭据续期可能已经生效，不能据此声称整个流程没有副作用。

## 原请求合同的建议方向

以下为草案，具体字段、身份作用域、表结构和版本协商须在实现前跨仓确认。

- 创建：绑定调用方原键及准确 tool/account/workspace/project/argv/replacement 意图。会话、创建任务、审计和原回执原子关联；同键同意图返回原会话，异意图冲突。已预约接管但尚未创建会话的结果需要独立、明确的身份，超时不允许自动再 POST。
- 停止：原键绑定准确会话、节点、后端和原停止任务记录；可收敛到已有停止任务，但不能隐式重新执行失败任务。Skill 保存操作另行关联并观察。
- 删除：原键绑定审阅时明确的会话集合；回执独立于被删会话保留。查询原键仍可确认删除结果，不能使用执行时的动态全部集合扩大范围。
- 转发：Rust `port_forwards.create` 在 POST 前保存 caller-key journal，并对准确 session/node 重新核验；desktop 只接收安全投影。一次性 token 不进入 desktop 持久记录；新建由受管 `forward --json` 进程持有 listener，跨重启恢复使用隐藏的 `forward --forward-id <full-uuid>` 精确查询、绑定并调用单次连接端点，绝不再次提交 create 或推断最新 forward。连接响应仅返回恢复 SSH 所需的非秘密 Node 路由字段，缺失时失败关闭；transport unknown 保留原 key，不能自动换 key。

CLI 继续作为本地公开原键/回执元数据的唯一持久来源，GUI 不另建远程执行状态机。原生终端只连接准确、已确认的会话 UUID；创建结果未知时不回退到 `fclaude run` 或猜测 latest。desktop 存储 v3 已增加 home/服务器/设备/会话上下文，保留 v1/v2 读取；没有上下文的旧历史只引导用户刷新、重新选择及确认。存活连接按 home、服务器、设备、会话及本地项目去重。本地项目仅决定终端归属及 cwd，不能等同于已验证匹配的远程 workspace。

## 验证顺序

本轮只读复核进一步确认：

- `fclaude run` 可先复用当前项目的会话；未显式指定账户时，这个会话不一定属于当前默认账户。`new` 与 interrupted replacement 的语义必须独立表达，Go 不能先强制套用默认账户再猜测 CLI 选择。
- `sync.status` 可提供规范本地目录、project key、workspace/sync ID 与目标指纹，但指纹不包含当前同步状态/冲突；这不是启动就绪证明。创建确认应由 Rust 形成统一意图，并在执行前重新验证同步条件。
- 现有 `create_with_takeover_wait` 在明确的“未创建会话”响应和原 takeover committed 后最多继续创建一次；这个流程没有通用调用方原键，不能直接当作跨重启 GUI 创建回执。
- Node Helper 的生命周期切片已补充跨进程持久 stop fence，并覆盖 Native、Docker、managed 启动/恢复/取消和 cleanup；Helper 仍缓存成功请求结果，因此历史回执和当前执行观察必须分开处理。缺少原 runtime 身份、部分批次丢失观察或历史 terminal 时保持 pending，不把 marker 或缓存重放当成物理终止证明。

创建的最小独立切片应先支持明确的新建，继续/替代行为随后通过 CLI 统一意图接入。向导依次展示选择、条件检查、确认和原请求结果；默认账户仅提示来源，选账户不顺便修改默认值。原键记录区分同步未完成、admission 待完成、会话已受理及原任务状态，CLI 独占持久化；Go 不保存可能包含提示词或敏感内容的原始 argv。用户拿到原请求返回的准确会话 UUID 后再独立确认连接。账户 affinity node 不是已选运行节点，实际节点/后端只能采用受理回执。

旧 `session_sync` 按 Mutagen 名称执行，不验证完整目标、暂停/冲突状态或 Git 锁；新的 Rust 内部 verified flush 已复用实际 identifier 选择并通过完整门禁，证据见[实施状态](implementation-status.md)，尚待创建合同接入。旧 fclaude 同步准备在 workspace ID 改变时可能清理旧 Mutagen 与映射，新创建向导要求用户先完成已有独立 ensure/recovery 流程，不能隐含这类清理。Server 目前按账户选择节点，新创建还必须验证所选节点与已确认同步节点相容；不相容时拒绝，不由客户端强制改写调度结果。

原创建键受理的进一步源码审查确认以下事务约束，尚未据此新增协议或数据表：

- 必须先取得用户内容锁，再重查原键并锁定、刷新账户/工作区/同步目标；不能先锁 Sync/Node 再调用会取得用户内容锁的创建流程。账户停用另有账户行锁，新受理也必须纳入该锁，不能只重复读状态。
- 创建实际使用 `Workspace.remote_path`，需与已审阅同步目标的远端目录及最终运行节点一致；工作区 owner/project/device 也需在同一事务复验。现有 `Workspace.local_start_path` 与 `SyncSession.local_path` 允许不同，不能未定义迁移语义就强制二者相等。
- Skill 准备可能已提交 takeover 或 migration，却尚未创建 Session。原键必须与这份“准备已提交、会话未创建”的结果原子保存；不能调用会自行 commit 的旧公开创建函数，再补写请求记录。
- 旧 `prepare_workspace` 可重用失败任务并替换 payload，而旧 `NodeTaskResult` 未按准备轮次版本化。它的结果处理与同步操作还有不同锁序。因此不能直接将当前任务 succeeded 与旧收据组合为准确准备证明，也不能在持有 Sync 后随意增加旧准备任务锁。新的受理合同需明确依赖当前同步观察还是补独立准备轮次；无论选择哪种，都不能替代客户端刚完成的 flush 或声称之后不会出现新冲突。

1. Server/Node 生命周期回归：重复及冲突结果、停止后迟到创建、启动前停止、旧工作进程晚回报、replacement 隔离与终止证明。
2. 创建原键、原子受理和 owner 范围精确观察；Rust CLI 与 desktop 共用实现；丢响应及跨重启恢复后独立确认 attach。
3. 准确停止、原任务观察及独立 Skill 保存进度，补启动中停止的完整保证。
4. 固定集合删除、Skill 保留约束、删除后原键查询和并发集合变化。
5. 转发原创建键、后台监听、TTL、准确实例停止及一次性 token 重连；桌面端监听编排已接入，继续补 Server/Node 实网与三平台联合证据。

每步增加对应 Go 界面确认、错误、未知结果和多语言布局验证；平台或权限不支持时说明原因。macOS 本机或隔离 Linux Node 证据不能替代 Windows/Linux 原生 desktop 验收。

相关代码入口：[Server 会话服务](../../agent-remote-server/src/agent_remote_server/services/sessions.py)、[Server 任务结果](../../agent-remote-server/src/agent_remote_server/services/nodes.py)、[Node Worker](../../agent-remote-node/internal/worker/worker.go)、[Node managed stop](../../agent-remote-node/internal/worker/managed_stop.go)、[fclaude 编排](../../agent-remote-cli/src/bin/fclaude.rs)。
