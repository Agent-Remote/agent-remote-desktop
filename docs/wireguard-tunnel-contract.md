# WireGuard 隧道只读计划契约

状态：`wireguard.tunnel.plan`、共享 Rust 原生目标观察、Go 网络页预览和 `wireguard.tunnel.apply` 已实现。计划用于展示配置、工具、实际目标与未满足条件；确认后由 Rust CLI 重新核对计划，在用户权限下读取配置并调用固定的平台 broker client，只有 broker operation receipt 和第二次观察确认目标状态后才返回回执。服务缺失、身份/permit/协议失败均 fail-closed，不回退 sudo、runas 或普通 helper。三平台 broker service 实现和安装材料已加入仓库，真实主机安装、授权、隧道实测与恢复仍需分别验收；当前 macOS/Linux/Windows 交叉构建不计作运行验收。

## 请求与结果

方法属于协议版本 1，声明 `effect: read`、`credential: none`。请求准确包含 `expected_home`、`action` 和 `config`；`action` 只能为 `up` 或 `down`，`config` 为必需可空字段，null 选择 `HOME/wireguard/agent-remote.conf`，否则要求规范绝对路径。home 必须与所选 CLI 环境完全一致。不创建 home、锁、数据库或凭据，不登录、不访问服务器、不启动隧道、不安装组件、不触发授权。

`wireguard.tunnel.apply` 是同一协议版本的严格确认入口，输入准确包含
`confirmed`、`expected_home`、`action`、`config` 和完整 `expected_plan`。它重新生成
只读计划并与确认计划逐字段比较；任何变化返回 `wireguard_plan_stale`。计划仍有任何前置问题时返回 `wireguard_precondition_failed`；无问题时才调用平台 broker client。broker 返回 operation receipt 后重新观察目标，只有目标达到 present/absent 才返回 observed 回执。

结果字段全部必需；可空字段显式为 null：

| 字段 | 含义 |
| --- | --- |
| `version` | 固定为 1 |
| `home / platform / action` | 准确环境、平台及本次选择的动作 |
| `config` | 与 `wireguard.check` 相同的七字段配置摘要 |
| `binding` | null，或 `file_identity / content_sha256 / public_key` |
| `tools` | wg、平台隧道工具，依次包含 `name / state / path / identity / sha256` |
| `target` | 下述原生目标观察 |
| `issues` | 固定顺序、不重复的条件代码 |

配置在同一个安全打开的文件 handle 上完成解析与最多 1 MiB 的内容摘要；前后复验对象、路径与权限。ready/unsupported 必须有 binding，其他状态为 null。公钥在 Rust 内从私钥推导，配置正文、私钥、PSK、hooks 值均不返回 desktop。发生变化时不能把不同时间的配置摘要和身份拼成有效计划。

工具快照只对可执行文件进行最多 64 MiB 的读取，并复验文件身份；失败时保留工具清单状态，identity/sha256 均为 null。不可用工具同样没有摘要。公开对象身份使用带独立用途域的 SHA-256，编码为 64 位小写十六进制；内容 sha256 记录所读字节。文件身份包含完整原生身份，Windows 使用完整 128 位 file ID。工具摘要仅表示当前文件，不证明签名、可信安装来源或提权资格。

## 原生目标事实

`target` 必需字段为 `logical_name`、`state`、可空 `reason`、`status_state`、可空 `identity`。

- state：absent / present / unknown / conflict。
- status_state：复用检查/状态契约的七种状态，增加 not_queried。
- reason：null，或 invalid_name / native_query_failed / mapping_missing / mapping_unsafe / mapping_stale / interface_not_wireguard / service_invalid / target_changed / status_unavailable / target_not_in_status。
- 只有 present 携带 identity；unknown/conflict/absent 的 identity 为 null。invalid_name 返回空 logical_name、unknown、not_queried。

逻辑名采用 Rust `Path::file_stem`：去掉最后一个扩展名，单独的点文件名保留；只读预览不强制 `.conf` 后缀，实际执行的命名限制仍需单独校验。Linux/macOS 采用固定 wg-quick 后端的 `[a-zA-Z0-9_=+.-]{1,15}`，另拒绝单独 `.`、`..`；Windows 对齐固定 WireGuard 1.1，长度 1–32、相同字符集，并拒绝首尾点、保留设备名及其带扩展名形式。名称仅用于查询候选目标，不足以生成停止授权。

| identity.kind | 其余必需字段 |
| --- | --- |
| linux_interface | name、正整数 index、namespace 身份摘要、可空 public_key |
| macos_interface | 实际 utun name、正整数 index、mapping_identity、socket_identity、可空 public_key |
| windows_service | service_name、binary_path、config_path、可空 config_identity、service_state、start_type、account、可空 public_key |

Linux 在同一线程的网络命名空间中进行有界 NETLINK_ROUTE 查询，绑定接口名、ifindex、WireGuard 类型及 namespace；只有内核明确报告该名称不存在才返回 absent。其他接口类型为 conflict；权限错误、异常或格式错误为 unknown。

macOS 从固定 `/var/run/wireguard/<logical_name>.name` 读取有界映射，nofollow 检查目录、映射和实际 `<utun>.sock`，绑定映射/socket 的身份及 utun 索引。映射缺失或陈旧为 unknown，不能证明没有遗留 utun；mtime 接近也不构成归属证明。测试用临时映射和本地 socket，不创建真实 utun。

Windows 使用本机 SCM 查询准确的 `WireGuardTunnel$<logical_name>`。只接受实际服务命令的三个参数：程序、`/tunnelservice`、绝对配置路径；不回传原始命令行。绑定服务配置、账户、启动类型、当前状态和安全读取到的配置对象身份。service_state 为 1–7，start_type 为 0–4，account 最多 512 字节且不含控制字符。仅 SCM 明确的服务不存在返回 absent，无法查询为 unknown；已停止服务仍然存在，不能把它当作 absent。配置身份可通过系统路径别名匹配，不要求路径字符串完全相同。

发现 present 后，原生前快照、一次有界 `wg show all dump` 和原生后快照固定在同一个操作系统线程完成，避免 Linux 查询与工具运行使用不同网络命名空间。前后原生事实变化返回 unknown/target_changed；status 失败仍保留 present 原生事实并说明 status_unavailable，成功但没有相应接口时说明 target_not_in_status。公钥仅来自匹配的接口，缺失时保持 null。

以上均为当前观察，不包含受保护的运行代次或所有权记录。手工目标、名字/ifindex/utun 复用及服务替换仍需未来执行桥独立核实。原生 SCM 等调用不能强制取消，外层 CLI 进程提供最终生命周期边界；取消和查询 future 丢弃会通知只读 worker，正在运行的 wg 使用既有有界清理。

## 条件代码与桌面交互

issues 只包含适用项，顺序固定：config_not_ready、config_unsupported、tool_unavailable、tool_snapshot_unavailable、target_unknown、target_conflict、target_already_present、target_absent、config_target_mismatch、target_key_unobserved、service_config_mismatch。

up 需要明确观察为 absent；down 需要实际 present 目标与配置公钥匹配，Windows 还要求配置对象身份匹配。

网络页由用户选择启动/停止方向及默认/自选配置后显式预览；提供计划、未满足条件和显式执行按钮；只有 issues 为空时按钮可用。默认展示动作、目标及配置，详情可查看快照身份；Windows 说明未来后端的自动启动和删除服务语义。切换动作或配置使旧计划失效，跨页保留结果、独立取消并丢弃迟到响应，配置发布开始时取消计划查询并使旧观察失效。

Go 在异步能力检查前冻结输入，严格校验平台、home、动作、路径、必需字段、可空字段、哈希、公钥及完整 issues 列表；不自动重试。所有 desktop 自有文案使用官方 mygo 中英文 catalog；desktop 测试、夹具及专项程序均在 desktop 仓库根 `tests/`。验证证据与缺口见[实施状态](implementation-status.md)和[平台矩阵](platform-validation.md)，不以交叉类型检查替代原生服务、权限桥或真实隧道验收。
