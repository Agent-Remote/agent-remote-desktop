# CLI 图形化覆盖清单

由 `cmd/command-catalog` 从实际二进制的 `--desktop-schema` 输出生成并用于覆盖审阅。
发现命令不等于已实现 GUI。下表为语法基线；执行适配、表单、确认、错误与恢复均需分别验收。
本轮逐叶子审计确认：当前 `agent-remote 0.2.58` 的 106 个节点和 `fclaude 0.2.58` 的 12 个节点均存在专用业务页、受管终端 workflow 或能力页 schema 表单入口；generic schema fallback 是合法的图形化覆盖路径。CLI 未声明客户端平台或权限时，桌面保留 `unknown`/`unverified` 状态并要求 CLI 在运行时作最终判断，不把未声明项计作已观测的全平台授权。
除已路由到专用业务页的命令外，每个已发现的叶子命令均可从能力页打开 schema 驱动的通用表单；表单覆盖默认值、必填项、choices、布尔旗标、重复值和 positional 参数，Review 阶段冻结完整 argv 后在受管终端启动。通用表单提供语法级图形化入口，不等同于每个业务命令已经具备专用确认、阶段回执、错误恢复或三平台原生验收。
本机 `device install`、`device uninstall` 和 `device launch` 虽然沿用受管终端表单执行，但属于 home 级本地 Device workflow，不要求先选择项目；未打开项目时桌面会使用受管的 CLI 工具项目作为工作目录，并保留 macOS 平台门禁。该入口由 `tests/app/TestDeviceTerminalWorkflowRunsWithoutWorkspace` 回归覆盖。
专用受管终端入口还会将 CLI 显式声明的 `effect`、`credential` 和 `confirmation` 与 desktop workflow 合同比较；声明不一致的 CLI 会显示为不可用，旧 CLI 省略这些可选元数据时保留兼容行为。
schema 发现阶段还会拒绝重复参数 ID、重复 positional index、非法 min/max 范围以及含空白或 NUL 的参数标识，避免同一命令在图形表单中产生歧义。
已实现流程及验证边界见 [实施状态](implementation-status.md)；本表不将其计为全量命令验收完成。

长操作可选择 `CallWithProgress` 的 version 1 JSONL 流：desktop 通过 `stream_events: true` 接收 CLI 的有界阶段事件（`started`、`running`、`awaiting_input`、`awaiting_confirmation`、`awaiting_approval` 等），并在最终 `result`/`error` 前按顺序展示当前阶段。事件只用于状态投影，不能代替服务端回执；旧 CLI 或未请求流时仍使用单一 terminal envelope。

真实 CLI 的语法覆盖可由根目录测试 `TestRealSchemaCommandsHaveDesktopCoverage` 审计：设置 `DESKTOP_CLI_BINARY` 与 `DESKTOP_FCLAUDE_BINARY` 后，测试会从两个二进制的 `--desktop-schema` 遍历所有叶子命令、可执行的 `forward` 父节点和 `fclaude` 默认入口，要求每项都有能力映射、专用受管终端合同或显式页面路由，并验证发现的参数可由通用表单冻结为 argv。该审计证明语法入口可达，不替代真实服务授权、业务回执和三平台原生验收。

`agent-remote status`、`doctor`、`deps status` 和 `ssh check` 已使用 Rust CLI 提供的 versioned desktop-control 合同，分别路由到结构化状态、Doctor、依赖和 SSH 页面。页面严格校验 schema、operation、home、平台、未知字段和授权上下文；Doctor 与依赖修复先打开双语的准确命令/home 审阅，再由用户明确确认后提交，不会在打开诊断页或点击修复入口时隐式执行。它们不解析终端文本，也不把其他诊断查询冒充为命令结果。

当前仍属于终端回退、没有专用结构化 GUI 的命令包括 ego-browser 的本地安装/注册和设备维护命令，以及 Skills 的库管理变更。Skills 的库变更入口现在明确进入 dedicated reviewed terminal workflow：按 schema 生成并冻结完整 argv，变更命令按 CLI 合同强制 `--yes`，用户在准确 Review 后才启动受管 PTY。`skill info`、`skill status/check/list`、`skill state list/conflicts/info/diff` 均从能力页进入 Skills 专用页面；其中 `skill state diff` 使用 `skills.state.diff` 结构化只读合同，仅展示有界的变更路径、类型、大小和摘要，内容正文、分支输入、原始路径及凭据留在 CLI/API。状态历史中的保留检查点现在提供“导出检查点”入口，按当前 CLI schema 预填准确 checkpoint/account/scope/Skill 标识，随后由通用表单要求用户填写输出目录并审阅冻结 argv；CLI 仍负责保留状态、目录安全和最终导出回执。ego-browser 的 `status`、list、requests/cancel、claim、binding 生命周期和 delete 生命周期已进入专用结构化页面。`node install` 已有专用 Node 页面：通过声明 `credential=user` 的 `nodes.list` 读取受管节点清单，用户筛选并显式选择节点，在冻结 home/Server/device、Node UUID 和 ego-browser intent 后审阅准确 argv；实际 SSH 安装、join code 和管理员权限仍由受管 Rust CLI PTY 执行，桌面不保存凭据或把清单视为安装成功。其它命令仍可从能力页进入 schema 表单或已审阅的受管终端，并按平台、CLI 合同和当前权限状态显示；这些入口不计入专用 GUI coverage。

`ego-browser status` 已新增 `ego_browser.status` v1 结构化只读合同并进入专用页面。CLI 返回本地 Bridge/存储存在性、服务器公开策略、注册设备和绑定的安全摘要；Go 严格校验 schema、operation、home/Server/device 上下文、未知字段、数量上限和枚举后呈现。该状态查询使用 user credential，远端设备登记不代表本地 Bridge 已安装；本地 admission 仍明确显示为 `unknown`，待真实 Bridge 运行态证据补齐。`list`、`requests`/`cancel-request`、`connect`/`claim`、binding 生命周期和 delete 生命周期也通过同一页面承载对应结构化流程；本地安装、注册和其它设备维护命令继续使用受管终端 workflow。

`ego-browser requests` 已进入绑定详情的专用请求页：`ego_browser.requests` 只投影有界请求元数据和 payload 大小，`ego_browser.request.cancel` 使用 user credential、exact-request 确认，并冻结 binding/request generation 与 sequence。CLI 会在取消前刷新绑定和请求清单；代次变化、未知结果或回执字段不完整时 desktop fail closed，不保存浏览器 payload，也不自动重试。ego-browser 的本地安装、注册和其它设备维护命令仍沿用受管终端 workflow；claim、binding 生命周期和 delete 生命周期由结构化页面承载。

`ego-browser` binding 的 `pause/resume/stop/revoke` 已接入 `ego_browser.binding.plan` 与 `ego_browser.binding.control` v1。页面先以 user credential 读取当前 binding 的 generation、状态和允许的动作，用户确认后再提交精确 generation-bound 生命周期请求；Rust 与 Go 均严格校验上下文、binding/device/session/node 标识、未知字段、代次和回执，未知结果保留固定操作语义且不自动重试。页面只投影安全状态摘要，不接收浏览器 payload。

`ego-browser delete-device` 与 `delete-binding` 已接入 `ego_browser.delete.plan` 与 `ego_browser.delete.control` v1。页面仅对已撤销 Device 和已停止/过期/失败/撤销 Binding 显示删除入口；用户先审阅精确目标、状态和 generation，再提交 user credential 的 exact-request。Rust/Go 会拒绝上下文或代次漂移、未知字段和不完整回执，未知结果不自动重试；本地索引只在远端成功回执后清理，回执同时标明本地清理是否完成。Bridge、Server/Node 权限和真实删除联调仍待目标环境。

`ego-browser connect`/`claim` 已进入 `ego_browser.candidates`、`ego_browser.claim.plan` 与 `ego_browser.claim.control` v1。页面读取新鲜的 Claude 会话候选，只投影有界会话、项目、Node 和运行时摘要；认领前冻结候选指纹、精确 tool session、home/Server/device 上下文，并显示全信任浏览器控制警告。CLI 会拒绝本地准入未知/关闭、Bridge 未安装、服务器执行准入关闭、候选绑定冲突和指纹漂移；确认后先关闭本地准入，再调用经过验证的 Device Client。Device Client 返回错误或未能证明 open admission 时结果保持 unknown，桌面不自动重试，也不接收浏览器 payload。真实 Bridge、Server/Node 权限和 macOS 目标环境仍需验收。

`fclaude` 默认入口、`run` 和 `list` 已从能力页路由到远程会话库存：页面读取同一 home、Server、device 下的 `sessions.list`，用户可从准确会话进入 exact attach；默认入口与 `run` 共享“选择已有会话或新建”的工作流。`fclaude new` 直接打开该页面的结构化 `sessions.create` 审核弹窗，提交前校验 account、workspace、project、参数摘要和同步目标，创建回执及跨重启查询仍由 CLI caller-key journal 负责。CLI 缺少对应 capability、上下文未就绪或权限未知时，页面按状态禁用提交；受管原生终端仍保留在工作台用于 Claude 参数透传和首次远程授权。

`fclaude attach` 与旧版 `agent-remote attach` 已接入 sessions.attach v1 原生终端合同：完整会话 UUID、成对 expected-server/expected-device、准确上下文确认及历史刷新；旧历史必须重新选择。能力页中的 schema 入口也强制冻结并审阅这两个身份参数，禁止无身份的 legacy attach；远程页仍负责从当前 home/Server/device 列表选择目标。CLI capabilities 现在额外声明 `attachment_transfer` v1 及 256 MiB/32 项限制；desktop 仅在该声明与 sessions.attach v1 同时存在时，把 mygo 的 `FileData`/剪贴板 payload 送入 fclaude exact 通道。Server/Node 仍为每次上传发放并验证绑定 lease，缺少声明时保持 text-only，不回退 legacy receiver；三仓库真实联调仍待完成。不代表 create/stop/delete 等命令已覆盖。见[连接设计](remote-session-attach.md)。

这两个 attach 入口还要求会话清单读取能力；缺少 `sessions.list` 时会显示对应的 setup、权限或不可用状态，并禁止绕过远程会话库存直接启动旧式 attach。
这两个 attach 入口的命令卡还要求 `sessions.list` 读取能力；即使 exact attach terminal capability 已声明，缺少会话清单权限时也会按权限显示并禁用入口，避免把终端能力误当成可选目标已可见。

当前 `fclaude` schema 的四个会话生命周期叶子（`stop`、`stop-status`、`delete`、`delete-status`）以及两个转发叶子（`forward list`、`forward stop`）均从能力页路由到已有的远程会话或端口转发页面；它们不会退回到未绑定身份的通用 argv 表单。根 `tests/` 的 CLI fixture 与 0.2.58 schema 保持 12 个节点，并逐一回归这些入口的页面路由。

能力页不再把 `fclaude run/new/list` 误呈现为通用 argv 表单：`run`/默认入口进入远程会话库存，`new` 进入结构化创建审核，`list` 进入只读清单。工作台仍允许用户直接启动受管 fclaude 终端，以保留 Claude 参数透传、首次授权提示及 CLI 原生交互；终端输出不会被伪造成桌面回执。

工作台中直接启动的 fclaude 终端历史只表示本地 pane、项目、cwd、命令标签和生命周期摘要；desktop 不从终端输出、默认账户或列表首项推断远程身份。通过结构化 create 或 exact attach 返回完整上下文后，历史才具备自动恢复所需的身份条件；旧的无上下文历史仍要求刷新列表并由用户重新选择、确认。
`ssh check` 已声明只读专用终端能力：桌面冻结完整命令和可选的精确会话 UUID，在受管终端中执行本机 SSH 版本检查及会话授权观察；不把 SSH 输出、授权命令或凭据写入桌面状态。该入口保留普通 CLI 的失败语义，并可在操作中心跨重启恢复安全的生命周期摘要。
`doctor` 已接入专用终端能力：桌面显示本地安装、配置、凭据、依赖和服务器检查的双语说明，冻结完整 `agent-remote doctor [--fix]` argv，并要求精确 Review 后在受管终端执行。`--fix` 可能创建或刷新本地元数据，桌面不会自动添加或重试该参数；CLI 仍负责实际检查、凭据访问、权限和失败语义。该流程不把只读 `diagnostics.inspect` 误当作 `doctor` 的替代，也不把能力声明当作检查成功证明。
`init` 在能力页中路由到组合初始化向导，而不是通用 argv 表单；向导按本地初始化、登录/设备登记、可选 WireGuard 计划与确认、诊断和 readiness 的固定顺序执行，并分别展示每一步的错误与未知结果。
`fclaude stop`/`stop-status` 的远程停止流程已接入 `sessions.stop` 与 owner-scoped 原 key 查询；桌面冻结会话、节点和后端身份，分别显示进程终止、Skill 保存和同步结果。桌面还严格核对 CLI capability 的 device 凭据与 exact-request 确认声明；缺少完整节点/后端身份的请求会在提交前拒绝。未知回执只能查询原 key，不能自动重试；该流程仍需跨 Server/Node/CLI 联合门禁后计入正式版验收。
`fclaude delete <id>` 与 `delete --all` 已接入固定集合删除：桌面先审阅完整终态 UUID 集合，且只允许 Claude 的 `failed`、`interrupted`、`stopped` 状态；Rust CLI 以原 key 提交并持久化，Server 返回包含 `reason=user_requested` 和完整运行时上下文的稳定回执。桌面严格核对 device 凭据与 exact-request capability，集合变化、Skill retention 阻塞及未知结果均保留原 key，只允许查询。桌面重启时只恢复 home/Server/device、排序 UUID、selection digest 和 intent fingerprint，显式入口查询原 key，并再次匹配回执集合，绝不自动重发或扩大删除范围。该流程的跨仓专项门禁已通过，三平台原生运行和正式版全量命令验收仍待完成。
能力页的结构化权限清单同时列出 `sessions.create.request`、`sessions.stop`/`delete` 及其原请求状态、`port_forwards.list/create/stop/reconnect` 和 `device.status/revoke/rotate_token`；每项按 CLI 声明的 effect、credential、exact-request 确认和当前 home/Server/device 条件评估。命令语法发现仍不等同于服务端授权或真实平台验收。
`agent-remote forward`/`fclaude forward`、`forward list`、`forward stop` 对应的端口转发流程已接入 `port_forwards.list/stop/reconnect`；CLI 仍提供受控的 `port_forwards.create` 给其他 desktop-control 调用方。桌面创建时由 Go 启动受管的 Rust `forward --json` 子进程；Rust 在真正绑定 `127.0.0.1` 后发出无秘密 startup event，并持续持有 listener、SSH/HTTP2 tunnel、TTL 与 bounded reconnect。桌面跨重启恢复使用隐藏的 `forward --forward-id <full-uuid>`，只对既有 ID 请求一次连接授权并恢复 listener，不重复 create、不推断最新 forward。Go 只接收安全投影，停止时先走正常 CLI 清理路径，超时才结束子进程；显式退出先等待启动 worker 再快照 runner，并等待异步停止 child，取消与 startup event 竞争时不发布新 listener。transport unknown 保留原 key 并禁止自动换 key，重连必须显式执行。三平台原生安装与真实网络仍需平台验收。
能力页还将带有可选 `list`/`stop` 子命令的 `agent-remote forward` 与 `fclaude forward` 父节点显示为可操作命令；父节点本身执行 create，直接进入同一上下文绑定的端口转发页，避免只显示子命令而隐藏创建入口。
能力页发现到的 `fclaude stop`、`stop-status`、`delete`、`delete-status`、`forward`、`forward list` 和 `forward stop` 不会打开 schema 通用 argv 表单。它们分别路由到远程会话管理或网络端口转发页；页面冻结会话、节点、后端和原请求键，并在 CLI 缺少对应 `sessions.*`/`port_forwards.*` 能力时显示明确的禁用原因。命令目录仍覆盖全部 CLI 语法，但不会把未绑定上下文的终端命令误认为完整的停止、删除或转发业务流程。
`account default get/set/clear` 已有共享状态的图形化流程，见[默认账户契约](../../agent-remote-cli/docs/desktop-account-defaults.md)。
`credentials list` 已接入 `credentials.list` 只读控制契约和独立凭据页：展示配置档案名称、状态、GitHub CLI/SSH 集成模式及更新时间；CLI 投影会去除 `user_id`、Git 身份原文和未来未知字段。`credentials create/bind/unbind` 已接入独立的精确上下文、确认和回执合同；创建只提交 profile 元数据，设备 token 由 CLI 安全存储读取，未知字段（包括 token、私钥等）会在 Rust 边界拒绝。桌面不把凭据参数放入通用 argv，也不保存 token 或原始秘密。
能力页的命令覆盖列表会将这三个敏感命令的入口路由到同一凭据专用页；不会因为通用表单的敏感字段门禁而显示为不可达命令。
能力页现在也将登录/注销、账户生命周期与配置导入命令导向远程账户清单，将运行时恢复、默认账户、同步、Mutagen 运行环境、WireGuard 配置/网络、设备撤销与令牌轮换、Skills 清单/冲突分别导向现有专用页面；`fclaude attach` 进入远程会话清单并在完整上下文确认后连接。远程页面加载期间保留待处理路由，加载失败时不会伪造可用的空表单。其余已有专用受管终端命令继续使用 schema 驱动的精确 argv Review。
`skill list` 已接入 `skills.list` 用户凭据只读工作流：CLI 通过用户安全会话读取目录，desktop 只接收代次、名称、启用/包含状态、解析版本、项目发现和模型加载状态，以及账户本地 Skill 的安全摘要。查询显式绑定 home/Server/device 上下文，范围不能同时指定 tool 与 account；上下文变化、未登录、未知字段和坏目录均 fail closed。内容、来源 locator、tracking、revision metadata 与凭据不会跨 control boundary。桌面 Skills 页支持刷新、名称/ID 筛选和中英文状态展示；启用/停用已由独立 `skills.change` 专用流程处理。
`skill info` 现在进入同一页面的专用详情面板，通过 `skills.info` v1 只读合同按精确 Skill 标识和可选 tool/account 范围查询。Rust 只投影类型、名称、代次、修订 ID/保留状态、生效规则和模型/发现状态；source locator、tracking、revision metadata、路径、内容及凭据仍留在 CLI/API。Go 在调用前后绑定当前 home/Server/device、拒绝模糊范围和未知字段；上下文漂移或能力缺失会明确停止。
Skills 页现在还提供受控的启用/停用变更：用户先审阅 Skill、动作和冻结的 catalog generation，再由 Rust `skills.change` 以用户凭据提交精确请求；桌面仅接收上下文绑定的公共回执并显示操作编号。CLI capability 缺失、代次冲突、上下文变化、非规范 Skill/account ID 或未确认请求均 fail closed。Skill 内容、来源、tracking、revision metadata、节点目标和凭据不会进入 desktop-control。`skill add/update/enable/disable/pin/unpin/inherit/remove/rollback` 已声明专用 terminal capability：桌面按 schema 生成并冻结完整 argv，变更模式强制注入 `--yes`（仅在该旗标存在于 schema 时），完成精确 Review 后才在受管终端执行；`skill add --list` 保留为同一受管终端下的只读发现模式，用户可清除 `--yes` 后审阅。CLI 仍负责来源校验、用户凭据、远程回执和失败恢复，操作中心只投影生命周期状态。它不等同于真实 Server/Node 授权或三平台原生验收。
`skill state prune/reset/migrate/resolve/restore` 使用 CLI 声明的专用 terminal capability，强制 `--yes` 并在托管终端启动前冻结完整 argv。`migrate`、`resolve`、`restore`、`conflicts` 能力还声明稳定的 `workflow` 标签；Skills 冲突页会把冲突 ID（以及迁移的 Skill/account 范围）预填到对应专用审阅表单，用户仍需补齐 CLI 要求的分支、路径或 checkpoint 参数并确认完整 argv。路径、保存的分支输入、digest、来源和内容不会跨 desktop-control。`skill state list` 现在使用 `skills.state.list` v1 结构化只读合同，按账户目录或单 Skill 查询 checkpoint 与 pending-finalization 分页，并只投影代次、digest、保留/head、存储位置、状态和时间等安全元数据；`skill info` 使用 `skills.info`，`skill state info` 使用 `skills.state.info`，`skill state diff` 使用 `skills.state.diff`，按精确 checkpoint 分页展示变更路径、类型、大小和摘要。账户 UUID、范围、limit 和两个分页游标严格绑定当前上下文；差异查询拒绝路径穿越、未知字段、内容正文和上下文漂移。`skill state export` 仍声明只读专用 terminal capability；desktop 在冻结 argv 时还拒绝输出目录的 NUL、缺失值和 `..` 父路径穿越，CLI 继续负责目录存在性、空目录、真实目录和可写父目录检查。`skill retry` 已使用用户凭据接入 `skills.retry.plan/control` v1：先读取失败目标和代次/指纹，再由用户精确确认并提交固定 `--yes`，过期计划或未知回执均 fail closed。CLI 负责最终回执、服务端权限和失败语义，操作中心只投影本次受管终端生命周期。desktop 现在跨重启保存固定命令标签、生命周期状态和 attention 摘要，但不保存 argv、终端输出、会话 ID 或凭据；pending/running 重启后统一变为 attention，精确回执仍只能从 CLI durable journal 查询，desktop 不自动重放。
Skills 页也提供只读 `skills.state.conflicts` 检查：用户输入精确账户 UUID 后，Rust CLI 使用用户会话分别读取发布冲突和迁移冲突页面；Go 只接收上下文绑定的状态、代次和安全元数据摘要。路径、保存的分支输入、digest、来源和解析选择不会跨 desktop-control；账户目录与单 Skill 选择互斥，分页游标和未知字段均严格校验。每个冲突现在提供解析审阅入口；迁移冲突额外提供迁移和恢复审阅入口并预填 Skill/account（checkpoint 仍需用户输入并由 CLI 验证归属）。这些入口都要求用户补齐敏感选择并确认冻结 argv，CLI 负责最终回执、权限和失败恢复，不自动解析、恢复或重试。
`node install` 与 `ego-browser` 命令族使用专用 terminal workflow：Rust capability 查询声明每条精确路径及协议版本，Go 根据声明和平台显示/禁用入口，冻结 schema 生成的 argv，并对变更命令固定 `--yes` 后要求精确 Review。命令仍由受管 PTY 执行，CLI 在运行时负责设备/服务器权限、失败码和最终结果；操作中心只投影运行中/完成/失败/attention；desktop 跨重启只保存固定命令标签、状态和时间，不保存 argv、路径、会话 ID、凭据或终端输出，也不会伪造 CLI receipt 或自动重放。
能力页同时要求 `node install` 的 `nodes.list` 目标选择能力和 `agent-remote node install` 的 terminal capability；混合版本中只提供前者时，入口明确显示 `cli_unsupported` 并禁用启动，不把节点清单权限误当成安装权限。
`device install/uninstall/status/launch/diagnose` 也使用 macOS 专用 terminal workflow：能力声明逐条列出精确命令路径，Go 在非 macOS 上禁用入口并冻结 schema 生成的 argv；`uninstall` 强制注入 CLI 已声明的 `--yes`，`install` 与 `launch` 以桌面精确 Review 作为确认边界，`status` 与 `diagnose` 保持只读。能力页中的 `device status` 与 `device diagnose` 都路由到 Device 结构化页面，先展示 `device.local.status` 和 `device.local.plan` 的有界观察，再保留准确 terminal workflow 的平台/权限门禁；不会让诊断命令退回未绑定上下文的通用 argv 表单。Device 页面现在先读取 `device.local.plan` v1 只读合同，展示 install/uninstall/launch/diagnose 的有界阶段、准确 terminal method、确认/破坏性标记及 `inspect_before_retry` 恢复策略；计划不会执行本地变更，也不读取凭据。CLI 仍负责本地签名、Gatekeeper、XPC、凭据和进程检查，desktop 不把能力声明或计划当作已安装或已授权证明。
`device status/revoke/rotate-token` 已接入 Device 专用页。页面先读取同一 home、服务器和当前设备的能力与在线状态，再以精确上下文审阅撤销或令牌轮换；撤销使用 user credential，轮换使用 user credential 并由 CLI 写入设备安全存储。设备令牌、用户令牌和本地路径不会进入回执；远端成功但本地清理或凭据保存不完整时，回执明确标记局部状态，传输未知结果保留 `device_outcome_unknown`，桌面不自动换 key 重试。通用 argv 表单不会承载这些操作。

Device 专用页同时读取 `device.local.status` v1 只读合同，展示固定用户 Applications 路径下本地 macOS Device app 的安装、社区签名、Gatekeeper、Network Broker XPC、GUI Executor XPC 与进程状态。该合同只投影安全摘要，Windows/Linux 返回结构化 `platform_unsupported`；安装、卸载、启动和诊断继续使用受管终端流程。
`account create/status/bind/verify/disable` 已接入确认与受理回执；`account attach` 使用准确绑定批次的原生终端契约，并在能力目录绑定 `accounts.binding.attach` terminal capability，缺少该声明时按不可用显示。见[账户协议](../../agent-remote-cli/docs/desktop-accounts.md)。
`account recover-runtime/recovery-status` 已接入管理员身份检查、准确原任务观察、三种恢复动作确认、原编号查询及 `--forget --yes` 对应的显式本地记录清理。用户凭据与设备凭据分离；CLI 是唯一 durable registry，手工旧编号只读查询不产生提交计划。三平台原生桌面及部署验收仍待完成，见[运行时恢复协议](../../agent-remote-cli/docs/desktop-runtime-recovery.md)。
`account import-config` 已有本地发现/读取清单、可选 Server 审计校验、独立上传确认、原请求查询及已结束本地记录清理。CLI 是跨重启恢复的唯一 journal；打开“配置导入记录”时 Go 自动查询 `accounts.import.pending`，完整 manifest 不进入确认请求，文件内容不进入 Go。未知结果不重试、不以 latest 猜测。Windows/Linux 原生及部署验收仍待完成，见[导入协议](../../agent-remote-cli/docs/desktop-account-import.md)。
`sync ensure/status/pause/resume/resolve/reset` 已有选定项目的确认/状态/阶段回执流程；dry-run 仍修改 Server/SQLite。另有 `sync.recovery.preview/apply` 的旧映射恢复，确认后仅清理准确本地关系，新关系另行审阅。macOS→Docker Linux 的真实 Node/SSH/Mutagen 链路已通过；这些证据不替代三平台原生桌面完整验收。见[同步协议](../../agent-remote-cli/docs/desktop-sync.md)。
`deps mutagen-environment` 已有独立运行环境页面，通过 `mutagen.environment.preview/apply` 检查全部 sync/forward 会话，确认摘要后重启共享服务并保留暂停状态；不需要项目或登录。外部服务注册自动迁移及 Windows/Linux 原生行为尚待验收。
`deps status`（包括可选的 `--fix`）统一路由到命令诊断结构化页面：打开命令只执行 `deps.inspect` 只读检查；用户在当前 home 与版本检查选项明确显示后，再单独点击修复动作才提交 `deps.repair`，该提交包含精确 home 与确认字段，等价于 CLI 的 `--fix`。页面加载或刷新不会隐式执行修复，也不会把修复参数塞入通用 argv。`dependencies.inspect` 继续作为旧诊断页提供兼容的只读投影。
诊断页仍提供独立的“依赖管理”只读页面，直接调用 `dependencies.inspect`，展示 CLI 平台、准确 home、manifest 来源、每项路径/存在状态/版本结果及许可证。页面的版本检查显式传递 `verify_versions`，可取消且不安装、修复或猜测权限；`dependencies.inspect` 的 `credential: none` 只按 CLI 返回的观察结果显示，不把文件存在当作执行或管理员权限证明。
`wireguard.bridge.status` 提供无提权的安装、系统批准、协议兼容和调用方认证状态，并在当前未安装受保护 broker service 时明确返回不可执行的 `unsupported`/`unavailable`；它不会把进程 elevation 或工具存在当成权限。`wireguard.tunnel.plan` 提供配置/工具快照、原生目标与未满足条件。`wireguard.tunnel.apply` 要求严格确认与完整计划绑定，重查通过后由 Rust 在用户权限下读取配置字节和摘要，经固定平台 broker client 发送；broker 必须返回绑定 operation receipt，随后再次观察目标并仅在状态匹配时返回回执。没有 sudo、runas 或普通 helper fallback；三平台原生 service 安装、授权、执行和运行环境仍需分别验收。详见[隧道契约](wireguard-tunnel-contract.md)。
`init` 的桌面组合向导按固定顺序编排已有控制方法：`setup.local`、`login.password` 或 `login.device_code`（包含可选设备登记）、可选 `wireguard.config.plan/apply`、`diagnostics.inspect`，最后执行 `context`、`accounts.list` 和 `sessions.list` readiness 查询。WireGuard plan 与 apply 分成两个用户动作；任何 transport unknown 都不会自动重放。该组合层不新增 Rust 协议，也不把通用 `init` schema 表单当作安全初始化替代品。

通用表单的能力门禁规则如下：命令未声明客户端平台时显示 `unknown`，不推断全平台支持；声明不匹配当前平台时禁用；已知能力会结合同一配置环境中的协议、上下文、组件和已观察权限显示 `available`、`setup_required`、`device_required`、`sign_in_required`、`denied`、`unsupported` 或 `unavailable`；未建立映射的命令仍允许用户明确尝试，CLI 负责最终授权。任何 review 后的 argv 都不会再次读取可变表单状态，也不通过 shell 拼接或自动重试。

通用表单不会暴露 CLI 全局 `--home` 覆盖项；桌面始终通过受管环境绑定当前配置目录，避免命令在 Review 后切换到另一套凭据或能力上下文。无项目也能独立运行的本地/配置命令（例如 `status`、`doctor`、`deps status`、Skills、Node 和 ego-browser）会在需要时使用桌面专用的 CLI 工具项目；工作台直接启动的 Agent/fclaude 终端仍要求用户先选择项目，远程会话创建弹窗则要求用户审阅服务端 workspace/project 与同步目标。

Schema 驱动表单在冻结 Review argv 前拒绝 NUL 值；这与受管 dedicated workflow 的同一边界保持一致，避免把操作系统无法表示的参数值延迟到进程启动阶段才失败。参数仍按 CLI schema 负责必填、重复值、choice 和 positional 校验，桌面不通过 shell 拼接命令。

命令目录中的结构化入口也沿用同一映射：Mutagen 环境、WireGuard、账户生命周期/默认账户、凭据、设备撤销/令牌轮换以及注销命令会显示对应 control capability 的当前状态；一个 CLI 命令包含多个阶段时（例如 `account import-config`、`wireguard config`），目录门禁使用首个只读计划能力，进入专用页后每个写入阶段仍重新检查自己的精确能力。

包含 password、token、secret、TOTP、设备码或私钥字段的命令会从通用 argv 表单中禁用。能力页会提示使用对应的专用安全流程，避免凭据进入命令行参数、冻结审阅文本或日志；专用流程由 Rust CLI 的受控输入通道负责。

平台声明仅针对当前命令在客户端操作系统上的实现；未声明表示未知，不继承父命令，也不代表权限、组件或 GUI 已就绪。

`ego-browser` 的旧版命令 schema 可能没有逐命令 `platforms` 字段。桌面端对其 reviewed terminal workflow 采用同一来源的运行约束：Bridge 与 Device Client 的社区组件仅支持 macOS，因此能力页和通用表单会显示 macOS 门禁并在 Windows/Linux 禁用入口；这不会把远端 Node 的操作系统误当成本机平台。`ego-browser register` 的 CLI schema 没有 `--yes`，该路径使用 desktop 的精确 Review 作为确认边界，不伪造 CLI 不接受的旗标；其它声明 `--yes` 的变更路径仍由 schema 驱动自动固定该旗标。

<!-- BEGIN GENERATED CLI INVENTORY -->

| 程序 | 版本 | 命令节点 | 参数绑定 |
| --- | --- | ---: | ---: |
| agent-remote | 0.2.58 | 106 | 628 |
| fclaude | 0.2.58 | 12 | 94 |

## agent-remote

| 命令路径 | 类型 | 参数数 | 客户端平台声明 |
| --- | --- | ---: | --- |
| `（默认入口）` | 命令组 / 默认入口 | 3 | 未声明 |
| `skill` | 命令组 / 默认入口 | 3 | 未声明 |
| `skill state` | 命令组 / 默认入口 | 3 | 未声明 |
| `skill state prune` | 操作 | 11 | 未声明 |
| `skill state reset` | 操作 | 10 | 未声明 |
| `skill state migrate` | 操作 | 11 | 未声明 |
| `skill state resolve` | 操作 | 12 | 未声明 |
| `skill state restore` | 操作 | 11 | 未声明 |
| `skill state list` | 操作 | 9 | 未声明 |
| `skill state conflicts` | 操作 | 9 | 未声明 |
| `skill state info` | 操作 | 7 | 未声明 |
| `skill state diff` | 操作 | 10 | 未声明 |
| `skill state export` | 操作 | 9 | 未声明 |
| `skill add` | 操作 | 15 | 未声明 |
| `skill check` | 操作 | 4 | 未声明 |
| `skill update` | 操作 | 12 | 未声明 |
| `skill enable` | 操作 | 10 | 未声明 |
| `skill disable` | 操作 | 11 | 未声明 |
| `skill pin` | 操作 | 11 | 未声明 |
| `skill unpin` | 操作 | 10 | 未声明 |
| `skill inherit` | 操作 | 11 | 未声明 |
| `skill remove` | 操作 | 8 | 未声明 |
| `skill rollback` | 操作 | 9 | 未声明 |
| `skill list` | 操作 | 10 | 未声明 |
| `skill info` | 操作 | 6 | 未声明 |
| `skill status` | 操作 | 8 | 未声明 |
| `skill retry` | 操作 | 8 | 未声明 |
| `init` | 操作 | 12 | 未声明 |
| `login` | 操作 | 12 | 未声明 |
| `logout` | 操作 | 4 | 未声明 |
| `status` | 操作 | 4 | 未声明 |
| `doctor` | 操作 | 4 | 未声明 |
| `deps` | 命令组 / 默认入口 | 3 | 未声明 |
| `deps status` | 操作 | 4 | 未声明 |
| `deps mutagen-environment` | 操作 | 4 | linux, macos, windows |
| `wireguard` | 命令组 / 默认入口 | 3 | 未声明 |
| `wireguard config` | 操作 | 4 | linux, macos, windows |
| `wireguard check` | 操作 | 5 | linux, macos, windows |
| `wireguard status` | 操作 | 3 | linux, macos, windows |
| `wireguard up` | 操作 | 5 | linux, macos, windows |
| `wireguard down` | 操作 | 5 | linux, macos, windows |
| `ssh` | 命令组 / 默认入口 | 3 | 未声明 |
| `ssh check` | 操作 | 4 | 未声明 |
| `sync` | 命令组 / 默认入口 | 3 | 未声明 |
| `sync ensure` | 操作 | 6 | 未声明 |
| `sync status` | 操作 | 5 | 未声明 |
| `sync pause` | 操作 | 5 | 未声明 |
| `sync resume` | 操作 | 5 | 未声明 |
| `sync resolve` | 操作 | 5 | 未声明 |
| `sync reset` | 操作 | 5 | 未声明 |
| `account` | 命令组 / 默认入口 | 3 | 未声明 |
| `account recover-runtime` | 操作 | 8 | 未声明 |
| `account recovery-status` | 操作 | 7 | 未声明 |
| `account list` | 操作 | 4 | 未声明 |
| `account create` | 操作 | 9 | 未声明 |
| `account bind` | 操作 | 4 | 未声明 |
| `account attach` | 操作 | 7 | 未声明 |
| `account import-config` | 操作 | 12 | 未声明 |
| `account verify` | 操作 | 4 | 未声明 |
| `account status` | 操作 | 4 | 未声明 |
| `account disable` | 操作 | 4 | 未声明 |
| `account default` | 命令组 / 默认入口 | 3 | 未声明 |
| `account default set` | 操作 | 5 | 未声明 |
| `account default get` | 操作 | 4 | 未声明 |
| `account default clear` | 操作 | 4 | 未声明 |
| `credentials` | 命令组 / 默认入口 | 3 | 未声明 |
| `credentials list` | 操作 | 4 | 未声明 |
| `credentials create` | 操作 | 8 | 未声明 |
| `credentials bind` | 操作 | 5 | 未声明 |
| `credentials unbind` | 操作 | 4 | 未声明 |
| `device` | 命令组 / 默认入口 | 3 | 未声明 |
| `device install` | 操作 | 4 | macos |
| `device uninstall` | 操作 | 4 | macos |
| `device status` | 操作 | 3 | macos |
| `device launch` | 操作 | 3 | macos |
| `device diagnose` | 操作 | 3 | macos |
| `device revoke` | 操作 | 5 | 未声明 |
| `device rotate-token` | 操作 | 4 | 未声明 |
| `node` | 命令组 / 默认入口 | 3 | 未声明 |
| `node install` | 操作 | 6 | 未声明 |
| `ego-browser` | 命令组 / 默认入口 | 3 | 未声明 |
| `ego-browser setup` | 操作 | 4 | 未声明 |
| `ego-browser connect` | 操作 | 5 | 未声明 |
| `ego-browser repair` | 操作 | 4 | 未声明 |
| `ego-browser upgrade` | 操作 | 4 | macos |
| `ego-browser remove` | 操作 | 5 | 未声明 |
| `ego-browser forget-this-mac` | 操作 | 5 | 未声明 |
| `ego-browser re-enroll` | 操作 | 4 | 未声明 |
| `ego-browser device-rotate` | 操作 | 4 | 未声明 |
| `ego-browser switch-server` | 操作 | 5 | 未声明 |
| `ego-browser register` | 操作 | 5 | 未声明 |
| `ego-browser status` | 操作 | 5 | 未声明 |
| `ego-browser list` | 操作 | 4 | 未声明 |
| `ego-browser requests` | 操作 | 5 | 未声明 |
| `ego-browser cancel-request` | 操作 | 6 | 未声明 |
| `ego-browser claim` | 操作 | 5 | 未声明 |
| `ego-browser pause` | 操作 | 6 | 未声明 |
| `ego-browser resume` | 操作 | 6 | 未声明 |
| `ego-browser stop` | 操作 | 6 | 未声明 |
| `ego-browser revoke` | 操作 | 6 | 未声明 |
| `ego-browser delete-device` | 操作 | 5 | 未声明 |
| `ego-browser delete-binding` | 操作 | 5 | 未声明 |
| `attach` | 操作 | 6 | 未声明 |
| `forward` | 命令组 / 默认入口 | 8 | 未声明 |
| `forward list` | 操作 | 3 | 未声明 |
| `forward stop` | 操作 | 6 | 未声明 |

## fclaude

| 命令路径 | 类型 | 参数数 | 客户端平台声明 |
| --- | --- | ---: | --- |
| `（默认入口）` | 命令组 / 默认入口 | 6 | 未声明 |
| `run` | 操作 | 6 | 未声明 |
| `new` | 操作 | 6 | 未声明 |
| `list` | 操作 | 14 | 未声明 |
| `attach` | 操作 | 9 | 未声明 |
| `stop` | 操作 | 7 | 未声明 |
| `stop-status` | 操作 | 8 | 未声明 |
| `delete` | 操作 | 7 | 未声明 |
| `delete-status` | 操作 | 8 | 未声明 |
| `forward` | 命令组 / 默认入口 | 10 | 未声明 |
| `forward list` | 操作 | 5 | 未声明 |
| `forward stop` | 操作 | 8 | 未声明 |

<!-- END GENERATED CLI INVENTORY -->
