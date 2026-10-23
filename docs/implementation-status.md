# 本轮生命周期增量

2026-10-23 追加本机合同复验：`bash scripts/remote-interop-acceptance.sh --contracts`
和 `bash scripts/exact-attachment-acceptance.sh --contracts` 均通过（Desktop
remote/cliadapter、Node attach/port-forward、Rust desktop-control/Mutagen 及
exact-attachment 合同均通过）。这两次运行仍明确把真实 Server/Node/SSH、真实
fclaude attachment 和 broker 运行证据保留为待验收状态。

同日 CI 原生证据入口改为显式绑定 runner 与 `darwin`、`linux`、`windows` 目标，
Unix 和 PowerShell 两个入口都会传递该目标参数；新增 release 回归测试锁定这组
映射，防止矩阵或 runner 名称漂移后把错误主机记录成正式平台证据。

远程会话库存现在支持独立的服务端筛选：Agent 工具单选和状态多选会传递到
`sessions.list` 的 `tool_type`/`statuses`，默认筛选为空表示全部；文本搜索仍只
过滤已加载结果。筛选面板默认收起，避免压缩 900×600 终端首屏；Go 端限制 16
个状态、拒绝重复值及控制字符，并覆盖中英文文案和参数合同。发布脚本还会把
mygo 生成的 `Agent Remote Desktop.exe` 规范化为稳定的
`agent-remote-desktop.exe`，随后由离线 verifier 校验，Windows/Linux smoke
发布复验已通过。

真实同步链路 wrapper 已接入 desktop 互通入口：
`scripts/remote-interop-acceptance.sh --sync-chain` 调用 Node 的
`tests/linux_sync_chain_test.sh`，要求匹配的 CLI SSH/SCP sibling、审阅过的
Mutagen source、Server pinned virtualenv 与 macOS Docker Desktop；前置条件缺失时
fail-closed。2026-10-23 通过 wrapper 的一次性运行证据位于
`/tmp/agent-remote-desktop-sync-chain-final-20261023`，覆盖真实 Server、Linux
Node worker/root helper、SSH forced command、Bubblewrap、Mutagen 双向文件、暂停/恢复、
冲突保留/人工解决/reset、未知与撤销 key 拒绝。该历史证据的
`restart_recovery` 仍为 `not_run`；wrapper 现要求 Node fixture 输出精确的
`RESTART_RECOVERY_STATUS=passed` marker，缺失时即使普通同步链路返回零也会
fail-closed；通过时还会记录 scope
`post_restart_persistence_and_mutation`。该流程实际停止并重启隔离 Mutagen daemon
与 Node worker poll 生命周期，再验证原 session 清单和新的 pause/resume 传输，不
宣称任意 in-flight task 崩溃恢复。当前环境未重新运行 Docker/Mutagen，因此不把新
流程写成已通过证据；
该证据仍不覆盖 systemd、WireGuard、已有连接即时撤销或 Windows/Linux 原生 desktop。

Rust CLI 的 WireGuard broker 执行边界也已收紧：tunnel plan 只有在只读 broker 状态为 `execution=ready` 时才移除 broker 阻塞；Windows broker 在调用 WireGuard 前重新计算并比较完整 target scope，`up` 要求目标 absent，`down` 要求目标 present 且 service `config_identity` 与审阅的配置身份一致，缺失或漂移均拒绝执行。对应 target/tunnel 单测和 native broker 合同测试通过；Windows 真实编译、broker 安装授权、隧道存活与恢复仍需目标主机证据。

原生发布证据合并器的结构化回执校验已进一步 fail-closed：platform probe 的每个 requirement 只接受 `pass`、`fail` 或 `unknown`，且 `required` 必须是布尔值；托盘回执必须声明 schema v1、字符串目标及布尔 host-runtime 字段。新增 `tests/release/native_evidence_test.go` 回归覆盖无效 requirement 状态和不支持的托盘 schema。该收紧只提高证据可信度，不改变 Windows/Linux 无发行者签名与 macOS 社区自签策略。

端口转发恢复边界已进一步收紧：恢复只接受经过当前 Server/Node/backend 复核的完整 `forward_id` 和本地端口，启动前拒绝 session、remote port、TTL、caller request 等创建态字段，避免混合参数让 helper 在 CLI 拒绝前绑定监听或误提交新的 create。列表、启动事件和回执均采用严格 JSON、数量上限、标识/文本字符集与尾记录校验；ego-browser binding/request 标识遵守 Rust 的 ASCII 长度合同，候选 fingerprint 拒绝控制字符。相关回归位于 `tests/remote/port_forward_runner_test.go`、`tests/remote/port_forward_test.go` 和 `tests/remote/ego_browser_test.go`。

Desktop 状态持久化现在对偏好输入 fail-closed：只接受已实现的午夜、樱幕和晨曦主题，拒绝 NaN/Inf、超出字体与侧栏范围的数值以及带控制字符的选中项目；旧的零值偏好仍在读取时归一化为默认值。会话管理器还把显式移除与应用关闭串行化，确保同一 Agent PTY/ConPTY 只有一个生命周期 owner。回归位于 `tests/storage/layout_test.go` 与 `tests/session/manager_test.go`，并已通过 session race 测试。

同步关闭恢复边界已补强：显式退出在取消同步 worker 前先持久化有限的 `running` 投影；如果 CLI 回执尚未到达，重启后的桌面会把该记录提升为 `attention`，要求用户重新查询，不会丢失操作也不会暴露旧确认按钮自动重放。`tests/app/sync_test.go` 的阻塞 control helper 回归覆盖了关闭、重启和无重复 `sync.pause` 提交。

远程 fclaude 历史现在保留有界的公开账户、节点和 backend 展示元数据。该投影与精确 `RemoteContext` 分离，重启后只恢复显示，不作为重新连接或授权条件；保存时拒绝空值、控制字符、超长字段、非 fclaude 会话或缺少准确远程上下文。历史恢复仍必须刷新当前 Server/Node 列表并重新确认。对应验证位于 `tests/storage/remote_context_test.go`，不会保存凭据、argv、环境或终端输出。

明确退出应用时，Desktop 现在先停止并等待所有受管本地 Agent PTY/ConPTY，再执行最终工作区快照。这样 Codex、Claude 等进程收到正常停止信号后产生的最终退出码会写入历史；普通关闭窗口仍只隐藏窗口并保持会话和托盘运行。`tests/terminalhost/host_test.go` 在 Unix PTY 测试开关下覆盖优雅停止与退出码可见性，Windows ConPTY 仍需目标主机验收。

平台运行时门禁已按当前产品实际窗口模型收紧：工作台全部使用 mygo `WindowOptions.Content` 原生 UI，没有 `NewWebView` 或 URL 窗口。GTK 3、Linux AppIndicator 和 Unix PTY/Windows ConPTY 仍是必需运行时；WebKitGTK 4.1/4.0 与 Windows WebView2 由平台探针记录为 optional 观察，只有未来增加页面/WebView 窗口时才应提升为必需。native evidence verifier 要求这两项观察存在但拒绝把它们标成 required，避免无关的 WebView 缺失阻断原生桌面支持。Debian 包可能因 pinned mygo 生成器的保守依赖仍声明 WebKitGTK；这不改变当前应用的运行时需求。

远程投影边界已同步收口：`accounts.list`、`sessions.list` 和 `ssh.inspect` 只接受有界、无控制字符的公开字段，列表超限、重复标识、缺失必填值或畸形授权上下文均保留 `CLIProtocol`，不会降级为空列表或继续启动终端。Server/Node 会话状态 `running` 与 `active` 都可进入 Claude attach 确认，`starting` 仍等待稳定终端目标；该语义由根 `tests/remote`、`tests/app` 和 `docs/remote-interop-acceptance.md` 覆盖。

Exact fclaude 附件的 CLI 声明上限现在会在 desktop 边界实际执行：文件拖放在生成 native transfer offer 前检查有界的条目数、总字节数、绝对路径、常规文件和 symlink；精确远程终端的原生剪贴板图片/文件 payload 也会在 offer 前按同一限制 fail-closed，超限内容不会降级成可执行的文本路径。CLI 仍在上传时复验身份、租约和文件状态，桌面检查只负责提前拒绝不合规 payload；文本复制/粘贴保持可用。回归位于根 `tests/attachment`、`tests/remote` 和 `tests/terminalhost`。

通用 desktop-control 长操作现在支持可选 JSONL progress 流：`cliadapter.Client.CallWithProgress` 发送 `stream_events: true`，只接受带相同 request ID、版本 1、已知阶段和有界对象 payload 的 progress 事件，再把它们按顺序交给 UI；最终 `result`/`error` 仍是唯一完成证明，未知阶段、超限、尾随记录或回执不一致都会 fail closed。普通 `Call` 与 `login.device_code` 的既有单结果/审批流保持兼容。Rust CLI 的 `started` 事件和 desktop adapter 合同测试已覆盖；服务端真实阶段仍由各具体 CLI 方法决定，桌面不会伪造完成进度。同步操作页已通过 `remote.RunSyncActionWithProgress` 接入该流：工作台只保留最新的受控阶段标识并在操作中心显示本地化状态，原始 progress payload 不进入桌面状态或持久化记录，最终同步 receipt 仍是唯一完成证明。

# 实施状态

性能诊断现在保留最近 512 个 Go/mygo 视图构建样本，并从该有界窗口计算 nearest-rank P95。`FrameStats.P95` 只描述应用 View 构建阶段，目标仍是 16ms/60Hz；它不包含 GPU 绘制、原生窗口调度、PTY 输出或输入事件端到端延迟。`FrameObserver` 在 UI 线程使用固定原子环形缓冲，不分配内存；`tests/theme/motion_test.go` 覆盖 P95 和窗口淘汰语义。可运行 `go run ./tests/performance-probe -frames 120` 采集一个 10 个持久会话、4 个可见布局的 host-only JSON 读数；该探针明确将终端渲染和输入反馈标为未测量。达到“10 个活跃会话、4 个可见终端、输入反馈 P95<50ms、接近 60 FPS”仍需在真实三平台窗口中采集独立证据。

> 当前覆盖清单以 [`docs/cli-coverage.md`](cli-coverage.md) 生成的表格和本轮真实 CLI 探测为准：`agent-remote 0.2.58` 为 106 个命令节点/628 个参数绑定，`fclaude 0.2.58` 为 12 个命令节点/94 个参数绑定。本文后续按时间倒序保留多轮实现与验收快照；其中出现的旧节点数、参数数或测试计数仅表示当时的基线，不得作为当前 GUI 覆盖结论。

本轮新增 Skills 失败部署重试的结构化切片：`skills.retry.plan`/`skills.retry.control` 先重查原始操作，只投影已结束且可重试目标；Go/UI 冻结 generation 与 plan fingerprint，确认后由 CLI 使用 deterministic idempotency key 提交。控制回执把 desktop request `operation_id` 与 `source_operation_id` 分开，未知网络或回执状态保留原操作 ID并禁止自动重试。`skill state export` 的受管 Review 还在冻结 argv 时拒绝 NUL、缺失值和 `..` 父路径穿越，避免输出目录审阅被路径语义绕过；CLI 继续负责目标目录存在性、空目录、真实目录和可写父目录检查。Rust 合同、Go remote、fixture、Skills 页面和中英文 i18n 已覆盖；真实 Server/Node 部署联调仍待目标环境。

通用 schema 表单现在也在冻结 Review argv 时拒绝 NUL 参数值，和 dedicated terminal workflow 使用同一进程边界校验；这让非法值在 Review 阶段 fail closed，而不是延迟到受管进程启动时才报错。回归位于 `tests/cliadapter/generic_form_test.go`。

能力页的命令门禁现同时保留结构化控制操作与受管终端两层结果：当 `node install` 的 `nodes.list` 可用但 CLI 缺少 `agent-remote node install` terminal capability 时，卡片显示 `cli_unsupported` 并禁止误导性启动，而不会把目标节点清单权限误当成安装权限。该混合版本边界由根 `tests/` 的能力页回归覆盖；其它需要终端 workflow 的命令沿用同一 fail-closed 规则。

正式三平台发布门禁已显式固定为 `darwin/arm64,windows/amd64,linux/amd64`：`.github/workflows/release.yml` 将该集合同时传给 `build-release.sh --platforms` 与 `verify-release.sh --require-targets`，避免构建默认值或 runner 环境变化发布缺少平台的制品。`verify-release.sh` 的目标集合选项只用于正式工作流；本地 smoke 仍可验证任意受支持子集。相关合同测试位于根 `tests/release/`。

端口转发桌面入口现支持与 CLI 一致的本地端口自动分配：本地端口字段可填写数字、`auto` 或留空，Go 在启动受管转发进程前从 `127.0.0.1:0` 获取空闲端口，再向现有 control/runner 契约发送显式端口；control 协议仍拒绝零端口，避免把未绑定监听误报为成功。自动分配和显式端口解析已有 remote 单元回归测试。

本轮 P3 确认收口：能力页进入 `agent-remote doctor` 或 `deps status --fix` 的结构化诊断后，点击修复只打开准确命令与受管 home 的双语审阅；用户再次明确确认后才提交 `doctor.repair`/`deps.repair`。打开诊断和打开修复审阅均不会发出写操作，相关回归测试验证了确认前日志中没有修复调用。

端口转发关闭路径现在为每个本地 runner 保留创建时的 home/Server/device 上下文；即使用户在退出前切换了认证环境，明确退出仍使用原 owner-scoped context 清理远程 forward，并等待本地 runner 停止完成。

记录日期：2026-10-23。项目仍在开发，尚未达到首个正式版验收标准。正式 Release 采用社区版：macOS 社区自签，Windows/Linux 不要求发行者代码签名；Apple Developer ID 与公证不作为发布门槛。

本轮 P4 视觉收口：樱幕角色立绘现在只在宽工作台显示；窗口收窄到 1100 DIP 以下、当前项目进入分屏或用户启用专注模式时自动收起，释放侧栏空间给项目/会话导航和终端内容。该规则由 `theme.PortraitVisible` 统一测试；终端和背景动效策略不变。三平台原生高 DPI、分屏和真实窗口 resize 仍需在 Windows/Linux 主机复验。

本轮原生验收脚本收口：CI 的 native acceptance 不再跳过 workbench，上传的主机证据包含 platform、installer、native-window、tray 和 workbench 五个探针；元数据记录解析出的 Go/mygo 版本，证据合并器拒绝缺失/unknown mygo 版本及重复结构化 probe 回执。Linux pkg-config 依赖详情记录 ABI 版本。实际 macOS ad-hoc smoke 暴露 release-native-smoke 误读安装器 `--status` 顶层 target 的问题，Bash/PowerShell 已改为校验 `current.target/current.version`，darwin/arm64 临时安装、状态和 `--version` 已通过；这些结果仍不替代 Windows/Linux 原生运行。

CI 编排现增加独立的 `verify-native-evidence` job：它在三个 host runner 完成后分别下载 macOS、Windows、Ubuntu 的证据目录，并调用 `scripts/verify-native-evidence.sh` 汇总检查五个探针、主机目标、mygo 版本和 `cross_compilation=false`。Ubuntu CI 使用 Xvfb，因此该 job 明确传入 `--allow-virtual-display`；这只生成可审阅的虚拟显示记录，不能关闭正式版要求的真实 Linux 显示验收，也不会把交叉构建当作运行证据。任一 runner 缺少 artifact 或证据不完整时，汇总 job 失败。

原生证据合同进一步固定正式架构为 `darwin/arm64`、`windows/amd64`、`linux/amd64`；Windows/Linux ARM64 和 macOS Intel 仍可单独做 smoke，但不能通过发布证据合并器。`native-acceptance` 现在在元数据中写入 `display_protocol`（Linux 为 `x11`/`wayland`，Xvfb 明确归类为虚拟 X11；Windows/macOS 分别为 `win32`/`appkit`）。发布审阅可传入 `--require-linux-display-pair`，让 verifier 接受两个 Linux 目录并强制 X11、Wayland 各有一份记录；默认 CI 仍保持单 Linux Xvfb 审阅模式。相关合同覆盖位于 `tests/release/native_evidence_test.go`。

本轮验证补充：普通 `go test ./...`、发布 race 测试和 Rust desktop-control 合同均通过；在当前 macOS 上直接运行全量 `go test -race ./tests/...` 时，`tests/app` 的账户导入 headless 渲染和 `tests/cliadapter` 的运行时恢复子测试分别触发 10 分钟测试超时，堆栈停在 mygo 绘制或等待受管 CLI 子进程，没有检测到 race 报告。该结果不改变普通门禁结果，后续应在隔离测试构建目录或按包/场景分组复验。

本轮新增正式发布编排：`.github/workflows/release.yml` 只接受与 `mygo.json` 精确匹配的 `v*` tag，先运行完整 desktop 门禁，再在临时 macOS keychain 导入项目社区证书，构建并离线验证三平台社区制品，保留 workflow 证据后才创建 GitHub Release。工作流不启用 ad-hoc fallback、Windows/Linux 发行者签名或 Apple 公证；发布成功仍只证明源代码版本和制品清单符合社区策略，不替代 Windows/Linux 原生窗口、托盘、PTY、权限 broker 或真实 Agent/Server/Node 验收。

平台发行补充：`mygo.json` 的 Linux Debian 包现在声明 `libayatana-appindicator3-1 | libappindicator3-1`，与桌面托盘实际 ABI 对齐；`tests/release` 以合同测试锁定该依赖，发布文档明确包元数据不能替代目标主机的 AppIndicator 和托盘探针。Windows/Linux 仍只有交叉构建与可执行验收入口，不能据此宣称原生运行或安装路径已完成。

本轮收口补充：远程会话创建在提交和查询前绑定当前认证 generation、home、Server、device 与 capability 上下文；登录/注销、创建/停止/删除、远程接管和历史恢复互斥，应用关闭会取消并等待创建 worker。创建表单对 tool/account/workspace/project、sha256 argv digest 与 sync target 做 fail-closed 本地校验，默认空值不会调用 CLI。通用 CLI 表单隐藏并拒绝覆盖全局 `--home`，无项目的 `status`、`doctor`、`deps`、SSH 检查、Skills、Node 与 ego-browser 会使用绑定当前 CLI home 的本地化工具项目；fclaude 默认入口、`run`、`list` 已路由到远程会话库存，`new` 路由到结构化创建审核；工作台直接启动的 fclaude 终端仍要求选择项目。ego-browser 与本地 device workflow 在能力页明确 macOS 门禁。

本轮新增两个结构化只读/受控流程：`ego_browser.requests` 按精确 binding 查询最多 128 条无 payload 请求元数据，`ego_browser.request.cancel` 使用 user credential、exact-request 确认并冻结 binding/request generation 与 sequence；CLI 会在取消前刷新清单，desktop 只展示 payload 大小并对代次变化、未知回执和未知字段 fail closed。Skills 页面新增 `skills.state.list` v1，按账户目录或单 Skill 分页读取 checkpoint 与 pending-finalization 安全摘要，严格校验账户 UUID、范围、limit、两个分页游标、digest、状态和当前 home/Server/device 上下文。三者均有 Rust desktop-control 合同、Go remote/app 测试和中英文 i18n；真实 Server/Bridge API 联调仍待目标环境。

本轮补充 `skills.state.diff` v1 结构化只读流程：当前分支或精确 checkpoint 只能二选一，CLI/Go 双侧绑定 home、Server、device、账户范围、UUID、分页游标、digest、路径排序和条目类型；UI 仅展示变更路径、类型、大小及摘要，不接收正文、manifest、保存分支或凭据。`skill state export` 仍使用 dedicated reviewed terminal workflow，用户审阅准确输出路径和冻结 argv 后才由受管 CLI 执行；Skills add/update/enable/disable/pin/unpin/inherit/remove/rollback 同样从能力页进入 dedicated review，并按 CLI schema 对变更路径强制确认/`--yes`。

ego-browser 终端工作流已按真实 schema 校正：`register` 没有 `--yes`，由 desktop 精确 Review 提供确认；其它变更路径仅在 schema 声明该旗标时固定 `--yes`，避免生成 CLI 无法解析的参数。回归位于 `tests/cliadapter/dedicated_workflow_test.go`。

ego-browser 删除流程已补齐：`ego_browser.delete.plan` 先按精确 home/Server/device、设备或 binding 标识和 generation 读取只读计划；仅已撤销设备及已停止/过期/失败/撤销 binding 可进入删除审阅。用户明确确认后才调用 `ego_browser.delete.control`，Rust 与 Go 双侧拒绝未知字段、上下文/代次漂移和不完整回执；提交失败或回执未知均保留未知语义，不自动重试。删除后的本地 binding 索引清理只在远端成功回执后进行，回执另行报告本地清理是否完成，真实 Bridge/Server 权限联调仍待目标环境。

ego-browser 全信任认领切片已补齐：`ego_browser.candidates` 返回有界、无秘密的 Claude 会话候选；`ego_browser.claim.plan` 与 `ego_browser.claim.control` 冻结候选指纹、精确 tool session 及 home/Server/device 上下文。页面明确显示全信任警告，Rust 在确认前拒绝本地准入未知/关闭、Bridge 缺失、服务器执行准入关闭、候选绑定冲突或指纹漂移；确认后先关闭本地准入，再调用 Device Client，错误或未证明 open admission 时保留 `ego_browser_claim_outcome_unknown` 且不自动重试。Go remote、UI、夹具和中英文 i18n 已覆盖；真实 Bridge、Server/Node 权限和 macOS 目标环境仍待验收。

Skills 库详情切片已补充：`skill info` 通过 `skills.info` v1 结构化只读合同进入 Skills 专用页面。查询冻结精确 Skill 标识和互斥的 tool/account 范围，Rust/API 只投影类型、名称、代次、修订 ID/保留状态、生效规则和模型/发现状态；source locator、tracking、revision metadata、路径、内容与凭据不会跨 control boundary。Go 双侧校验当前 home/Server/device、能力、范围、上下文和有界修订列表；失败、未知字段或上下文漂移均 fail closed。Skills 页同时承载 `skill status/check/list` 只读入口以及 `skill state list/conflicts/info/diff` 的结构化状态/检查点查询；后者通过 `skills.state.list`、`skills.state.conflicts`、`skills.state.info`、`skills.state.diff` 仅投影有界元数据和变更路径摘要，检查点详情支持精确 UUID 查询。`skill retry` 已补充 `skills.retry.plan/control` v1 结构化流程，按用户凭据读取失败目标并冻结代次/计划指纹，用户精确确认后才提交固定 `--yes`；过期计划、未知回执和上下文漂移均保留 fail-closed 语义。`skill state export` 以及 add/update/enable/disable/pin/unpin/inherit/remove/rollback 仍由现有受管终端流程承载。

本轮继续补齐 ego-browser binding 生命周期：`ego_browser.binding.plan` 与 `ego_browser.binding.control` v1 支持 pause、resume、stop、revoke。页面先读取绑定的精确 generation 和当前状态，再显示允许的动作；用户确认后才提交 user credential 的 exact-request 控制。Rust 与 Go 双侧校验 home/Server/device、binding/device/session/node 标识、动作、代次、未知字段和回执一致性；控制结果未知时保留未知语义，不自动重试，也不传递浏览器 payload。真实 Bridge、Server/Node 权限和三平台原生运行仍待验收。

`deps status`（含 `--fix`）现统一进入结构化诊断页：打开和刷新只调用 `deps.inspect`，修复是用户在准确 home 与版本检查选项下单独确认后提交的 `deps.repair`，不会把修复参数塞进通用 argv 或隐式执行。对应 Go 页面/调用日志、Rust 合同和发布脚本合同测试已通过。

覆盖边界补充：`agent-remote status`、`agent-remote doctor`、`agent-remote deps status` 和 `agent-remote ssh check` 已接入 CLI 的 versioned desktop-control 合同与桌面结构化页面。页面严格校验 schema/version、operation、home、平台、未知字段和 SSH 授权上下文；`doctor --fix` 与依赖修复要求显式确认并绑定准确 home。`ego-browser status`、`list`、`requests`/`cancel-request`、`connect`/`claim`、binding 生命周期和 delete 生命周期也已由 Rust/Go 结构化合同与专用页面承载，分别展示本地 Bridge/存储存在性、候选/请求摘要及精确 generation-bound 计划；user credential、上下文和未知字段校验在 Rust/Go 两侧 fail closed。本地 admission 仍为 unknown，不能据此宣称 Bridge 已可执行。desktop 不解析终端文本，也不把 `diagnostics.inspect` 当作这些命令的替代；本地安装、注册和其它设备维护命令的终端回退仍不计入专用 GUI 完成数。

Node 安装切片现已从能力页进入专用节点清单：`nodes.list` v1 返回有界、无秘密的 Node 摘要并绑定 home、Server、device，上下文漂移、未知字段、重复 ID 和超限列表均 fail closed。用户必须显式选择 Node 并审阅准确 UUID、安装意图和 `agent-remote node install --node <uuid> --yes`（可选 `--enable-ego-browser`）；实际 SSH、join code、发行版校验和注册仍由 Rust CLI 受管 PTY 执行，desktop 不把列表或终端生命周期当作安装成功。真实 Server/Node/SSH 联调和三平台原生验收仍待完成。

当前仓库已有 mygo 工作台、项目与会话管理、分屏/标签、受管终端、desktop 专属存储，以及 CLI 命令发现和 schema 驱动的通用表单入口。所有已发现的叶子命令都能按参数 schema 生成默认值、必填项、choices、布尔旗标、重复值和 positional 表单，提交前可审阅并冻结 argv，再在受管终端执行；平台声明、已知能力、配置前提和已观察权限会分别显示。远程管理页可查询账户与会话，账户页可创建、查询状态、绑定、验证和停用，并提供本地配置导入与原请求跨重启恢复；独立管理员维护页提供运行时恢复审阅、明确确认及原请求查询与清理；默认账户页可查询、选择与清除共享默认值，并提供密码/MFA 与浏览器设备码登录、可选设备登记及注销，并提供环境诊断、本地初始化/元数据修复、WireGuard 配置预览/发布、独立网络检查/状态、只读权限桥/隧道计划和端口转发管理入口。项目同步页已接入建立、查询、暂停、恢复、冲突刷新、重建及旧映射恢复的确认流程；独立同步运行环境页可检查全部 sync/forward 会话，并另行确认共享 daemon 的环境恢复。无项目时，`agent-remote status`、诊断、依赖、Skills、Node、ego-browser 及本地 `device install/uninstall/launch` 等 home 级 utility 命令会自动建立受管 CLI 工具项目；Device workflow 保留 macOS 平台门禁并不要求用户先选项目。UI 回归覆盖能力页发现、Review、Start 及项目创建。专用业务工作流、三平台实际运行、发行升级和完整视觉动效验收仍待完成；代码存在不代表功能已全部验收。

专用受管终端 workflow 现在会校验 CLI 对 `effect`、`credential` 和 `confirmation` 的显式能力声明：省略字段的旧 CLI 继续兼容，声明与 desktop 预期不一致时按 `cli_unsupported` 禁止进入 Review/启动。这样不会把权限较弱或确认语义不同的命令误显示为可用。该门禁由 `tests/cliadapter/dedicated_workflow_test.go` 覆盖；它仍不替代真实 Server/Node 授权或三平台原生验收。

终端工作台接收 mygo 原生文件拖放：拖入路径会在当前聚焦的本地 Agent 终端中作为安全转义的输入插入，不会自动执行；路径中的空格、引号、Shell 操作符和 Unicode 均保持为数据。远程 exact fclaude 会先检查 CLI capabilities 的 `sessions.attach` v1 与 `attachment_transfer` v1；两者均声明且限制合法时，拖放路径转为 mygo Kitty clipboard offer，fclaude 再通过独立 exact SSH 通道由 Server/Node 校验 session/device/实际 SSH key/lease 后上传。缺少声明时仍保持 text-only，拒绝文件/图片且不回退 legacy receiver。三仓库真实 SSH、会话删除/设备撤销和丢失回执联调仍是发布前验收项。合同见 [exact session attachment transfer](../../agent-remote-cli/docs/exact-session-attachment-transfer.md)。

## 最新本机验证（2026-10-23）

- 重新执行 macOS arm64 原生验收：`DESKTOP_TERMINAL_TESTS=1 DESKTOP_NATIVE_EVIDENCE_DIR=/tmp/agent-remote-desktop-native-current-2026-10-23 bash scripts/native-acceptance.sh` 的 platform、installer、native-window、tray 和 workbench 探针均通过；WireGuard broker 因未提供真实 CLI 明确记录 `status=not_run`。同一主机的 `scripts/remote-interop-acceptance.sh --contracts` 与 `scripts/exact-attachment-acceptance.sh --contracts` 通过；二者仍不构成真实 Server/Node/SSH 或 broker 运行证据。

- 当前完整门禁已通过：`DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh`（官方 mygo catalog 当前 1169 条，中文复数 warning 已修复）、`go test ./tests/... -count=1 -timeout=600s`、`go vet ./...`、`go test -race ./tests/release ./tests/remote ./tests/session ./tests/storage ./tests/theme -count=1`、`bash -n scripts/*.sh`，以及 `darwin/arm64`、`windows/amd64`、`linux/amd64` 三目标交叉构建。交叉构建只证明编译，不替代目标主机验收。

- 2026-10-23 追加复验：`go test ./... -count=1 -timeout=600s`、`go vet ./...`、三目标 `CGO_ENABLED=0` 交叉构建、`bash -n scripts/*.sh`、发布/平台/终端定向测试，以及 `remote-interop-acceptance.sh --contracts`、`exact-attachment-acceptance.sh --contracts` 均通过。远程和附件入口仍明确报告 live Server/Node/SSH 证据待提供；ignored live 测试没有被当作通过。

- 当前 macOS arm64 原生验收目录 `/tmp/agent-remote-desktop-native-current-2026-10-23` 的 platform、installer、native-window、tray 和 workbench 探针通过；WireGuard broker 因未提供真实 CLI 明确记录 `status=not_run`。Windows/Linux 原生窗口、PTY、托盘、安装器、权限 broker、真实 Agent TUI 及 Server/Node/SSH 联调仍需目标环境证据。

- 最新复验将原生证据写入 `/tmp/agent-remote-desktop-native-current-2026-10-23-final2`；五个 macOS 探针均通过，WireGuard 仍明确为 `not_run`，未将其误报为 broker 运行证据。

- 追加 macOS arm64 原生复验写入 `/tmp/agent-remote-desktop-native-current-2026-10-23-final3`；platform、installer、native-window、tray、workbench 全部通过，`cross_compilation=false`，WireGuard 仍明确为 `not_run`。单目录 verifier 正确拒绝将缺失的 Windows/Linux 目录当作完整三平台证据。

- 追加 macOS arm64 原生复验写入临时目录 `/tmp/agent-remote-desktop-native-current-2026-10-23-final4.EnGwvV`；platform、installer、native-window、tray、workbench 全部通过，`cross_compilation=false`，WireGuard 仍明确为 `not_run`。该轮覆盖本次远程创建合同和已安装 `.app` 签名验证改动。

- 追加 Linux runtime smoke：Docker `linux/arm64` 容器内安装 GTK 3、AppIndicator、Xvfb 和 xauth 后，platform、installer、native-window、tray、workbench 五个探针均通过，证据目录为 `/tmp/agent-remote-desktop-native-linux-docker-20261023`。该记录明确为 Xvfb 虚拟显示和 arm64 容器，不能替代正式 `linux/amd64` 真实 X11/Wayland 主机证据；WebKitGTK 仍是当前 native-content 工作台的可选观察。

- 当前性能探针 `go run ./tests/performance-probe -frames 120` 在 macOS arm64 记录 10 个持久会话、4 个可见 pane、123 个样本、0 个慢帧，Go/mygo View 构建 P95 为 0.047ms（最大值 3.161208ms）；该读数不包含 GPU、原生窗口调度、PTY 渲染或输入反馈，三平台真实性能指标仍待主机采集。

- 版本发布策略已固定：macOS 正式制品使用项目社区自签身份；Windows/Linux 不要求发行者代码签名；Apple Developer ID、公证和 ad-hoc smoke 均不作为正式发布路径。正式三平台清单由 `scripts/build-release.sh` 与 `scripts/verify-release.sh --require-targets darwin/arm64,windows/amd64,linux/amd64` 共同门禁。
- 发布清单的应用名称与 bundle identifier 现在固定为 `Agent Remote Desktop` / `dev.agent-remote.desktop`；离线 verifier 与随包 installer 都会 fail closed 校验这两个字段，避免在没有 Bash 的 Windows 主机上仅替换元数据后安装其它应用。相关身份篡改回归位于 `tests/release` 和 `tests/installer`。

- 最新 Linux smoke 发布目录 `/tmp/agent-remote-desktop-release-linux-final-20261023e` 已由 `DESKTOP_ALLOW_ADHOC=1 bash scripts/build-release.sh --platforms linux/amd64 --clean --skip-dmg` 构建，并通过 `scripts/verify-release.sh --allow-adhoc --require-targets linux/amd64`；清单确认 Windows/Linux 无发行者代码签名、macOS 社区自签策略与关闭公证。该 smoke 只验证 Linux 制品完整性，不替代三平台正式 Release 或 Windows/Linux 原生运行证据。

- 本轮生成三目标候选 smoke 目录 `/tmp/agent-remote-desktop-release-candidate.gwwfXX`，包含 `darwin/arm64`、`windows/amd64`、`linux/amd64` 共 18 项制品；离线 verifier 和 macOS 安装后 `release-native-smoke` 均通过。该目录版本为 `0.1.0-dev`，macOS 明确标记为 `ad-hoc/smoke`，不能作为正式社区自签 Release。

- 正式 `.github/workflows/release.yml` 现把 macOS 安装后 smoke 接在离线 verifier 之后、归档之前；工作流回归测试和完整 desktop 门禁通过。该步骤仍只验证打包主机上的 macOS 安装路径，不替代 Windows/Linux 原生主机证据。

- 当前 macOS keychain 已发现项目社区自签身份 `Agent Remote Community Code Signing`（SHA-1 `53EE33F5BE5A109F8575A75D9E126826B5BB75CE`）。使用该身份生成的三目标候选 `/tmp/agent-remote-desktop-community-candidate.Dtxay3` 已通过 macOS bundle 签名、三目标离线 verifier 和安装后 smoke；版本仍为 `0.1.0-dev`，且工作树尚未形成正式版本 tag，因此仍属于候选制品。

## 2026-10-21 本机验证快照（历史记录）

- 本轮增量收口：CLI WireGuard broker 的 replay journal 现按平台持久化到受保护目录，启动时严格恢复，重复、损坏、越界或满 journal 均 fail-closed；desktop bridge status 仅允许精确 `up/down` 动作。Windows WebView2 探针按 stable→beta→dev→canary 和同通道最新版本选择运行时；ConPTY 在宿主 Job Object 不可嵌套时受控降级，安装器 Unix rename 后执行父目录 best-effort fsync。macOS 原生 acceptance（platform、installer、native-window、tray、workbench）本机通过；WireGuard broker 未提供真实 CLI 时明确记录 `status=not_run`。

- 本轮完整门禁：`DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh` 通过，官方 mygo catalog 为 1147 条（仅保留既有简体中文复数类别 warning）；Go 全量测试、WireGuard/Skills 定向测试、发布合同测试、脚本语法检查和 Rust `desktop_control_contract` 69 项均通过。`skills.retry.plan/control` 已进入结构化能力和双语页面，按用户凭据冻结失败目标、代次与计划指纹，精确确认后才提交；`skills.state.info` 新增为只读用户凭据结构化查询并纳入合同；ego-browser binding 生命周期的 plan/control 允许 revoke，删除流程严格绑定设备/binding generation。该结果仍不替代 Windows/Linux 原生窗口、GTK/WebKitGTK/ConPTY/托盘、安装器、WireGuard broker 和真实 Server/Node/SSH 联调；WebView2/WebKitGTK 观察在当前 native-content 工作台中仍为 optional。

- 本轮 export Review 收口：状态历史中的保留检查点新增“导出检查点”入口，按当前 CLI schema 预填 checkpoint/account/scope/Skill 标识，再要求用户在通用表单填写输出目录并审阅冻结 argv；`skill state export` 真实 schema fixture 已包含必填 `--output`。desktop 端冻结 argv 会拒绝缺失值、NUL、重复 `--output` 和 Unix/Windows `..` 父路径穿越，并以中英文 catalog 呈现错误。CLI 继续负责目标目录不存在或为空、真实目录、可写父目录和最终导出回执。Go Skills 定向测试、i18n 1149 条校验及 UI/adapter 路径安全回归通过。

- 本轮停止恢复收口：`sessions.stop` 返回 caller key 后，desktop 立即保存 home、Server、device、session、Node 和 runtime backend 的安全恢复指针；用户开始新的停止审阅但尚未拿到新 key 时，旧指针仍保留，应用重启后继续只查询原请求，不会重复提交停止。Go app/storage 定向测试和 race 复验通过。

- 本轮 CLI schema 边界收口：`cliadapter.Decode` 现在 fail closed 拒绝重复参数 ID、重复 positional index、非法 min/max 范围和含空白/NUL 的参数标识；实际 `agent-remote 0.2.58`（106 个命令）与 `fclaude 0.2.58`（12 个命令）schema 重新发现通过，schema 合同测试和 `go vet` 通过。

- 最新结构化边界复验：ego-browser claim 已登记 `candidates`、`claim.plan` 和 `claim.control`，计划按精确会话与候选指纹刷新，control 要求全信任确认、关闭本地 admission 后才调用 Device Client；失败在该边界后统一保留 unknown，桌面不接收浏览器 payload。Rust desktop-control 合同 67 项通过。远程端口转发启动事件现在严格匹配 forward/session/node、端口、client instance 和 caller request，resume 只使用原 forward ID；Go 定向 race 通过。Windows WebView2 探针已按 mygo 的 EdgeUpdate ClientState、固定目录和架构 DLL 规则检查，Linux WebKitGTK 接受 mygo 的 4.1/4.0 规则。Windows/Linux 仍没有真实主机运行证据。

- Node 管理切片已接入 `nodes.list` v1 结构化只读合同：Rust 只返回有界、无秘密的节点清单和 home/Server/device 上下文，Go 严格拒绝未知字段、重复或越界节点及上下文漂移。能力页的 `node install` 进入双语 Node 页面，用户筛选并选择准确节点后审阅冻结的 `node install --node <uuid> --yes`（可选 ego-browser intent），再由受管 Agent Remote 原生终端执行实际 SSH、release 校验、join code 和安装阶段；桌面不接触凭据或 join code，操作中心沿用 pending/running/completed/failed/attention 摘要，重启不自动重放。该切片的 Go remote/app/cliadapter 测试和 Rust desktop-control 合同通过；真实 Server/Node/SSH 安装及三平台原生验收仍待目标环境。

- 本轮收口复验：`go test ./... -count=1`、`DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh`、`go test ./tests/release -race -count=1` 和 `bash -n scripts/*.sh` 均通过；官方 i18n catalog 为 1045 条。账户导入恢复测试改为先过滤命令面板，远程历史测试使用稳定的 Remote 标签及远程专用审阅文案，覆盖英文、简体中文和 en-XA，避免新增入口或伪本地化造成误点。该结果仍不替代 Windows/Linux 真实 GTK/AppIndicator/ConPTY/托盘/安装器、WireGuard broker、Server/Node/SSH 与 Agent TUI 验收；WebKitGTK/WebView2 仅为当前 native-content 工作台的 optional 观察。

- 本轮远程创建与 CLI 覆盖收口：`go test ./... -count=1`、`DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh`、`go test ./tests/release -race -count=1`、`bash -n scripts/*.sh`、Windows/Linux amd64 与 macOS arm64 `CGO_ENABLED=0` 交叉构建及 `scripts/exact-attachment-acceptance.sh --contracts` 均通过；远程验收入口现在实际执行 Rust `desktop_control_contract`、Mutagen environment 和 exact attachment fixture 测试，不再用 `--no-run` 仅编译来冒充合同通过。通用表单的 desktop-managed `--home` 防逃逸、无项目 utility project、ego-browser macOS 平台门禁和原生验收证据目录清理均有适配器、存储和发布回归覆盖。该结果仍不替代 Windows/Linux 真实 GTK/ConPTY/托盘/安装器、WireGuard broker、Server/Node/SSH 与 Agent TUI 验收；当前 native-content 工作台的 WebView2/WebKitGTK 只保留 optional 观察。
- 远程 live 证据在 Windows 上不会把 Unix-only Mutagen 合同的零测试 Cargo 成功状态记作通过；提供 Mutagen 二进制时仍写入 `mutagen=not_run`，并保留 Server/Node/SSH 结果的独立状态。
- 当前 macOS 原生复验：`DESKTOP_TERMINAL_TESTS=1 DESKTOP_NATIVE_EVIDENCE_DIR=/tmp/agent-remote-desktop-native-current-2026-10-21 bash scripts/native-acceptance.sh` 通过 platform、installer、native-window（语言切换、隐藏/显示、PTY 退出码）、tray（真实 mygo 托盘创建/菜单/销毁）、workbench（多会话、焦点、拖动、退出、准确远程上下文/复用/历史替换和账户绑定）探针；WireGuard 明确记录 `status=not_run`，因为没有提供真实 broker CLI。该证据只覆盖 macOS 主机。
- 原生平台门禁增量：Linux `platform-probe` 将 GTK 3、AppIndicator ABI 和 native display 作为必需运行时依赖，WebKitGTK 4.1/4.0 仅作 optional 观察；缺少必需运行库或无法只读验证时立即失败。Windows WebView2 同样只作 optional 观察，因为当前工作台使用 native-content 窗口；未来增加页面/WebView 窗口时才提升为必需检查。`tests/tray-probe` 在三平台实际创建/更新/销毁 mygo 托盘图标与菜单，`verify-native-evidence.sh` 强制要求 host-only `tray.log`；原生 `installer-probe` 还覆盖 install→upgrade→rollback 后运行锁占用拒绝与释放重获。目标主机不可用、虚拟显示或交叉编译都不能生成正式通过证据。ConPTY 由必需 PTY 分配探针和 native-window 验收覆盖；Windows/Linux 原生窗口、安装器和权限服务仍待目标主机执行。
- 新增三平台 WireGuard broker host-only 验收入口：CLI 提供 Linux/macOS shell 与 Windows PowerShell 安装/身份/`wireguard.bridge.status` 证据脚本，桌面可选接入并由 `verify-native-evidence.sh --require-wireguard` 强制检查。默认结果为 status-only，未安装、未完成 caller identity handshake、交叉编译、mock/fixture 或虚拟显示均 fail-closed；真实 up/down、重放拒绝和服务重启恢复仍待目标主机执行。
- 本轮增量复验：官方 i18n catalog 为 1014 条；结构化 `status.inspect`、`doctor.inspect/repair`、`deps.inspect/repair` 和 `ssh.inspect` 的 CLI 合同、Go 严格 adapter、能力页路由及双语页面测试通过。`DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh`、`go test ./... -count=1`、`go test ./tests/release -race -count=1`、Windows/Linux amd64 与 macOS arm64 `CGO_ENABLED=0` 交叉构建、shell 语法检查及 `scripts/exact-attachment-acceptance.sh --contracts` 均通过。macOS `scripts/native-acceptance.sh` 通过 platform、installer、原生窗口/PTY 与 workbench 探针；该证据仍只覆盖 macOS 主机。
- ego-browser P3 状态切片复验：`ego_browser.status` v1 已由 Rust desktop-control 返回本地 Bridge/存储存在性、公开策略、设备和绑定安全摘要，Go 适配器严格拒绝未知字段、上下文漂移、非法标识和超限列表；能力页命令 `ego-browser status/list/requests/cancel-request/connect/claim/pause/resume/stop/revoke/delete-device/delete-binding` 路由至同一双语专用页面，并按结构化 contract 执行候选认领、请求取消、binding 生命周期和删除审阅。Rust `desktop_control_contract` 及 Go remote/app/cliadapter 定向测试通过。本地 admission 仍固定为 `unknown`，该合同不替代真实 Bridge、Server/Node 权限或三平台原生验收；ego-browser 本地安装、注册和其它设备维护、Device 安装生命周期及 Node 安装继续使用受管终端 workflow。
- Device 本地状态与生命周期计划切片：`device.local.status` v1 已由 Rust 复用既有 macOS Device bundle 检查逻辑，返回安装、社区签名、Gatekeeper、两个 XPC helper 与进程状态的安全摘要；新增 `device.local.plan` v1 只读合同，严格绑定 home、action 和 install source，投影有界阶段、准确 terminal method、确认/破坏性标记及 `inspect_before_retry` 恢复策略。Go Device 页与远端设备卡片并列展示状态和诊断计划，计划不会执行变更或读取凭据。Windows/Linux 由 CLI 返回 `platform_unsupported`，Device 的安装生命周期仍保持受管终端 workflow；真实 macOS bundle、签名和 XPC 验收仍需目标主机证据。
- 本轮收口复验：`go test ./... -count=1`、`go test ./tests/release -race -count=1`、`bash -n scripts/*.sh`、三平台交叉构建及 `scripts/exact-attachment-acceptance.sh --contracts` 均通过。能力页命令目录补齐未映射 `agent-remote status` 的 `unknown` 状态；`deps status`（含 `--fix`）统一进入命令诊断结构化页面，打开时只执行 `deps.inspect`，修复必须由用户在准确 home 上单独确认后提交 `deps.repair`。该结果不替代 Windows/Linux 原生窗口、真实 broker、Server/Node/SSH 或 Agent TUI 验收。
- 原生发布脚本审计增量：新增 Windows 无 Git Bash 可运行的 `scripts/release-native-smoke.ps1`，按当前主机架构从 manifest 选择制品，在临时用户目录执行安装、状态读取和 `--version`；PowerShell 入口同样强制 Windows/Linux 无发行者签名、macOS 社区自签/显式 ad-hoc 规则。`verify-native-evidence.sh` 现在要求平台必需检查、结构化 installer 回执、零 PTY 退出码和工作台成功回执。该改动已通过 release 定向测试、race、shellcheck 与完整 desktop 门禁；Windows/Linux 真实主机、安装路径和 broker 服务仍待实机运行。
- P4 发布证据兼容性增量：原生证据合并器现在同时接受 Windows PowerShell 5 的 UTF-8 BOM 与 PowerShell 7 的无 BOM 输出，覆盖摘要、主机元数据及 WireGuard JSON；验证器还拒绝 release/evidence 脚本的未知或额外参数，避免把误传输入静默当成通过。相关回归测试位于根 `tests/release/`，不改变 Windows/Linux 无签名与 macOS 社区自签策略。
- 能力页命令路由增量复验：登录/注销、账户、凭据、同步、Mutagen、WireGuard、设备、Skills、`agent-remote attach` 与 `fclaude attach` 的入口测试通过；`go test -race ./tests/app -run 'TestCapabilitiesRoutes' -count=1` 通过 94.5s。该测试证明页面路由和异步远程清单 hand-off，不替代真实 Server/Node 权限或 Windows/Linux 原生验收。
- Skills 冲突页已提供解析、迁移和恢复的专用审阅入口。CLI capability 为这些 workflow 声明 effect、credential、confirmation 和稳定标签；desktop 只预填冲突 ID 或 Skill/account，分支、路径和 checkpoint 仍由用户补齐并由 CLI 校验，不自动解析、恢复或重试。Rust `desktop_control_contract` 56 项、Go adapter/remote/i18n/app 定向测试通过。
- 远程 exact session 附件现在按 CLI capabilities 门禁：只有 `sessions.attach` v1 与 `attachment_transfer` v1 同时声明且限制合法时，desktop 才把文件拖放/图片剪贴板作为 mygo Kitty payload offer 交给 fclaude；fclaude 的独立 SSH 通道仍绑定 session、device、实际 SSH key 和租约，并由 Server/Node 拒绝路径穿越、未知字段、超限、摘要不匹配及 cleanup unknown。未声明能力时 UI 保持 text-only，不回退 legacy receiver。三仓库真实 SSH、会话删除/设备撤销和丢失回执联调仍待完成。

Exact 附件发布前联调入口为 `scripts/exact-attachment-acceptance.sh`：`--contracts` 串联 Server、Node、CLI 和 Desktop 的隔离合同；`--live` 在明确提供隔离 home、session UUID 和 device UUID 后运行 CLI ignored 测试，通过真实 Server/Node/SSH forced-command gateway 完成一份临时文件上传及 scoped cleanup receipt。`--live` 缺少这些条件会 fail-closed，不会把跳过当作通过。当前仅有合同入口复验，尚无真实三仓库 SSH 结果，因此 desktop 仍不把能力声明视为真实服务验收证据。
- Rust CLI 最新完整门禁：1363 项测试通过、3 项条件跳过，doc tests 通过；其中包含附件 frame 合同、Skills 冲突合同和三平台 WireGuard broker 合同。该轮运行耗时约 241 秒，唯一慢测为 verified sync 合同，最终仍通过。
- 此前最终复验：`DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh` 通过（982 条 catalog，全部 Go 测试）；Windows/Linux amd64 与 macOS arm64 交叉构建通过；`DESKTOP_TERMINAL_TESTS=1 bash scripts/native-acceptance.sh` 在 macOS arm64 通过平台、安装器、原生窗口/PTY 与 workbench 探针。对应 Rust `scripts/run-quality-checks.sh` 通过 1342 项测试、3 项条件跳过和 doc tests。Rust 现在包含 macOS LaunchDaemon、Linux system D-Bus/Polkit 和 Windows SCM/named-pipe protected broker service，以及 caller identity、one-shot permit、配置字节/摘要绑定合同；这些边界在未安装真实 broker service 时均 fail-closed，三平台原生安装、授权、执行和运行验收仍待完成。
- 此前复验：`DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh` 通过：982 条中英文 catalog 消息、生成代码、gofmt、vet、全部 Go 测试和根 `tests/` 目录规则通过。mygo 仍报告既有的中文复数类别 warning，不影响门禁。侧栏在 900×600 最小窗口下采用紧凑动态高度，主导航、项目切换、远程历史会话和操作状态入口均保持真实可点击；远程历史、恢复布局和同步项目切换回归已通过。
- 统一操作中心和 Skills 专用工作流的定向测试已通过；Skills 变更使用冻结的 catalog generation、精确上下文和 CLI 公共回执。新增只读 `skills.state.conflicts` 使用 Rust 用户会话读取发布/迁移冲突摘要，Go 严格校验精确账户 UUID、范围、分页和安全字段，不跨边界传递路径、digest、保存输入或解析选择。针对操作中心、Skills 及同步页面的 race 定向测试通过（desktop app 156.584s、remote 6.543s、cliadapter 9.709s）。
- `node install`、macOS `device install/uninstall/launch/diagnose`、Skills 库变更命令与 `ego-browser` 已接入专用 terminal workflow：Rust `terminal_operations` 声明精确命令路径，Go 在 schema 表单入口执行平台/能力 gate；变更操作冻结 argv 并固定 `--yes`（仅在 schema 提供该旗标时注入），`device uninstall` 使用 CLI 的 `--yes`，`device install/launch` 使用精确 Review 作为确认边界；`skill add/update/enable/disable/pin/unpin/inherit/remove/rollback` 均使用用户凭据和精确确认；只读 `device diagnose`、`skill check/status/list` 及 `skill state export` 冻结查询参数但不要求确认，Review 后才启动受管 PTY。`skill info` 已改由 `skills.info` 结构化只读详情面板承载，查询只投影有界安全元数据；`skills.state.list`、`skills.state.conflicts`、`skills.state.info` 和 `skills.state.diff` 已改由结构化 Skills 状态页面承载，查询 checkpoint/pending-finalization、冲突、精确检查点详情和变更路径摘要，并使用独立分页游标。`device status` 已改由 `device.local.status` 结构化投影承载。`skill retry` 使用用户凭据、精确确认和固定 `--yes`。操作中心只投影本次生命周期；desktop 现已持久化固定命令标签、状态和时间的安全摘要，重启将 pending/running 转为 attention，且不保存 argv、会话 ID、终端输出或凭据；精确 CLI 原始回执仍由 CLI durable journal 提供，desktop 不伪造回执、不自动重放。`tests/cliadapter/dedicated_workflow_test.go` 覆盖 Skills/Device 只读查询、retry、state 维护、平台、能力、确认、argv 错配和缺少 `--yes`。该切片仍不等同于真实设备/服务器授权或三平台原生验收。
- `fclaude` 默认入口、`run` 和 `list` 的能力页入口现在复用远程会话库存；`new` 直接打开 `sessions.create` 结构化审核弹窗。创建/列表/attach 均绑定当前 home、Server、device，未知或未满足 capability 时按状态禁用；exact attach 仍强制 `--expected-server` 与 `--expected-device` 成对带值，拒绝无身份的 legacy attach。工作台仍保留受管 fclaude 原生终端以承载 Claude 参数透传和首次授权交互，终端输出不投影为桌面回执。对应 Go 路由、CLI caller-key 合同和 app 覆盖测试已通过。
- CLI 能力页现在也暴露带可选子命令但自身可执行的 `agent-remote forward`/`fclaude forward` 父节点，并将其路由到端口转发创建页；fclaude 无子命令的默认调用作为远程会话库存入口，保留工作台中的受管原生终端后备路径。这样命令目录中的可执行父节点和根默认语义不会因 `HasSubcommands` 被隐藏；定向 app/cliadapter 测试已通过。
- `fclaude stop/stop-status/delete/delete-status` 与 `forward` 的能力页入口现路由到远程会话或端口转发专用页面，显示上下文、回执和恢复语义；不会把生命周期命令伪装成通用 argv 表单。缺少对应 capability 时显示双语的不可用原因。该路由已由 adapter 映射回 `sessions.*`/`port_forwards.*`，仍需真实 Server/Node 和三平台验收。
- 当前 `fclaude 0.2.58` 的 12 个 schema 节点已同步到根测试 fixture；`stop`、`stop-status`、`delete`、`delete-status`、`forward list`、`forward stop` 六个叶子均有能力页页面路由回归，确保后续 schema 更新不会只保留命令发现而丢失 GUI 入口。
- `cargo fmt --all -- --check`、`cargo check --workspace`、Clippy 和 Rust 全量质量脚本已通过；最终 nextest 为 1317 项通过、3 项按条件跳过，doc tests 通过。此前 WireGuard 隧道合同曾因执行桥阻断项断言未同步而失败，现已把 `execution_bridge_required` 纳入最终合同；Windows 原生编译仍受当前 macOS 缺少 MSVC C 头文件限制，不能替代目标主机验收。
- 端口转发 runner/parser 定向测试及 race 通过。`forward --json` 仅在真实 loopback listener 绑定后发出无秘密启动事件，Go 受管进程负责上下文绑定、优雅停止和退出清理；跨重启使用完整 forward ID 精确恢复，不重复 create。显式退出先取消并等待启动/动作 worker，再收集并停止所有 runner，同时等待此前由停止回执触发的异步 child shutdown；启动事件与取消同时到达时会再次检查 context，避免关闭后漏登记 runner。
- `tests/platform-probe` 新增只读 host-runtime 能力报告，明确记录 `goos/goarch`、显示会话、shell、原生终端（Unix PTY/Windows ConPTY）及平台运行时依赖的 `pass/fail/unknown`；Linux GTK 3、AppIndicator 和 native display 为必需检查，WebKitGTK 4.1/4.0 只作 optional 观察。终端检查会分配 100×30 的当前平台 PTY、读取正尺寸并关闭，不启动命令或读取用户状态。报告固定 `host_runtime=true`、`cross_compilation=false`；Windows 现按 mygo 相同契约只读观察 HKCU/HKLM 两个注册表视图的 EdgeUpdate ClientState\<channel>\EBWebView 目录，并确认 msedgewebview2.exe 与架构对应的 EmbeddedBrowserWebView.dll；缺失、陈旧或访问未知状态只记录 optional 结果，不阻断当前 native-content 工作台。必需目标/shell/terminal/display/GUI 依赖检查失败时退出非零。统一入口 `scripts/native-acceptance.sh` 先运行该探针，再运行 installer、native-window、tray 和 workbench，Linux 的 Xvfb 也会通过同一报告被明确记录。该探针和 macOS 本机运行通过；Windows/Linux 仍需目标主机运行，交叉构建不会填充原生证据。
- `scripts/build-release.sh` 已验证 Linux smoke 和 macOS 正式社区自签构建；`SHA256SUMS` 全部通过，macOS `codesign --verify --deep --strict` 通过，清单记录 Windows/Linux 无发行者签名、未公证/上传及社区证书信息。正式 macOS 签名会改变嵌入 dylib 的字节，清单同时记录上游与交付哈希及变换原因。
- 新增离线 `scripts/verify-release.sh` 作为交付目录门禁：它复核 manifest/SHA256SUMS 的完整集合、文件大小和字节哈希，拒绝未列出的目标目录/文件与符号链接，并强制 macOS 社区自签、Windows/Linux 无发行者签名和关闭公证。构建脚本在生成清单后自动调用；macOS 原生主机可进一步执行 `codesign`，其他主机明确记录签名待原生复核。安装助手还拒绝把 destination 放入 release 源树，避免 staging 递归复制。新增 `tests/release` 和 installer 回归覆盖 unsigned、ad-hoc 显式 opt-in、策略/制品篡改、FreeBSD 目标目录及源树内安装；定向测试通过。该发行切片的桌面完整门禁已在最新 catalog 与依赖页修正后重新通过。
- 安装助手新增只读 `--status`：升级前可查看受管当前版本和上一版本是否存在；空目标返回 `installed=false`，未受管目录、残留 previous 树及状态不一致均 fail closed。同步与 Mutagen 环境操作在重启后恢复最多 8 个阶段名/状态的本地化安全摘要，不保存请求键、路径、argv、计划或完整回执；新一次无回执的失败不会继承旧阶段。相关安装器、storage 和 app 回归均位于根 `tests/`。
- 历史增量复验：catalog 曾增至 964 条；远程停止恢复指针、exact 远程 text-only 附件边界及 UI 状态展示已通过定向 race/合同测试。能力页现在同时显示每个叶子命令的平台声明和当前权限/前置条件状态；工作台品牌、Agent 列表、端口转发占位符和平台名称均走 catalog。新增的工作台背景装饰只在普通模式运行一次性 180ms 进入动效，聚焦模式禁用、减少动效时静态低透明度，终端面板不参与装饰层；策略与 UI 回归测试通过。`DESKTOP_TERMINAL_TESTS=1 bash scripts/native-acceptance.sh` 在 macOS `darwin/arm64` 上通过平台、安装器、隐藏/显示窗口、PTY 和 workbench 探针；这仍不替代 Windows/Linux 原生运行证据。CLI WireGuard machine helper 的 regular-file、链接、硬链接和权限边界专项通过，未改变 broker 未安装时的不可执行状态。
- 能力页的权限清单已补齐会话创建查询、停止/删除及其原请求状态、端口转发四种操作和设备状态/撤销/令牌轮换；这些入口现在按准确的 home/Server/device 依赖显示状态，设备变更还要求用户授权观察。能力清单只描述当前 CLI 合同，不把列表读取或命令发现当作写操作已获准。

## 新增要求同步情况

| 要求 | 文档状态 | 当前代码与后续工作 |
| --- | --- | --- |
| desktop 全项目使用官方 i18n | 已同步产品方案、双语 README 和开发规范；已核实固定 mygo 版本支持 | 已配置官方 catalog，当前提供中英文消息及生成的 Go 函数（数量以官方 i18n 校验输出为准）；工作台、菜单、托盘、设置、远程列表、状态、校验与错误呈现已接入。后续页面沿用同一规则 |
| 测试统一放在仓库根 `tests/` | 已明确所有测试、辅助代码、夹具及专项程序的布局和验收规则 | 已迁移全部既有 `_test.go` 及原生验证程序，改为通过包导入验证；脚本和 CI 已更新，目录门禁已启用 |

语言设置支持中英文及跟随系统，偏好交给 mygo 管理；已有错误在重绘时按当前语言解析。业务层使用稳定错误码，外部未知错误显示本地化摘要及原始诊断。语言切换不重启会话。

同步修复：慢启动不再持有会话管理器锁阻塞 UI 快照，退出时等待并收回正在创建的进程；不兼容的存储版本明确拒绝写入；后台保存失败显示在工作台并由关闭操作返回。工具路径检测改为缓存及手动刷新；高级参数只接受 JSON 字符串数组。

生命周期契约已收敛为可测试的 Go 策略：普通窗口关闭只隐藏并保留受管会话与托盘入口；首次应用退出请求被拦截并进入明确确认，确认后才允许原生 quit 回调完成；确认退出固定停止受管会话和本地端口转发。`tests/app/lifecycle_test.go` 覆盖隐藏、退出提示、确认放行及清理计划，真实窗口 hide/show 与 PTY 存活仍由 `tests/native-probe` 验证。托盘菜单按语言变化重建，应用关闭时销毁托盘；托盘运行库缺失会在工作台显示本地化错误并进入原生证据失败。

关闭竞态补充：异步本地会话启动/停止回调在应用进入 closing 状态后丢弃，不再重开停止对话框或向已关闭的保存队列写入；远程创建/停止/删除 worker 在取消前保留短暂收尾窗口，并在关闭前消费已缓冲的 CLI 回执，确保 caller key 不因 UI 尚未渲染下一帧而丢失；这覆盖窗口关闭后后台运行与明确退出之间的尾部 worker 生命周期。

## 工作台布局与会话历史

- 数据版本 3 保存命名会话元数据、标签顺序、分屏方向/比例、每个标签的焦点、侧栏宽度与详情栏偏好，以及远程 home/服务器/设备/会话上下文；兼容读取版本 1/2，保存时原子升级。保存前深拷贝布局和远程上下文，拖动期间合并写入；关闭窗口或退出时刷新待保存状态。
- 重启恢复未连接的历史面板，不自动运行 Agent。Codex 使用 `resume`，Claude 使用 `--resume`，可打开工具自身的历史选择器或传入用户指定 ID；远程先刷新准确身份再确认 `fclaude attach`，旧 UUID-only 历史需重新选择。恢复成功后替换原面板引用，保留邻接分屏与比例；不重放旧 argv，不持久化环境变量或终端内容。
- 支持标签拖动排序、隐藏后从侧栏找回、项目/标签/面板键盘导航、按命令/项目/会话查询的命令面板。快捷键在对话框打开期间不改变背后的工作区。对应快捷键见双语 README。
- 这不是对任意外部进程的接管，也不等于 Agent 历史会话已在真实账户中全面验收。远程列表已可选择运行中的 Claude 会话，真实服务器下的接管、附件与网络恢复仍待验收。

## 远程控制协议与查询页面

- Rust CLI 新增独立 `--desktop-control` 入口，版本 1 当前提供 `capabilities`、`context`、`accounts.list`、`credentials.list`、只读 `skills.list/info/state.list/state.conflicts`、精确确认的 `skills.change`（启用/停用）、`ego_browser.status/requests`、精确确认的 `ego_browser.request.cancel`、`ego_browser.binding.plan/control` 以及 `ego_browser.delete.plan/control`，及严格确认的 `credentials.create/bind/unbind`、`accounts.create/status/bind/verify/disable`、`accounts.import.discover/prepare/validate/apply/pending/status/forget`、`accounts.runtime_recovery.authority/inspect/apply/status/pending/forget`、`accounts.default.get/set/clear`、`sync.status/ensure/pause/resume/resolve/reset`、`sync.recovery.preview/apply`、`mutagen.environment.preview/apply`、`sessions.list`、`sessions.stop` 与只读 `sessions.stop.request/status`、固定集合删除 `sessions.delete` 与只读 `sessions.delete.request/status`、`port_forwards.list/create/stop/reconnect`、`diagnostics.inspect`、`dependencies.inspect`、`wireguard.config.plan`、`wireguard.check`、`wireguard.status`、只读 `wireguard.bridge.status`、严格确认并重查执行结果的 `wireguard.tunnel.plan/apply`、显式确认的 `wireguard.config.apply`、`setup.local`、`login.password`、`login.device_code`、`logout`。`wireguard.bridge.status` 只返回安装、系统批准、协议兼容、调用方认证和执行可用性的分离观察；当前未安装受保护 broker 时固定为不可执行状态。`skills.list/info` 使用 CLI 用户安全会话，只投影目录/详情元数据；`skills.change` 只接收规范 Skill ID、动作、排序 scope 和代次绑定回执，内容、来源 locator、tracking、revision metadata、节点目标与凭据留在 CLI 内。隧道 apply 在冻结计划后调用平台 helper，执行后再次观察目标状态；权限不足、helper 缺失或状态不匹配均返回固定失败/未知结果，不伪造成功。凭据 mutation 只接收 profile 元数据和精确账户/profile ID；设备 token 留在 CLI 安全存储，未知字段（包括 token、私钥等）在 Rust 边界拒绝，回执只投影无秘密字段。会话创建/停止/删除与端口转发还会在 Go 边界严格匹配 capability 的 effect、device 凭据和 exact-request 确认模式；停止要求完整节点/后端身份，删除仅接受 Claude 的 `failed`、`interrupted`、`stopped` 固定集合。复用已有配置、设备凭据及 API；返回固定错误码与显式筛选的业务字段，不返回凭据或原始服务端错误。协议详见 [CLI 契约](../../agent-remote-cli/docs/desktop-control.md)。
- Go 适配器校验协议版本、请求 ID、事件/阶段、退出码和响应大小，不解析人类输出或自动重试。每个查询独立子进程；超限终止进程，取消只结束本地等待。列表携带实际服务器/设备上下文，和界面上下文不一致时拒绝展示。
- 登录/注销结束后失效列表与待确认连接，取消旧查询并按查询代次丢弃迟到结果，提示显式刷新。认证操作进行期间禁止根据远程列表新建连接，避免在上下文切换中使用旧会话 ID。
- 原生远程页显示共用配置目录、服务器、账户与会话，支持名称/ID/路径/状态筛选、手动刷新和窗口重新聚焦时刷新。权限失败与空列表分开呈现；已有错误随语言切换重新翻译。菜单与命令搜索提供远程入口，快捷键为 `Mod+Shift+R`。
- 选定本地项目后，可确认 home、服务器、设备、完整远程 UUID、本地目录与接管语义，再启动带 `--expected-server`/`--expected-device` 的 `fclaude attach`；仅 home/服务器/设备/会话/本地项目全部相同的存活连接可复用。未确认或取消时不启动进程；缺少 `sessions.attach` v1 能力时不回退旧命令。
- 远程会话卡片在能力和权限允许时提供“停止会话”。确认时冻结完整会话 UUID、节点、运行时后端及配置上下文；Rust CLI 生成 caller key，桌面保存不含内容的恢复指针并只查询原 key。恢复指针包含 key、home、Server、device、session、node、runtime backend 及可选 stop task ID；重启后须显式打开恢复入口，不自动查询或重放。回执独立展示进程停止、Skill 保存/发布和同步观察，`accepted`、`stopping`、`pending`、`unknown`、`saved`、`failed` 与删除后的状态不会互相推断；未知结果禁止提交第二次停止。关闭窗口继续后台运行，明确退出仍只确认停止本地会话。
- 远程会话卡片在能力和权限允许时提供“停止会话”和“删除会话”。删除确认冻结完整终态 Claude session UUID 集合、节点、运行时后端及配置上下文；Rust CLI 生成 owner-scoped caller key，桌面只保存并查询原 key。Server 的稳定回执保留 `reason=user_requested`、集合摘要、完整 `session_contexts`（不含 `user_id`）及删除/retention 阶段；集合变化、保存阻塞和未知结果都禁止扩大集合或提交第二次删除。关闭窗口继续后台运行，明确退出仍只确认停止本地会话。
- 能力列表表示实现支持，服务器权限仍由实际调用验证。端口转发页已接入 `port_forwards.list/stop/reconnect`（Rust 仍保留受控 `port_forwards.create`），按 home、服务器、设备、会话、节点和运行时后端复核上下文；桌面创建会启动受管 Rust `forward --json` 子进程，只有在其真实绑定 loopback listener 后才显示本地监听地址，Rust 继续负责一次性凭据、TTL、SSH/HTTP2 tunnel 与 bounded reconnect，Go 只保留安全投影和进程状态。跨重启恢复使用 Rust 隐藏 `forward --forward-id <full-uuid>` 精确复用既有 forward，先绑定本地 listener 再签发单次连接凭据，不重复 create；Server 缺少非秘密 SSH 路由字段时失败关闭。停止先让 CLI 完成远端清理，超时才终止子进程；transport unknown 会显示未知结果并保留原 key，不能自动换 key。统一操作中心已接入侧栏和命令搜索，聚合登录、账户、同步、恢复、网络、远程会话、初始化向导及后台查询的安全状态投影，并可回到所属页面；真实服务端/Node 网络、具体业务阶段和同步端到端验收仍待实现。通用阶段事件现由可选 `stream_events`/`CallWithProgress` 提供 started 基础事件，设备码登录保留专用审批事件。

## 统一操作中心

- 工作台的操作中心以只读安全投影聚合各业务页的进行中、完成、失败和需要处理状态。条目只包含本地化名称、状态和所属页面入口，不包含请求键、账户/会话 ID、路径、argv、提示词或凭据；精确上下文与原始回执继续由所属页面和 CLI journal 管理。
- 远程列表、诊断、能力、Skills 等只读查询仅在实际运行期间显示，缓存快照不会伪装成近期操作。未知或未完成的停止、删除、同步、导入和恢复回执显示为“需要处理”；停止恢复入口只查询原 key，不提供重新提交按钮。
- 关闭窗口、切换页面和语言不会清除操作投影；“打开详情”只导航回原页面，不重新提交。中心跨重启恢复不含敏感信息的生命周期摘要；专用受管终端同样只保存固定命令标签、状态和时间，精确 request key、回执和上下文仍由 CLI journal 管理，中心不代替 durable journal，也不会伪造回执或自动重放。

## 准确远程连接与原会话任务结果

本切片仅完成已有 Claude 会话的准确连接、历史恢复及 Server 旧任务结果保护。`sessions.attach` v1 作为原生终端能力单独声明；Go 冻结公开 home/服务器/设备/会话及本地项目，Rust 在登录锁与配置锁内先复验上下文再读取/刷新设备凭据，不登记新设备。一次授权 POST 后，只 GET 原逻辑任务和原记录 UUID，严格比较节点、SSH 目标、tmux 名称与转发策略，缺字段、未知状态、目标变化和失联均不重放 POST、不改用旧 attach 或新建会话。

Node 从 `SSH_ORIGINAL_COMMAND` 仅接受固定会话及 expected-device，将其与强制命令实际设备比较，再向新的精确验证接口提交实际 SSH key。服务端核对当前用户/设备/key/会话/节点、同步及转发策略，网关严格校验全部身份回显、字段和运行时后端，不跟随重定向、不回退旧接口。每次运行时报告独立探测已安装网关的 `--session-protocol-info`，仅更新 worker 不会误报支持；旧 binding 协议与普通 CLI 短 ID 连接保留。精确连接暂禁旧图片/文件接收通道，因为它尚不核验同一 expected-device/key；文字复制粘贴保留。

桌面现将该边界投影为严格的 `AttachmentStatus` v1：缺少 CLI `sessions.attach` v1 或 `attachment_transfer` v1 时为 `text_only`，确认页显示文字剪贴板可用、文件与图片明确禁用；两项能力均声明且限制合法时为 `exact_transfer`，mygo 将拖放/图片剪贴板作为 Kitty payload offer 交给 fclaude。fclaude 仍通过独立 exact 通道由 Server/Node 校验 session/device/实际 SSH key/lease，desktop 不插入本地路径、不复用 legacy 附件桥；三仓库真实传输与回执联调仍待验收。

历史存储 v3 兼容读取 v1/v2。准确历史先查询当前上下文并再次确认，旧仅 UUID 的历史引导用户重新选择；取消、失败及未确认不改变原面板，成功后保留原项目、名称、cwd 与分屏。确认详情可滚动，页脚固定；刷新、登录/注销和项目变化使旧确认失效。重复/空列表 ID 在加载时拒绝，防止目标歧义及 mygo key 冲突。CLI 清单现为 agent-remote 106 个命令节点/628 个参数绑定，fclaude 12/94；发现命令不计完整 GUI 覆盖。

已验证证据：Go→真实 Rust→隔离 HTTP→记录参数的 SSH 替身六场景全部通过，19.777s（`/tmp/agent-remote-exact-session-desktop-interop.log`），涵盖成功、丢失响应、原任务记录变化、缺少状态及确认后服务器/设备变化；验证只 POST 一次、只观察原键、不初始化旧附件通道、无秘密输出。Node 完整门禁退出 0、覆盖率 63.5%（`/tmp/agent-remote-exact-session-node-full-gate.log`），使用 711 个源码文件 SHA-256 一致的隔离副本，仅收紧副本权限，原目录未修改。Node API/gateway race 通过；首轮 runtime/worker race 的独立网关探针超过两秒而失败，未改生产阈值或断言，串行包复验 runtime 5.779s、worker 10.548s 通过（`/tmp/agent-remote-exact-session-node-race-recheck.log`）。

UI 定向 race 通过：app 124.848s、session 1.482s、storage 1.546s、cliadapter 1.565s，追加刷新失败/取消 11.724s；准确连接的稳定 Go/UI/PTY 切片通过，覆盖实际 PTY argv、准确/旧历史分屏替换、不同服务器/设备/项目隔离与相同范围复用。完整 `workbench-probe` 已补齐账户绑定和 remote-scope 阶段并在 macOS 本机通过。官方 catalog 629 条中英文消息；900×600 三语言截图位于 `/tmp/agent-remote-exact-attach-ui/`，中文及 en-XA 已目视检查。Windows/Linux amd64 最终交叉构建通过，仍不代表两平台原生运行或真实远程 SSH/tmux 验收。

最终 desktop 完整门禁启用真实 Rust CLI 与 `DESKTOP_TERMINAL_TESTS=1` 后退出 0，app 56.683s、cliadapter 134.754s，官方 catalog/生成代码、vet、所有测试与目录规则均通过（`/tmp/agent-remote-exact-session-desktop-final-gate.log`）。Rust CLI 最终完整门禁退出 0：1242 通过、3 跳过，152.678s，格式、Clippy、Shell/Ruby、依赖缓存及 doc tests 均通过（`/tmp/agent-remote-session-attach-final-gate.log`）。首轮 CLI 为 1241 通过、1 失败、3 跳过，失败源于新增普通 CLI 兼容夹具漏填旧 DTO 必需的 `command_args`；补齐测试夹具后完整复验通过，未改生产兼容语义。

最终双 CLI 二进制重新构建后，六个 Go→Rust 场景强制复验全部通过，12.250s（`/tmp/agent-remote-exact-session-desktop-interop-final.log`）。这条调用链采用隔离凭据、HTTP 与 SSH 替身，没有接入真实用户设备或部署远端 Node；源码与本机测试通过不代表真实三平台交付完成。

Server 旧 create/stop 结果现在绑定原任务记录、会话及不可变终态收据；准确重放不重复应用业务状态，冲突或终态缺收据时拒绝。停止 caller key 账本还保留节点、运行时后端、原停止任务及删除后的 owner-scoped 观察；失败任务不会被新键隐式重试。迟到创建仅推进原来仍处于 starting 的会话。它不证明 Node 进程已停止：无 spec 时 stop 先返回、迟到 create 再启动的实际运行时竞态仍需后续生命周期锁与持久停止意图保证。准确停止的 Go 适配器和确认 UI、创建原键、固定集合删除和端口转发 caller key 已接入，但跨端联合门禁、三平台原生运行和 Skill 保存/转发监听的最终验收仍按[会话管理计划](remote-session-management-plan.md)继续，不能计为正式版验收完成。

Server 最终完整门禁全部通过：2197 通过、110 跳过，284.88s，覆盖率 91.78%，753 文件格式/Ruff、695 源文件 mypy、docstrings 与 diff 均通过（`/tmp/agent-remote-session-attach-server-quality.log`）。仓储分层后的真实 PostgreSQL 并发结算 2 项通过，4.80s，验证等待原锁后的唯一收据重放和冲突拒绝（`/tmp/agent-remote-session-result-postgres-validation.log`），临时容器已删除。首轮完整门禁 2194 通过、110 跳过、1 失败：既有管理列表夹具用 `session-test` 代替合法原会话 UUID，被新增保护拒绝；将夹具改为合法 UUID，并新增缺失/损坏原任务身份拒绝回归后，最终完整复验通过。首轮日志保留为 `/tmp/agent-remote-session-attach-server-quality-first.log`，未通过放宽身份规则解决失败。

## 会话生命周期与创建前同步基础

准确连接之后正在推进持久停止意图、Server 停止失败/删除防护和 Rust 已有同步关系的 verified flush 基础。Node 生命周期接入及各自门禁已完成；桌面已接入准确停止确认、原 key 查询和独立进程/保存状态展示，但跨 Server/CLI/Node 的联合停止恢复协议与三平台验收仍未完成。既有停止任务的历史失败不会自动重试；准确恢复协议仍待后续实现。具体边界见[会话管理计划](remote-session-management-plan.md)。

Server 已实现逐后端生命周期能力规范化和旧停止/删除保护。普通 stop 失败保留 `stopping`，cleanup 失败保留 `interrupted`；单项与批量删除检查全部原停止任务及唯一、匹配的成功收据，批量在释放 Skill 引用前完成全量检查。历史成功仅保留兼容语义，不升级为物理终止证明。stop、删除和自动 cleanup 入队按用户历史→Session→任务的锁序处理，锁后刷新原会话，迟到 inventory 不能覆盖后续状态或删除结果。

Server 首次接受普通 `stop_tool_session` 成功回报前，会校验 `status=stopped` 与冻结任务的完整 `session_id`，以及节点提供时的 `runtime_backend`/`runtime_resource_id`；不匹配返回 `SESSION_STOP_RESULT_INVALID` 并保留 `stopping`，不能写出 `process_stopped=true`。旧 Node 省略可选 runtime 字段仍保持兼容。

Server 最终完整门禁退出 0：2240 项通过、127 项跳过，321.05s，覆盖率 91.82%，Ruff、mypy 702 文件、docstrings 与 diff 检查通过（`/tmp/agent-remote-server-stop-deletion-gate.log`）。聚焦 109 项通过、3 项跳过；真实 PostgreSQL 专项 19 项通过、23.08s，覆盖 stop/delete、cleanup/delete、迟到 inventory 及原结果回执竞争，临时 PostgreSQL 容器已移除。首批聚焦夹具因 cleanup 上报漏填 backend 而失败，补齐夹具后通过；首次完整门禁仅被新增能力测试的 import 排序阻止，修正后完成上述最终检查，未放宽生产身份或状态规则。

Node [持久停止意图](../../agent-remote-node/docs/session-lifecycle.md)已接入 Native、Docker Sandbox、managed start/recovery/cancel 与 cleanup，并通过 Helper 私有 `session_execution` 协商、跨进程锁、scope 绑定、原子 no-follow marker、缓存回放和 pending 保护。最终完整 Node 门禁在 788 个非 `.git` 工作区文件字节一致的隔离镜像中退出 0，覆盖率 63.2%；`go vet`、全部 Go 测试、安装脚本、managed Skill 一致性、Ruby 发布合同与 diff 检查通过，日志 `/tmp/agent-remote-node-lifecycle-final-gate.log`。此前首次镜像因测试所需的受保护目录权限未收紧而被 artifact 权限检查拒绝；仅调整隔离镜像权限后重新执行通过，未改变源码或断言。窄范围 race（runtimehelper/worker/runtime）及 Debian bookworm-slim root Linux 生命周期测试也通过，覆盖永久 marker 重开、跨后端关闭、坏链接/硬链接/损坏权限、跨进程 flock、停止先于启动、缓存三字段和批量重复 ID。缺少原 runtime 身份、部分批次已执行后丢失观察或历史 terminal 的情况仍返回 `SESSION_STOP_PENDING`，不伪造停止、删除或新启动授权；恢复闭环与 Server/CLI 联合门禁尚待后续。

Rust [verified flush 基础](../../agent-remote-cli/docs/verified-session-sync.md)已完成库实现及完整门禁：1259 项通过、3 项跳过，nextest 150.277s，脚本退出 0；fmt、Clippy、Shell/Ruby、依赖缓存、doc tests 与 diff 检查通过。最终日志 `/tmp/agent-remote-verified-flush-final-gate.log`。它只核验和 flush 已有的准确同步关系，不初始化或修复同步、不启动 daemon、不创建远程会话，也尚未接入新的机器方法或普通 `fclaude` 启动流程。

专项覆盖完整库→隔离 HTTP→Mutagen 替身的 18 个情形，包含实际 identifier、目录/配置/Server 目标漂移、前后 Git 锁及冲突、暂停、取消/超时和进程组子进程回收。Unix SQLite 文件身份检查改为不打开数据库文件的 metadata 观察，避免临时 FD 关闭释放同进程其他连接的 POSIX 锁；真实外部 writer 回归证明读事务在检查前后仍阻止写入，释放后才能写。该修复前的首轮完整检查主动中断，日志 `/tmp/agent-remote-verified-flush-full-gate.log` 不计为通过证据；修复后最终完整门禁如上。

`flush_may_have_run` 仅描述同步命令，所有错误都可能发生在凭据续期之后，不能据此声称整个流程无副作用。原生 Keychain/Windows 凭据调用可能延迟响应取消，外部进程期限仍是最终边界。此轮进程组与数据库证据来自 macOS；Windows Job 与 Linux 原生运行不因复用实现而算作已验收。以下项目同步记录属于此前已完成的 GUI 切片。

## 项目同步操作

- 所选项目可建立、查询、暂停、恢复、刷新冲突与重建同步；状态观察区区分 Server 同步状态、Server 冲突标记、本地 Mutagen 暂停/阶段/冲突数量，不把终端 Agent 存活当作同步正常。无法观察时明确为未知，未观察到冲突不等于所有文件已同步。
- 确认绑定 home、服务器、设备、规范化本地目录、项目键、workspace/sync ID，以及覆盖 node/endpoint/Mutagen 名称/模式/Git/排除规则的指纹。CLI 重新核对共享映射与远端目标，新 Server 的事务目标前置条件避免同 ID 改端点仍执行旧确认；旧 Server 不静默降级。
- `dry_run` 在 UI 标为“跳过本地执行”，明确仍修改 Server 和共享 SQLite，仅跳过本地 Mutagen 动作。`resolve` 不选择覆盖方向，必须先处理文件冲突；`reset` 重建 Mutagen 基线，可能重新发现冲突或恢复曾删除文件。
- 回执保留顺序阶段与 completed/partial/unknown；跨页、切项目和刷新不重放写入。刷新后的观察与原回执上下文分别展示；登录/注销完成使旧查询与确认失效，已提交回执仍保留。运行中操作可停止等待，取消后不能假定已回滚；关闭应用等待受管子进程清理。重启后操作中心会恢复不含请求键、路径或回执内容的阶段摘要，并标记为需处理；精确回执仍由 CLI journal 提供，desktop 不重建或自动重放。
- 旧 SQLite 映射可显式检查和恢复：只有准确资源的 Server 404 才进入恢复计划，403、超时和不可用不会判为陈旧。预览同时展示完整映射、清理范围、Mutagen namespace 与准确会话标识/端点/配置摘要；确认时重新核对不可变快照。仅同步会话不存在时保留 workspace，workspace 也不存在时移除两类映射；本地项目文件与远端资源不删除。
- 旧 Mutagen 会话存在时按准确 identifier 终止，确认消失后才 CAS 移除旧映射；daemon 停止、配置漂移、重复名称或缺元数据时不会擅自清理。数据库提交结果不明保留 unknown，不能宣称回滚。恢复不启动/重启 daemon、不迁移全局 SSH 环境。旧操作及恢复结果跨导航保留，完成恢复后用户先显式读取当前关系，再独立确认 ensure，不能自动重放或自动重新绑定。
- 六个 GUI 入口和恢复流程已实现不等于 P2 同步全部验收。真实 Node 链路证据限于 macOS 控制端与隔离 Docker Linux；外部服务注册的 Mutagen 环境迁移、Windows/Linux 原生桌面与正式安装环境仍待验收。

本切片的 Go 适配器/UI 定向测试通过：协议 3.367s、UI 6.287s；附加能力页/同步 UI 回归 4.722s；i18n 0.441s、presentation 0.353s。`go test -race ./tests/app ./tests/cliadapter -run TestSync -count=1` 通过，app 137.235s、cliadapter 74.393s（`/tmp/agent-remote-sync-go-race.log`）；该运行未设置真实 CLI，不能当作 Rust 或 Node 验收。真实 Go→Rust 同步联调随后通过（2.117s，`/tmp/agent-remote-sync-interop.log`）：隔离 loopback Server/临时凭据，覆盖缺失映射状态、dry-run 建立和四动作的实际 Server/SQLite 写入、目标前置条件、同 ID 端点漂移和上下文漂移拒绝、500 未知回执及不重试。含真实 CLI 的定向复验 app 2.278s、cliadapter 2.419s 通过（`/tmp/agent-remote-sync-final-targeted.log`）。

同步切片整仓门禁启用真实 Rust CLI 和 PTY 后退出码 0，471 条官方中英文消息、生成代码、格式、vet、所有测试及测试目录门禁通过，app 17.251s、cliadapter 17.753s，日志 `/tmp/agent-remote-sync-desktop-gate.log`。Windows/Linux amd64 桌面交叉构建再次通过，仍不是两平台原生运行证据。Rust CLI 完整门禁为 1076 通过、3 跳过、48 个测试目标，nextest 68.320s（`/tmp/agent-remote-sync-cli-gate.log`）；后续精确名称/自定义 SSH 用户、查询环境、真实冲突枚举、人类 dry-run 输出及历史 healthy/conflicted 准备状态修正追加 14 项同步合同、7 项 Mutagen 单测、原生 v0.18.1 探针及 Clippy 复验通过，没有将追加测试伪称为另一次完整门禁。最终冲突枚举的 Go 定向回归 app 0.715s、cliadapter 4.289s（含真实 CLI）及官方 i18n 471 条检查通过，日志 `/tmp/agent-remote-sync-enum-final.log`。

Mutagen 原生探针采用隔离短路径数据目录和暂停会话，验证官方 JSON、精确会话选择、重复名称及环境标记不匹配拒绝。随后以同版本官方 agent 和仅启动本地测试 agent 的 SSH 替身，验证真实创建/flush、双向文件往返、暂停/恢复、重建及冲突双方内容保留，1.44s 通过（`/tmp/agent-remote-sync-native-transfer.log`）；没有真实 SSH 网络或 Node 授权，不能替代跨机器验收。首次临时目录过长导致 macOS Unix socket 无法连接，改用 `/tmp` 下短路径后通过，没有放宽生产连接期限。查询显式使用既有 Mutagen 数据目录，避免 Windows 默认路径的隐藏属性写入；瞬时客户端取消会清理进程组/Job，明确启动的守护进程独立存活。当前对已有 daemon 状态目录但 SSH 环境标记不匹配的安装仍拒绝自动迁移（即使 daemon 已停止），避免中断其他项目；该初始切片未提供全局迁移；现有独立环境恢复已实现无外部注册服务的确认重启，陈旧映射恢复也已实现，见下方专项记录。

同期 Server 修复了迟到准备结果覆盖暂停、resolve 后旧 healthy 状态的连接兼容及 conflicted 状态的修复访问，并加入事务内完整目标条件、能力版本与检查回执。最终完整门禁 2031 通过、104 跳过、覆盖率 91.62%，197.73s，日志 `/tmp/agent-remote-sync-server-full-final.log`；聚焦合同 114 项通过。Node 同步网关要求当前 SSH key ID 与服务端回显匹配，需先升级 Server 再升级 Node。工作区准备采用逐层固定目录 FD、原子 marker、仅作用于已打开 inode 的 ownership/Linux ACL；拒绝硬链接和特殊文件，新增执行权限不会扩大其他 ACL 条目原有有效权限，非 Linux helper 在文件操作前拒绝。Node 最终完整门禁退出码 0、覆盖率 63.3%（`/tmp/agent-remote-sync-node-full-final.log`），gateway/API/workspace race 通过；Linux 一次性容器中另有 5 项真实 FD/ACL 检查，含完整准备两次幂等。最初 tmpfs 不支持 POSIX ACL，改用临时 overlay 文件系统后通过，没有跳过权限检查。因原工作目录权限不符合既有严格来源门禁，Node 完整检查使用 762 个源码文件 SHA-256 全部一致的隔离镜像，只收紧测试副本权限，原目录未修改。上述证据不覆盖其他旧 runtime 权限路径，也不代表真实部署验收。

最终 Rust 构建后再次执行 Go→Rust 同步互通测试，2.055s 通过（`/tmp/agent-remote-sync-interop-final.log`）。所有跨仓改动保留在工作区，未提交、推送、部署或发布。

首次官方 i18n 生成失败，因为 ICU select 分支不能直接使用带连字符的 Mutagen 状态名；呈现边界改为下划线键、协议仍保留原状态标识，重新生成通过。首次 900×600 UI 测试发现滚动正文中的动作按钮无法直接到达，已将操作与确认固定在页脚并重新通过测试。中英文/en-XA 的状态与确认共六张截图在 `/tmp/agent-remote-sync-ui/`，已人工检查伪本地化状态与中文确认；详情可滚动，操作栏可见。这些是 mygo UI 渲染测试截图，不是三平台原生窗口验收。后续补充回执自身的历史观察展示；登出测试原断言“所有 Mutagen 文本消失”因此失败，改为要求最新观察消失、原回执保留后通过。Server 状态/冲突/模式使用 catalog，本地冲突数使用官方数字格式且保留 uint64 精度，unknown/paused 观察拒绝含零计数的误导响应。

## 旧同步映射恢复与真实 Node 链路

恢复 Go/UI 定向测试通过：app 2.101s、cliadapter 1.577s；官方 i18n 0.362s、presentation 0.332s。覆盖旧部分完成结果→404 预览→明确确认恢复→保留旧回执→fresh status→独立 ensure 确认；权限失败、不可用、不可恢复计划均不能执行；取消返回未知结果并保留原范围，切换项目使未提交确认失效。适配器严格拒绝 namespace 改变、已完成 Mutagen 阶段缺少消失观察、失败阶段伪造观察及已变化的回执计划。持久化失败的 partial 与提交未知的 unknown 分别保留。

真实 Go→Rust 恢复互操作 6 场景通过，5.937s（`/tmp/agent-remote-recovery-desktop-interop.log`）：保留或移除 workspace、旧 Mutagen 会话已不存在、配置漂移保留映射、终止未知保留映射、daemon 不可用。该联调使用隔离 HTTP/Mutagen 工具夹具，不伪称真实服务部署。原生 Mutagen v0.18.1 恢复探针另外验证暂停变化不影响不可变确认、配置漂移拒绝、准确 identifier 终止保留两个无关会话、冲突两侧文件保留、错误环境标记不修改及 daemon 停止后不可清理。中英文/en-XA 的 900×600 恢复预览和结果共六张 mygo 渲染截图位于 `/tmp/agent-remote-sync-recovery-ui/`；已视觉检查，详情可滚动，确认/后续操作固定可见，较长英文自动换行。这不是三平台原生窗口验收。

Node 仓库的 `tests/linux_sync_chain_test.sh` 已通过真实 Server→非 root worker→root helper→SSH forced command→Bubblewrap→Linux Mutagen agent 链路（`/tmp/agent-remote-sync-chain.log`）。六个同步方法均经过该链路，验证双向文件、实际 UID/ACL、暂停不传输、冲突两侧内容保留、手动编辑一致后的 resolve、reset 保留内容，以及未知/已撤销 SSH key 被拒绝（包括 authorized_keys 仍保留旧 key）。CLI 制品 SHA-256 为 `7c1acee6867af8a31ec90a7e8cd0e630f237d593cf8b5db3966686931c526cbb`。临时容器、Mutagen 进程及文件已清理。控制端为 macOS Docker Desktop；该证据不覆盖 systemd 安装、WireGuard、已有连接即时撤销或 Windows/Linux 原生 desktop。

本轮 desktop 最终整仓门禁在真实 Rust CLI/PTY 开启下退出码 0：484 条官方中英文消息、生成代码、格式、vet、所有测试与目录规则通过，app 19.736s、cliadapter 24.340s，日志 `/tmp/agent-remote-recovery-desktop-gate.log`；未变化的终端包使用缓存。Windows/Linux amd64 `CGO_ENABLED=0` 交叉构建通过，仍不是原生运行证据。恢复定向 `go test -race ./tests/app ./tests/cliadapter -run SyncRecovery -count=1` 通过，app 112.972s、cliadapter 127.040s，无竞态；该次未启用真实 CLI，真实互通由前述独立测试验证。

CLI 本轮完整门禁 1100 通过、3 跳过，nextest 100.077s；最后的数据库提交未知语义调整另通过静态检查与 25 项同步合同（3.10s），没有将追加复验称为第二次完整门禁。Mutagen 最终 11 项单测 0.40s、原生恢复探针 1.76s 通过，日志 `/tmp/agent-remote-recovery-mutagen-unit.log`、`/tmp/agent-remote-recovery-mutagen-native-final.log`。原生探针第一次因同期 Rust nullable 解码器缺少 Deserialize trait 作用域而编译失败，改用 fully-qualified 调用后重新编译并通过，并非原生运行失败。Node 链路首次临时目录过长使 Unix socket 不可用，改用短 `/tmp` 路径后通过；脚本编辑期间一次尾部解析失败，最终完整脚本已另行重跑并退出 0。未放宽生产断言、超时或权限规则。

## 共享 Mutagen 运行环境恢复

- 独立“同步运行环境”页从环境诊断、项目同步页、命令搜索进入；不要求项目、Server 配置、设备或凭据。页面打开不调用 CLI，显式检查通过 `mutagen.environment.preview` 读取指定 CLI home、共享数据目录、原生 namespace/daemon 身份、工具内容身份及完整 sync/forward 持久清单；已停止 daemon 的预览不会启动服务。对应普通命令为 `deps mutagen-environment`，`--repair` 需独立交互确认。
- 确认页区分“上次记录的运行环境”与“重启后的目标环境”，不把 marker 当作当前 daemon 实际继承环境的证据。完整清单显示精确 identifier、端点、配置摘要和 paused；最多 4096 个且不截断，UI 每页 8 个并明确确认包含全部页。共享重启会暂时影响其他项目同步及转发，已暂停会话必须保持暂停。外部 launchd 等注册和不可验证状态拒绝自动迁移，需后续专门集成。
- 提交只发送 `confirmed`、`expected_home` 与完整计划 fingerprint，避免完整清单超过 64 KiB 请求上限；CLI 重新构建并比对全部 scope。Go 冻结原计划、验证回执原计划与请求一致，并要求成功后完整会话配置、paused、工具和 namespace 一致，运行身份与记录环境符合目标。四阶段 `stop/start/verify/persist` 分别展示，partial/unknown 不宣称回滚或远端健康；取消包含 CLI 明确返回的 `sync_mutagen_environment_outcome_unknown`，不重放。
- 全局恢复与旧项目映射清理分别记录结果；共享环境变更使当前项目观察失效但保留历史回执。跨导航和新检查保留本页原结果，只有明确结束本次操作才清理内存历史。操作中与共享配置/项目变更互斥，应用退出收回客户端进程。操作中心跨重启恢复不含敏感信息的生命周期摘要，精确回执和上下文仍由 CLI journal 管理。

Go/UI/adapter 定向验证通过：app 3.253s、cliadapter 2.428s；后续 SIGTERM unknown 固定码及能力页补充回归 app 4.744s、cliadapter 2.601s、capability 0.349s、i18n 0.369s、presentation 0.322s。包括无项目/登录、完整 sync+forward、分页、160 会话计划超过 64 KiB 仍只发送摘要、外部注册阻断、坏回执与配置/暂停/namespace 漂移、partial/unknown 和取消不重放。环境定向 race 通过 app 80.208s、cliadapter 117.567s；该运行不含真实 CLI，且先于后续固定错误码小改。

真实 Go→Rust→官方 Mutagen daemon 联调通过 19.835s（`/tmp/agent-remote-environment-desktop-interop.log`）：子进程隔离 OS HOME、短 `/tmp` namespace 和 direct daemon，验证停止服务时完整持久清单只读且不自动启动、确认 startup、运行服务再次 restart、暂停配置及项目文件保留、旧确认与 pause 漂移拒绝、准确 daemon 身份变更，以及不初始化配置/凭据/SQLite。测试只操作隔离 daemon，父进程 HOME 与用户 namespace 不变。测试源为 `tests/cliadapter/mutagen_environment_interop_test.go`，通过 `DESKTOP_MUTAGEN_BINARY` 显式选择官方制品。

三语言 900×600 预览、会话详情和结果共九张 mygo 渲染截图位于 `/tmp/agent-remote-mutagen-environment-ui/`；已视觉检查中文预览、en-XA 会话与回执，正文滚动、长文本换行和固定双行操作栏可用，不能代替原生平台验收。首次生成时自动 placeholder 提取误把 ICU 分支常量声明为参数，导致生成警告和编译失败；改为显式类型化参数并重新生成后定向及 i18n 检查通过。该环境恢复切片的实际命令库存为 agent-remote 106 节点/623 参数绑定、fclaude 11/84，生成脚本同步保留现有能力边界，不把发现命令当作全部 GUI 验收。

本切片整合验证：desktop 开启真实 Rust CLI 与 PTY 的完整门禁通过，518 条官方中英文文案、生成文件同步、格式、vet、测试目录规则及全仓测试通过；app 33.552s、cliadapter 43.050s，日志 `/tmp/agent-remote-environment-desktop-gate.log`。之后仅将共享重启影响提示移动至审阅标题下，最小窗口三语言九图重新生成，布局专项 1.266s 通过，首屏可见影响范围。Windows/Linux amd64 的 `CGO_ENABLED=0` 交叉构建通过，不代表原生运行。CLI 完整门禁 1149 通过、3 跳过，nextest 81.969s（`/tmp/agent-remote-environment-cli-gate-final.log`）；随后环境摘要改为保留非 Unicode 环境变量的原始编码，避免不同输入被 lossy 转换为同值，最终静态检查与 9 项环境协议合同 0.89s 通过，日志 `/tmp/agent-remote-environment-final-static.log`、`/tmp/agent-remote-environment-final-contract.log`。追加复验不描述为第二次完整门禁。 最后使用重编译后的真实 Rust CLI 复验 Go→Rust→官方 daemon 联调，16.199s 通过（`/tmp/agent-remote-environment-desktop-interop-final.log`），覆盖最终环境摘要实现。

macOS 最终两项真实 Mutagen 探针通过：环境恢复 62.19s、原项目同步/传输/冲突/准确终止 61.04s（`/tmp/agent-remote-environment-native-final.log`）。新增边界包括新空 namespace 初始化、已有停止环境不能隐式重启、持久配置/paused 漂移、home 目录替换、marker namespace/运行代次变更、外部启动后旧 marker 失效和 macOS launchd 注册阻断。Linux arm64 的独立非 root 容器另外通过 14 项库存/原生锁单测（1.24s）及官方 Mutagen 环境生命周期（51.71s），使用最新原始字节摘要和兼容 O_PATH 的目录同步，源码摘要前后相同；完整记录在 `/tmp/agent-remote-mutagen-linux-native-proof/validation.log`。容器、daemon 和临时状态已清理。Linux 证据只覆盖容器内本地 sync/forward，不覆盖 systemd、真实 SSH 传输或 Windows。

本轮失败均保留记录：官方模板额外输出空行导致首次真实准备超时，修正只读清单解析后通过；新增启动失败夹具因系统对无效映像进行 shell 回退而实际已产生进程，正确结果是 unknown，改用明确的执行权限拒绝验证 spawn 前 partial，未放宽生产结果判断；旧原生测试最后仍写 v1 marker，已迁移到 v2 marker 后完整复验。全局 marker 的路径及摘要格式现在先验证，拒绝不可信记录污染审阅；发布使用已打开目录相对原子替换。macOS 外部服务接管、Windows 原生验收、持久回执与完整 P2–P4 仍未完成。未提交、推送、部署或发布。

管理员运行时恢复已进入独立 P2 切片，复用现有 v1/v2/v3 和原 request ID；当前实现与验证见下方专项记录。账户配置导入此前审计发现的共享采集、caller request key 和确切任务恢复缺口已由下节工作补齐；具体部署与平台验收仍以实际证据为准。


## 账户配置导入与原请求恢复

- 账户详情提供本地 Claude 配置导入，打开页面不读取来源内容。本地发现只读元数据；“读取文件并生成审阅清单”才调用 Rust 读取内容。`include_resume_history`、`exclude_skills` 均默认 false；排除顶层 skills 不影响插件内技能，也不会在 Server 拒绝后自动改变选择。
- 明确展示目标 home/Server/device/account、本地 source home、包含/排除范围、完整文件清单及路径/字节数/摘要。最多 2,048 项分为每页 8 项，不截断，确认涵盖全部分页。Go 不接收文件内容；准备摘要还绑定来源身份和 Server intent。准备时同时只读加载 CLI 已保存请求，同账户原上下文仍有未清理请求时，GUI 禁止再次生成上传 key，须先查询并确认清理已结束记录。CLI 的文件/总量/深度等采集限制见[导入契约](../../agent-remote-cli/docs/desktop-account-import.md)。
- Server 校验为可选且独立确认的 POST，会产生审计，不发送内容，也不授权未来上传。上传另行确认；Go 只传准确 context/account/selection/source/digest/预先保留 UUID，CLI 重读全量内容并核对原摘要后一次提交，不发送超过 64KiB 的完整计划作为控制输入。Server 接受只是任务已创建，不能当作配置已应用完成。
- CLI SQLite 是唯一 durable journal，提交前记录原请求编号、公开上下文、摘要与可选准确 task ID，不保存来源路径、manifest、内容、凭据或原始服务端错误。desktop 无第二份 journal；命令搜索/远程页的“配置导入记录”在打开时自动读取 CLI `accounts.import.pending`，因此重启后仍能恢复；切 Server 后仍保留原上下文。查询只针对 original key 和 exact task，不读来源、不修改 journal，不以 latest 猜测或以新 key 重试。404 不能证明原 POST 未提交。
- 已明确拒绝的单次提交可确认清除本地引用；已接受的任务必须再次查询原 key/准确 task 并确认终态才能清除，unknown/pending/404 不清除。精确结构化 `SKILL_MANAGER_OWNS_PATH` 的 409 被标为明确拒绝，并映射固定 `account_import_skills_managed`；其他未知 409 仍保留未知。界面提示用户先显式 forget，再自行选择排除顶层 skills 并重新 prepare，未清理引用不会生成新的上传 key。删除不影响 Server 任务或配置。停止等待、切页或关窗口不重放请求；退出应用取消并回收受管 CLI，原 durable key 留待恢复。
- 业务错误、任务状态和两种固定风险提示通过官方中英文 catalog 呈现，拒绝未知服务端文案及不完整/多余/不一致协议字段。真实来源内容、路径名称和协议标识不翻译。所有新增测试与夹具位于根 `tests/`。

Go 定向复验通过：app 6.336s、cliadapter 3.539s、i18n 0.412s、capability 0.364s；补充跨重启未清理请求禁新 key 后再次通过 app 8.948s、cliadapter 3.338s、i18n 0.476s。覆盖独立本地读取/审计/上传确认、可选校验、完整分页、500 文件 digest-only 输入、完整/安全 manifest、exact receipt、取消与未知、跨 App 重启恢复原 key、404 保留、准确 task、不自动排除 skills、显式 forget。该次使用 CLI 替身，真实 Rust/Server 验证另行记录；不能替代部署验收。定向 `go test -race ./tests/app ./tests/cliadapter -run '^TestAccountImport' -count=1` 通过，app 147.701s、cliadapter 111.820s；此轮在后续 GUI 新 key 禁用补充前执行，后续对重启后的新 key 禁用、取消和显式 forget 三项追加 app race 77.305s 通过。真实 Go→Rust 与隔离 HTTP/来源/journal 的 5 场景互通通过 13.708s，日志 `/tmp/agent-remote-account-import-interop-current.log`，包括受理、响应丢失、来源漂移、链接拒绝、无确认拒绝和不读来源的原请求恢复；不是实际 Server/Node 部署。当前实际 schema 为 agent-remote 106 命令节点/626 参数绑定（`account import-config` 12 个参数）、fclaude 11/84，已由新二进制重生成覆盖表。

提交前失败的审阅恢复补充：只有 apply 已完成并返回 `account_import_source_changed`、`account_import_empty`、`invalid_params`、`confirmation_required` 或 `unsupported_method`，且后续只读 pending 成功证明原 key 不在 CLI journal，界面才显示“未提交”。用户显式结束本次审阅后清除该内存预留引用，之后可重新准备和确认上传；不会自动重试，也不调用远端删除。pending 失败、journal 仍包含原 key、通用 protocol/transport/unknown 均保留原引用。9 个 mygo UI 子场景定向通过 6.721s；全部导入 app 9.900s、cliadapter 2.044s、i18n 0.429s、capability 2.615s 通过，官方 catalog 检查为 571 条。该次使用 CLI 替身复验 UI 操作与错误边界，不替代真实 Rust/部署验收；补充后的完整门禁结果见下文。

最小窗口 900×600 的 en/zh-Hans/en-XA 审阅、清单、确认、回执共 12 张 mygo 渲染截图位于 `/tmp/agent-remote-account-import-ui/`，中文审阅与伪本地化确认/回执已视觉检查，长文本换行、滚动和底部操作可用。分页曾复用会话文案，已改为导入专用文件分页文案。

首轮 desktop 整仓 `DESKTOP_CLI_BINARY=... DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh` 退出码 0：官方 catalog 570 条，app 72.652s、cliadapter 87.904s，包含格式、生成一致性、vet、测试布局及其余测试，未变化的终端/session 包使用缓存。日志 `/tmp/agent-remote-account-import-desktop-gate.log`。随后使用包含最后拒绝记录校验修正的 Rust 制品，强制复跑真实 Go→Rust 五场景，23.434s 通过（`/tmp/agent-remote-account-import-interop-final.log`）；同时验证同 key 二次提交及新 key 绕过未决请求均被拒绝。最终 Go 源码的 Windows/Linux amd64 交叉构建通过，仍不是原生运行证据。第一次联调误用了旧临时目录制品，返回 unsupported_method；改用本轮新构建后通过，旧制品结果不作功能失败或通过证据。

提交前失败界面修正后的最终 desktop 完整门禁也退出码 0，使用最终 Rust 制品并启用终端测试；官方 catalog 571 条，app 50.768s、cliadapter 55.716s，格式、生成一致性、vet、i18n、测试布局及全部测试通过（`/tmp/agent-remote-account-import-desktop-final-gate.log`）。Windows/Linux amd64 交叉构建随该修正再次通过；交叉构建不代表原生平台验收。 提交前失败的 9 场景最终追加 race 通过，app 308.572s；未放宽 unknown 或 journal 查询失败时的保留规则。

Rust 最终完整门禁退出码 0，1,192 项通过、3 项按条件跳过，Nextest 174.814s，doc tests 通过（`/tmp/agent-remote-account-import-cli-final-gate.log`）；最后源码的格式、Clippy、shell、缓存/工作流及 diff 检查另行复验通过（`/tmp/agent-remote-account-import-cli-final-static.log`）。共享采集覆盖读取期间增长、FIFO 替换、同内容换 inode、清单新增及限额；journal 校验键与记录身份、阶段和 task 的一致性。首次并行测试暴露祖先目录因无关兄弟目录变化而误报来源变化，现仅对祖先验证对象身份和类型，真正 `.claude` 与选中范围仍保留完整元数据和两次遍历核对。恢复后已知 task 可清理最初 task 为空的记录，但明确无 task 的拒绝记录不能携带伪造 expected task。只有准确 `SKILL_MANAGER_OWNS_PATH` 的 409 可归为明确拒绝，其余 409 保留未知；用户自行清理记录并重新选择排除范围。

Server 完整质量门禁退出码 0：2,088 项通过、105 项按条件跳过，440.55s，覆盖率 91.64%；格式、lint、mypy（678 文件）、docstrings 与 diff 检查均通过（`/tmp/agent-remote-config-import-server-quality.log`）。原子受理、同 key 并发、来源内容不回显、Unicode 摘要及完整状态查询不更新设备活动均有测试；请求流在 JSON 解析前限制为 16MiB。最后将分块缓存改为 bytearray，避免小块数量造成额外无界开销，此局部修正另经格式、类型与 chunked/replay 两项测试验证。独立 PostgreSQL 17 的完整迁移至 `0057_config_import_requests`、并发首次空键只生成一份任务/审计、原回执恢复及非空记录拒绝降级验证通过，4.41s（`/tmp/agent-remote-config-import-postgres-validation.log`）；临时容器已移除。首次完整门禁遗漏新表及迁移 head 的固定测试期望而失败，65% 时中止，更新后上述最终完整门禁通过；失败日志保留为 `/tmp/agent-remote-config-import-server-quality-interrupted.log`。所有跨仓改动仅在工作区，未提交、推送、部署或发布。

首次官方生成因 placeholder 类型误用 `string` 失败，改为官方 `String` 后生成通过；初次合同测试因复用 helper 的返回值类型不匹配编译失败，修正后定向通过。适配器复审发现 task ID 含命名空间冒号，不适用通用账户 ID 校验，现按 `import_tool_account_config:{account}:{UUID}` 检查并同步测试。以上失败不计作通过证据。最后新增的 skills-managed 明确拒绝→读取 pending→确认 forget→用户选择 exclude-skills→fresh prepare/apply 流程，与 unknown 重启保护合跑 app 2.489s 通过。原生 Windows/Linux、正式安装、真实用户来源及部署后 Node 应用行为仍待验收；项目整体未达到首个正式版。

## 管理员运行时恢复

独立维护页接入 `accounts.runtime_recovery.authority/inspect/apply/status/pending/forget`，按用户管理员身份操作，支持用户登录但不登记设备的配置。打开页面不自动访问网络；显式检查权限后，观察准确原迁移任务并选择验证目标、验证源或修复源。观察不表示可执行资格，所有提交另行确认；编辑账户、原任务或动作会使审阅失效。手动原键查询与新提交分开，历史回执不会变成执行计划。

Rust 与普通 CLI 共用恢复服务，机器调用严格读取用户凭据，不刷新、迁移或初始化；设备凭据不提供管理员身份。管理员与账户所有者分别绑定。Server 新增只接受 user token 的四个入口，拒绝设备身份也不更新 last_seen；沿用既有任务、lease/result 和 writer lock，在锁内复验完整原任务投影，不增加数据库表或 Node 执行协议。同键重放返回原回执，不拿更晚的 backend/node/profile 否定已受理历史。

CLI SQLite 是唯一公开请求键与不可变回执元数据来源；本地记录不复制远程任务状态机。首次 POST 前保存原键，未知结果禁止自动重试或以新键绕过；状态只 GET 原键，手动历史查询不创建本地记录。清理明确拒绝的记录可离线执行，其余必须重新验证准确终态；丢失受理回执时允许使用新观察到的准确 binding，但删除时仍比较原存储记录。UI 保留原请求、取消仅停止本地等待，并在多记录清理确认中突出准确上下文、账户、请求及摘要。全部新增界面文案沿用官方 catalog，测试与夹具位于根 `tests/`。

该切片已完成本机集成验证，尚未替代三平台原生验收。Server 新接口与旧恢复回归第二轮 66 passed / 3 skipped（88.01s）；跳过的 PostgreSQL 场景已在真实 PostgreSQL17 专项连同新并发/锁内漂移检查共 9 passed（7.58s），日志 `/tmp/agent-remote-reviewed-recovery-postgres-validation.log`，临时容器已移除。Server 最终完整门禁退出码 0：2127 passed、108 skipped，544.98s，覆盖率 91.72%；742 文件 ruff、684 source files mypy、docstrings 和 diff 检查通过，日志 `/tmp/agent-remote-reviewed-recovery-server-quality.log`。初次全门禁在 pytest 前发现新增测试 query 参数的 mypy 类型错误，改为显式字符串后重新完整执行；未改生产语义。CLI 与 desktop 最终门禁结果见下文；这些证据不等于三平台实际部署验收。

首轮真实 Go→Rust 联调失败已定位：Go `ApplyRuntimeRecovery` 将 `json.Marshal` 产生的 `[]byte` 复用于对象响应接收，JSON 解码因此失败，把已受理操作显示为 unknown。三种动作的正常受理均失败，丢失响应、手动历史只读、权限撤销、畸形回执及拒绝后离线清理通过；日志 `/tmp/agent-remote-runtime-recovery-interop.log`（整轮115.755s，失败）。原始 JSON 回执诊断证明 Rust 返回正确结果且 registry 为 accepted；已改用独立 `json.RawMessage`，保留严格校验；后续复验结果见下文。没有重放原请求或放宽成功判定。

修复后的真实 Go→Rust 四组十场景全部通过，155.626s，日志 `/tmp/agent-remote-runtime-recovery-interop-fixed.log`。覆盖三动作正常受理与丢失回执、准确终态清理、畸形受理保留 unknown 且阻断原键和新键、明确拒绝离线清理、手动历史已知 binding 只读复查，以及提交前撤销管理员身份。该次使用较早 Rust 构建；后续最终源码还增加读后凭据复验与 URL 规范化纯解析，最终构建复验另行记录。

桌面替身专项 app 4.617s、cliadapter 2.791s、capability 0.490s 通过；authority URL 等价边界专项 0.813s 通过，允许 host 大小写、默认端口和尾斜杠差异，拒绝跨 host/path/scheme 和含用户信息、查询或片段的 URL。三语言 review/confirm 共六张 900×600 截图位于 `/tmp/agent-remote-runtime-recovery-ui/`，已目视检查中文及 en-XA 确认与英文审阅：正文可滚动，固定确认按钮可见。首次 en-XA 测试尝试点击已滚出视口的字段失败；测试改为按实际矩形滚动，同时收敛固定页脚动作后通过。这些是渲染测试，不能替代三平台原生窗口验收。

最终 Rust 构建（隔离 target，含读后凭据复验和 URL 纯解析）真实 Go→Rust 复验通过，60.929s，日志 `/tmp/agent-remote-runtime-recovery-interop-final.log`；额外确认配置 URL 与输入尾斜杠等价、回执继续使用 canonical 身份。Go UI race 87.762s 通过；官方 catalog 已增至 617 条，中英文完整 i18n 测试 0.309s 通过。导航布局修正后，Windows/Linux amd64 交叉构建再次通过，仍不是两平台原生运行证据。

首轮启用最终真实 CLI 与 PTY 的 desktop 完整门禁未通过：adapter 129.835s 通过，但 app 121.816s 中有五项 en-XA 最小窗口回归（配置导入、绑定审阅、停用后重绑、账户停用审阅、远程连接确认）。新增维护入口撑高导航，导致准确账户或本地项目确认信息不可见；日志 `/tmp/agent-remote-runtime-recovery-desktop-gate.log`。已将管理员恢复入口移出固定标题栏，放入可滚动管理区；无设备用户仍保留直接入口，搜索入口保持可用。没有修改测试或断言，五项旧布局回归与新恢复布局一次复验通过（app 5.888s），随后重新执行完整门禁。

CLI 首轮完整门禁为 1220 passed、1 failed、3 skipped（183.405s）：旧 capability 测试 write 白名单遗漏新的 apply/forget，生产能力输出正确。已补齐合同白名单，重新执行完整检查；原失败日志 `/tmp/agent-remote-runtime-recovery-cli-gate.log`。更早的聚焦 fixture 将整个多进程场景限制为 15 秒而出现超时，已调整为逐请求 15 秒有界空闲窗口，同时去除纯 URL 校验中不必要的 TLS 客户端初始化；聚焦 13 组在修复后 65.58s 通过，日志 `/tmp/agent-remote-runtime-recovery-focused-final.log`。

最终 CLI 完整门禁退出码 0：1221 passed、3 skipped，138.311s；fmt、Clippy all-target `-D warnings`、shell/Ruby/cache、doc tests 与 diff 检查通过，日志 `/tmp/agent-remote-runtime-recovery-cli-final-gate.log`。最终二进制生成时间早于上述最终 Go→Rust 联调，生产源码没有后续改动；较早失败门禁仍保留，不能用它替代本次通过记录。

导航修复后的 desktop 完整门禁启用最终真实 CLI 和 PTY，退出码 0，官方 617 条消息、生成代码同步、格式、vet、全部测试与目录规则通过；app 56.162s，未变化 adapter 使用上轮已通过的 129.835s 缓存结果。日志 `/tmp/agent-remote-runtime-recovery-desktop-final-gate.log`。另渲染修复后的远程列表三语言截图，布局专项 0.838s 通过，已查看 en-XA 导航；截图 `/tmp/agent-remote-runtime-recovery-navigation/`。当前 CLI inventory 为 agent-remote 106 命令节点 / 628 参数绑定、fclaude 11 / 84；新增 `recovery-status --forget/--yes` 已进入实际二进制清单。

契约见 [CLI 运行时恢复协议](../../agent-remote-cli/docs/desktop-runtime-recovery.md)及 [Server reviewed requests](../../agent-remote-server/docs/runtime-recovery-requests.md)。

## 默认账户共享操作

默认账户已增加独立入口，对应 CLI 的 `account default get/set/clear`，通过新协议 `accounts.default.get/set/clear` 访问共享状态。它是 P2 账户操作的一个完整流程，不代表账户命令族或全量 CLI 覆盖已完成。

- 按工具查询当前 home 内默认值，查询不创建数据库；设置从实际账户列表选择完整 ID，CLI 使用当前设备凭据验证账户与工具。清除只移除本地选择，无需凭据或服务器，不删除或禁用远端账户。
- 确认绑定 home、服务器、设备及旧默认值。共享 Rust 服务在发布时锁定配置、重查上下文并在 SQLite 事务中比较旧值，普通 CLI 同时接入；并发切换或另一端已修改时拒绝覆盖。沿用既有按 home/工具存储的键，默认值跨服务器共用，界面明确提示。
- Go 严格校验能力、必需可空字段、请求关联和回执一致性；提交前冻结确认参数，操作中可跨页返回，取消后不自动重试。操作中心跨重启仅恢复安全生命周期摘要；精确回执和上下文仍由 CLI journal 查询。结果未知时显式刷新查询当前值，不能将当前值当成原操作已成功的证明。
- 新界面沿用官方中英文 catalog，新增测试及夹具均在根 `tests/`。协议与错误边界见[默认账户契约](../../agent-remote-cli/docs/desktop-account-defaults.md)，本切片的最终验证结果另记于验证边界。

## 账户生命周期与原生绑定终端

- 创建表单覆盖工具、名称、区域、时区、locale 与有序标签；详情区分别展示账户和绑定的两次查询观察，不假定原子快照。提交前确认准确 home、服务器、设备与账户，CLI 锁定上下文并只提交一次。
- 创建、绑定、验证、停用返回筛选后的受理回执；任务已受理不等于登录或验证已完成。显式刷新查看后续状态，失败或取消不自动重放。回执支持跨页查看，目前仍仅存在内存。
- 绑定登录使用独立终端能力 `accounts.binding.attach` 与固定 `agent-remote account attach` 参数，不执行服务端 `connect_command`，不把绑定 ID 当成普通会话 ID。当前设备 SSH 公钥通过 Server 授权；轮询只观察原授权任务。CLI 和 Node 都检查准确账户与绑定批次，Node 额外验证实际 SSH key；Server 还要求 Node 心跳声明从实际网关探测到的准确绑定与密钥校验能力；缺少声明时连接失败，不创建授权任务，也不降级到旧命令。
- Go 用 `agent-remote` 启动受管 PTY，同项目、同上下文且同绑定的存活连接可重新聚焦。历史仅保存公开账户/绑定引用；重启后的恢复入口重新读取当前上下文及状态，用户再次确认后才连接，不重放旧批次。终端连接关闭不表示远端 tmux 已停止。
- 停用使账户不可用并撤销端口转发授权，不删除账户、清除工具凭据或停止已有会话。Server 拒绝对已停用账户直接验证，并通过账户行锁阻止迟到绑定/验证任务将其重新激活。用户仍可按既有 CLI 语义显式重新绑定停用账户，确认页说明它会重新进入绑定流程；这不同于迟到任务隐式恢复可用。
- 中英文文案使用官方 catalog；测试和原生探针全部位于根 `tests/`。该初始账户切片不覆盖配置导入及跨重启导入恢复；两项现已另行实现，见下方专项记录。管理员运行时恢复现已另行接入并正在验证；其他精确回执恢复仍由各自 CLI journal 契约负责，完整 CLI 覆盖与三平台实际账户验收尚未完成。本切片最终测试结果见验证记录；代码存在不等于全部验收。

账户切片验证已取得：CLI 完整门禁 1051 通过、3 跳过、47 个测试二进制，nextest 76.151s（`/tmp/agent-remote-accounts-cli-gate.log`）；gate 后测试 fixture 的空 PID 文件竞争修正另通过取消回归与 Clippy。真实 Go→Rust 生命周期联调 2.734s；账户适配器/终端能力及 session 定向 race 195.537s / 1.445s。macOS 原生工作台探针验证准确 binding argv、home/项目、PTY 输入、同连接聚焦、关闭及历史重新查询；原生 locale/window/PTY 探针也通过。Windows/Linux amd64 交叉构建通过，均非原生运行证据。命令库存更新为 agent-remote 105 个节点、619 个参数绑定，fclaude 11/84；不等于 GUI 验收完成。

本轮首次 desktop 完整门禁失败：`TestAccountsMutationReceiptsMatchRequestAndAcceptance/create/account-null-context` 的能力预检子进程在 90 秒后超时，尚未进入提交；不是接受了损坏回执。同环境该子测试连续三次通过（总计 1.063s），此前整组 race 通过；静态核查未发现此场景的预检阻塞路径，原因仍未确认，未放宽断言或超时。日志为 `/tmp/agent-remote-accounts-desktop-gate.log` 和 `/tmp/agent-remote-accounts-preflight-repro.log`。提交后旧 status 在登录切换时未失效的问题已另行修复，保留 receipt、清除旧查询及连接入口；Accounts 全组 app race 277.910s 通过；补齐停用账号显式重新绑定后专项普通测试 5.014s、race 35.994s、i18n 0.625s 通过，catalog 为 435 条。最终 desktop 完整门禁已通过，见下方最终记录。

第二次 desktop 完整门禁在另一场景 `bind/` 的能力预检同样超时 90 秒（`/tmp/agent-remote-accounts-desktop-final.log`）。专项重复运行期间捕获两个尚未进入 Go runtime 的 helper：macOS sample 仅显示单线程 `_dyld_start`、112 KiB footprint，之后恢复；这确认了测试二进制启动层延迟，不是账户 JSON 解码阻塞。每个子测试此前都会复制完整测试二进制；Unix 测试安装现改为指向已运行映像的 symlink，Windows 保留复制；修改前专项 10 轮通过但耗时 195.209s，修改后 5 轮通过共 4.869s。未放宽生产超时、测试断言或未知结果语义。取样位于 `/tmp/agent-remote-helper-diagnostic/sample-28103.txt` 与 `sample-28438.txt`。

Server 完整门禁通过：1952 通过、104 跳过、coverage 91.56%，340.48s；格式、Ruff、Mypy、docstring 与 diff 检查通过（`/tmp/agent-remote-binding-server-final-gate.log`）。Node 完整门禁及新增 binding/runtime/worker race 通过、coverage 63.2%；原工作区既有 managed Skill 源目录为 0775，严格来源检查拒绝，因此完整门禁在 690 个源码文件字节相同的隔离副本运行，仅调整副本权限，未更改原目录权限（`/tmp/agent-remote-binding-node-isolated-final-gate.log`）。这些证据不替代真实 SSH 网络、工具登录或 Windows/Linux 原生运行。

900×600 三语言 create/status/binding confirmation/disable 截图位于 `/tmp/agent-remote-accounts-ui/`，已抽检中文确认、伪本地化创建检查和英文绑定详情；正文滚动、底部按钮可达。首次扩展工作台探针因取消检查页后未滚回连接按钮而失败，补足实际滚动后复验通过；该失败是探针交互定位问题，未以自动重试掩盖生产连接失败。

账户切片最终复验：`DESKTOP_CLI_BINARY=/tmp/agent-remote-wireguard-macos.aB73C6/debug/agent-remote DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh` 退出码 0，435 条官方消息、生成文件一致、格式/vet/全部测试及布局规则通过，app 15.805s、cliadapter 15.248s（日志 `/tmp/agent-remote-accounts-desktop-verified.log`）。Unix helper 改动后没有再次出现预检超时；这不证明 OS 装载层永远没有延迟。最后新增 Node 能力不足的明确升级提示，在首次授权和后续观察时都不重放 POST；专项 2.40s 通过，随后 CLI 完整原脚本 1052 通过、3 跳过、47 个二进制，nextest 90.285s、退出码 0（`/tmp/agent-remote-accounts-cli-final.log`）。最终 Windows/Linux amd64 desktop 构建通过，仍无两平台实际账户/窗口运行验收。

## 能力说明与 Agent 可用性

- 侧栏与命令搜索提供“能力说明”。页面被动读取 CLI 协议、所选配置上下文及 `agent-remote`/`fclaude` 命令清单，可刷新、取消和筛选命令；不读取凭据、不访问服务器、不执行组件版本命令。当前二进制清单为 agent-remote 106 个节点/628 个参数绑定、fclaude 12 个节点/94 个参数绑定；`fclaude delete-status` 也已列入清单。
- 已实现的 desktop 工作流区分 CLI 不支持、配置缺失、设备未登记、权限未验证、已观察允许与拒绝。权限观察只复用同一 home/服务器/设备的已完成远程查询；认证变更使旧快照失效。凭据存在、方法声明或平台支持都不构成服务端授权。
- CLI schema v1 增加可选的逐命令 `platforms`；旧 CLI 没有该字段时显示未声明，不推断全平台支持，也不继承父命令限制。当前经源码核实的声明仅覆盖 5 项 macOS Device 本地命令、5 项三平台 WireGuard 命令和 macOS 的 `ego-browser upgrade`，详见 [CLI schema](../../agent-remote-cli/docs/desktop-schema.md)。命令库存不代表对应 GUI 已交付。
- 新建和历史恢复检测 Codex、Claude、fclaude 的可执行程序；缺失时禁用启动并提供安装说明、官方指南及手动刷新。检测使用缓存，重绘不重复查找、不运行版本命令；启动前使用相同解析器再次验证，工具消失时保留表单。程序存在不代表 Agent 已登录或远程已授权。
- 能力页命令入口已按业务上下文路由：登录/注销和账户命令先进入远程上下文，`fclaude attach` 与旧版 `agent-remote attach` 进入远程会话清单，恢复/默认账户/凭据/同步/Mutagen/WireGuard/设备/Skills 命令进入对应专用页面；远程清单加载期间保留待处理路由，失败时清除路由，不显示未绑定上下文的可提交表单。没有专用页面的命令仍使用 schema 驱动受管终端并按平台与能力状态禁用或提示。
- 新建/恢复弹窗采用可滚动正文与固定操作栏；针对中英文及伪本地化验证 900×600 布局。实际 Agent 授权、完整 TUI 和另外两平台原生行为仍待验收。

## 密码登录与设备登记

- 原生登录页支持服务器、用户名、密码、可选 MFA、设备名称、公钥文件选择及可选 WireGuard 公钥；也可仅登录用户而不发起设备登记。检查页显示服务器、账户和设备作用范围，确认后才调用协议。与普通 CLI 共用 `auth::login::finalize`，同服务器登记复用现有设备。
- 密码/MFA 仅作为受控 stdin 的临时请求数据；不进入命令参数、桌面数据库或日志。提交、离开页面时清除表单凭据；Go 请求调试格式也隐藏敏感字段。CLI 返回操作 ID 和非敏感结果，不向 desktop 返回 access/refresh token。
- 共用配置通过临时文件原子替换；桌面确认绑定原始服务器/设备上下文，发布时再次检查，避免覆盖并发切换。登录发布、注销、设备令牌读取/续期、撤销及轮换共用进程锁，用户令牌仍沿用现有独立锁。跨服务器用户登录不会保留旧服务器的设备；同服务器用户模式保留既有设备选择以兼容 CLI。
- 区分认证拒绝、设备登记拒绝、结果未知、本地保存不完整及配置已变化。超时/取消本地等待不等于回滚，禁止自动重放；退出应用期间若登录尚未完成，会在退出确认中说明该语义。
- 登录、注销期间允许切换工作台页面，侧栏及命令搜索提供返回操作状态的入口；重返页面保留进度、错误和回执，不重新执行操作。完成结果仅在操作页明确返回远程管理后清理；操作中心跨重启恢复安全生命周期摘要，精确 request key、回执和上下文仍由 CLI journal 查询。
- 公钥输入有大小、单行格式和类型检查；私钥及非普通文件拒绝，FIFO 不会等待写端。初始设备登记缺少服务端幂等回执恢复仍是待办，不能据此宣称完整初始化或所有登录方式已完成。

## 浏览器设备码授权

- 登录页可选择密码或浏览器授权；确认服务器、配置目录及设备登记参数后，CLI 请求设备码并发出最多一条审批进度，展示授权地址、用户验证码和剩余有效期。打开浏览器与复制验证码均由用户点击，CLI 内部 device_code 和 access/refresh token 不返回 desktop。
- CLI 仅对明确 pending 响应继续轮询；拒绝、过期、未知结果与配置变化分别处理，断网不自动重放不确定的 completion。批准后复用共享登录发布逻辑。进度有效期扣除请求耗时，Go worker 记录绝对截止时间，隐藏窗口不延长验证码寿命。
- Go 流式适配器验证请求/操作 ID、事件顺序、大小、终态及退出码；终态后有界等待 EOF，取消同时关闭读取 pipe，防止子进程继承 stdout 导致等待无法结束。
- 授权 URL 对内部 device_code 的多层百分号编码泄露有回归检查。共享 `PreparedRegistration` 在认证前冻结已验证公钥内容，授权等待期间修改原公钥文件不会改变实际登记内容。设备登记现由 CLI 保存原始 UUID 和公开意图，Server 按同 key/同意图返回加密保存的原始回执，传输丢失时只查询原 key；本地发布全部成功后才清除 key。设备码 completion 也保存调用方 key，CLI 在 transport/5xx/超时后只调用精确 completion GET；找不到、过期、撤销或损坏回执保持 unknown。真实账户授权及 Windows/Linux 原生浏览器路径仍待验收。

## 环境诊断

- 侧栏“环境诊断”通过 CLI 结构化查询显示配置、路径、SQLite、凭据及依赖状态。页面首次进入执行被动检查；用户点击运行检查后检查实际 SQLite 可读性、凭据存在/绑定及可选服务端 healthz，不向 desktop 返回凭据。
- 深检使用诊断专用凭据读取，不改变普通登录的 fallback 行为；区分系统库访问失败和凭据缺失。文件限制 64 KiB 并拒绝 FIFO/非普通文件，系统查询工具有时间与输出上限，取消/超时清理子进程；合法 fallback 仍可报告凭据存在，但不表示服务端授权有效。
- 显式版本检查运行真实依赖的 `--version`，区分文件存在、执行权限、命令退出、超时、输出超限与未经检查。依赖使用受控 argv，无 shell 插值；逐组件与总时间、输出量有上限。版本探测不等于版本符合要求或远程可用。
- 页面包含状态、路径、版本输出、许可证及建议；诊断本身不执行修复；独立确认的本地初始化/元数据修复及组合初始化向导已接入，依赖安装和实际系统凭据库行为需按发布平台验收。
- 依赖管理页从环境诊断和能力页命令覆盖进入：`deps.inspect` 结构化页显示 CLI 平台、home、manifest 来源及每个依赖的状态、版本与许可证，版本探测可显式开启并可取消；`deps.repair` 只有在用户明确点击修复并绑定准确 home 后才执行。兼容的 `dependencies.inspect` 旧只读页不提供安装按钮，也不把 `credential: none` 误报为管理员权限或安装成功；Windows/Linux/macOS 原生依赖行为仍需平台验收。
- Unix 版本探测使用独立进程组，取消时清理后代；Windows 使用暂停启动、加入关闭即清理的 Job Object 后恢复执行。Windows 模块独立交叉检查已通过，但完整 Rust CLI 的 Windows 交叉检查因本机缺少 MSVC C 头文件受阻；这不是 Windows 运行验收。

## 本地初始化与元数据修复

- 环境诊断及命令搜索提供独立确认页，核对 CLI home 和修改范围。预检只读取能力声明；提交时再次验证 CLI 协议、平台和 `setup.local/write`。旧 CLI 不会执行不支持的变更，确认前不初始化 CLI 数据。
- `setup.local` 不读取服务器配置或凭据，不访问网络、不执行依赖程序；固定报告 home、bin、dependencies、secrets、wireguard、ssh、数据库和 manifest 八步。步骤状态为已就绪、已修改、失败或因前置失败跳过，部分失败仍保留独立成功步骤，不声称整体回滚。
- Rust 的普通 `init` 和 `doctor --fix` 复用同一本地准备服务；`deps status --fix` 仅准备其依赖元数据。普通 CLI 随后的既有检查/交互行为仍独立于 desktop 的无子进程修复入口。
- 数据库初始化在单一写事务内检查版本、完整性和受用表结构，成功后提交；拒绝未来版本、损坏或不兼容结构，并保留原记录。依赖 manifest 通过临时文件同步后原子发布，竞争时保留已发布文件；已有清单先有界验证，不覆盖无效内容。受管目标拒绝链接及非预期文件类型，系统祖先路径别名仍可使用。Unix 新元数据文件使用 0600、新目录使用 0700；Windows 元数据继承父目录 ACL，不通过外部权限工具修改既有权限。
- Go 校验操作 ID、准确 home、八步顺序/路径、状态和完成标记；损坏或丢失回执报告结果未知，不自动重试。工作跨页继续，侧栏/命令搜索可返回；只有明确返回诊断才清理回执。结束后失效旧诊断与能力快照，迟到查询按代次丢弃。
- 停止等待/退出不回滚已完成步骤；结果未知时先刷新诊断再决定是否重新提交。单独的本地修复操作不安装依赖、不登录、不配置/启用隧道；组合向导另行按固定顺序编排登录、可选配置和最终 readiness。操作中心跨重启仅恢复安全生命周期摘要，精确回执和上下文仍由 CLI journal 管理。

## WireGuard 配置预览与发布

- 环境诊断与命令搜索提供默认/自定义输出位置。`wireguard.config.plan` 只读有界配置和文件元数据，不读取凭据、联网或写文件；计划包含 home、服务器/设备、解析后的准确输出路径、新建/覆盖及目录/目标身份。用户修改输入后必须重新预览。
- 确认后通过 `wireguard.config.apply` 提交完整计划；Go 在异步能力检查前深拷贝确认参数，校验协议、平台、read/write、凭据与 exact_request 声明。普通 CLI 和 `init` 的可选 WireGuard 步骤调用同一服务。桌面不打开 CLI 配置、数据库或私钥文件。
- 严格凭据读取区分确定缺失和不可访问；拒绝/超时/无效内容不能触发替换私钥。显式 file backend 仅用文件；系统模式不会用 fallback 掩盖失败。私钥原生保存在 macOS/Windows 系统凭据库，Linux 凭据工具有界运行；凭据和含私钥的配置正文不进入桌面协议、命令参数或日志。
- 写操作按登录锁、用户级服务器/设备凭据锁、输出目录锁排序；跨 home 的 WireGuard 密钥生成/登记共用用户级锁。`AGENT_REMOTE_WIREGUARD_LOCK_ROOT` 可为隔离环境指定共享锁目录，同一用户的协作 home 必须保持一致；这不修复既有登录/注销跨 home 凭据锁的所有限制。
- 发布前核对路径身份，使用目录 handle、私有暂存和原子发布；Unix 配置为 0600，Windows 在写秘密前设置当前用户与 LocalSystem 权限。服务器字段有大小/格式检查，节点名称不直接拼入配置注释；只登记公钥，不启动隧道。
- 回执分别保留密钥、登记及文件阶段；密钥已保存后发生上下文或输出变化也保留事实。丢失/损坏回执、超时或取消不表示回滚、不自动重试。跨页结果保留至明确返回诊断；操作中心跨重启仅恢复安全生命周期摘要，精确回执和上下文仍由 CLI journal 管理。原生凭据 API 的内部调用无法强制截止，整体进程终止后的效果可能未知；Linux 工具清理已有隔离运行证据，真实系统凭据交互仍待逐平台验收。
- 配置发布不启动隧道；检查、状态与权限桥只读观察见下一节。组合初始化向导已接入并保留独立步骤回执；操作中心跨重启仅恢复安全生命周期摘要，精确回执由 CLI journal 管理。详细边界见 [WireGuard 配置契约与实现说明](wireguard-configuration-plan.md)。

## WireGuard 配置检查与运行状态

- 独立网络页通过环境诊断和命令搜索进入，页面进入不调用 CLI；显式检查默认/自选配置或读取状态，无需服务器、登录和设备登记。每个查询分别管理取消、跨页结果和迟到响应；配置发布开始时取消在途查询并失效旧结果，发布期间禁用查询，完成后由用户显式重查。
- `wireguard.check` 有界读取私有文件，拒绝链接、重解析点、硬链接、FIFO、目录、宽权限、无效及超限内容。只返回地址、DNS、MTU、peer 数量和固定不支持指令名；不返回 PrivateKey、PSK 或 hooks 值、不执行 hooks。返回路径与用户选择的词法路径相同。
- `wireguard.status` 仅执行一次 `wg show all dump`，10 秒预算、1 MiB stdout/16 KiB stderr 上限，在 Rust 边界剔除私钥/PSK，严格解析接口、peer、握手和完整 uint64 计数。超时/取消/输出过大收回工具进程树，不提权、不调用网络或凭据。空接口成功、工具缺失、查询失败和无效输出分别呈现。
- 当前进程 elevated/standard/unknown 仅为观察，不能证明 CAP_NET_ADMIN 或接口授权；不使用英文 stderr 判断权限。工具覆盖路径无效时不回退到其他工具。Windows 配置允许 LocalSystem 只读 ACL，不放宽一般凭据文件验证。
- Go 适配器校验 read/credential:none、平台、home、精确路径、必需字段、可空字段、整数范围及数量上限；无自动重试。UI 提供接口/peer 分页、官方 mygo 数字与本地时间格式，中英文文案和测试均按全项目规则维护。
- 普通 CLI/helper 的 check 共享新服务并修复 `--dry-run`；普通人类 status 保留原有 `wg show`、sudo/UAC 行为，不能把新的只读状态契约套用于它。详细边界见[检查与状态契约](wireguard-inspection-contract.md)。桌面启停现经三平台受保护 broker client：macOS LaunchDaemon、Linux system D-Bus/Polkit 服务、Windows LocalSystem SCM 服务；三平台真实安装、授权、隧道存活和恢复验收仍待完成。

## WireGuard 隧道只读计划与目标观察

`wireguard.bridge.status` 已接入网络页，严格显示安装、系统批准、协议、调用方认证和执行就绪状态；当前 CLI 在没有受保护 broker service 时返回 `unsupported`/`unavailable`，不会从进程 elevation 或工具存在推断授权。Rust CLI 现在提供三平台 broker client：macOS 固定 LaunchDaemon Unix socket 与社区自签身份观察，Linux system D-Bus/Polkit unique sender 与 UID/PID/start-time 绑定，Windows 本机 named pipe、远程拒绝、进程/会话/token SID 观察；请求还绑定配置字节、content/plan/target digest 和一次性 execution permit。对应服务安装材料已加入仓库：macOS 固定 LaunchDaemon plist、Linux systemd/D-Bus/Polkit 和 root 安装脚本、Windows SCM LocalSystem 服务与 `-InstallBroker` 安装路径。它们在服务缺失、身份不匹配、协议错误或 permit 缺失时均 fail-closed，不宣称 broker 已安装或已授权。`wireguard.tunnel.apply` 会重新生成并比较完整只读计划，在普通用户权限下读取配置后才向平台 broker 发送字节和摘要；只有 broker receipt 才能继续生成 observed receipt，没有 sudo、runas 或普通 helper fallback。三平台真实主机安装、系统授权、执行、隧道存活及恢复仍待验收；请求绑定与目标锁独立于 CLI home，不能自动重试。

- Desktop 还将这组独立观察投影为本地生命周期门槛：安装、系统批准、协议兼容、调用方认证和执行授权按固定顺序逐项阻断。只有全部事实满足且状态为 `ready` 时 `CanExecute()` 才为真；未知、中间或 `unsupported` 状态不会被 UI 当作可执行。该投影不改变 CLI 协议、不授予权限，也不代替三平台原生 broker 验收。

- 网络页可选择启动/停止方向及默认/自选配置，显式生成 `wireguard.tunnel.plan`。计划无 issues 时显示确认执行按钮；Go 适配器冻结完整计划并校验 `wireguard.tunnel.apply` 回执。执行结果保留 broker operation、观察和未知状态，取消或上下文变化不会重放；三平台原生权限、broker 安装后的真实隧道行为和跨重启恢复仍待验收，代码存在不等于正式版 up/down 覆盖完成。
- 隧道 apply 的外层 desktop request ID 与受保护 broker 的 digest-bound operation ID 是两个不同身份：cliadapter 只对该方法放行非空 broker 回执，Go 仍要求其为 64 位小写摘要；其它 mutation 继续要求回执回显外层 request ID。测试夹具使用同一形状的 broker 摘要，不能替代真实 broker 回执。
- 同一安全文件 handle 解析配置、推导公钥和计算最多 1 MiB 内容摘要，前后复验身份与权限；工具采用最多 64 MiB 快照及身份复验。私钥、PSK、hooks 值和配置正文不进入 Go。工具哈希不构成可信安装或提权资格。
- Linux 通过原生 netlink 绑定当前 namespace、接口名、ifindex 和 WireGuard 类型；macOS 验证固定运行目录中的 `.name` → utun 映射、socket 和接口索引；Windows 通过 SCM 查询准确服务、程序/配置路径、配置身份、账户、启动类型与服务状态。权限或查询失败为 unknown，不推断不存在；macOS 映射缺失也为 unknown，停止的 Windows 服务仍为 present。
- 原生前快照、wg 查询及后快照固定同一 OS 线程，保证 Linux namespace 一致；变化使观察失效。公钥只从对应接口提取。快照不证明目标归属或运行代次，未来执行桥必须另行复验并记录。原生 SCM 等调用不能强制取消，外层 CLI 进程提供最终生命周期边界。
- Go 冻结输入并校验完整结构、平台/home/动作/路径及固定 issues；修改配置或动作使旧计划失效，跨页保留结果，取消后丢弃迟到响应，配置发布取消在途预览并使旧结果失效。已确认的 tunnel apply 使用独立写操作取消边界：取消会保留 `wireguard_outcome_unknown`、使 reviewed plan 失效并禁止盲重试，即使 CLI/原生 helper 的回执尚未抵达；只有重新预览后才能继续。文案使用官方中英文 catalog，所有测试和夹具在根 tests/。详见[只读计划契约](wireguard-tunnel-contract.md)。

## WireGuard 执行基础（内部，受保护 broker client 已接入）

- `ExecutionConfig` 在调用者权限下单次读取准确快照，并在传输接收侧复验大小、语法、摘要和公钥；拒绝 hooks、SaveConfig、Table。秘密缓冲及解析临时值使用 Zeroizing，不实现 Debug/Serialize/Clone，不进入 Go 或操作记录。
- 系统保护目录按持有 handle 验证祖先、文件身份和私有权限。macOS 额外拒绝扩展 ACL；Windows 采用原生相对句柄创建及创建时 SYSTEM/Administrators ACL。生产路径不接受测试权限，测试临时根仅通过 cfg(test) 显式建立。
- 每个请求保留原逻辑名的配置副本及非秘密记录；先锁请求 ID，再锁不依赖 CLI home/调用者的目标。Windows/macOS 的逻辑名在锁键中按 ASCII 小写规范化，避免大小写别名绕过互斥，确认名称和配置基名保持原样。
- prepared → submitted → observed/failed 为单向状态；已有 ID 的任何阶段均只读，残缺记录拒绝重建，drop 不删除服务配置或记录。submitted 和 failed 不证明已回滚，observed 不自动代表请求成功。未来 IPC 仍须认证调用者并独立计算计划和目标作用域，当前哈希只是内部绑定字段。
- 文件同步与目录同步分开记录；Windows 明确返回目录同步 Unsupported，不能视为完整持久提交或执行许可。每个 request 现在同时维护 canonical、连续序号、哈希链校验的 `submission.log`；追加事件会在记录发布前 flush 文件及目录，缺失、截断、空行、乱序、重复或篡改均 fail closed，`ExecutionStore::submitted_records` 仍只读且不创建 lease、不执行 broker、不重放请求。该 journal 是恢复证据，不是 broker 授权；三平台认证 IPC、原生 durable flush 及跨平台真实启停与恢复验收仍待完成。
- `execute_reviewed` 在用户权限下读取并验证配置，再将配置字节、绑定、动作、plan digest 和 target scope digest 发送给平台 broker client；平台返回的 operation ID 与二次观察才可形成执行回执。服务缺失、caller identity 不匹配、permit/replay 失败或协议错误均固定失败，不回退到 sudo、runas、PATH 探测或普通 helper。服务实现和安装材料已落盘；真实主机安装、系统授权和三平台原生验收仍待完成。

本切片专项证据：macOS 同仓执行基础测试 23/23 通过（10.22s），Linux aarch64 原生隔离容器执行基础 23/23、保护目录 9/9 通过（10.37s / 0.01s）。包含真实跨进程锁竞争、进程在 prepared/submitted 后直接退出并重开记录、持久化边界故障注入、目录/配置替换及同 ID 不重放。进程退出和注入 I/O 错误不是磁盘掉电验证。macOS 独立保护目录 10/10 通过，包含 0700/0600 下 ACL 额外授权拒绝；最终 CLI 整仓门禁也验证了同一模块（lib/bin 各 10 项）。Windows 原始保护目录模块与测试只通过独立交叉 Clippy，未作原生运行。

执行基础最终整仓验证：CLI 原脚本完整门禁退出码 0，1014 项通过、3 项跳过、46 个测试二进制，nextest 78.962s，Clippy 20.03s，全流程约 147s；日志 `/tmp/agent-remote-execution-cli-gate.log`。desktop 以最新 CLI 和 PTY 开关运行 `bash scripts/check.sh` 再次通过，官方 catalog 366 条，未变化的 Go 包使用测试缓存；另以 `DESKTOP_CLI_BINARY=... go test ./tests/cliadapter -run WireGuard -count=1` 对最新 Rust 二进制强制回归，9.811s 通过。日志 `/tmp/agent-remote-execution-desktop-gate.log`、`/tmp/agent-remote-execution-desktop-interop.log`。本轮没有修改 Go 界面或终端实现，因此不把此前 race/交叉构建写成本轮重新运行。

本切片失败记录：首次临时测试根权限不符合私有策略，修复测试专用初始化以在已验证的当前用户句柄上收紧权限，生产规则未放宽。macOS ACL 修正过程中构建的 execution_ 测试 13 通过、10 失败，原因是 Darwin 无扩展 ACL 的 fd 返回 NULL/ENOENT；仅对有效且未解除链接的 fd 识别该返回，其他错误保持拒绝，最新同源 23/23 复验通过。复审另修复了锁对象身份读取同时失败可能比较为相等的问题，并补齐了原生观察的状态/原因/公钥/平台身份一致性校验。

## 注销与部分结果

- 原生注销页显示所选 CLI home、服务器、设备及远端撤销选项，确认前不执行。默认撤销用户及当前设备凭据，也可仅离线清理本地凭据；当前服务器地址保留，不删除 SSH/WireGuard 私钥、项目或其他服务器的凭据。OS 凭据库键按服务器/设备作用域共用，多个 CLI home 可引用同一凭据；home 目录不构成系统凭据隔离，跨 home 的 keychain 并发锁仍是现有 CLI 限制。
- 与普通 CLI 共用 `auth::logout::execute`，使用登录/设备续期锁，操作前检查确认上下文，配置写入再次比较。仅本地模式不调用刷新/会话交换/撤销 API。注销不发送远端停止请求，不终止本地 Agent；现有连接可能失去授权。
- 回执分开报告本地用户/设备/Broker 凭据、刷新元数据、配置清理及远端用户/设备撤销。系统凭据库删除失败不能被文件清理成功掩盖。远端失败后仍尝试本地清理；设备清理失败时保留当前设备选择，便于后续显式修复；普通 CLI 在本地清理不完整时返回错误。
- Go 校验操作 ID、服务器/设备、所有必需布尔结果和撤销状态。取消、超时或损坏回执显示结果未知，不声称回滚、不自动重放。结果页提供刷新共用状态入口；尚无持久注销日志或丢失回执恢复。

## 阶段与下一步

### 会话创建 caller-key 切片（已实现，仍待真实三平台/部署验收）

Server 已加入 `session_create_requests` 持久 ledger（迁移 `0058_session_create_requests`），并在
现有 `POST /api/v1/sessions` 上支持可选 v1 caller key；同一用户同一 key 的相同意图重放原始受理
凭据，异意图固定冲突。owner-scoped `GET /api/v1/sessions/requests/{request_id}` 在会话删除后仍
返回不可变 receipt 和 `session_deleted` 观察，迁移/接管尚未创建会话的阶段也绑定同一 key。

Rust CLI 生成规范 UUID v4，使用只保存上下文、摘要、阶段及公开任务身份的 SQLite journal；创建
响应不确定时只查询原 key，不自动换 key 或猜测 latest/default session。desktop 已接入
`sessions.create` 与 `sessions.create.request`，并在 CLI 返回原 request UUID 后持久化只含
home/Server/device/query key 的恢复指针；重启后显式打开“恢复远程创建请求”入口，冻结当前
配置上下文后只能查询原 key，不提交新的创建请求。desktop 仅保存 Go 工作台所需的公开上下文并展示
`pending`、`unknown`、`conflict`、`deleted` 和 Node mismatch。三仓定向测试、格式/静态检查和
desktop 门禁通过；CLI 完整门禁中两个与本机并行/系统调度有关的既有长时测试首轮超时，单独重跑均
通过（详见验证日志），因此该证据仍不替代真实 Server、Node、SSH/tmux 及 Windows/Linux 原生验收。

能力页的 `agent-remote init` 入口现路由到完整组合初始化向导，避免通过通用 schema 表单绕过固定步骤；覆盖验证见 `tests/app/capabilities_test.go`。

| 阶段 | 当前证据与剩余交付 |
| --- | --- |
| P0 可行性与契约 | macOS 原生窗口/PTY 验证通过，Windows/Linux 交叉构建通过；已有 CLI 命令发现、能力说明、部分逐命令平台声明、i18n 和测试门禁。仍需另外两平台实际运行、完整支持矩阵及机器操作协议 |
| P1 工作台 | 项目、本地进程、命名会话、多标签、分屏、隐藏/唤回、布局及历史恢复、标签拖动、导航快捷键、命令搜索、工具缺失提示/安装指引、主题与语言切换已有实现和针对性测试；macOS `native-probe` 现在还验证真实窗口 resize 传到 PTY cell size。受管 PTY 现在捕获 OSC 标题作为不持久化的会话临时状态，用户命名仍优先显示。仍需三平台真实 Agent TUI/历史恢复/附件/输入法验收及后续体验收敛 |
| P2 远程基本闭环 | 首批结构化查询、共用配置、账户/会话列表、账户创建/状态/绑定/验证/停用及准确绑定终端、配置导入与原请求恢复、管理员运行时恢复、共享默认账户查询/设置/清除、确认连接、密码/MFA 与设备码登录、可选设备登记、注销、环境诊断、本地元数据修复、WireGuard 配置预览/发布、网络检查/状态、只读权限桥/隧道计划和端口转发基础 UI/协议、统一操作中心安全状态聚合已有实现。完整初始化组合向导已接入“本地初始化 → 密码或设备码登录/设备登记 → 可选 WireGuard 计划与二次确认 → 在线诊断 → 账户/会话 readiness”顺序，所有步骤复用现有 desktop-control 且保留独立错误；端口转发已有真实 loopback listener、TTL、bounded reconnect 和精确停止的 Go→Rust 编排及合同测试；真实服务端联合网络、同步三平台原生验收和外部服务注册迁移仍待完成，操作中心仅恢复安全生命周期摘要，精确回执继续由 CLI journal 提供 |
| P3 全量 CLI 覆盖 | 已完成 schema 驱动的叶子命令通用表单：覆盖当前清单中的 agent-remote 106 个命令节点和 fclaude 12 个命令节点，支持 defaults、required/min/max、choices、SetTrue/SetFalse/Count、Append、positional 及 Review 后冻结 argv；平台/能力状态按 unknown、unsupported、unavailable、denied 等显示。包含 password/TOTP 的 `agent-remote login` 已从命令清单路由到凭据安全登录页，不再被敏感字段门禁误显示为不可达。检测到 password、token、secret、TOTP、设备码或私钥字段时改由专用安全流程承载，不进入通用 argv。开发者凭据、Device `status/revoke/rotate-token`、Skills 启用/停用与只读状态冲突摘要，以及 `ssh check`、`doctor`、`node install`、Skills `add/update/enable/disable/pin/unpin/inherit/remove/rollback` 及 ego-browser 本地安装/注册/设备维护终端流程均已完成精确上下文、能力/平台门禁、显式确认和 Rust 边界错误语义；ego-browser status/list/requests/cancel/claim/binding 生命周期/delete 生命周期则由结构化页面与 versioned desktop-control 合同承载；`doctor --fix` 仍明确要求用户审阅完整 argv，且不自动添加或重试修复参数。Device 令牌永不进入 desktop，远端已成功但本地清理不完整会显式投影局部状态，未知结果保留原操作语义。操作中心现可跨重启恢复同步与 Mutagen 的安全阶段摘要；专用终端精确回执恢复、三平台原生运行和其余服务端联合验收仍待完成，见 CLI 覆盖文档 |
| P4 视觉与发行 | 已有深浅原生配色、可切换的樱幕主题、矢量角色立绘、侧栏装饰、工作台低透明度背景光斑、统一 180ms 过渡预算和终端独立可读配色。工具栏主题快捷按钮按 `midnight → sakura → daybreak → midnight` 轮换，设置页仍提供逐项选择；终端显式设置光标、选择区和选中文字颜色，保持二次元外壳与长时间终端可读性分离。背景装饰只在普通工作台显示，采用一次性 180ms 进入动效；专注模式会隐藏侧栏和全部装饰，减少动效时保留静态低透明度装饰，终端面板保持不透明。工作台同时遵循 mygo 原生 `ReduceMotion`（macOS Reduce Motion、Windows 动画设置、Linux 桌面设置），设置页提供持久化“减少动效”开关；减少动效会跳过装饰性进入/位移过渡并把颜色动画设为零时长。`internal/theme.FrameObserver` 为 Go/mygo 视图构建记录 16ms/60Hz 目标内的帧数、超预算帧数、总时长和最大值；`App.MotionStats` 提供原生探针读取，`tests/theme/motion_test.go` 与 `tests/theme/background_test.go` 验证动效预算、减少动效、专注模式和可读性边界，`tests/theme/theme_test.go` 验证主题轮换及终端交互色完整性。三平台社区构建脚本、依赖清单、SHA-256 校验以及 macOS 项目社区自签验证已完成。安装 helper 现提供只读 `--status`，报告受管版本与回滚树可用性，并拒绝将未受管或中断状态误判为空安装；安装、升级、回滚的跨平台原子替换单测和本机 installer probe 通过。Windows/Linux 原生运行、运行中锁定文件和三平台安装路径验证仍未完成；本地策略测试不替代三平台原生验收。 |

P1 原生终端 resize 边界：`terminalhost.Host` 现在公开带正尺寸校验的 `Resize`/`Size`，与 mygo terminal view 使用的同一原生 PTY/ConPTY 句柄。`tests/terminalhost/host_test.go` 在真实终端测试开关下验证初始 100×30、调整到 132×41 及拒绝零列，覆盖当前主机的 Unix PTY；Windows ConPTY 需在 Windows 原生探针运行同一测试。该 Go 边界不替代目标平台的窗口拖动、IME、全屏 TUI 和实际 Agent 验收。

P1 终端临时状态：`terminalhost.Host` 通过 mygo terminal 的 `OnTitle` 捕获 Agent/TUI 发出的 OSC 标题，并限制为有界、清理控制字符的 UTF-8 文本。工作台面板标题只在当前进程存活期间显示该状态；用户设置的会话名称仍固定在首行，OSC 内容不会进入 desktop SQLite、恢复参数或身份判断。`tests/terminalhost/host_test.go` 在 `DESKTOP_TERMINAL_TESTS=1` 下使用 macOS 当前主机的真实 Unix PTY 验证中文与 emoji 标题；Windows/Linux 的原生标题与输入法仍需各自主机验收。

远程会话管理的下一切片审计与实施顺序见[会话管理草案](remote-session-management-plan.md)：原创建键、准确停止、固定集合删除和端口转发 caller key 已接入；删除回执恢复现已跨桌面重启保存原 key、认证上下文、排序会话 UUID 及两个摘要，仅支持显式 query-only 查询并拒绝集合漂移；桌面端现已通过受管 Rust `forward --json` 子进程持有真实 loopback listener，继续补充真实 Server/Node/三平台联合门禁和正式版运行证据。启动中停止仍需 Node 生命周期保证，Server 状态比较不能替代物理终止证明；准确停止仍需跨 Server/Node/CLI 联合门禁后才能计入正式版验收。

Exact session create/stop/delete 的 Go 预检现按 Rust desktop-control 相同规则要求规范、非零 UUID：session、node、account、workspace 以及 caller-key request ID 的大写、花括号、短 ID 和全零值都会在 CLI 调用前拒绝。该边界由 `tests/remote/session_exact_identity_test.go` 覆盖；设备、Server 授权和真实 Node 运行时身份仍由 Rust/Server/Node 再次核验，合同测试不替代远程实网证据。

端口转发 create/stop/reconnect 与受管 `forward --json` 启动事件的身份校验也已收紧：session、forward、node 和 caller request ID 必须为规范非零 UUID；不规范节点或会话不会进入 CLI，也不会被解析为本地 listener 的目标。loopback 地址、端口绑定及子进程生命周期继续独立 fail closed，真实 Server/Node 隧道和权限仍待目标环境验收。

`StartForward` 现在在 `exec.Cmd.Start` 前检查 forward/session/node/caller UUID、client instance、runtime backend 与 TTL；malformed launch identity 不会启动 helper。恢复启动即使携带 session/节点用于事件匹配，也只使用完整 forward UUID 作为 Rust reconnect 目标，不会隐式创建新 forward；回归在 `tests/remote/port_forward_runner_test.go`。

继续按原计划补齐 P0/P1 缺口，再推进 P2/P3 的专用工作流和 P4 发行验收；通用表单入口不替代逐命令业务确认、恢复和三平台测试。首个正式版仍以全量 CLI 操作和三平台验收为门槛。未提交、推送或发布。

本地初始化/元数据修复及完整初始化组合向导已作为 P2 切片接入。向导先调用 `setup.local`，再冻结 CLI 返回的 home/server/device 上下文，提交 `login.password` 或 `login.device_code`；设备登记由登录请求的 `skip_device_registration` 和公钥字段控制，凭据始终留在 CLI。启用 WireGuard 时，登录完成后先调用只读 `wireguard.config.plan`，用户确认完整计划后才调用 `wireguard.config.apply`；最后执行在线/版本/深度诊断并调用只读账户/会话列表，分别展示 query error 与 readiness，不把失败列表当空成功。停止等待不会回滚或自动重放已完成/未知步骤。操作中心跨重启仅保存非敏感生命周期摘要与阶段；密码、验证码、token、私钥、精确 request key 和完整回执仍由 CLI journal 管理。应用重启不会自动重放操作。[WireGuard 配置方案](wireguard-configuration-plan.md)已落为共享服务与确认界面；真实服务端幂等回执、依赖安装、三平台权限桥和原生验收仍按边界推进。

远端结果恢复仍有一个明确边界：completion GET 只接受原始调用方 key；服务端回执缺失、过期、撤销或解密校验失败时必须保留 unknown。WireGuard enroll 已接入 `peer_id`、公开 `peer_version` 与公钥 compare-and-swap：配置读取返回可校验的 peer identity，已有 peer 的登记冻结 `expected_peer_id/expected_public_key/expected_peer_version`，漂移返回 `WIREGUARD_PEER_CHANGED`，成功回执带 device/peer/version，私钥始终留在 CLI。设备登记的服务端幂等键、原始回执加密保存、同意图重取和异意图冲突已接入，但仍需真实部署迁移与三平台联调验收。现阶段不得以本地操作日志猜测 device-code 或 WireGuard 结果，也不能自动换 key 重试。

## 验证边界

默认账户切片最终验证：CLI 原有 `scripts/run-quality-checks.sh` 在隔离 target 通过，1026 项通过、3 项跳过、46 个测试目标，nextest 77.011s，含 12 项新协议合同；格式、Clippy、脚本及 diff 检查通过。新增合同覆盖普通 CLI 与 desktop 双向共享、明确确认、SQLite 比较更新、HTTP 等待期间配置/旧值变化、竞争清除、超大无效元数据脱敏和 SIGTERM 中断后的锁释放。日志 `/tmp/agent-remote-defaults-cli-gate.log`。

修正取消转发后，desktop 使用最新真实 Rust CLI、开启 PTY 的最终整仓门禁通过，391 条官方中英文消息、生成代码同步、格式、vet、全部测试及目录规则通过；app 40.164s、cliadapter 28.677s，未变化包使用缓存。独立真实 Go→Rust 默认账户联调 `-count=1` 为 1.834s，验证 GUI 设置后普通 CLI 可见、普通 CLI 前缀设置/清除后 GUI 可见，以及陈旧确认、工具不匹配和无确认无网络。日志 `/tmp/agent-remote-defaults-desktop-final.log`、`/tmp/agent-remote-defaults-interop.log`。

默认账户定向 race：UI 58.866s、适配器 104.969s 通过。最终取消修正另有 Unix 实际 SIGTERM 证据测试，三个方法均在收到信号后写清理标记并退出；专项 race 8.203s 通过，日志 `/tmp/agent-remote-default-sigterm-race.log`。这验证信号转发及有界清理机会，不证明不可中断的原生凭据 API 已回滚。Rust CLI 的原生 SIGTERM 合同另行验证服务等待被取消且不发布默认值；未在真实 Linux Secret Service 下运行本轮账户流程。

本轮失败与修正：Rust 首次扩展合同发现测试 HTTP 账户列表返回了错误 envelope（数组而非 `data.items`），修正夹具后 12 项通过；测试服务器的 Clippy 条件写法亦已修正。首次 Go race 中 500ms 取消早于能力检查完成，断言误认为 mutation 已提交；改为等待实际提交屏障后取消，保留生产期限。交叉审查发现默认账户方法遗漏 Unix 优雅取消配置，已加入现有 SIGTERM/超时终止流程，并通过上述专项和最终整仓门禁；没有以忽略失败或放宽结果语义通过检查。

最终 Windows/Linux amd64 `CGO_ENABLED=0 go build` 再次通过，仍不构成原生运行验收。默认账户列表及确认页的英文、中文、en-XA 共六张 900×600 截图已渲染；中英文/伪本地化确认页及伪本地化列表已检查，正文滚动、底部确认操作可见。截图位于 `/tmp/agent-remote-default-ui/`。新增测试、信号辅助程序及夹具均位于根 `tests/`；没有新建真实账户、修改真实服务器状态或发起提权。


只读隧道计划切片的聚焦证据：macOS WireGuard 库测试 33/33 通过，包含原生目标模块 8 项；相关机器合同及聚焦 Clippy 通过。Linux aarch64 原生隔离容器通过 target 单元 8/8、隧道计划合同 4/4，覆盖实际只读 netlink 查询、权限错误不作 absence、配置/工具快照及脱敏。macOS utun 映射测试使用临时映射和 Unix socket；没有创建真实接口。Windows SCM 原始模块和测试通过独立类型检查，未作 Windows 原生运行。

该切片专项的已知问题记录：首轮 macOS 编译发现 cap metadata 扩展 trait 缺失/歧义，修正后 target 8/8 通过；Windows 隔离入口暴露子模块路径解析差异，补显式路径后类型检查通过。Linux 首条复合命令的单元 8 项通过后，因合同 target 名误写而退出，改用实际 `desktop_wireguard_tunnel_contract` 后 4 项通过；其后追加 Clippy 未执行，因为固定镜像未安装 cargo-clippy。此项不记为 Linux Clippy 通过；macOS 聚焦 Clippy 证据独立保留。最终整仓门禁结果另行记录，不以聚焦测试替代。


本轮网络检查/状态切片：macOS CLI 原有默认 8 路完整门禁通过，924 项通过、0 失败、3 跳过，共 45 个测试目标，测试执行 58.851 秒，脚本退出码 0。仍使用隔离 Cargo target，未改变默认并发或测试集合。Linux aarch64 原生隔离容器另通过 43 项：WireGuard 单元 23、检查/状态机器合同 9、共享诊断回归 11；涵盖敏感字段过滤、显式工具覆盖、SIGTERM、超时及 leader 退出后的子进程清理。Windows 原始状态/工具/进程 Job/ACL 模块及测试独立 Clippy 类型检查通过，未作原生运行。

该轮失败与修正：第一次完整门禁被 `platform.rs` import 排序的 rustfmt 检查阻止，修正后重新执行完整脚本；新增超时测试曾在 300 ms 内尚未写入启动 PID，后将测试预算调整为 1 秒，保留 3 秒启动/清理上界，生产查询预算仍为 10 秒。macOS 无法创建测试用非 UTF-8 文件名，改为验证解析前拒绝此路径，以实际换行文件和链接覆盖真实路径约束。Go 测试夹具曾通过 float64 丢失 uint64 精度，已改为 UseNumber；真实联调中的宽泛“private”字符串断言误击 macOS `/private` 路径，已改为检查准确的夹具私钥、PSK 和原始错误文本。首次定向竞态检查还发现配置快照测试仅依赖外部文件屏障，未建立 Go 内存同步关系；现改用已观测到能力子进程启动后的环境快照锁交接，并增加实际请求捕获断言。该单项竞态复验已通过（4.913s），未使用 race 排除、延时规避或生产测试专用接口。上述失败不计为通过证据，最终验证使用修正后的代码。

WireGuard 切片已在 Linux aarch64 的隔离 `rust:1.90-bookworm` 容器运行专项：配置合同 15/15、严格凭据合同 4/4、严格读取/设备凭据单元测试 10/10、目录同步 3/3、WireGuard/身份锁相关单元测试 8/8，共 40 项通过。测试使用临时文件、loopback HTTP 和假 secret-tool，不触发真实隧道或用户凭据库；这不是 Linux amd64 桌面运行或实际 Secret Service 的验收。

该轮 Linux 验证首次暴露目录同步缺陷：capability 目录在 Linux 可使用 O_PATH，直接 sync_all 返回 EBADF，导致配置合同 7 项失败、严格凭据库测试 3 项失败。共享同步实现现通过已持有目录相对打开 `.` 取得可同步句柄，不重新按外部路径解析、不忽略同步错误；密钥存储和配置发布均复用。上述 40 项是修复后的独立复验；新增回归明确覆盖 O_PATH、目录改名/原路径被替换及无效句柄错误传播。

本轮还修复了 macOS 条件编译后的测试 needless_return，使全目标 Clippy 可通过。一次使用原有大构建目录的完整门禁停在 nextest 枚举、尚未开始测试，因源码已更新而主动结束（退出码 143），不能计为通过；随后改用隔离 CARGO_TARGET_DIR 执行原有门禁脚本。未放宽断言、期限或默认 8 路测试并发。WireGuard HTTP 夹具较早也曾因 macOS 接受的 socket 继承非阻塞模式发生 WouldBlock，已显式恢复阻塞读取并保留原超时。

前一轮能力说明及 Provider 改进后，`DESKTOP_CLI_BINARY=... DESKTOP_TERMINAL_TESTS=1 go test -race ./tests/app ./tests/capability ./tests/cliadapter ./tests/session` 全部通过（305.060s / 20.331s / 51.809s / 1.930s），无竞态报告。

该轮完整 `workbench-probe` 在 macOS 原生应用循环下通过启动、焦点、拖动、退出、历史恢复、准确远程连接和账户绑定复用；使用临时 Agent/CLI 与真实 PTY，未使用真实账户。Windows、Linux amd64 的 `CGO_ENABLED=0 go build` 再次通过，包含本轮能力及解析器改动；这仍不构成另外两平台的实际运行验收。

新增能力页、Provider 可用性与统一平台/契约校验后的 `DESKTOP_CLI_BINARY=... DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh` 通过：267 条官方中英文消息、生成文件同步、gofmt、vet、全部测试及目录规则检查通过。真实 CLI 被动发现未创建状态文件；平台不匹配/缺失、重复方法、非法 effect、旧 CLI 缺少写方法与操作结果返回路径均通过最新代码串行复验。

本地初始化/元数据修复及复审修改后的 `DESKTOP_CLI_BINARY=... DESKTOP_TERMINAL_TESTS=1 DESKTOP_RENDER_DIR=... bash scripts/check.sh` 最终复验通过（app 18.235s、cliadapter 10.048s，脚本退出码 0）：285 条官方中英文消息、生成代码同步、格式、vet、全部测试和目录规则均通过。真实 Rust CLI 验证首次初始化、重复执行、损坏配置保留及未来数据库版本拒绝；UI 验证确认前不写入、取消/跨页结果、迟到诊断失效及中英文与伪本地化的 900×600 确认页/部分结果页。截图人工检查确认正文可滚动、返回操作栏可见。

复审修改后的 `DESKTOP_CLI_BINARY=... DESKTOP_TERMINAL_TESTS=1 go test -race ./tests/app ./tests/cliadapter -run 'LocalSetup|Diagnostic' -count=1` 再次通过（81.665s / 56.795s），包括新加的空闲诊断刷新、确认后能力变化及明确写入前失败回归。

复审额外修复：setup 在后台完成时，已加载且空闲的诊断页现在自动进行被动刷新；确认后 CLI 能力变化显示不支持，阻止提交；已知写入前配置错误保留准确原因，不误报写入结果未知。CLI 初始化入口及普通 wrapper 对 home 尾 `/`、`/.` 做词法规范化，先拒绝受管目录链接再创建文件，目标目录保持无写入；正常路径及系统祖先别名保留。

以下为当前开发工作树的本机验证，不代表正式发布验收：

| 最新检查 | 结果 |
| --- | --- |
| desktop 整仓门禁（真实 CLI、PTY 开启） | 账户生命周期、绑定终端、停用重绑及 Unix 测试映像复用修正后通过，435 条官方消息；app 15.805s、cliadapter 15.248s，未变化包复用缓存；完整脚本退出码 0 |
| desktop app / cliadapter / capability 定向 race | 只读隧道/网络范围通过：app 133.962s、cliadapter 252.453s、capability 4.860s，无竞态报告、退出码 0。历史网络/诊断及配置全包 race 证据保留于本文 |
| Windows / Linux amd64 desktop 交叉构建 | 账户生命周期、绑定终端及停用重绑最终源码再次通过，非实际运行验收 |
| CLI 格式、Clippy、脚本与 diff 检查 | 通过 |
| CLI 默认 8 路完整测试 | 账户/绑定及 Node 能力错误提示最终源码运行原脚本通过：1052 通过、0 失败、3 跳过，47 个测试目标，测试执行 90.285 秒；完整门禁退出码 0。历史失败见本文 |
| Linux aarch64 WireGuard / 凭据专项 | 隧道目标观察 8 项及只读计划合同 4 项通过；此前检查/状态/共享诊断 43 项通过；此前配置/严格凭据 40 项通过。不替代 Linux 桌面、真实隧道或系统凭据服务验收 |
| CLI 相同全量测试改为 4 路执行 | 787 通过、3 跳过，147.787 秒；断言、超时和用例集合不变 |
| CLI 文档测试 | 通过，当前无 doc test 用例 |
| 最新 Rust CLI 与 desktop 互通 | 完整桌面门禁启用最新隔离构建的真实 CLI，覆盖账户默认值与创建/状态/绑定/验证/停用、诊断、本地初始化、WireGuard config.plan/config.apply/check/status/tunnel.plan；包含 uint64 精度、脱敏、状态取消后进程树清理、无效响应及未知结果不重试 |

只读隧道计划切片的最终整仓验证：`DESKTOP_CLI_BINARY=/tmp/agent-remote-wireguard-macos.aB73C6/debug/agent-remote DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh` 退出码 0，366 条中英文消息、生成文件同步、格式、vet、全部测试和测试目录门禁通过。Go→Rust 联调覆盖 up 默认配置及 down 显式配置，公钥等于登记公钥、内容摘要绑定准确字节，无额外 HTTP、秘密输出或文件变更。CLI 完整门禁使用相同隔离 Cargo target、4 路编译与原脚本默认 8 路测试，948 通过、3 跳过、46 个二进制，nextest 98.783s，脚本退出码 0；首次 fmt 发现模块排序问题，修正后从头完整复验。Windows/Linux amd64 desktop 交叉构建通过。日志分别为 `/tmp/agent-remote-tunnel-desktop-gate.log` 与 `/tmp/agent-remote-tunnel-cli-gate.log`。

只读隧道/网络定向竞态检查 `go test -race ./tests/cliadapter ./tests/app ./tests/capability -run 'Tunnel|Network' -count=1 -timeout=300s` 通过：cliadapter 252.453s、app 133.962s、capability 4.860s，退出码 0，无竞态报告，日志 `/tmp/agent-remote-tunnel-race.log`。此次 race 使用隔离子进程夹具，未设置真实 CLI/PTY 开关；真实 CLI/PTY 的证据来自上述完整普通门禁，不能将两者混称为本轮真实 CLI 全包 race。

新增预览界面的关联专项通过（37.879s），900×600 的英文、中文和 en-XA 表单/预览/详情/身份页已渲染；复核中文预览和伪本地化身份页，执行桥缺失前提可见，长内容滚动且底部操作可用。Go 严格协议专项额外覆盖只读/无凭据能力要求、目标状态矛盾、不可见公钥、目标更换及规范 utun 名称；全部新增测试、辅助代码和夹具仍在根 `tests/`。

本轮定向竞态命令为 `DESKTOP_CLI_BINARY=... DESKTOP_TERMINAL_TESTS=1 go test -race ./tests/app ./tests/cliadapter ./tests/capability -run 'Network|Diagnostic|Capability' -count=1 -timeout=600s`。首次 app 与 capability 分别通过（122.638s / 1.389s），适配器组因上述测试屏障失败（188.970s）；修复后同一筛选范围的完整 cliadapter 组再次通过（189.292s），退出码 0、无竞态报告。生产实现未因该测试调整；随后重新执行 desktop 完整门禁通过（app 29.366s、cliadapter 16.367s，脚本退出码 0）。

网络页新增及 catalog 分页措辞修正后，真实 CLI/PTY 开启的 desktop 完整门禁通过：346 条官方中英文消息、生成文件同步、格式、vet、全仓测试及目录检查均通过。900×600 中英文与伪本地化的表单、配置、状态、peer 和工具页已重新渲染，人工检查中文 peer 页和最终伪本地化分页页；主要操作可见，长内容可滚动。Go→Rust 联调验证生成后检查默认/显式配置没有额外 HTTP 或文件变更；假 wg 与真实 CLI 验证完整 uint64、IPv6 scope、秘密字段过滤、空接口与失败区别、无 home 创建及取消后的工具/子进程清理。隔离夹具不替代实际隧道验收。

WireGuard 确认参数在异步能力检查前冻结，专项覆盖原表单指针/切片改变后仍提交原确认内容；当前整组竞态验证也包含这些回归。官方 catalog 与生成代码同步，全部 desktop 测试、辅助程序和夹具位于根 `tests/`。三语言 900×600 确认页与结果页重新渲染后已查看中文确认页及伪本地化结果页，长路径可滚动，阶段摘要和底部操作可见。

- `DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh`：官方 i18n 校验、生成代码同步、gofmt、vet、全部测试及目录规则通过。翻译测试覆盖完整性、占位符、复数、回退及直接 UI 字符串；动态拼接仍需要代码评审。
- `DESKTOP_TERMINAL_TESTS=1 go test -race ./tests/session ./tests/terminalhost ./tests/storage ./tests/app`：通过。包括隐藏终端输入输出、实际退出码、超时后独立强制终止、慢启动与关闭竞争、存储兼容和 UI 操作。
- `go run ./tests/native-probe`：真实 macOS 窗口下，中英文切换、回调、隐藏/显示与受管 PTY 往返通过，退出码 0；未覆盖完整 Agent TUI。
- `workbench-probe` 在 macOS 原生应用循环中完整通过 Headless UI 事件及临时测试 Agent 的真实 PTY，验证三会话启动、标签切换/排序后的输入路由、面板焦点、退出和重启后的显式历史恢复；旧启动参数没有用于恢复，新连接保留了原面板位置和邻接面板。准确远程与账户绑定段覆盖确认后的 `attach <ID>`、所选 home/项目路径、终端输入、重复点击复用连接及重启后的状态刷新。该验证未使用真实 Agent 账户。
- 新增存储及 UI 测试覆盖版本 1 升级、不可变保存快照、无效布局拒写、跨项目引用拒绝、隐藏历史重启后找回、拖动排序持久化、侧栏尺寸、搜索导航、恢复 ID 校验及对话框快捷键隔离。新建会话主要控件在中英文/伪本地化的 900×600 窗口下通过边界检查。
- 中英文及 `en-XA` 在最小 900×600 窗口下通过主要操作布局测试；中英文切换后可继续使用已输入的项目路径，已有错误随语言更新。已渲染并检查欢迎页长文本效果。
- 包含设备码授权、注销、诊断与操作导航修复的 Windows amd64、Linux amd64 `CGO_ENABLED=0 go build` 交叉构建通过。三平台 CI 已配置原生探针，但尚未推送运行；Windows/Linux 的真实桌面行为和其他架构未验收。
- 上一轮密码登录实现的 CLI `scripts/run-quality-checks.sh`：743 项测试通过、3 项按条件跳过，doc tests 通过；包含格式、Clippy、shell 与 diff 检查。随后新增的配置切换/凭据锁与公钥 FIFO 边界，通过控制协议专项复验。后续定位到 macOS 的 HTTP 夹具接受 socket 可能继承非阻塞模式，读取在请求到达前报 WouldBlock；已显式切回阻塞读取并保留原有超时，登录专项 12/12 通过。此前并行测试也出现过等待超时，不能一概归因于环境。
- desktop 新增协议与界面测试覆盖请求关联、版本/阶段/退出码异常、超限输出、取消、共用 home、独立权限错误、配置切换、筛选、确认取消和快捷键隔离；最终 `scripts/check.sh` 与 `go test -race ./tests/cliadapter ./tests/app ./tests/presentation` 通过。测试及辅助程序均在 `tests/`。`DESKTOP_CLI_BINARY` 可启用对真实 Rust CLI 的隔离目录互通测试；此测试不使用真实账户。
- 已渲染并查看远程页的英文、简体中文及 `en-XA` 900×600 布局；首个连接按钮与主要导航可见，列表可滚动。这是 mygo UI 渲染验证，不替代真实服务器的终端连接验收。

上一轮桌面整仓检查在启用 `DESKTOP_CLI_BINARY` 真实 Rust CLI 和 `DESKTOP_TERMINAL_TESTS=1` 后通过，包含 248 条官方中英文 i18n 消息、生成文件同步、测试目录规则、vet 和全部测试。已重新生成并检查设备码授权、注销及诊断的三语言最小窗口截图。新增 UI 回归覆盖跨页返回与回执保留、登录/注销期间禁止发起连接、认证终态后丢弃旧查询结果；设备码流覆盖提前 EOF、终态后进程不退出及继承 stdout 的子进程。

上一轮 `DESKTOP_CLI_BINARY=... DESKTOP_TERMINAL_TESTS=1 go test -race ./tests/cliadapter ./tests/app ./tests/presentation` 全部通过（44.234s / 253.905s / 1.318s），无竞态报告。

失败记录：较早的 CLI 全门禁有 1 项既有 Ctrl-C 契约等待超时（749 项通过），随后同一测试独立通过；并行运行的控制协议及 Go race 也出现过启动等待超时。已采到部分子进程停在 `_dyld_start`、尚未进入应用代码的栈；该证据只解释被采样进程，不能替代对全部失败的逐项验证。未放宽断言或超时；后续第一轮整合 CLI 门禁 772 项通过、3 项跳过，尚不包括随后新增的凭据诊断修复测试。

加入凭据诊断修复后，默认 8 路并行整轮 CLI 门禁连续两次为 786 项通过、1 项失败、3 项跳过；仍为既有 `ctrl_c_interrupts_status_http_without_replaying_creation` 在发出 SIGINT 前等待请求启动超过 10 秒。同一最新二进制独立运行该用例 3.70 秒通过，未更改代码或超时；这两次整轮失败保留记录，不算通过，当时默认门禁的不稳定性尚未解决；后续定位与修复见下文。

随后使用 `cargo nextest run --workspace --test-threads 4 --no-fail-fast` 执行相同完整集合，787 项全部通过、3 项按条件跳过；既有 Ctrl-C 用例 4.730 秒通过。`cargo test --workspace --doc` 通过。降低测试并行度只是本机验证方式，不修改默认脚本，不证明默认 8 路门禁的问题已经修复。

后续采样定位到该测试辅助进程在初始化 macOS 系统 TLS 证书时，经 `CFBundleGetMainBundle` 扫描含 671,732 个目录项的 `target/debug/deps`；651/669 个采样栈停在目录扫描。相同二进制从原目录并发运行 8 次均超时，复制至独立临时目录后 8 次均通过。测试夹具现将原样辅助二进制复制到私有临时目录执行，保留 TLS、原断言、10 秒就绪和 3 秒中断期限，不修改生产代码、不清理用户构建缓存。修复后的该文件 9/9 通过，8 并发专项 8/8 通过（0.41–0.52 秒/个）。随后默认 `scripts/run-quality-checks.sh` 完整通过：8 路 788 项通过、3 项按条件跳过，122.208 秒，doc tests 通过（当前 0 项），脚本退出码 0；没有修改默认并发、重跑失败或放宽期限。

能力入口统一校验期间，一次与 CLI 测试枚举同时运行的 Go 专项在进入 runtime 前采样停于 `_dyld_start`；启动后 `other-platform` 场景等待 UI 文案 15 秒超时，`missing-platform` 通过（11.79 秒），进程总耗时 101.607 秒。该二进制编译早于最终统一校验修改。此运行记为失败，不能仅凭宿主延迟推断所有断言失败原因；保留原期限，以后续最新代码串行门禁作为另一次独立证据。

新增登录验证包括：Rust 的桌面登录→普通 CLI 复用同一设备、拒绝/未知结果不重放、敏感内容不进入回执/SQLite；Go→真实 Rust CLI→loopback HTTP 夹具的用户登录与共用配置读取；原生 UI 的确认前不提交、取消语义、凭据清理、三语言 900×600 布局。所有数据与凭据均为临时测试夹具，未登录真实用户服务器。

正式发布前记录准确代码版本、平台和全量结果。本机测试、交叉编译不能代替 Windows/macOS/Linux 的实际运行验收；语言持久化及系统区域设置还需逐平台验证。

相关文档：[开发规范](development-standards.md)、[平台验证矩阵](platform-validation.md)、[CLI 覆盖清单](cli-coverage.md)、[产品方案](../../agent-remote/docs/agent-remote-desktop-design.zh-CN.md)。

本轮新增 `scripts/remote-interop-acceptance.sh` 与[远程闭环联调验收](remote-interop-acceptance.md)。`--contracts` 已在当前工作区通过 Server session attach/verify、attachment transfer、Node attach、port-forward（82 passed、2 skipped）、Node attach 包、Rust desktop-control/Mutagen/exact attachment fixture 合同实际执行及 Desktop remote/cliadapter 测试；这些是 fixture/protocol 证据，不代表真实远程连接。`--host-only` 已在 macOS arm64 通过实际 `agent-remote`/`fclaude` 可执行文件、Go→Rust `capabilities` 请求和远程方法门禁，证据明确写入 `remote_execution.status=not_run`。真实 Server/Node/SSH exact attachment 仍需使用 disposable session 运行 `--live`；Mutagen live probe 仅在显式提供固定原生 Mutagen 二进制时执行，任何缺失身份、目标主机或 broker 都 fail closed。

## 2026-10-23 交付候选复验

本地交付提交为 `db15008697effd7f2db6b0704d2d44c8d58883d4`（工作树干净）。在该提交上重新执行 `DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh`、`go vet ./...`、darwin/arm64、windows/amd64、linux/amd64 的 `CGO_ENABLED=0 go build ./...`，全部通过。账户导入受管 Skills 场景的独立 race 用例通过（39.36 秒）；完整 `go test -race ./tests/app` 曾在 headless mygo intrinsic layout 阶段超过 600 秒，因此不计作全量 race 通过，也没有放宽生产超时或删除该场景。

从干净提交构建的社区自签候选位于 `/tmp/agent-remote-desktop-release-delivery`：版本仍为 `0.1.0-dev`，目标为 `darwin/arm64`、`windows/amd64`、`linux/amd64`，manifest 的 `git_dirty=false`，共 18 项制品。macOS 使用 `Agent Remote Community Code Signing`（SHA-1 `53EE33F5BE5A109F8575A75D9E126826B5BB75CE`），Windows/Linux 不使用发行者代码签名，未启用公证。`scripts/verify-release.sh --require-targets` 和 macOS 安装后 `scripts/release-native-smoke.sh` 均通过。

仍未关闭的发布门禁：Windows 真实主机窗口/ConPTY/托盘/安装器、Linux amd64 X11/Wayland/GTK/AppIndicator/PTY/托盘/安装器、三平台真实 WireGuard broker、真实 Server/Node/SSH/fclaude 会话与附件联调、Mutagen restart recovery，以及 GPU/PTY/input P95 性能测量。当前候选是可审阅、可安装的代码交付版本；在这些主机和服务证据补齐前不标记为正式发布。
