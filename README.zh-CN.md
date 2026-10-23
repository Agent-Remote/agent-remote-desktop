# agent-remote-desktop

正式 Release 按社区版交付：macOS 沿用项目社区自签，Windows/Linux 不要求发行者代码签名。
Apple Developer ID 与公证不作为发布前提。`scripts/build-release.sh` 已提供构建基础；
完整安装、受保护组件和升级验收仍属于各平台发布前检查。正式发布目标为 Windows、macOS
和 Linux，FreeBSD 不在计划中。详见[社区版构建指南](docs/release-build.md)。

带版本 tag 的发布由仓库的[社区版 release workflow](.github/workflows/release.yml)负责：tag 必须是
`v*` 且与 `mygo.json` 版本完全一致；工作流只在临时 macOS keychain 中导入受保护的项目社区证书，
离线校验完整制品后才发布。Windows/Linux 不启用发行者代码签名，也不会发起 Apple 公证。原生主机
窗口、托盘、PTY、权限和 Agent/Server/Node 联调仍是独立的发布门禁。
工作流会显式构建并校验首个正式版的完整目标集合
`darwin/arm64,windows/amd64,linux/amd64`；本地 smoke 构建仍可选择子集。

[English](README.md) | 中文

基于 mygo 的全 Go 原生多 Agent 工作台，为本地 Codex、Claude 与远程 fclaude 提供项目、终端会话和分屏管理。

当前处于开发阶段，已建立可运行的工作台、CLI 命令发现契约和 schema 驱动的通用命令表单。叶子命令可按 schema 编辑参数、审阅并冻结 argv，再在受管终端执行；二次元背景装饰与一次性进入动效首个切片已实现，完整视觉验收、专用业务工作流、发行升级和三平台运行验收仍在实施中；当前构建不是正式版。

工作台采用重构方案的设计 token（沿用 prototype-v3 色值）：浅色、深色、次元，存储 ID 仍为 `daybreak`、`midnight`、`sakura`。新安装默认跟随系统浅/深色；旧版本保存的状态保持原来的明确主题。工具栏从当前显示的主题开始轮换，并转为明确选择。终端面板按主题使用不透明背景，光标与选区颜色经对比度校验，ANSI 颜色不被重新着色。设置 → 外观新增「降低透明度」（实色表面、关闭模糊；系统「增强对比度」也会开启）；「减弱动效」让位移即时完成、保留短暂淡入淡出，并与 mygo 原生减弱动态效果偏好同时生效。原 Sakura 立绘已移除。
`App.MotionStats` 保留最近 512 个视图构建样本并计算 nearest-rank P95；运行 `go run ./tests/performance-probe -frames 120` 可输出 10 个持久会话、4 个可见布局的 host-only JSON。该探针只覆盖 Go/mygo 视图构建，不代表原生 GPU、PTY 渲染或输入反馈性能；这些指标仍需目标平台主机证据。

账户管理已接入创建、状态、绑定、验证与停用的确认流程。登录绑定使用对应账户和准确绑定批次的原生终端；历史记录先刷新状态，再重新确认连接。请求受理不表示登录完成；停用撤销端口转发授权，但不清除凭据或停止已有会话。详见[账户协议](../agent-remote-cli/docs/desktop-accounts.md)。

管理员运行时恢复已接入用户专用身份检查、准确原迁移观察，以及目标端验证、源端验证、源端权限修复的独立确认。CLI 统一保留原请求编号；桌面重启后读取该记录，按原绑定查询，仅显式移除明确拒绝或已确认终态的本地记录。手工输入的旧编号仅供查询，不生成恢复计划。观察与受理不代表恢复已完成。详见[运行时恢复协议](../agent-remote-cli/docs/desktop-runtime-recovery.md)；Windows/Linux 原生桌面验收仍待完成。

账户配置导入已提供独立的本地发现、读取审阅、可选 Server 策略校验和上传确认。完整文件清单分页展示，不截断；历史与 skills 选项沿用 CLI 默认值，文件内容仅由 Rust 处理。CLI 在上传前保存原请求编号和公开上下文；重启后打开“配置导入记录”会自动从 CLI journal 读取，并仅查询原请求和准确任务，不会自动重发未知操作。移除已结束的本地记录需要独立确认。详见[导入协议](../agent-remote-cli/docs/desktop-account-import.md)。Windows/Linux 原生运行和真实部署 Server/Node 验收仍待完成。

项目同步页已接入所选项目的 `sync ensure/status/pause/resume/resolve/reset`。提交前核对共享上下文和准确目标指纹；Server、本地 Mutagen 与 Agent 会话状态分别呈现。部分完成与未知结果按阶段保留，跨页不丢失、不自动重放。“跳过本地执行”沿用 CLI dry-run 语义，**仍修改 Server 与共享 SQLite 状态**。解决冲突仅刷新用户已处理的文件，不自动选择覆盖方向；重建基线可能恢复曾删除的文件。详见[同步协议](../agent-remote-cli/docs/desktop-sync.md)。旧映射现提供只读恢复预览：核实 Server 404、完整本地映射和准确 Mutagen 身份后，另行确认移除；项目文件和远端资源保留。建立新关系需要重新读取并独立确认，旧结果仍保留。隔离的 macOS→Docker Linux 链路已通过真实 Server/Node/SSH/Mutagen 文件传输；独立“同步运行环境”页现可审阅完整共享 Mutagen 服务后确认重启；Windows/Linux 原生桌面验收仍待完成。
应用重启后，操作中心会恢复同步和 Mutagen 运行环境的受限阶段摘要；准确回执仍由 CLI journal 提供，desktop 不重建或自动重放。

“同步运行环境”可从环境诊断、项目同步页和命令搜索进入，不要求项目或 Server 登录。页面展示上次记录与目标 CLI/SSH 环境，以及全部同步/转发会话、暂停状态和已停止 daemon 的持久清单。独立确认后才重启共享服务；原回执保留，未知结果不自动重放。外部服务注册会明确阻止操作，需后续专门服务集成。详见[运行环境协议](../agent-remote-cli/docs/desktop-mutagen-environment.md)。

## 本地运行

需要 Go 1.27.1 或更新版本。终端插件首次运行时下载固定校验和的 libghostty-vt；正式打包时会携带原生库。macOS 需 13+；Linux 需 GTK 3，托盘另外依赖 AppIndicator，WebKitGTK 仅在使用页面/WebView 窗口时需要；Windows 使用原生 ConPTY，WebView2 同样只在页面/WebView 窗口时需要。

```sh
go run .
```

使用隔离的配置环境：

```sh
go run . --home /path/to/agent-remote-home
```

配置路径与 CLI 一致，遵循 `--home`、`AGENT_REMOTE_HOME` 和平台默认目录的优先级。桌面数据库位于该目录下的 `desktop/desktop.sqlite3`，不会直接读写 CLI 数据库或凭据。

每个发布目标旁都包含 `agent-remote-desktop-installer`（Windows 为
`.exe`）。用户取得完整发布目录后，helper 会先校验目标清单和 SHA-256，
再在用户可写目录中原子安装或升级；`--rollback` 可恢复上一版本。它不会
下载、提权、签名或安装外部 Agent CLI。具体用法见[社区版构建指南](docs/release-build.md)。
交付前还可用离线的 [`scripts/verify-release.sh`](scripts/verify-release.sh)
复核完整制品集合、SHA-256、每个目标的 desktop 可执行文件与 installer helper，以及社区版签名策略；任意占位制品无法通过。
它还会把清单绑定到 `Agent Remote Desktop` 与 `dev.agent-remote.desktop`，避免仅替换元数据就把其它应用伪装成交付物。
随包安装器在复制文件前也会重复校验该身份，即使 Windows 主机没有 Bash 也不能绕过这层绑定。
升级前可使用 helper 的只读 `--status` 查看当前版本和是否存在可回滚版本；未受管的目标目录
会被拒绝，避免把中断的安装误判为成功。
Linux Debian 制品还会声明托盘所需的 AppIndicator 运行时替代依赖
（`libayatana-appindicator3-1 | libappindicator3-1`），使隐藏窗口后仍可从托盘唤回；
原生平台探针和托盘探针仍会在目标主机验证实际运行环境。

1. 打开项目目录。
2. 新建 Codex、Claude 或 fclaude 会话；对应工具需已安装。
3. 使用横向或纵向分屏同时查看最多四个终端面板。
4. 侧栏切换已有会话；隐藏面板保留进程。
5. 关闭窗口后应用留在后台，可通过托盘、Dock 或重新启动应用唤回。
6. 明确退出时确认停止本地会话。超时后再次选择是否强制结束；远程 tmux 会话不会因断开本机终端而停止。

从 Finder、资源管理器或桌面拖入的文件会以安全转义后的路径插入当前聚焦的本地
Agent 终端，desktop 不会自动执行路径。远程 fclaude 面板在具备绑定身份的上传协议前会
明确提示仅支持本地拖放，不会把本机路径误传给远端。

远程会话卡片在 CLI 声明 `sessions.stop` 时支持准确停止。确认会冻结完整会话、节点、运行时后端和配置上下文；CLI 保存 caller key，desktop 在结果未知时只能查询原 key。进程停止、Skill 保存/发布和同步观察分别显示，进程停止不等于项目已保存。

Skills 页面已接入 `skills.list` 目录读取及启用/停用专用流程。用户先审阅 Skill、动作、准确上下文和冻结的 catalog generation，再由 CLI 以用户凭据提交精确请求；页面只展示安全的目录和账户本地状态摘要，支持刷新及名称/ID 筛选，并在上下文过期或范围冲突时拒绝继续。Skill 内容、来源元数据和凭据留在 CLI 内。页面还提供精确账户 UUID 绑定的只读 `skills.state.conflicts` 检查，发布和迁移冲突分页后只投影安全状态摘要。每条冲突都提供专用审阅入口：发布冲突可解析，迁移冲突可解析、迁移或恢复。desktop 只预填安全的冲突/Skill/account 标识；分支、路径和 checkpoint 仍由用户填写并由 CLI 校验，桌面不会自动解析、迁移、恢复或重试。

同一页面提供 `skills.state.list` 状态历史：可按账户目录或单个 Skill 查询 checkpoint 与 pending-finalization 分页，只投影代次、digest、保留/head、存储位置、状态和时间等安全元数据。账户 UUID、范围、数量上限和两个分页游标都会绑定当前上下文并严格校验；路径、manifest、保存输入和凭据留在 CLI 内。

同一页面新增 `skill info` 专用详情面板，对应 `skills.info` v1 只读合同。查询绑定精确 Skill 标识、可选 tool/account 范围和当前 home/Server/device 上下文，只投影保留修订 ID、生效规则以及模型状态等安全元数据；来源 locator、tracking、路径、内容和凭据仍保留在 CLI 内。上下文过期、范围冲突或未知字段会安全停止。

ego-browser 页面可在每个 binding 下查看活动请求。`ego_browser.requests` 只返回有界请求元数据和 payload 大小；取消操作要求用户凭据、精确请求确认，并冻结 binding/request generation 与 sequence。CLI 会在提交前刷新 binding，desktop 不接收浏览器 payload，代次变化或未知回执会安全停止且不会自动重试。

ego-browser 页面还提供 binding 的 `pause`、`resume`、`stop`、`revoke` 结构化生命周期操作。提交前先用用户凭据读取精确代次绑定的 plan，用户确认后调用 `ego_browser.binding.control`；状态不允许、代次过期或回执未知时 fail closed，不自动重试。页面只展示安全状态摘要，不接收浏览器 payload。

对已撤销的 Device 以及已停止、过期、失败或撤销的 binding，页面还提供 `delete-device`/`delete-binding` 的专用删除审阅。删除先读取精确目标和 generation 的 plan，用户明确确认后才提交 control；上下文变化、未知字段或未知回执会停止并保留未知状态，本地索引只在远端成功后清理，回执会标明本地清理是否完成。

Device 页面提供经过审阅的 `device.status`、`device.revoke` 和
`device.rotate-token` 流程。页面冻结 home、服务器和设备上下文，按操作使用设备或用户凭据，分别展示远端成功与本地清理/凭据保存结果；设备令牌不会进入 desktop。

`node install` 与 `ego-browser` 命令族使用专用原生终端流程。desktop 按 CLI 能力和平台门禁每条准确路径，冻结审阅后的 argv，变更命令强制加入 `--yes`，终端输出保留在受管 PTY；操作中心只展示安全的运行中/完成/失败状态。

高级 Agent 参数在工作台终端入口中以 JSON 字符串数组填写，按 argv 传递，不通过 shell 拼接。能力说明页的 fclaude 默认入口、`run`、`new`、`list` 已使用远程会话库存、结构化创建审核和只读清单；首次远程工作区授权及 Claude 参数透传仍由 fclaude 原生终端承载。

## 工作台与恢复

创建会话时可指定名称。拖动标签调整顺序，拖动分屏边界调整尺寸；“搜索”可查找工作台命令、项目和会话。隐藏标签后，其会话仍保留在侧栏中。

desktop 数据库读取版本 1/2/3，写入版本 3；公开远程上下文经过校验和深复制，旧应用不能覆盖新版存储。

desktop 保存标签顺序、分屏比例、面板焦点、侧栏宽度及会话元数据。应用重启后，历史面板显示为未连接；点击“继续历史会话”，可打开 Codex/Claude 自带的历史选择器，或输入具体会话 ID。远程恢复先刷新当前配置和会话列表，只有 home、服务器、设备及准确运行中会话与保存的上下文一致时才进入确认。没有上下文的旧历史须从刷新列表重新选择会话，不能把旧 UUID 默默绑定到当前服务器。成功恢复保留原项目、cwd 和分屏位置。

本地恢复使用工具当前配置。desktop 历史只保存公开远程上下文，不保存原始启动参数、环境变量、凭据或终端输出。移除未连接的历史记录仅删除 desktop 元数据，不删除 Agent 自身历史或远程会话。

新建和恢复会话对话框会检查所选 Codex、Claude 或 fclaude 是否可用。缺失工具时禁用“启动会话”或“继续”，同时显示原因、安装说明和需明确点击的官方指引入口；安装后可点击“刷新工具检测”，设置页也提供安装指引。检测结果使用缓存，界面重绘不会反复查找程序或执行版本命令。正式启动前会再次检查可执行文件，避免工具在检测后消失时仍按旧结果启动。Codex/Claude 保留原生历史选择与恢复语义，fclaude 恢复始终要求审阅已有远程会话及准确上下文。

桌面数据库在保存时将版本 1/2 升级为版本 3；不兼容的未来版本、无效布局引用或不完整的远程上下文会被拒绝，原数据保留。

下表 `Mod` 在 macOS 上为 Command，在 Windows/Linux 上为 Control：

| 操作 | 快捷键 |
| --- | --- |
| 命令与导航 | `Mod+Shift+P` |
| 打开项目 / 新建会话 | `Mod+O` / `Mod+T` |
| 横向 / 纵向分屏 | `Mod+Shift+D` / `Mod+Shift+E` |
| 下一个 / 上一个标签 | `Ctrl+Tab` / `Ctrl+Shift+Tab` |
| 下一个 / 上一个项目 | `Mod+Alt+Right` / `Mod+Alt+Left` |
| 聚焦下一个面板 | `F6` |
| 隐藏当前面板 | `Mod+Shift+W` |
| 专注模式 / 设置 | `Mod+Shift+F` / `Mod+,` |
| 远程管理 | `Mod+Shift+R` |

## 开发要求

“远程管理”需要 PATH 中的 `agent-remote` 提供版本 1 的 `--desktop-control` 协议。页面读取所选 CLI 配置，显示工具账户和远程会话，支持服务器端 Agent/状态筛选、已加载结果的文本搜索、刷新和取消查询；进入页面或窗口重新获得焦点时刷新。连接前先选择本地项目，确认框显示完整会话 UUID、服务器、设备、home、本地项目/目录、账户、节点及终端接管说明。CLI 必须声明 `sessions.attach` v1；desktop 传递成对的 `--expected-server`/`--expected-device`，不降级到旧连接方式。存活连接仅在 home、服务器、设备、会话和本地项目全部相同时复用。本地目录仅为终端 cwd，不代表远程 workspace。此准确连接路径支持文字复制/粘贴，暂不支持图片和文件粘贴。详见[连接与历史设计](docs/remote-session-attach.md)。

能力页中的 `agent-remote node install` 会进入专用节点选择页。页面读取有界的 `nodes.list` 摘要，冻结所选 home、Server、device、Node UUID 和可选 ego-browser 意图，用户审阅准确命令后才启动受管 Agent Remote 终端执行 SSH、制品校验、join code 交换和安装。桌面端不会保存 join code 或凭据；重启只保留安全的操作摘要，不会自动重放安装。

能力说明中的 fclaude 默认入口、`run` 和 `list` 会进入这个远程会话库存；`new` 会打开结构化的 caller-key 创建审核，账户、workspace、project 和同步证据绑定当前 home、Server、device。工作台仍提供受管的 fclaude 原生终端，用于 Claude 参数透传和首次远程授权；终端输出不会被当作桌面回执。

远程页面也可对运行中的 Claude 会话发起准确停止，并显示 `accepted`、`stopping`、`pending`、`unknown`、`saved`、`failed` 和删除后的状态；未知结果不会再次提交停止，`stop-status` 只查询原 caller key。

远程创建也支持应用重启后的原请求查询。CLI 返回原请求 UUID 后，desktop 只保存该 UUID 及 home/服务器/设备范围；恢复弹窗为只读查询，冻结当前上下文并调用 `sessions.create.request`，不会保存回执、argv、同步计划或凭据，也不会提交替代创建请求。

查询复用 CLI 设备凭据，desktop 不接收 token；错误通过结构化码本地化。“登录服务器”支持密码及可选 MFA 验证码、设备名称、SSH 公钥文件选择和可选 WireGuard 公钥；检查准确信息后确认提交，也可选择仅登录、不登记设备。提交后清除密码/MFA 输入，不写入桌面持久化。停止本地等待可能留下未知的远端结果，界面会明确说明并且不自动重试。
设备登记由 CLI 生成并持久保存 UUID 和公开意图，Server 按原 key 提供同意图回执恢复；传输丢失时不会从设备列表猜测，也不会换 key 重试。只有本地凭据、设备状态和配置全部发布成功后才清除原 key。

“默认账户”可按工具查询、从远程账户列表选择或清除默认值。确认页显示配置目录、服务器/设备、旧默认值与新选择；其他 CLI 已修改时要求刷新，避免覆盖。默认值与 CLI/fclaude 共用，作用域为当前 home 和工具，跨服务器共用；清除只移除本地选择，不禁用或删除账户。未登录或远程查询失败时仍可查询和清除本地默认值。操作结果可跨页查看，停止等待后不自动重试；操作中心跨重启恢复安全的生命周期摘要，精确回执仍由 CLI journal 查询。详见[默认账户契约](../agent-remote-cli/docs/desktop-account-defaults.md)。

“开发者凭据”页面提供不含秘密的配置档案清单，以及经过审阅的创建、绑定和解除绑定操作。desktop 只通过 CLI 控制通道提交配置档案元数据和精确 ID；设备 token 留在 CLI 安全存储中。token、私钥等未知字段会在访问网络前拒绝，回执只投影元数据；通用 CLI 表单不会暴露这些操作。

“注销登录”先核对共用配置目录、服务器和设备，默认同时撤销远端凭据，也可取消勾选以仅离线清理本地凭据。结果分别展示本地清理和远端用户/设备凭据撤销状态，包含部分失败。停止等待只表示结果未知，不自动重试。注销影响 CLI 共用登录态，不停止本地 Agent、不向远端会话发送停止请求；现有连接可能失去授权。

登录页也可选择浏览器授权：核对服务器及设备登记选项后，显示授权地址、短验证码和剩余有效期，由用户点击打开浏览器或复制验证码。轮询与凭据保存在 CLI 内完成，desktop 不接收内部 device_code 或 token；停止等待可能留下未知结果。
CLI 会持久化调用方生成的 completion key；传输或超时后只按该原 key 精确查询完成回执。找不到、过期或损坏的回执继续显示未知，不会重新轮询、换 key 或生成新的授权。

登录及注销期间可以切换页面，通过侧栏或命令搜索中的状态入口返回。结果保留至用户在操作页点击“返回远程管理”明确确认；返回状态页不会再次提交操作。操作中心跨重启恢复不含敏感信息的生命周期摘要；精确 request key、回执和执行上下文仍由 CLI journal 保存，desktop 不会自动重放操作。

“环境诊断”入口提供配置与依赖检查；`status`、`doctor`、`deps status` 和 `ssh check` 命令现进入由版本化 desktop-control 支持的结构化页面。页面把结果绑定到当前 home 和平台，未知字段或上下文漂移会安全拒绝；Doctor 与依赖修复必须显式确认。诊断页和能力页的命令列表还可打开独立的“依赖管理”页面，调用 `dependencies.inspect` 展示 CLI 平台、清单来源、每项状态/版本/许可证。`credential: none` 只表示只读检查，不代表管理员权限或安装成功。进入诊断页面为被动检查，点击“运行检查”后验证本地状态并按选项检查服务器健康；版本检查会显式运行有界的依赖版本命令，展示缺失、权限、版本输出、许可证及建议。“本地初始化与修复”在确认所选配置目录后，创建缺失目录、数据库表及依赖清单；八步回执分别显示已有数据、已修改、失败和前置失败导致的跳过。已有有效内容保留，损坏/较新版本数据库及无效清单不会被替换；不登录、不安装依赖、不启动隧道，保存的服务器配置不可读时仍可修复本地元数据。

本地修复可跨页继续，通过侧栏状态、统一操作中心或命令搜索返回；明确关闭回执后刷新诊断。停止等待或退出可能留下未知结果，不会回滚已完成步骤；先检查诊断，再显式决定是否重跑。统一操作中心跨重启保留不含敏感数据的生命周期摘要，并可打开所属页面查看精确回执；精确 request key、回执和上下文仍由 CLI journal 负责，desktop 不会自动重放。依赖安装仍待实现。

“完成初始化”可从侧栏和命令搜索进入，按顺序执行本地初始化、密码或浏览器/设备码登录、可选设备登记、可选 WireGuard 发布，最后检查诊断以及账户/会话就绪状态。WireGuard 会先暂停展示只读计划，用户需要再次确认准确路径和动作后才写入。所有步骤都通过 CLI desktop-control 执行，密码、设备码轮询凭据、访问令牌和私钥不会进入桌面或回执；取消或丢失回执保持未知，不会自动重放。

“WireGuard 配置”可从环境诊断和命令搜索进入。选择默认位置或绝对输出路径，预览服务器、设备和准确的新建/覆盖范围后确认。普通 CLI 命令、`init` 的可选配置步骤与桌面共用同一 Rust 服务；桌面只接收计划和密钥选择/保存、登记、文件发布的阶段回执，私钥留在 CLI。

部分失败保留已完成阶段；登记回执丢失时维持结果未知，不自动重试。可跨页返回操作状态，停止等待或退出不会回滚已保存密钥或服务器变更。发布配置文件不等于启动隧道；网络页现可在审阅计划后执行启停，Rust 会通过固定平台 broker client 发送配置字节和摘要，并在 broker 回执后确认目标状态；没有受保护 broker 时明确显示不支持/不可用，不宣称具备执行授权。macOS LaunchDaemon、Linux systemd/D-Bus/Polkit 和 Windows SCM LocalSystem broker 的实现与安装材料已加入仓库；持久恢复、真实主机授权及三平台原生发行验收仍待完成。

桌面会按固定顺序把权限桥观察投影为本地生命周期门槛：安装、系统批准、协议兼容、调用方认证和执行授权。只有完整观察到 `ready` 才显示具备执行条件；中间或未知状态始终阻断，也不会授予权限。

“WireGuard 网络”可从环境诊断和命令搜索进入，无需登录即可显式检查默认/自选配置、读取状态或查询权限桥；页面进入不自动查询。配置检查只返回经过校验的非敏感摘要，不支持的 hooks 仅显示名称、不会执行。状态使用一次有界机器格式查询，在 Rust 内剔除私钥和预共享密钥，区分空接口成功与查询失败，不自动请求管理员权限。权限桥查询只显示安装、批准、协议和调用方认证观察，不把组件存在或进程权限当成执行授权。支持接口/节点分页，完整计数和握手时间使用官方本地化。各查询可分别取消；配置发布会使旧结果失效。详见[检查与状态契约](docs/wireguard-inspection-contract.md)与[权限桥状态合同](docs/wireguard-bridge-status-contract.md)。

“隧道计划”可在网络页选择启动/停止方向及配置后显式预览，展示准确配置与工具快照、原生接口/服务观察和未满足条件。详情可查看身份摘要；切换动作或配置会使旧计划失效，跨页保留结果并支持取消。当计划没有未满足条件时，页面会启用显式执行按钮。CLI 会重新检查冻结计划，通过固定平台 broker client 发送用户侧读取的配置字节，获得 broker operation receipt 后确认目标状态并返回回执。详见[隧道契约](docs/wireguard-tunnel-contract.md)。

“能力说明”只读展示 CLI 声明的桌面操作、所选配置环境及 `agent-remote`、`fclaude` 命令清单，支持刷新、取消查询和命令筛选，不执行清单中的命令或主动探测服务器权限。未声明适用平台的命令标为未知，不推断为支持所有平台；权限信息仅采用同一配置环境中已经观察到的查询结果，其余保持未验证。该页面说明 CLI 支持情况及当前前置条件，不代表全部命令已完成图形化操作。

`agent-remote node install` 与 `agent-remote ego-browser` 命令族进入专用终端工作流。CLI 为每条精确命令路径声明版本化 terminal capability；桌面检查该声明和客户端平台，冻结 schema 生成的 argv，对变更操作固定加入 CLI 的非交互 `--yes`，并在启动受管终端前要求用户审阅精确请求。最终设备/服务器授权及命令错误仍由 CLI 负责；操作中心显示并跨重启恢复安全的生命周期摘要（运行中、完成、失败或需关注），不会自动重放命令。

desktop 全项目统一使用 mygo 官方 i18n catalog 与生成的 Go 文案函数，覆盖界面、菜单、通知、校验、错误及恢复提示等全部自有用户可见文案。首版语言为英文 `en`（源语言/回退）和简体中文 `zh-Hans`，支持跟随系统及手动切换语言；Agent 原始终端输出、用户内容和协议标识保持原样。

所有 desktop 测试、测试辅助代码、夹具和原生测试程序必须位于**本 Git 仓库根目录 `tests/`**，与主程序目录分离；测试使用根 Go 模块，通过公开接口导入生产包。

当前工作台已接入中英文 catalog，可在设置中选择语言；切换时保留项目输入和运行中的终端。既有测试及原生验证程序已迁入 `tests/`。目录布局、命令和 CI 门禁见[开发规范](docs/development-standards.md)。

## 当前验证入口

```sh
bash scripts/check.sh
DESKTOP_TERMINAL_TESTS=1 go test -race ./tests/session ./tests/terminalhost ./tests/storage ./tests/app
go run ./tests/native-probe
go run -race ./tests/workbench-probe
scripts/native-acceptance.sh
# 远程协议/适配器合同（夹具，不声明真实远程成功）
scripts/remote-interop-acceptance.sh --contracts
# 当前主机 Go→Rust 能力预检（remote_execution=not_run）
scripts/remote-interop-acceptance.sh --host-only
```

检查脚本验证翻译、生成代码同步、测试位置、格式、vet 和测试。`native-probe` 打开真实原生窗口，切换语言，在隐藏期间验证受管 PTY 输入输出，再显示窗口并自动退出。它只在临时目录运行无副作用的 shell 测试，语言覆盖不写入用户偏好。

`workbench-probe` 在原生应用循环中，通过 UI 事件和真实 PTY 验证多会话启动、焦点输入路由、标签排序、退出及显式历史恢复。远程专项验证确认后连接的准确 ID、配置目录及项目路径，以及重复连接复用已有会话。它使用临时测试 CLI 和 Agent，不启动已安装的 Agent，也不使用账户。
当前完整探针已通过账户绑定复用、状态刷新和 remote-scope；验证只使用临时测试 CLI/Agent，不连接真实账户。
Server/Node/SSH 与 Mutagen 边界的统一入口见[远程闭环联调验收](docs/remote-interop-acceptance.md)。
`--live` 仅接受已认证的 disposable session；主机预检或夹具通过不会被当作真实远程成功。
需要真实同步链路时，使用 `scripts/remote-interop-acceptance.sh --sync-chain`，并提供已审阅的 Mutagen v0.18.1 源码、CLI 及相邻 SSH/SCP sibling；该入口会真实运行一次性 Server/Node/SSH/Mutagen Docker 链路，缺少依赖时 fail-closed。

`native-acceptance.sh` 还会在临时目录执行安装、升级、拒绝降级和回滚探针，并将主机系统、架构、显示模式和命令输出写入 `/tmp`（可用 `DESKTOP_NATIVE_EVIDENCE_DIR` 覆盖）。脚本不会设置 `GOOS` 或 `GOARCH`，交叉构建必须单独记录；无显示器的 Linux 仅在可用时使用 `xvfb-run`，并明确记录为虚拟显示。
Windows 没有 Git Bash 时可用 `pwsh -File scripts/native-acceptance.ps1`；两个入口都会在失败时保留包含失败步骤的 `summary.txt`。取得 release 目录后，可运行 `scripts/release-native-smoke.sh dist/release`，它会离线校验清单，在临时用户目录安装并检查状态，再启动已安装二进制的 `--version`。Windows 没有 Git Bash 时使用等价的 `pwsh -File scripts/release-native-smoke.ps1 -ReleaseDir dist/release`；随 release 提供的安装器仍会离线校验 manifest 和 SHA-256，不会提权、安装服务或伪造 WireGuard broker 证据。
协调机暂时没有目标主机时，可给 Bash 入口传入 `--target windows|linux|darwin`（PowerShell 使用 `-Target`）；它只记录 `status=not_run` 并以非零退出，证据合并器会拒绝该记录，不会执行交叉编译制品或伪造原生通过。
只有审阅 macOS ad-hoc smoke 目录时才应额外传入 `-AllowAdhoc`。
三台发布主机都完成后，可运行 `scripts/verify-native-evidence.sh <macos-dir> <windows-dir> <linux-dir>` 汇总默认三平台审阅证据；它要求正式架构 `darwin/arm64`、`windows/amd64`、`linux/amd64`，以及 macOS、Windows、Linux 的主机运行记录和五个探针日志（含真实托盘创建/销毁），并解析每个 `platform.log` 的结构化报告，强制校验 `host_runtime=true`、目标匹配、`supported=true` 及所有必需检查通过；Linux 必须明确报告 required display、GTK 3 和 AppIndicator，并记录 `display_protocol=x11|wayland` 及 optional WebKitGTK 4.1/4.0 观察，Windows 记录 optional WebView2 观察。正式发布前再传入 `--require-linux-display-pair`，并提供两个 Linux 目录（总共一台 macOS、一台 Windows、两台 Linux），以强制 X11 与 Wayland 各有一份真实主机记录。mygo 仅在页面/WebView 窗口中加载这些运行时；当前工作台全部使用 native-content 窗口，缺少 optional 运行时不会阻断支持。安装器日志必须包含结构化安装/升级/回滚/运行锁回执，原生窗口日志必须报告 PTY exit 0，托盘日志必须包含 host-only 且 status=passed 的报告，工作台日志必须包含成功回执；跳过工作台或托盘的 CI 记录不能作为正式证据。它拒绝交叉编译或缺少实际 probe JSON 的记录。`--allow-virtual-display` 只用于审阅 Linux Xvfb 证据，不能作为正式桌面验收。
首个平台探针还会分配、读取尺寸并关闭当前主机的原生 PTY（Windows 为 ConPTY）；这只是终端句柄可用性前置检查，不代表输入法、全屏 TUI 或具体 Agent 行为已经验收。

`DESKTOP_LOCAL_AGENT_TESTS=1 go test ./...` 可额外验证本机已安装工具的 `--version` 在受管 PTY 中启动；这不等于完整 Agent 交互验收。

生成当前 CLI 的命令清单，需要包含 `--desktop-schema` 新接口的 CLI 构建：

```sh
go run ./cmd/command-catalog \
  --cli ../agent-remote-cli/target/debug/agent-remote \
  --launcher ../agent-remote-cli/target/debug/fclaude > /tmp/desktop-catalog.json
python3 scripts/update-cli-coverage.py /tmp/desktop-catalog.json
```

完整计划见 [产品与技术方案](../agent-remote/docs/agent-remote-desktop-design.zh-CN.md)。当前事实与下一步见 [实施状态](docs/implementation-status.md)、[平台验证矩阵](docs/platform-validation.md) 和 [CLI 覆盖清单](docs/cli-coverage.md)。

## 许可证

GPL-3.0-only。依赖声明见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
