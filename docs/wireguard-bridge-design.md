# WireGuard 权限桥实施提案

状态：**只读 `wireguard.bridge.status` 已接入；Rust 已实现 macOS/Linux/Windows broker client 与对应受保护 service 实现、caller identity、配置字节/摘要绑定和一次性 execution-permit 合同。** 仓库已提供 macOS LaunchDaemon、Linux systemd/D-Bus/Polkit、Windows SCM LocalSystem 的安装材料；未安装真实 service 时所有执行仍 fail-closed。三平台 broker 现在在固定受保护目录持久化 one-shot request journal，启动时恢复并对损坏、重复或不安全状态 fail-closed。三平台主机安装、授权、执行和运行验收仍待完成，不把交叉编译或替身测试当作平台验收。`wireguard.tunnel.plan` 保持只读；`wireguard.tunnel.apply` 严格重查冻结计划，普通用户侧读取配置后经平台 broker client 请求执行，并仅在 broker receipt 与第二次目标观察确认后返回回执。Rust [执行基础](../../agent-remote-cli/docs/desktop-wireguard-execution.md)记录受保护配置传递、caller-auth 合同和操作存储的后续增强设计。

本提案用于接续 P2。desktop UI 与业务保持 Go/mygo，权限操作复用 Rust CLI，
仅通过薄原生绑定使用系统授权 API。目标为 Windows、macOS、Linux；FreeBSD 不在计划中。
既有后端约束见[隧道设计](wireguard-tunnel-plan.md)，进度以[实施状态](implementation-status.md)为准。
只读状态字段和 fail-closed 规则见[权限桥状态合同](wireguard-bridge-status-contract.md)。

## 权限边界与社区发行选择

认证回答“这条连接由哪个进程和本机主体建立”；授权回答“该主体是否获准执行这次系统修改”。
有效签名、同一 UID、远程账户登录、工具摘要或收到确认按钮事件，均不能单独替代本机授权。
被动状态刷新不安装组件、不唤起授权对话框；安装/注册与修改网络分别由显式操作触发。
desktop 不接收管理员密码，不重启为管理员，也不直接读 CLI 凭据、数据库或私有配置。

已决定正式版为面向公开开源构建的社区版；商业二进制签名、Apple Developer ID 和
Apple notarization 不作为发行门槛。系统授权、受保护安装和原生进程身份校验仍须完成，
不能因没有商业签名而用普通校验和替代调用者认证。

macOS 已选择沿用持久的项目社区自签身份，固定项目公有证书身份与代码 identifier，
建立跨版本代码约束；Windows/Linux 不要求发行者代码签名，不设 Authenticode 门槛。
公有证书可作为信任配置交付，私有签名密钥、P12 和密码不得进入仓库或公开发行制品。
社区自签不等于 Apple 信任或公证；不能只校验可自选的 bundle identifier，也不能直接
沿用 Device 的自签 profile 推定 root daemon 安装可用。仍须验证 SMAppService 注册、
系统审批和 IPC 的实际行为，不承诺当前已可用。

平台矩阵仍需通过验证收敛 Linux 发行版、桌面环境及安装形式，不以此阻断实施；macOS 13+
是结合 mygo 与 SMAppService 的建议下限，最终支持范围以明确的产品矩阵和实测为准。

建议每次网络修改申请本机管理员授权，不默认持久授权所有登录用户。
协议上限、固定错误码、锁顺序、连接超时等属于实现细节；信任策略应通过受保护构建配置或
管理员批准的受保护安装建立，不能由任意 UI 参数、用户可写配置或服务端响应指定。

## 三平台原生通道

| 平台 | 安装与生命周期 | 通信和认证 | 修改授权 |
| --- | --- | --- | --- |
| macOS | desktop/CLI 配套固定 LaunchDaemon plist 和 root broker；社区自签身份固定，socket 使用 root:admin（gid 80）`0660` | 固定 Unix socket；服务从 `LOCAL_PEERCRED`、peer PID 与进程启动时间获取调用者事实，客户端核验固定社区身份和协议 | broker 在服务侧绑定 caller、计划、目标和一次性 permit；真实主机安装与权限验收单列 |
| Linux | 系统包安装 root 拥有的 Rust broker、systemd/D-Bus 服务及 polkit action；用户目录 CLI 连接系统服务 | 系统 D-Bus；使用内核/总线提供的 unique sender 身份，并绑定 UID/PID/start-time | polkit 的 `unix-process` subject，使用 `auth_admin`，不使用无条件允许规则 |
| Windows | 受保护安装的 SCM LocalSystem broker，不要求商业 Authenticode 签名；配置和记录位于私有 ProgramData 子目录，长期隧道由 SCM 管理 | 仅限本机的受保护 named pipe；双方核验 live process handle、token、会话、SID 及受保护安装绑定 | `-InstallBroker` 注册固定服务；每个请求只消费一次 permit，拒绝普通 `runas`/helper fallback |

**macOS。** 公开的 `xpc_connection_set_peer_code_signing_requirement` 从 macOS 12 可用，
会检查连接上收到的消息；SMAppService daemon 从 macOS 13 可用。可使用 Rust 原生绑定处理
libxpc，Go 只保留注册/状态所需的薄绑定，不引入 Swift 业务实现。授权外部形式只通过已认证
连接传递，不进入日志或 Go；daemon 验证固定 right，不能接收调用者自选 right。
安装审批不等于每次操作授权。代码身份要求必须覆盖实际 CLI 与 daemon，不能只检查 app 外壳。
项目自签方案需固定可信公有证书和代码 identifier，结合原生连接事实核验动态代码；
不能把任意临时自签或 ad-hoc 当成已固定的项目身份，也不自动信任系统里的所有自签证书。
既有 Device XPC 是应用级服务，其同 UID 检查不能原样用于 root daemon。

若最终实现选用 Unix socket，公开 `LOCAL_PEERTOKEN` 可获得 audit token，再通过
`kSecGuestAttributeAudit` 验证动态代码；不能退化成读取一个 PID 对应路径后检查磁盘签名。
此项只是可替代的原生实现方式，首选通道仍为 libxpc。

**Linux。** 建议使用窄配置的 Rust `zbus` 依赖，将 polkit subject 绑定 D-Bus unique sender，
避免自行拼接 PID、`/proc` starttime 与授权请求造成进程复用竞态。服务必须从消息上下文获取
sender，不能接受请求中的自报身份。客户端查询 well-known name 当前 owner 的系统身份，
随后将调用固定到该 unique name；owner 变化使在途确认失效。没有系统 D-Bus/polkit 时报告
缺失组件，不静默退化到 sudo。任意用户可写的 `~/.agent-remote/bin` 不能作为 root 服务程序路径。

**Windows。** CLI 创建 pipe 时限制 DACL、拒绝远程客户端并要求首次实例独占；随机名称和
nonce 仅用于绑定连接，不能替代身份校验。CLI 保留 UAC 返回的进程 handle，检查连接者 PID
对应同一个仍存活的进程对象、creation time、elevated token 和可信镜像。worker 也核验 pipe
server 的进程对象、原用户 SID/logon identity，以及实际 CLI 镜像与受保护安装对象的绑定。
不能仅对用户可写路径计算摘要并将其列为可信；安装来源和确认清单必须先由管理员批准并
放入不可由普通用户改写的安装域，镜像检查还须防止路径/文件替换。安装清单约束允许的代码
对象，进程 handle/token 认证实际连接者；两者不能互相替代。不要求项目制品具有 Authenticode
发布者签名；附带上游签名的第三方制品按该依赖既定策略核验。
SID 相同不能证明对端就是
发起本次操作的 CLI；PID 数值相同不能排除进程复用。其他管理员代为授权时，原请求主体与
授权管理员是不同身份，必须分别记录和验证，不能简单要求两端 SID 相同。

UAC、pipe I/O 和 worker 退出均使用有界等待。临时 worker 清理不能终止已创建的长期
WireGuard service。现有 helper 的 `runas` 退出码和无限等待不具备这些条件。

## 请求与确认绑定

以下是权限桥和回执的边界。`wireguard.tunnel.apply` 的基础请求字段已发布为严格的
fail-closed 合同并加入当前 capability 清单；其余长期桥接方法尚未发布：

| 拟议入口 | 输入/输出范围 | 边界 |
| --- | --- | --- |
| `wireguard.bridge.status` | 返回安装、批准、协议兼容及认证通道状态 | 已接入只读查询；未安装/服务不可观察仍为 fail-closed，Windows 可观察固定 SCM 安装事实但 caller/auth 仍未就绪，Linux system-bus 与 macOS socket 观察均不触发授权，不能把“组件存在”显示为可执行 |
| 内部 `target.inspect` | 合法逻辑名；返回 broker 原生 scope、目标观察、观察者身份 | 已认证通道，只读；与现有用户态观察区分来源 |
| `wireguard.operation.status` | 原 request ID；返回允许该主体读取的原操作记录 | 不重放，不泄露其他调用者记录；不宣称 observed 就是成功 |
| 内部 prepare/submit | challenge、request ID、完整确认计划和私有配置字节；返回绑定回执 | 只有认证、授权、可信工具、持久化和目标检查全部具备后才能接入 |
| `wireguard.tunnel.apply` | `confirmed`、精确选择与完整计划；Rust 先重新生成并比较只读计划，用户权限侧读取配置并发送字节/摘要 | 仅调用固定平台 broker client；broker 必须认证 caller、校验 permit 并返回 operation receipt，执行后再重查目标并返回 observed/失败/未知回执。没有 sudo、runas 或 helper fallback；受保护 service 安装、授权和平台运行验收仍待完成 |

broker 先产生随机 challenge，并返回协议版本和安装身份。CLI 通过同一认证连接提交
完整已确认计划及独立长度分帧的私有配置字节；每段和整次请求均有大小、时间和输出上限。
私有字节不放入 desktop JSON、Go、argv、错误详情或操作记录。

Rust CLI 已实现可供原生传输调用的严格 caller-auth 合同：挑战 ID、nonce、broker
安装身份和 30 秒有效期组成一次性上下文；broker 发送前先把 challenge 登记到有界 guard，
响应以 HMAC-SHA256 绑定独立取得的 caller digest、完整 request digest 及挑战全部字段。
broker 使用常量时间比较，并通过该 guard 消费 challenge；未登记、篡改、重复、过期、时钟
倒退、上下文变化、未知字段和容量耗尽均 fail closed。credential 只作为内存 key 使用。
该模块不读取 OS caller 身份、不授权请求、不安装 broker；平台 client 已将 audit token、D-Bus
sender 或 Windows process/token 事实作为 broker 返回 permit 的绑定输入，服务缺失或绑定失败
仍保持 `bridge.status` 的 `unsupported`/`unavailable` 和执行拒绝。

CLI 在原用户权限下调用 `ExecutionConfig::from_source`，broker 对传入字节调用
`from_transfer` 独立解析，核对内容摘要、公钥和允许的指令。broker 不以 root 重开调用者路径。
原 source file identity 是来源声明；仅凭传入字节，broker 不能证明原文件身份。
需要验证的是确切字节与完整确认内容一致，并且修改已由认证主体获得授权。

caller、scope、plan 摘要由 broker 根据认证上下文、原生观察和规范编码自行计算，
不信任 UI 自报摘要。重新观察发现变化时要求重新确认，不能悄悄替换成最新计划。
连接认证使用进程/audit token 等即时事实；持久操作归属使用稳定本机 principal 和安装域，
让 CLI 重启后仍能认证并查询原请求，不能把短命 PID 作为唯一持久归属。
准备状态不产生可离线复用的长期 bearer ticket；断开连接不能自动重新提交。

执行作用域固定为 broker 所在主机和网络 namespace，不接受任意 namespace 路径或切换要求。
Linux 即使目标不存在也必须返回独立 scope；当前 absent observation 缺少这一绑定，不能直接
成为可执行计划。up/down 都在同一 scope 内重新检查目标，down 另须验证真实目标及归属代次。

## 可信工具和持久化前置条件

特权工具、解释器和动态依赖必须来自受保护安装，不能把用户可写文件的 SHA-256 当信任根。
清理 PATH、动态加载及 WireGuard 实现覆盖环境变量，使用固定参数向量和固定可信程序路径。

当前 `wireguard.tunnel.apply` 在调用打包 helper 前还会执行一次本地边界检查：路径必须是绝对路径，目标必须是未链接的 regular file；Unix 拒绝硬链接、group/other 可写或没有执行位的文件。这个检查用于阻止明显的链接和权限替换形式，不建立文件内容信任，不能替代受保护安装、调用者认证、系统授权或 broker；任何失败都在 helper 启动前返回，不会把工具存在或当前进程权限解释为执行许可。

- macOS 沿用 wg-quick 时需交付固定、受保护的 Bash 4+、wg、wg-quick 和 wireguard-go，
  并管理长期路由/DNS 监视器，不能假设系统 Bash 满足要求。
- Linux 需检查 Bash、wg、ip、DNS 和路由/防火墙依赖的可信来源及实际可用性；
  不能把未打包的 wireguard-go 回退当成已提供功能。
- Windows 需验证受保护 WireGuard 安装、被批准的来源和确切工具身份；采用附带上游签名的
  制品时，按该制品策略核验签名。项目自身不因此要求商业签名。自动启动服务所需配置必须
  跨 worker 和主机重启保留。安装、运行、接口出现、握手是不同事实。

**Windows submission 持久化仍是执行阻断项。** 当前存储使用写临时文件、文件 sync、rename、
目录 sync；Windows 目录 sync 返回 `Unsupported`。仅成功认证 IPC 或 flush 临时文件，
不能将该结果改称 durable submission，更不能授予执行许可。

执行存储现已建立受保护的 append-only submission journal：每个 request 追加 canonical、
连续序号和哈希链事件，持锁 flush 文件及目录后才发布阶段；原 per-request JSON 仍作为
物化记录。缺失 journal、空行、不完整尾部、乱序、重复、身份不一致或哈希篡改均禁止恢复
和新执行，不自动重建以绕过防重放。该 journal 是恢复证据，不是 broker 授权或掉电保证；
Windows 目录 flush 仍明确为 `Unsupported`，所以 Windows 原生 durable submission、安装持久化、
损坏恢复与三平台运行验收仍需在目标系统完成。不能为了推进 UI 将现有 `Unsupported` 降级为可接受成功。

## 实施与验收顺序

1. 完成三平台安装/注册状态和认证 `bridge.status`，由 desktop 页面实际消费；
   当前只完成无提权的被动查询和 fail-closed 状态投影，仍需区分未安装、待批准、身份错误、协议不兼容与 ready。
2. 接入认证目标观察和按主体授权的原记录查询；呈现实际 scope、来源与未知状态，
   不改变现有只读 plan 的执行限制。
3. 在同一通道接入配置传递、完整确认验证和 journal；隔离测试覆盖伪造身份、进程退出/复用、
   owner 变化、超时、超限、取消、连接中断、同 ID 冲突及每阶段禁止重放。
4. 完成可信工具、Windows durable submission、执行前目标复验和长期进程归属，
   再固定并发布 apply 契约及 Go 确认/恢复界面。
5. 三平台分别验收实际安装、原生授权、启停、取消、回执丢失、应用/主机重启和服务存活。
   交叉编译、隔离工具测试、进程崩溃测试都不能替代这些运行验证或证明掉电安全。

默认测试不提权、不安装系统组件、不创建真实隧道。desktop 所有测试、辅助程序和夹具继续
放在仓库根 `tests/`；新增 UI 文案同步官方 mygo 英文和简体中文 catalog。
只有真实执行与恢复闭环通过平台验收后，才能计为该平台 up/down GUI 覆盖完成。
