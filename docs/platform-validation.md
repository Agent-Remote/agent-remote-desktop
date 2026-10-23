# 平台验证矩阵

记录日期：2026-10-23。正式版目标为 Windows、macOS、Linux，FreeBSD 不在计划中。本表区分已取得的证据和发布前仍需完成的运行验收，不把交叉构建或测试夹具当作实际产品验收。

正式发行矩阵已经收敛为三个 artifact triple：`windows/amd64`、`darwin/arm64`、`linux/amd64`。Windows ARM64、macOS Intel 和 Linux ARM64 可以做兼容性 smoke，但在取得对应真实主机窗口、终端和安装证据前不属于正式支持范围。Linux 正式验收固定记录 Ubuntu/Debian 发行版版本，并分别覆盖 X11 与 Wayland；其它发行版或桌面组合按 best-effort 记录，不得用 Xvfb 结果替代真实显示环境。`verify-native-evidence.sh` 默认验证三平台的正式架构；发布前再传入 `--require-linux-display-pair`，让同一证据合并器要求两个 Linux 目录分别声明 `display_protocol=x11` 与 `display_protocol=wayland`。

工作台的性能指标通过 `App.MotionStats` 暴露最近 512 个 Go/mygo View 构建样本的 nearest-rank P95；16ms 是 60Hz 的构建预算。该读数只覆盖 Go/mygo 构建阶段，不能替代原生 GPU、窗口调度、PTY 渲染或输入反馈测量。正式性能验收仍需在真实主机记录 10 个活跃会话、4 个可见终端、输入反馈 P95<50ms 和接近 60 FPS，并按平台、DPI、主题切换和 Reduce Motion 分开采样。

## 最近复验

2026-10-21 增量：Windows WebView2 选择器已按 loader 通道优先级和同通道版本排序固定；Unix 安装器在关键 rename 后执行父目录同步；WireGuard broker replay journal 已接入 macOS/Linux/Windows 的持久恢复与 fail-closed 检查。当前 macOS arm64 的五个主机探针通过；WireGuard broker 因未提供真实 CLI 明确记为 `status=not_run`。Windows/Linux 仍需各自主机完成原生窗口、托盘、PTY、安装和受保护 broker 的运行验收。

CI 的三平台 host runner 现在由独立 `verify-native-evidence` job 汇总：该 job 下载 `native-acceptance-macos-latest`、`native-acceptance-windows-latest` 和 `native-acceptance-ubuntu-latest` 三份 artifact，并执行 `scripts/verify-native-evidence.sh`。测试矩阵显式把三个 runner 绑定到 `darwin`、`windows`、`linux`，并把目标传入 Bash/PowerShell 验收入口；目标映射变化会由 release 回归测试拦截，而不是由汇总阶段事后猜测。Ubuntu runner 的 UI 使用 Xvfb，CI 汇总显式传入 `--allow-virtual-display`，所以结果仅用于审阅主机依赖和探针完整性；正式版仍必须提供真实显示环境的 Linux 证据。artifact 缺失、探针失败、目标/版本不一致或被标记为交叉构建都会使汇总 job 失败。

2026-10-23 本机 macOS arm64 已通过 `DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh`（当前 1169 条中英文 catalog）、`go test ./... -count=1`、`go test -race ./tests/release ./tests/remote ./tests/session ./tests/storage ./tests/theme -count=1`、三目标 desktop 交叉构建和 `scripts/native-acceptance.sh` 的 platform、installer、native-window、tray、workbench 探针。Rust CLI 结构化诊断合同与完整相关测试通过。Windows/Linux 仍只有交叉构建和可在目标主机执行的验收脚本，不能据此宣称原生运行、安装或权限服务验收完成。复用 evidence 目录时，native acceptance 入口会清理本轮日志和 WireGuard 子目录，避免旧回执污染新一轮结果。

同日追加 macOS arm64 原生验收临时目录 `/tmp/agent-remote-desktop-native-current-2026-10-23-final4.EnGwvV`；五个探针均通过，覆盖远程创建同步目标合同和安装后 `.app` 签名复核。WireGuard broker 仍因未提供真实 CLI 保持 `not_run`，Windows/Linux 真实主机证据仍待目标环境提供。

同日 Linux/amd64 smoke release `/tmp/agent-remote-desktop-release-linux-final-20261023e` 已通过构建和离线 verifier；它只证明制品清单、哈希、安装器元数据及社区签名策略符合合同，不能替代真实 Linux 图形会话或正式三平台发布。

本轮三目标候选 smoke `/tmp/agent-remote-desktop-release-candidate.gwwfXX` 已通过构建、离线 verifier 和当前 macOS 安装后 smoke；包含 `darwin/arm64`、`windows/amd64`、`linux/amd64` 共 18 项制品。版本仍为 `0.1.0-dev`，macOS 使用显式 `ad-hoc/smoke`，不能作为正式社区自签发布证据。

当前机器已发现项目社区自签身份 `Agent Remote Community Code Signing`。使用该身份生成的三目标候选 `/tmp/agent-remote-desktop-community-candidate.Dtxay3` 已通过 macOS bundle 签名、离线 manifest 校验和安装后 smoke；它仍是 `0.1.0-dev` 候选，不能替代正式 tag 和 Windows/Linux 真实主机验收。

另有 Docker `linux/arm64` + Xvfb runtime smoke：`/tmp/agent-remote-desktop-native-linux-docker-20261023` 的 GTK 3、AppIndicator、PTY、安装器、托盘和 workbench 探针通过。该结果用于验证 Linux 原生依赖在隔离容器中的行为，不作为正式 amd64 或真实 X11/Wayland 主机证据。

Exact fclaude 的文件/图片 payload 现在由 desktop 在 native transfer offer 前执行 CLI 声明的 `max_bytes`/`max_entries`：拖放路径必须是当前存在的常规绝对文件，symlink、目录、超限条目或总大小都会 fail-closed；精确终端的原生剪贴板附件也遵循同一限制，超限内容不会回退为文本。该检查不替代 CLI 在 Server/Node 租约和上传时的第二次校验，真实三仓库附件联调仍按下方范围单独验收。

## 社区版构建基础

桌面仓库现在提供可复用的 [`scripts/build-release.sh`](../scripts/build-release.sh)。默认以 mygo 固定版本构建 `darwin/arm64`、`windows/amd64` 和 `linux/amd64`，并生成 `artifacts/<goos>-<goarch>/`、`metadata/release-manifest.json` 与 `SHA256SUMS`。清单记录 Go/mygo 版本、模块哈希、mygo 原生插件清单、Ghostty 原生库的来源与上游 SHA-256，以及构建机找到的 `agent-remote`、`fclaude`、`codex` 和 `claude` 版本/哈希；这些 CLI 是外部运行依赖，不会被桌面制品静默复制。

构建完成后会自动调用 [`scripts/verify-release.sh`](../scripts/verify-release.sh)。它在离线环境复核清单与 `SHA256SUMS` 的完整集合、文件大小、字节哈希和目标目录中的未列出文件，并要求每个目标同时包含 desktop 可执行文件和 `agent-remote-desktop-installer`（Windows 为 `.exe`）；任意占位文件无法通过。它还强制检查 macOS 社区自签、Windows/Linux 无发行者代码签名及关闭公证。Linux Debian 包还通过 `mygo.json` 声明 `libayatana-appindicator3-1 | libappindicator3-1`，与应用实际托盘后端保持一致；这项元数据不能替代目标主机的 AppIndicator ABI 和托盘探针。非 macOS 主机只能记录 macOS bundle 签名待在原生主机复核，不会把交叉构建当成签名证据；`DESKTOP_ALLOW_ADHOC=1` 的 smoke 目录必须显式使用 `--allow-adhoc`，不能作为正式版。桌面门禁还会拒绝放在根 `tests/` 之外的 Go `_test.go`，避免测试与主程序目录重新混放。

远程闭环联调先使用 [`scripts/remote-interop-acceptance.sh`](../scripts/remote-interop-acceptance.sh)：`--contracts` 只验证 Server/Node/CLI/Desktop 协议夹具，`--host-only` 验证当前主机实际 Go→Rust capabilities 与二进制身份并明确写入 `remote_execution=not_run`，只有 `--live` 且提供 disposable session/device 才能记录真实 Server/Node/SSH attachment；Mutagen 原生 probe 需显式提供固定二进制。详细边界见[远程闭环联调验收](remote-interop-acceptance.md)。

三平台原生证据在发布前使用 [`scripts/verify-native-evidence.sh`](../scripts/verify-native-evidence.sh) 汇总。它只接受每台主机 native acceptance 的通过记录，核对正式架构、`cross_compilation=false`、主机目标、解析出的 mygo 模块版本与元数据一致及五个完整探针日志（含真实托盘创建/销毁），并默认要求真实主机显示。Linux 记录还必须声明 `display_protocol=x11` 或 `display_protocol=wayland`；Xvfb 只能声明为 X11 的虚拟审阅记录。正式发布命令增加 `--require-linux-display-pair` 后，合并器允许两个 Linux 目录并强制两种协议各出现一次，从而落实 X11/Wayland 双环境要求。它还会解析 `platform.log` 中的结构化 `agent-remote-desktop.platform` 报告；Linux 的 pkg-config 依赖详情会保留检测到的 ABI 包版本，便于复核运行时匹配。要求 `host_runtime=true`、`cross_compilation=false`、目标与摘要一致、`supported=true` 且所有 required 检查为 `pass`；Linux 必须明确报告 required display、GTK 3 和 AppIndicator 检查，并记录 optional WebKitGTK 4.1/4.0 观察，Windows 必须记录 optional WebView2 观察。mygo 只有在创建页面/WebView 窗口时才加载这些 WebView 运行时；当前工作台全部使用 native-content 窗口，因此缺失 optional 运行时不阻断支持。`installer.log` 必须包含目标匹配的结构化 install/upgrade/rollback/运行锁回执，`native-window.log` 必须报告 PTY exit 0，`tray.log` 必须包含 host-only 且 status=passed 的 `agent-remote-desktop.tray` 报告，`workbench.log` 必须包含成功回执；跳过工作台或托盘的 CI 记录不会被视为正式证据。只有日志文件存在而没有实际 host probe JSON 的记录会被拒绝。`--allow-virtual-display` 只用于审阅 Xvfb/CI 记录，不能将虚拟显示报告为正式桌面验收。

平台与托盘结构化回执还会进行 schema 与类型校验：每个 platform requirement 的 `status` 只能为 `pass`、`fail` 或 `unknown`，`required` 必须为布尔值；tray probe 必须是 schema v1，并提供字符串 `goos`/`goarch` 以及布尔 `host_runtime`/`cross_compilation`。畸形回执会在合并前拒绝，避免仅凭日志中的成功标记生成原生发布证据。

正式 tag 发布由 [`.github/workflows/release.yml`](../.github/workflows/release.yml) 编排：它在 macOS runner 上使用临时 keychain 的项目社区证书构建三平台制品，先离线验证清单与签名策略，再创建 GitHub Release。该 workflow 的成功状态只证明版本 tag、打包和社区发布策略通过；它不会代替下方要求的 Windows/Linux 原生窗口、托盘、PTY、权限 broker、安装升级或 Agent/Server/Node 联调证据。

证据合并器兼容 Windows PowerShell 5 写出的 UTF-8 BOM 与 PowerShell 7 的无 BOM 文件，涵盖摘要、元数据和 WireGuard JSON；这样不同 Windows shell 版本不会改变证据解析结果。

WireGuard broker 验收是独立的 host-only gate。设置
`DESKTOP_NATIVE_WIREGUARD_CLI_BINARY` 后，`native-acceptance.sh`（Windows
使用 `-WireGuardCliBinary`）会调用 CLI 仓库的
`native-wireguard-broker-acceptance` 入口，并把 `wireguard/evidence.json`
写入同一证据目录。该证据要求真实安装材料、服务身份和
`wireguard.bridge.status` 握手；`simulated`、`fixture`、`mock`、`xvfb`、
交叉编译和仅有端点文件都会被拒绝。使用 `verify-native-evidence.sh
--require-wireguard` 时，三台主机都必须提供完整且已完成的 broker 证据；
Windows 当前服务端身份握手尚未完成，
因此即使 SCM 安装存在也必须保持未完成状态。默认只做 status-only，up/down
回执、重放拒绝和服务重启恢复必须由目标主机另行执行并记录，不能由
contract/unit 测试代替。

正式构建脚本要求通过受保护环境传入持久项目社区自签身份及证书 SHA-1，并在 macOS 上复验证书为自签证书及 bundle 的实际签名；明确跳过公证且不上传。只有显式设置 `DESKTOP_ALLOW_ADHOC=1` 才能做不计入正式版的 smoke 构建；即使通过环境变量显式传入 `DESKTOP_MACOS_SIGNING_IDENTITY=-` 也不能绕过该门禁。Windows/Linux 不配置发行者代码签名。Windows NSIS 安装器仅在构建机有 `makensis` 时产生。构建基础验证命令为：

```sh
DESKTOP_MACOS_SIGNING_IDENTITY="Agent Remote Community Code Signing" \
DESKTOP_MACOS_SIGNER_CERTIFICATE_SHA1="<40 uppercase hex characters>" \
scripts/build-release.sh --out /tmp/agent-remote-desktop-release --clean
cd /tmp/agent-remote-desktop-release && sha256sum -c SHA256SUMS
```

该命令证明打包、社区签名和完整性清单可复验；它不替代下表的原生窗口、终端、后台生命周期、权限和 Agent/CLI 联调。macOS `.dmg` 与实际签名必须在 macOS 机器上执行；交叉构建不会证明 Windows/Linux 原生运行。

每个目标还会附带 `agent-remote-desktop-installer`。它只接收已取得的完整发布目录，校验目标清单、文件大小和 SHA-256 后，将用户可写安装目录通过 staging 原子替换，并保留一个上一版本供 `--rollback` 恢复；只读 `--status` 可在升级前报告当前版本与回滚可用性。已安装桌面启动时会持有 destination 旁的运行锁，安装、升级和回滚在锁被占用时返回 `ErrInstallBusy`，避免替换正在运行的版本；锁文件可在进程崩溃后由操作系统释放并复用。原生 `installer-probe` 现在还会在 install→upgrade→rollback 后验证并发运行锁拒绝及释放后重获。它不下载、不提权、不做 Windows/Linux 发行者签名，也不安装外部 CLI。当前证据为 `go test ./tests/installer`、Linux smoke 的 `SHA256SUMS` 全通过，以及 Windows/Linux/macOS helper 交叉构建；真实三平台安装器和系统包管理器行为仍待平台主机验收。

## 当前证据

| 检查 | macOS arm64（本机） | Windows amd64 | Linux（架构见单项） |
| --- | --- | --- | --- |
| desktop 编译 | 本机 Go 构建及测试通过 | `CGO_ENABLED=0` 交叉构建通过 | amd64 `CGO_ENABLED=0` 交叉构建通过 |
| 当前主机运行时能力边界 | `tests/platform-probe` 输出 host-only JSON，区分必需目标/shell/native PTY/display；Linux 必须通过 GTK 3 和 AppIndicator 检查，并记录 optional WebKitGTK 4.1/4.0 观察；Windows 记录 optional Evergreen WebView2 观察（HKCU/HKLM 两个注册表视图）。mygo 仅在页面/WebView 窗口出现时加载 WebView；当前工作台使用 native-content。`tests/tray-probe` 创建并销毁实际托盘图标/菜单；native PTY 会实际分配并读取尺寸后关闭当前平台的 Unix PTY，Windows 对应 ConPTY；不读取交叉构建结果 | 待实际运行 | 待实际运行 |
| 原生窗口、语言切换、隐藏/显示、PTY 往返 | `native-probe` 通过真实窗口 resize→PTY cell size、`Window.Close`/OnClose 隐藏回调及 PTY 往返；`native-acceptance.sh` 记录 metadata | 待实际运行 | 待实际运行 |
| 工作台项目、多会话、分屏、焦点、历史恢复 | `native-acceptance.sh` 中完整 `workbench-probe` 通过账户绑定复用、状态刷新和 remote-scope；使用临时测试 CLI/Agent | 待实际运行 | 待实际运行 |
| 准确远程会话连接与公开上下文历史 | Go/UI/race、storage v3、三语言 900×600 及原生 race PTY 探针通过；真实授权/SSH 验收另记 | 待原生运行 | 待原生运行；Node 运行证据不能替代桌面验收 |
| CLI 登录、注销及诊断协议 | 真实 Rust CLI + 隔离 HTTP/凭据工具夹具通过 | 待实际运行 | 待实际运行 |
| 共享默认账户 get/set/clear | 真实 Go→Rust/普通 CLI 双向互通、并发确认、取消与三语言最小布局通过 | 桌面交叉构建通过，待原生运行 | amd64 桌面交叉构建通过，待原生运行 |
| 账户配置导入与原请求恢复 | Go 合同/UI/race、跨 App 重启恢复夹具、真实 Go→Rust+隔离 HTTP/来源/journal 五场景及三语言 900×600 渲染通过 | 待原生运行 | 待原生运行；Server/Node 部署验收未完成 |
| 项目同步与旧映射恢复 | 真实 CLI、Go/UI/race、三语言渲染、官方 Mutagen 文件/恢复探针及 macOS→Docker Linux 的真实 Server/Node/SSH 链路通过 | 交叉构建通过，待原生运行 | Docker Linux Node 实际 UID/ACL、SSH/Mutagen 双向传输、冲突/暂停/恢复/重建通过；桌面与正式安装仍待原生验收 |
| 共享 Mutagen 运行环境恢复 | 真实 Go→Rust→官方 Mutagen 隔离 daemon 的 stopped/running 检查及确认重启通过；sync/forward、paused、配置、文件保留与三语言渲染通过 | 待原生运行 | 持久清单与服务重启仍待 Linux 原生运行；Node 传输证据不覆盖此工作流 |
| 本地初始化、元数据修复与部分结果 | 真实 Rust CLI、数据保留及三语言 900×600 确认/结果页测试通过 | 待实际运行 | 待实际运行 |
| WireGuard 配置计划、确认发布与部分结果 | Rust 合同、真实 Go→Rust 互通及三语言 900×600 布局专项通过 | 待实际运行 | aarch64 隔离容器 15 项配置合同通过；桌面待运行 |
| WireGuard 配置检查与运行状态 | 真实 Go→Rust、脱敏/uint64/取消、三语言 900×600 布局通过；真实隧道待验收 | 状态/工具/进程 Job/ACL 原始模块及测试独立类型检查通过，待运行 | aarch64 原生单元 23、机器合同 9、共享诊断回归 11 项通过；真实隧道/桌面待验收 |
| WireGuard 只读隧道计划与原生目标观察 | target 专项 8 项与 WireGuard 库 33 项通过；映射/socket 为隔离夹具，真实 utun 待验收 | SCM 原始模块与测试独立类型检查通过，待原生运行 | aarch64 target 原生专项 8 项、隧道计划合同 4 项通过；无真实隧道变更 |
| WireGuard 受保护 broker 与 caller identity | macOS LaunchDaemon plist、root/admin socket、launchctl 状态、项目社区自签 identity 观察及 Unix-socket client/permit 合同已接入；真实 daemon 安装、授权、执行和目标重观察待 macOS 主机验收 | SCM LocalSystem broker、protected named pipe、远程客户端拒绝、进程/会话/token SID 绑定、一次性 permit、Program Files/ProgramData ACL 与安装脚本已接入；无原生服务验收，Windows 不要求发行者代码签名 | system D-Bus/Polkit service、systemd/D-Bus/Polkit 安装材料、unique sender 与 unix-process UID/PID/start-time 绑定、严格 payload 和 client/receipt 合同已接入；无 Linux 原生 systemd/Polkit 授权或真实执行验收 |
| 严格凭据、目录同步与跨 home 锁 | 聚焦库/合同测试通过，实际系统交互待验收 | 原生模块与测试独立交叉检查通过，待运行 | aarch64 凭据合同 4 项、严格读取 10 项、目录同步 3 项、WireGuard/锁 8 项通过；系统 Secret Service 待验收 |
| 能力说明、命令平台声明、Agent 缺失提示 | 真实 CLI 被动发现、隔离工具解析及三语言最小布局专项通过 | 待实际运行 | 待实际运行 |
| 诊断取消后的子进程清理 | Unix 进程组及真实子进程测试通过 | Job Object 模块独立交叉检查通过，待运行 | Unix 实现待 Linux 实际运行 |
| Codex/Claude/fclaude 的完整 TUI、输入法及附件 | 待真实工具和账户验收 | 待实际运行 | 待实际运行 |
| OS 凭据库锁定、拒绝、授权交互及共享作用域 | 已有隔离替身契约；系统交互待验收 | 待实际运行 | 待实际运行 |
| 社区安装包、平台身份校验、升级及回滚 | manifest/SHA-256 校验、用户目录原子 install→upgrade→rollback helper 测试、运行锁占用拒绝/释放重获及 `installer-probe` 本机通过；macOS 社区自签验证保留 | helper amd64 交叉构建与清单校验通过，仍待 Windows 原生安装路径/锁定文件验收；CI 含同主机 installer-probe | helper amd64 交叉构建与清单校验通过，仍待 Linux 原生安装路径/包管理器验收；CI 含同主机 installer-probe |

终端 resize 现在有明确的 Go 进程边界：`terminalhost.Host.Resize`/`Size` 直接操作当前平台的 Unix PTY 或 Windows ConPTY，拒绝零列/零行请求；`tests/platform-probe` 还会在不启动命令的情况下分配一次原生终端、读取正尺寸并关闭，作为主机端 ConPTY/PTY 可用性的必需检查。真实 PTY 回归位于 `tests/terminalhost/host_test.go`，通过 `DESKTOP_TERMINAL_TESTS=1` 执行。该证据只证明主机原生终端句柄的分配与尺寸更新，窗口拖动触发、IME、全屏 TUI 以及 Windows/Linux 桌面仍需在各自图形主机上验收。

工作台面板会读取受管终端的临时 OSC 标题，用于显示 Agent/TUI 的当前模式或项目状态；用户设置的会话名称优先，标题不写入 desktop 状态数据库，也不参与恢复身份或命令判断。macOS PTY 的中文/emoji 标题由 `tests/terminalhost` 在开启真实终端测试时验证；该测试不代表 Windows/Linux 标题、IME 或真实 Agent TUI 已完成原生验收。

Rust CLI 的完整 Windows 交叉检查受本机缺少 MSVC C 头文件阻碍；Job Object 模块单独编译通过不代表整个 CLI 可发行。其他 CPU 架构尚未验收。

Linux WireGuard/凭据证据来自 `rust:1.90-bookworm` 的 aarch64 原生隔离容器，共 40 项通过，使用临时 home、loopback HTTP 和假 secret-tool。首轮发现 O_PATH 目录不能直接 fsync，导致配置合同 7 项和严格凭据库测试 3 项失败；修复为从已持有目录相对打开可同步句柄后，上述专项全部复验通过。没有启动真实隧道，不涉及 Linux 图形桌面、窗口管理器或系统凭据授权验收。详细失败记录见[实施状态](implementation-status.md)。

能力说明及 Provider 可用性改动后已再次通过：真实 CLI/PTY 开启的 desktop 完整门禁、app/capability/cliadapter/session 竞态检查、macOS 原生工作台探针，以及 Windows/Linux amd64 desktop 交叉构建。本地初始化/元数据修复及复审修改后，真实 CLI/PTY 开启的 desktop 完整门禁最终复验通过（脚本退出码 0），官方 catalog 共 285 条消息；CLI 默认 8 路完整门禁在路径复审修复后为 832 通过、0 失败、3 跳过（42 个测试目标，测试执行 109.660 秒，完整门禁 411.39 秒）；历史失败及测试辅助进程启动目录修复记录见实施状态。

本地修复切片的 app/cliadapter/capability 竞态整组已通过（354.155s / 81.749s / 20.370s）；复审修正后的 setup/diagnostics 定向竞态复验通过（app 81.665s / cliadapter 56.795s），Windows/Linux amd64 desktop 交叉构建也再次通过。

WireGuard 配置及 Linux 目录同步修复后，CLI 原有默认 8 路完整门禁在隔离构建目录通过：877 通过、0 失败、3 跳过，44 个测试目标，测试执行 89.942 秒，脚本退出码 0。随后 desktop 使用这份最新 CLI、开启真实 PTY 的整仓门禁通过，官方 catalog 为 314 条消息，app 35.212s、cliadapter 17.854s；Windows/Linux amd64 desktop 交叉构建通过。900×600 的中英文及伪本地化确认/结果页已重新渲染，阶段摘要和底部操作保持可见。

同一代码的 app/cliadapter/capability 整组竞态检查启用真实 CLI/PTY 并使用 `-count=1`，通过（484.017s / 199.064s / 20.569s），无竞态报告、退出码 0。这是本机运行证据，不替代 Windows/Linux 桌面和真实 Agent 账户验收。

网络检查/状态切片后，macOS CLI 默认 8 路完整门禁通过：924 通过、3 跳过，45 个测试目标，测试执行 58.851 秒，退出码 0。同一代码下 desktop 启用真实 CLI/PTY 的整仓门禁通过，346 条官方中英文消息；首次 app 29.403s、cliadapter 15.721s；快照测试同步修正后的最终完整门禁再次通过，app 29.366s、cliadapter 16.367s，退出码 0。最终三语言最小布局重新渲染并检查，分页文案明确表示页码。Windows/Linux amd64 desktop 交叉构建再次通过，仍不替代两平台原生运行。Linux aarch64 在相同隔离镜像中另通过 43 项检查/状态及共享诊断测试；新工具和进程测试不使用真实网络、提权或用户凭据。Windows 独立类型检查涵盖 SYSTEM 只读 ACL 例外和 suspended-child Job Object，但整 CLI 构建及原生行为仍需目标平台验证。

本轮网络/诊断/能力定向竞态检查通过：app 122.638s、capability 1.389s，cliadapter 在修正快照测试同步后完整复验通过 189.292s，无竞态报告。首次失败和修正记录见实施状态；此项为定向范围，此前配置切片的全包竞态证据仍保留。

只读隧道计划新增验证：Linux aarch64 通过实际只读 netlink 查询及隔离计划合同，共 12 项；Windows SCM 查询/命令解析/有界读取仅有原始代码和测试的交叉类型检查，不是服务运行证据。Linux 追加 Clippy 因镜像未安装组件没有执行，不能计为通过。macOS 临时映射/socket 测试不替代真实 utun、长期路由/DNS 进程或授权桥验收；本切片未安装、启停真实隧道或提权。专项失败及修复完整记录见实施状态。

只读隧道计划切片后最终全量复验通过：macOS CLI 原脚本默认 8 路测试 948 通过、3 跳过、46 个二进制，nextest 98.783s，完整门禁退出码 0；desktop 启用真实 CLI/PTY 的整仓门禁退出码 0，366 条官方中英文消息，app 60.139s、cliadapter 39.592s。Windows/Linux amd64 desktop 交叉构建再次通过。三语言 900×600 预览布局已渲染并检查；真实联调使用隔离配置和工具，不替代目标系统的真实隧道与授权验收。

同一切片的隧道/网络定向 race 通过：cliadapter 252.453s、app 133.962s、capability 4.860s，退出码 0、无竞态报告；该轮使用隔离 CLI 夹具，真实 CLI/PTY 由完整普通门禁另行验证。

受保护执行基础的新增证据：macOS 同仓执行基础 23 项通过，独立保护目录 10 项通过（包括扩展 ACL）；Linux aarch64 原生隔离容器执行基础 23 项、保护目录 9 项通过。Windows 原始保护目录及测试独立交叉 Clippy 通过，目录持久性明确 Unsupported；这不证明真实 ACL 安装路径、服务运行或执行桥可用。进程退出恢复和 I/O 故障注入不替代掉电验收。

执行基础同源最终门禁通过：macOS CLI 1014 通过、3 跳过、46 个二进制，nextest 78.962s，完整脚本退出码 0；其中保护目录的同仓 lib/bin 测试各 10 项通过。desktop 完整检查再次通过（366 条消息，未变化 Go 包使用缓存），另对最新 Rust CLI 强制运行 WireGuard 回归 `-count=1`，9.811s 通过。未重新运行 Go race 或交叉构建，历史证据继续保留。

三平台 CI 已配置，但未推送运行，不能引用为通过证据。CI 的 Linux job 安装 GTK 3、AppIndicator 和 Xvfb；WebKitGTK 可继续安装以覆盖 optional 观察，但不是当前 native-content 工作台的支持门槛。无图形会话时使用 Xvfb，只能提供相应探针证据，不能替代桌面环境和窗口管理器验收。独立 `cross-build` job 构建 darwin/arm64、windows/amd64、linux/amd64 的 desktop 与 installer，明确标注为交叉构建，不计入原生运行证据。

默认账户切片补充：CLI 默认门禁 1026 通过、3 跳过、46 个目标（77.011s）；desktop 最终真实 CLI/PTY 门禁通过，391 条消息，app 40.164s、cliadapter 28.677s。定向 race 为 app 58.866s、cliadapter 104.969s；取消流程修正后实际 SIGTERM 专项 race 8.203s。Windows/Linux amd64 最终交叉构建再次通过。失败与修正、缓存边界及截图证据见[实施状态](implementation-status.md)，不替代两平台原生权限/凭据运行验收。

账户生命周期及绑定终端切片：最终真实 CLI/PTY desktop 完整门禁通过（435 条消息，app 15.805s、cliadapter 15.248s）；CLI 最终完整门禁 1052 通过、3 跳过、47 个目标，90.285s。macOS 原生探针覆盖固定 binding 参数、项目/home、PTY 输入、存活连接复用及重启历史先查询。账户 app race 277.910s、适配器 race 195.537s、session race 1.445s；停用重绑补充 race 35.994s、实际 SIGTERM/确认快照 race 16.584s。最终 Windows/Linux amd64 desktop 交叉构建通过。

配套 Server 完整门禁 1952 通过、104 跳过、91.56% 覆盖率；Node 完整门禁与专项 race 通过、63.2% 覆盖率。Node 原树既有 Skill 目录 0775 不满足来源检查，完整门禁在源码字节一致的隔离副本通过，未改变原目录权限。Desktop 两次预检超时及 macOS dyld 采样、Unix 测试映像复用修正和最终复验详见[实施状态](implementation-status.md)。没有真实服务器登录、远端 SSH/tmux 接管或 Windows/Linux 原生运行证据，不能以这些隔离门禁替代正式版验收。

## 发布前记录要求

发行策略已由用户确定：正式 Release 为社区版，macOS 使用项目社区自签身份，Windows/Linux 不要求发行者代码签名，不以 Apple Developer ID、公证或商业证书作为门槛。验证仍需覆盖固定社区证书与安装信任、受保护路径、OS 调用者身份、制品完整性及更新兼容；公开构建记录本身不授予系统权限。

每次运行记录 desktop/CLI 准确代码版本、mygo 与原生库版本、OS/架构、图形会话、Agent 版本、执行命令及结果。使用隔离 home；真实账户测试需显式选择测试服务器与设备。截图及日志不得包含凭据或真实终端敏感内容。

在目标系统上运行基础门禁与原生探针：

```sh
DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh
go run ./tests/native-probe
go run ./tests/workbench-probe
# Host-only runtime/dependency report (never sets GOOS/GOARCH)
go run ./tests/platform-probe
# Current-host runtime evidence (writes logs/metadata; never sets GOOS/GOARCH)
scripts/native-acceptance.sh
```

Windows 主机没有 Git Bash 时可使用等价的 PowerShell 入口；它运行相同的
platform、installer、native-window、tray 和 workbench 探针，并把证据写入同样的
`metadata.txt`、逐步日志和 `summary.txt`：

```powershell
pwsh -File scripts/native-acceptance.ps1 -EvidenceDir "$env:TEMP\agent-remote-desktop-native"
```

协调机暂时没有目标主机时，可显式指定目标运行
`scripts/native-acceptance.sh --target windows`（PowerShell 使用
`-Target windows|linux|darwin`）。入口只写入 `status=not_run` 的
`target-preflight.log`/摘要并返回非零；证据合并器会拒绝该记录，绝不会
把交叉构建或缺失主机伪造成原生通过。

两个入口均使用 Go 的 `GOHOSTOS`/`GOHOSTARCH`，并清除继承的 `GOOS`/`GOARCH`，
因此不会把交叉构建误当成运行验收。每个失败步骤仍会留下 `summary.txt`，其中
包含 `status=failed`、退出码、步骤名、主机目标和 `cross_compilation=false`；
失败日志可直接作为平台缺口记录。Linux 没有图形会话时，Bash 入口可用
`xvfb-run`，证据会标为 `ui_display=xvfb`，它不等价于窗口管理器或实体桌面。
元数据同时写入 `display_protocol`：Linux 真实会话必须是 `x11` 或 `wayland`，而
Xvfb 明确记录为虚拟 `x11`；macOS 和 Windows 分别记录 `appkit` 与 `win32`。

拿到完整 release 目录后，还应在目标主机执行打包制品 smoke。该脚本先离线校验
manifest/checksum，再用临时用户目录运行真实安装器、读取 `--status`，最后从已
安装目录启动桌面二进制的 `--version`；不会写系统路径或提权：

```sh
scripts/release-native-smoke.sh dist/release
```

Windows hosts without Git Bash use the equivalent packaged smoke entry point:

```powershell
pwsh -File scripts/release-native-smoke.ps1 -ReleaseDir dist/release
```

It chooses the current native Windows architecture, installs only the declared
target into a temporary user directory, verifies installer status, and checks
the installed executable's version. It does not turn a cross-build or an
unsigned artifact into native evidence.

On macOS, the Unix smoke entry point also walks from the installed executable
to the copied `.app` bundle and runs `codesign --verify --deep --strict` after
installation. This verifies the installer output itself in addition to the
source release directory's signature check.

`DESKTOP_ALLOW_ADHOC=1` 只用于显式的 macOS ad-hoc smoke 目录，不能替代项目社区
自签正式验收。上述 smoke 仍不覆盖真实 Agent 登录、凭据库授权、WireGuard broker
安装或完整 TUI，这些项目必须在各自平台保留独立日志和失败记录。

测试逻辑、辅助程序及夹具均在仓库根 `tests/`；以上命令在 Windows 使用 Git Bash 等支持所示环境变量语法的 shell。`native-acceptance.sh` 的 `DESKTOP_NATIVE_EVIDENCE_DIR` 可指定证据目录；它分别记录当前主机和 Linux 的 Xvfb 虚拟显示，不会将交叉编译结果写成运行证据。CI 编排见 [.github/workflows/ci.yml](../.github/workflows/ci.yml)。

基础门禁之外，还需逐平台验证：

- 中文 IME、emoji、全屏 TUI、复制粘贴、图片/文件附件、拖放、字体缩放及窗口/分屏 resize。
- 关闭窗口后持续运行，托盘/Dock/重新启动唤回，明确退出停止本地会话；远程 tmux 在断开后保持运行。
- 浏览器授权打开失败、过期、拒绝、取消与后台返回；密码/MFA 及可选设备登记；跨页查看结果不重放操作。
- OS 凭据库访问拒绝、离线注销、远端撤销失败及本地部分清理；诊断取消/超时后无遗留探测进程。
- WireGuard 检查/状态在真实工具与权限下的观察、工具缺失及取消清理；配置新建/替换、原生 ACL、凭据库拒绝/锁定、取消与发布失败阶段；Windows 文件同步不等于 Unix 父目录同步，需分别验证目标平台实际行为。
- 隧道只读计划在真实 Linux namespace/接口、macOS 映射/utun/socket 和 Windows SCM 服务上的身份观察与变更检测；未来权限桥的可信工具、配置快照、运行代次、取消恢复及成功后台服务存活仍需独立验收。
- 旧 CLI、旧服务端、未安装组件、权限不足、配置改变及网络中断时的可解释状态和恢复入口。

能力说明页已区分声明、配置前提与同上下文的权限观察；逐命令平台声明当前仅覆盖已核实的部分命令，未声明状态保持未知。完整组件/服务端能力协商及全量 CLI 图形化入口仍在开发。本表记录验证范围，不代替运行时权限判断；凭据存在或 CLI 声明支持方法均不能证明服务端授权。

相关文档：[实施状态](implementation-status.md)、[CLI 覆盖清单](cli-coverage.md)、[产品方案](../../agent-remote/docs/agent-remote-desktop-design.zh-CN.md)。

项目同步切片：六个 GUI 入口及准确目标确认、阶段回执、取消/登出失效通过本机 Go 定向测试；race 为 app 137.235s、cliadapter 74.393s，未启用真实 CLI。900×600 en/zh-Hans/en-XA 状态及确认渲染截图位于 `/tmp/agent-remote-sync-ui/`；详情滚动、操作栏固定可见，不能代替原生窗口测试。此初始切片尚未覆盖旧映射 404 后恢复及真实 Node 双向同步；本轮新增证据见下文。Windows/Linux 原生 desktop 仍待验收。真实 Rust 互通已通过 2.117s，隔离 loopback Server/临时凭据验证六方法、实际 dry-run Server/SQLite 写入、目标前置条件、上下文/端点漂移及500未知不重放；含真实 CLI 定向 app 2.278s、cliadapter 2.419s 通过。随后真实 CLI/PTY 整仓门禁通过，471 条消息，app 17.251s、cliadapter 17.753s，Windows/Linux amd64 交叉构建通过。CLI 完整门禁 1076 通过、3 跳过；最终修正另有 14 项同步合同、7 项 Mutagen 单测、官方 v0.18.1 暂停模型及本地代理文件往返/冲突探针、Clippy 证据。隔离探针不替代真实 Node 文件传输；具体日志及失败修正见实施状态。

旧映射恢复新增证据：Go/UI 定向测试 app 2.101s、cliadapter 1.577s；真实 Go→Rust 恢复 6 场景 5.937s，覆盖 workspace 保留/移除、会话已不存在、配置漂移、终止未知与 daemon 不可用。官方 Mutagen 原生恢复探针另验证不可变身份、准确终止、无关会话及冲突文件保留。900×600 三语言恢复预览/结果截图位于 `/tmp/agent-remote-sync-recovery-ui/`，操作固定可见，详情滚动，不代表原生窗口验收。

真实同步链路新增证据来自 macOS Docker Desktop 控制端与一次性 Linux 容器：真实 Server、worker、root helper、SSH forced command、Bubblewrap 及 Mutagen agent 完成六个同步方法；双向文件、UID/ACL、暂停无传输、冲突双方保留、人工解决后刷新、重建及未知/撤销 SSH key 拒绝均通过，日志 `/tmp/agent-remote-sync-chain.log`。该历史记录的 restart recovery 仍为 `not_run`；最新 wrapper 已要求 Node fixture 输出精确 restart marker，但当前环境未重新运行 Docker/Mutagen，不能把它写成通过。不覆盖 systemd 安装、WireGuard、已有连接即时撤销、远程生产部署或 Windows/Linux 原生桌面。全局 Mutagen SSH 环境恢复现已提供完整影响范围的预览与独立确认；外部服务注册的自动迁移仍待专门集成。

恢复切片最终 desktop 整仓门禁开启真实 Rust CLI/PTY 后通过，484 条官方消息，app 19.736s、cliadapter 24.340s（未改终端包使用缓存）；Windows/Linux amd64 `CGO_ENABLED=0` 交叉构建通过。恢复定向 race 为 app 112.972s、cliadapter 127.040s，未启用真实 CLI；真实恢复联调单独 5.937s。CLI 完整门禁 1100 通过、3 跳过（100.077s），最后持久化未知语义另有静态检查及 25 项合同复验；Mutagen 11 单测 0.40s、原生恢复 1.76s 通过。日志及首次编译/短 socket 路径/脚本解析失败和最终复验记录见[实施状态](implementation-status.md)。

共享运行环境恢复新增本机证据：Go/UI/adapter 定向 app 3.253s、cliadapter 2.428s，固定 unknown 错误码及能力页补充回归 app 4.744s、cliadapter 2.601s；定向 race app 80.208s、cliadapter 117.567s，先于上述错误码小改。真实 Go→Rust→官方 Mutagen daemon 联调 19.835s，日志 `/tmp/agent-remote-environment-desktop-interop.log`，验证停止状态预览不写入/启动、完整 sync+forward、明确启动及重启、paused/配置/文件保留、漂移拒绝与无认证状态初始化。父进程 HOME 不变，child 使用隔离短 namespace 和 direct daemon；不覆盖用户 launchd、外部注册自动迁移或 Windows/Linux 原生服务行为。三语言 900×600 九张渲染截图已检查，路径 `/tmp/agent-remote-mutagen-environment-ui/`。

管理员运行时恢复新增本机证据：Go/UI/adapter 定向 app 4.617s、cliadapter 2.791s、capability 0.490s，独立 i18n 0.309s 通过，官方 catalog 为 617 条。覆盖无设备的用户身份、非管理员/过期拒绝、三动作单独确认、受理与执行完成分离、CLI journal 跨重启、unknown/404 保留原 key、手工旧 key 只读、终态清理、停止等待及提交前失败恢复。定向 race app 87.762s 通过；该轮先于最后的固定错误码呈现和可执行文件消失时错误归属修正。正常 URL 等价与不同 host/path/scheme、userinfo/query/fragment 拒绝另测 0.813s 通过。三语言 900×600 审阅/确认共六张 mygo 截图位于 `/tmp/agent-remote-runtime-recovery-ui/`，中文与伪本地化确认及英文审阅已目视检查，正文可滚动、操作固定可见。初次 apply 回执接收因误用 `[]byte` 导致保守 unknown，已改 `json.RawMessage` 并补三动作正常回执回归；初次长文本 UI 测试点击视口外字段，现按实际控件矩形滚动，操作栏同时精简，后续复验通过。上述使用 CLI 替身的 UI 证据不替代真实 Rust、部署或三平台原生桌面验收，最终整仓与真实联调证据见[实施状态](implementation-status.md)。


准确会话连接切片补充：固定 `sessions.attach` v1 与 `fclaude attach <完整非零 UUID> --expected-server ... --expected-device ...`，home 由环境传递。列表及历史确认绑定 home/Server/device/session/project，旧历史须重新选择；storage v3 读取兼容 v1/v2，公开上下文深复制、身份校验及旧应用写回保护专项通过。普通 UI 聚焦回归 5.386s，追加刷新失败/取消回归 0.781s；定向 race 为 app 124.848s、session 1.482s、storage 1.546s、cliadapter 1.565s。最终 `go run -race ./tests/workbench-probe` 退出 0，覆盖 legacy 和准确历史的分屏位置/邻居/名称保留、实际 PTY argv/home/cwd、不同 Server/device/project 同 UUID 不复用、相同上下文复用及关闭清理。全部使用临时 Agent，不连接真实账户。

能力页的 fclaude 默认入口、`run`、`new`、`list` 路由回归使用 CLI control fixture：默认/`run`/`list` 进入同上下文远程会话库存，`new` 在声明 `sessions.create` 后打开 caller-key 创建审核；缺少写能力时创建入口按 `unsupported` 禁用。该证据只覆盖 Go 路由、异步上下文 hand-off 和 schema/capability gate，不替代真实 Server/Node 创建、同步、授权或三平台原生终端验收。

官方 catalog 检查为 629 条中英文。900×600 的英文、中文、en-XA 确认页位于 `/tmp/agent-remote-exact-attach-ui/{en,zh-Hans,en-XA}-remote-exact-history.png`，身份与固定操作区已目视检查；伪语言较长内容可在详情区滚动。首轮发现重复会话 UUID 会触发 mygo 重复 key，现已在列表响应入口拒绝重复/空 ID；Provider 检测后丢失会在确认内重新检测并显示错误。测试滚动坐标曾落在列表区域外，已按实际列表视口修正。desktop 现在按 CLI capabilities 的 `sessions.attach` v1 + `attachment_transfer` v1 门控 mygo 文件/图片 payload；fclaude 的独立 exact 通道仍由 Server/Node 验证 lease，未声明时保持 text-only，不能回退旧 SSH 接收器。文字复制/粘贴可用。三仓库真实 SSH、原生 Windows/Linux 和真实完整 Agent TUI 仍待验收。

实际 Rust 二进制发现结果已更新覆盖清单：agent-remote 106 个命令节点/628 个参数绑定，fclaude 12/94；terminal_operations 同时声明 accounts.binding.attach v1 与 sessions.attach v1。该被动发现及本机夹具证据不代表全量 CLI 图形化验收完成。

Exact 附件真实联调现在有明确的 opt-in 入口：
`scripts/exact-attachment-acceptance.sh --contracts` 会串行运行 Server 的租约/清理合同、Node 的
forced-command gateway 与接收器合同、CLI exact-transfer 编译合同，以及 Desktop 的 capability/status 合同。
这些测试使用夹具，不是 Server/Node/SSH 真实链路证据。`--live` 会在通过上述合同后运行 CLI 的 ignored
测试 `tests/exact_attachment_live_contract.rs`，要求 `AGENT_REMOTE_HOME`、
`AGENT_REMOTE_EXACT_ATTACHMENT_TEST_SESSION` 和 `AGENT_REMOTE_EXACT_ATTACHMENT_TEST_DEVICE`，通过真实
Server 授权、Node forced-command SSH gateway 上传一个临时文件并确认 scoped cleanup receipt；缺少任一变量
或 `ssh` 时脚本以失败退出，不会把测试标记为 skipped。当前只复验了 `--contracts`；没有配置测试账户，不能
宣称真实三仓库附件传输已完成。

## 2026-10-23 交付候选

干净提交 `85cf783e2a97fc2b74513b4f5fac12302719bad4` 的普通桌面门禁、三目标交叉构建和发布离线校验均通过。候选目录为 `/tmp/agent-remote-desktop-release-delivery-final`，包含 `darwin/arm64`、`windows/amd64`、`linux/amd64` 三目标及 18 项制品；macOS 项目社区自签安装后 smoke 通过。该证据仍只代表当前 macOS 主机和可重复构建，不替代 Windows/Linux 原生主机、WireGuard broker 或真实 Server/Node/SSH 验收。
