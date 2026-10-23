# WireGuard 检查、状态、只读计划与后续启停设计

状态：只读检查与状态服务、机器协议和 Go 网络页已实现，见[检查与状态契约](wireguard-inspection-contract.md)；配置生成和发布已另行接入，见[配置契约](wireguard-configuration-plan.md)。隧道计划、原生目标观察、Go 预览页和 `wireguard.tunnel.apply` 已接入，见[隧道契约](wireguard-tunnel-contract.md)。apply 严格重查冻结计划，通过 macOS LaunchDaemon、Linux system D-Bus/Polkit 或 Windows SCM LocalSystem broker 执行，并在第二次观察确认目标状态；服务实现、安装材料和持久 journal 已落盘，三平台真实安装、授权与运行验收仍待完成。

## 现有实现与改造入口

受保护 broker、调用者认证/授权边界、持久 journal 和安装验收顺序已整理为[权限桥实施设计](wireguard-bridge-design.md)。桌面网络页现可显式查询 `wireguard.bridge.status`；当前 CLI 在没有受保护 broker 时返回 `unsupported`/`unavailable`，不会将普通 helper 或进程权限显示为可执行授权。正式 Release 已确定采用社区版：macOS 社区自签，Windows/Linux 不要求发行者代码签名。

普通 CLI 和 helper 的 `check` 已共用有界配置校验服务，并修复两者 `--dry-run` 参数不一致。新的 desktop `status` 共用 Rust 机器格式解析和有界进程管理；普通人类 `status/up/down` 仍保留原 helper 流程，status 调用 `wg show` 并根据英文错误判断提权。Unix 使用 sudo，Windows 使用 runas，后者只有退出码且等待没有上限。不能直接将这些输出作为 desktop 协议。

工具发现、配置检查、机器状态观察已提取到 Rust library；后续执行服务和权限桥需继续供普通 CLI、独立 helper 和 desktop-control 共用。desktop 保持 Go UI/业务适配，不接收管理员密码，不拼装 sudo，也不以管理员身份重启整个桌面。

## 当前机器协议与后续执行入口

以下方法均为本地系统操作，`credential: none`，不以远程登录或设备登记为前提。check/status、tunnel.plan 与 tunnel.apply 字段已固定并实现严格校验；apply 只在冻结计划重查通过后调用受控 helper，并以执行后观察作为回执依据。

| 方法 | 类型 | 请求与结果范围 |
| --- | --- | --- |
| `wireguard.check` | read | 输入 expected_home、可空绝对 config；返回词法路径、非敏感配置摘要、工具与当前进程权限观察；不提供执行授权或文件身份令牌 |
| `wireguard.status` | read | 输入 expected_home；返回接口、peer、握手时间、收发计数及明确的不可观察状态 |
| `wireguard.tunnel.plan` | read，已实现 | 输入 expected_home、up/down、必需可空绝对 config；返回配置/工具快照、原生目标观察及固定条件代码，不授权执行 |
| `wireguard.tunnel.apply` | write / exact_request，已实现 | 输入 confirmed、精确选择和完整计划；重新生成并比较计划，匹配后调用打包 helper，执行后重查目标并返回 observed/失败/未知回执 |

当前计划绑定版本、home、平台、动作、规范路径、配置对象及内容身份、工具快照和实际 tunnel/service 观察。执行前重新核对，确认过程不能悄悄换成最新计划；helper 缺失、权限失败或执行后目标不匹配会返回明确失败/未知结果。down 必须绑定真实接口或服务，不能只由文件名推导目标；macOS 要明确配置名称与 utun 的对应关系。

状态读取使用原生 API 或 Rust 解析的机器格式。若使用 `wg show all dump`，必须在 Rust 内丢弃私钥及 preshared key，任何响应、日志和错误均不得携带它们。工具成功、接口存在、握手可用是不同事实，分别呈现；无握手不能推断接口已停止。

## 配置、权限与执行边界

- 复用有界读取、nofollow、目录 handle、文件身份及权限检查。特权执行需使用已经确认的配置快照；前后 hash 比较不能消除工具重开可替换路径的竞态。
- `wg-quick` 配置允许 shell hooks。桌面首切片拒绝 PreUp/PostUp/PreDown/PostDown、SaveConfig 及无法安全解释的指令，明确呈现原因；不能把用户可写路径直接交给高权限进程。
- 被动检查和页面刷新不触发 UAC、sudo 或授权对话框。区分权限不足与无法观察，不从英文 stderr 推断所有系统权限状态。
- Windows 使用受保护 SCM LocalSystem broker 和本机 named pipe；macOS 使用 root LaunchDaemon；Linux 使用 systemd/D-Bus/Polkit 受控服务。仅返回 privilege_required 可以交付检查入口，不能计为三平台启停完成；三平台仍需真实主机安装、授权和隧道存活验收。
- 每个目标使用跨进程操作锁。执行有时间、输出和进程清理上限；启动后的取消可能留下实际变更，回执显示已知事实或未知，不自动重试或反向执行。
- 区分临时执行 helper 与成功启动的长期服务，不能在正常清理时杀掉 Windows tunnel service 或 macOS 后台隧道。

desktop 使用独立网络页，复用确认、异步提交、跨页状态、退出等待和本地化模式；未来启停与配置发布互斥，完成后使旧诊断和状态失效。目前发布会取消在途只读查询并丢弃旧观察，发布期间禁用新查询，完成后由用户显式重新查询。操作能力区分 CLI 方法支持与本地 OS 权限，不照搬配置生成的服务器/设备要求。

## 验证要求

本轮执行基础仅包含 Rust 内部边界：在原用户权限下单次读取已确认配置，接收端独立校验传递字节；系统保护目录按 request_id 保留准确基名的配置副本和非秘密记录。请求绑定调用者、完整计划、目标作用域、动作、逻辑名称和配置身份；这些摘要须由未来已认证桥计算，不能信任 UI 自报。锁顺序为 request 锁后 target 锁，target 锁按作用域和逻辑名称共享，不随 CLI home 或调用者改变。

操作记录只允许 prepared → submitted → observed/failed；提交记录必须先持久化并报告目录同步结果，Windows 的 Unsupported 不能当成完整持久化或执行许可。相同 ID 的任何已有阶段都只能读取原记录，异请求拒绝；残缺记录保留并拒绝复用。submitted 遗留表示结果未知，observed 不等于执行成功，failed 不证明回滚。broker 调用已接入协议，三平台真实安装、跨重启恢复和运行验收仍待完成。验证进度以[实施状态](implementation-status.md)为准。

默认测试用隔离 fake wg、wg-quick、服务与权限桥，不触发真实提权或网络变更。验证严格协议、目标绑定、配置/工具替换、链接与 FIFO、hooks、取消、锁竞争、大小上限、敏感字段剔除，以及成功服务不被清理。所有 desktop 测试、helper 和夹具放在仓库根 `tests/`，新增文案同步官方中英文 catalog；UI 覆盖跨页保留、明确确认、权限原因和三语言最小窗口布局。

三平台真实权限桥、接口映射和隧道行为单列显式运行验收；交叉编译和测试工具不能替代它们。Windows、macOS、Linux 是目标平台，FreeBSD 不在计划中。

## 已核查平台前提

以下结论来自本地源码和打包所固定的上游版本静态核查。只读 `tunnel.plan`、原生目标观察和严格 `tunnel.apply` 已实现；专项测试使用替身 helper，未宣称真实隧道、安装组件或系统授权已验收。Windows、macOS、Linux 的正式版启停要求不变。

**macOS 的 Bash 与实际 utun 映射。** [构建脚本](../../agent-remote-cli/scripts/build-managed-tools.sh)固定 WireGuard tools `1.0.20210914`，原样复制 `wg-quick/darwin.bash`，另外构建 `wireguard-go`；[安装列表](../../agent-remote-cli/scripts/install.sh)没有受管 Bash。[上游 darwin.bash](https://github.com/WireGuard/wireguard-tools/blob/v1.0.20210914/src/wg-quick/darwin.bash#L42)明确要求 Bash 4 以上，首行却通过 `/usr/bin/env bash` 找解释器，因此不能假定系统自带 Bash 或用户 Homebrew 满足要求。沿用该后端时须补固定版本、校验及受保护安装的 Bash，并由执行桥指定可信解释器；原生替代后端则必须完整承担路由和 DNS 生命周期。

同一脚本的 [`get_real_interface/add_if`](https://github.com/WireGuard/wireguard-tools/blob/v1.0.20210914/src/wg-quick/darwin.bash#L110)通过 `WG_TUN_NAME_FILE=/var/run/wireguard/<配置名>.name` 请求动态 `utun`，再读取映射并检查 `<utun>.sock` 和两者修改时间差；文件名不是实际接口名。up 的计划应绑定逻辑配置名、配置快照及预期目标不存在，不能预先虚构一个确定的 utun。特权服务创建后记录实际 utun、运行代次、接口公钥和受控映射身份；down 计划必须返回这一真实目标并在执行前复验。仅复制上游的时间差检查不足以证明归属，旧映射、替换映射和已复用的 utun 必须单独处理。上游 [`monitor_daemon`](https://github.com/WireGuard/wireguard-tools/blob/v1.0.20210914/src/wg-quick/darwin.bash#L323)还保留长期路由/DNS监视进程，launchd 模式会在脚本末尾等待它；不能把只读 probe 的“退出后杀整组”策略原样用于成功启动的隧道。

**Linux 的接口身份及系统依赖。** [上游 linux.bash](https://github.com/WireGuard/wireguard-tools/blob/v1.0.20210914/src/wg-quick/linux.bash#L40)将 `.conf` 基名限制为 1–15 个指定 ASCII 字符，直接用它执行 `ip link add <name> type wireguard`；down 最终执行 `ip link delete dev <name>`。因此确认需同时绑定网络命名空间、实际接口名、ifindex、WireGuard 类型及接口公钥；ifindex/名字被复用后必须重新确认，不能按配置基名直接删除。每个真实目标的锁须跨 CLI home 生效。当前打包提供 wg/wg-quick，仍要核查受信任的 `ip`、DNS 管理及路由/防火墙依赖；上游有用户态回退分支，但当前 Linux 包没有构建 wireguard-go，不能把这一回退当作已提供的能力。

**Windows 的服务目标和持久配置。** 当前 [helper](../../agent-remote-cli/src/bin/agent-remote-wireguard.rs)对 up 执行 `/installtunnelservice <config>`，down 只取 `file_stem()` 执行 `/uninstalltunnelservice <name>`。[打包脚本](../../agent-remote-cli/scripts/package-release.ps1)默认固定 WireGuard for Windows `1.1`；该版 [`ServiceNameOfTunnel`](https://github.com/WireGuard/wireguard-windows/blob/v1.1/conf/name.go#L122)产生 `WireGuardTunnel$<name>`，[`InstallTunnel/UninstallTunnel`](https://github.com/WireGuard/wireguard-windows/blob/v1.1/manager/install.go#L120)通过 SCM 创建自动启动服务、记录 `/tunnelservice <configPath>`，以及停止并删除服务。新计划须绑定确切服务名、服务配置中的程序/配置路径和文件身份、账户及启动类型，并通过 SCM 查询实际服务状态；不得仅凭同名文件停掉另一来源服务。受保护配置副本需要保留到服务卸载与清理完成，不能当一次性临时文件删除，因为自动启动服务以后还要读取它。确认页必须披露自动启动及 down 删除服务的现有语义；安装命令成功、服务运行、接口出现和握手分别记录。

**现有签名与权限能力可借鉴，但不是平台验收证据。** [Windows relaunch](../../agent-remote-cli/src/bin/agent-remote-wireguard/windows.rs)保留给普通 CLI 兼容路径；desktop apply 已改为受保护 SCM LocalSystem broker，通过本机 named pipe 绑定 token/SID/session/process 和一次性 permit。Windows 安装脚本提供 `-InstallBroker`，不要求发行者 Authenticode 签名。[macOS CLI 打包](../../agent-remote-cli/scripts/package-release.sh)的签名分支仍只签主 CLI；WireGuard broker 使用固定 LaunchDaemon 和项目社区自签身份，不能把 app 签名直接等同于 root 隧道权限。Linux 通过 systemd、system D-Bus 与 Polkit action 安装受控服务。三平台服务实现与安装材料已落盘，但真实安装、系统授权、隧道启停、重启/取消恢复仍需在目标主机验收。

**建议按可验证阶段推进，三平台最终闭环。** 共享目标解析与只读 plan 已实现：使用原生/机器格式观察补充平台目标身份，无法观察时明确阻断，不把权限失败当作目标不存在；当前快照不证明归属或运行代次。下一步完成三平台桥的安装、调用者身份、请求绑定和受保护快照传递；Rust 私有配置字节只在 CLI 与特权服务之间传输，不经过 Go UI。执行桥以调用者权限核验原配置，独立解析并核对计划内容身份后写入受保护的执行副本，执行工具只读取该副本；副本目录可随机化，但必须保留已确认的合法配置基名，避免改变接口/服务目标。特权工具及其依赖也必须处于受保护路径并固定身份，清理继承的 PATH 和 WireGuard 实现覆盖变量。

最后接入 apply 与独立观察回执，分别记录授权、提交、执行、目标观察及清理阶段；按请求 ID 保留最小受保护操作记录，以便丢失回执后核查原操作而不重放。macOS/Linux 的长期进程交给明确的服务生命周期，Windows 服务交给 SCM，临时 worker 清理不得杀掉已经启动的服务。既有手工启动目标需要显式选择和身份核对，不能因基名相同而自动接管。上述阶段可以先用隔离工具和 IPC 合同验证，但只有三平台真实授权、正确目标启停、重启/取消恢复和服务存活验收完成，才计为启停 GUI 覆盖完成。
