# WireGuard 配置 GUI：协议与实现边界

**状态：共享服务、桌面协议和 Go 界面已实现，本机整仓门禁及 Linux 专项通过。** 本文记录 P2 `wireguard config` 的确认和执行契约；不代表三平台运行验收或完整 CLI GUI 覆盖已完成。最新验证证据以[实施状态](implementation-status.md)为准。

范围为选择输出位置、确认、登记设备公钥、获取配置并安全发布文件。普通 `agent-remote wireguard config --output`、`init` 的可选 WireGuard 步骤和桌面必须调用同一 Rust 服务；桌面 UI 与业务适配仍为 Go。该操作不启动隧道；已实现的 `check/status` 另见[检查与状态契约](wireguard-inspection-contract.md)。`up/down` 已有严格计划/确认与平台 broker client 执行路径，`wireguard.bridge.status` 已提供无提权状态查询；三平台受保护 broker service 实现和安装材料已落盘，平台授权、真实运行和恢复验收仍待完成。

## 该切片修复的原有问题

- 此前 CLI 先读取/生成私钥，再 enroll、GET 配置和直接覆盖文件，尚未固定输出路径身份。
- 原有普通凭据读取会忽略部分系统错误，不能据此认定私钥不存在并生成新密钥。
- 原有配置渲染直接拼接服务器字段；必须防止换行和配置指令注入。
- 原有文件写入在覆盖后设置权限，需要改为私有暂存和原子发布。
- Server 的 enroll 已返回设备、peer 和接口地址，但原有 CLI 丢弃回执；GET config 不包含当前设备公钥，不能用于消除丢失 enroll 回执后的不确定性。

## 协议与确认身份

提供 `wireguard.config.plan` 和 `wireguard.config.apply`，继续使用版本 1 的单请求 JSONL 协议。方法由 capabilities 显式声明，Go 严格校验字段和状态组合；机器契约见 [CLI 协议](../../agent-remote-cli/docs/desktop-control.md)。

`plan`：`effect: read`、`credential: none`，参数为：

```json
{"expected_home":"/absolute/cli-home","output":null}
```

`output: null` 使用 CLI 默认配置路径；桌面自定义输出必须为绝对路径。普通 CLI 的相对 `--output` 由共享服务按当前目录解析。计划只读取有界配置与文件元数据，不读取凭据、不联网、不生成私钥，不创建目录、锁文件或日志。

计划结果包含以下内容：

| 字段 | 含义 |
| --- | --- |
| `plan_version` | 初始为 `1` |
| `home` | 有效绝对 CLI home |
| `context` | 精确 `server_url` 和 `active_device_id` |
| `output.path` | 展示并确认的规范绝对输出路径 |
| `output.action` | `create` 或 `replace` |
| `output.anchor_path`、`anchor_identity` | 最近现存祖先目录及其平台文件身份 |
| `output.missing_directories` | 锚目录以下待创建的目录后缀 |
| `output.target_identity` | 不存在时为 null；存在时记录文件身份、尺寸及高精度修改/变化时间 |

身份值为 64 位小写十六进制 SHA-256 摘要。目录绑定 Unix device/inode 或 Windows volume/file ID，目标文件还绑定尺寸与时间等元数据；不读取配置正文，不包含私钥。`target_identity` 字段必须显式出现，新建时为 null。受控 home、目录和目标拒绝 symlink、junction、特殊文件；目标也拒绝硬链接。解析系统祖先别名后展示实际路径，兼容 macOS `/var` 等系统别名。

`apply`：`effect: write`、`credential: device`、`confirmation: exact_request`，参数为 `confirmed: true` 和完整 `expected_plan`。不得只确认路径字符串，也不得在提交时默默重算计划替代用户确认。

确认页展示服务器、设备、完整输出路径、新建/覆盖，以及“可能生成本地私钥并登记或更新设备公钥；不启动隧道”。修改输入、切换上下文或路径身份变化必须重新计划和确认。全部页面、错误和阶段文案同步进入官方 mygo 中英文 catalog；路径、协议标识及原始用户内容不翻译。

## 共享模块与执行顺序

| CLI 共享模块 | 职责 |
| --- | --- |
| `wireguard/configuration.rs` | `plan(paths, output) -> ConfigPlan`；`apply(paths, &ConfigPlan)` 与 `apply_cancellable(paths, &ConfigPlan, cancel)` 返回 ConfigOutcome，共享业务流程 |
| `wireguard/publication.rs` | 路径身份、父目录 handle、发布锁、安全暂存和原子发布 |
| `wireguard` 验证模块 | 检查 API 字段；renderer 每次渲染前执行验证，不拼接未经验证的字段 |
| `secrets/strict.rs` | 严格凭据/私钥读取及安全保存；秘密类型不实现 Debug/Serialize |
| `auth` | 登录锁内核对上下文、严格加载设备凭据并复用必要刷新逻辑 |
| `api.rs` | 网络请求及 typed enroll 回执，不在桌面或文件模块复制 HTTP |
| `desktop_control/wireguard.rs` | 参数与协议映射；普通 `app/account.rs` 调用同一服务 |

执行顺序：

1. 校验确认、home、上下文和完整路径快照；依次获取 home 登录锁、用户级服务器/设备 WireGuard 凭据锁，然后重新核对。
2. 安全准备缺失目录，持有父目录 handle，并取得父目录内共享发布锁，串行化正常 CLI/桌面写入。先创建同目录私有暂存文件并设置权限，尽早发现输出不可用。
3. 严格读取设备 token 与私钥；仅确定不存在时生成私钥，并在 enroll 前可靠保存。读取失败或保存不确定时不得 enroll。
4. 发送一次 enroll，验证 typed 回执设备身份；确认后才 GET config。接口地址与后续配置必须一致。
5. 校验响应设备、大小和 peer 数量，验证 IP/CIDR、DNS、端点 host/port、Base64 公钥与 keepalive。拒绝控制字符及多行字段；节点注释清理或省略，不能直接拼接原始名称。
6. 再次核对上下文，然后向私有暂存文件写入已验证配置并 sync_all；发布前复验目标身份。
7. 新建使用不覆盖发布；替换使用平台原子替换。Unix 另同步父目录，Windows 当前不执行目录 fsync；若发布已发生但后续同步失败，不能声称文件未改变。不能把平台内已完成的同步步骤等同于跨平台断电持久性保证。

锁的作用域必须覆盖实际共享数据：系统凭据键按服务器/设备共享，现有每个 CLI home 的登录锁不足以串行化两个 home 对同一 WireGuard 私钥的生成和登记。实现使用用户级、按服务器/设备绑定的共同锁，普通 CLI、`init` 和桌面共享服务均遵守同一获取顺序；不得把输出目录锁视为凭据锁。独立锁回归覆盖同一身份互斥、不同身份和取消；真实系统凭据交互仍需平台验收。现有登录/注销的跨 home 凭据锁限制仍应单独保留记录。

私钥及含私钥的配置正文不进入 GUI、协议输出、argv 或日志。获取计划失败不得触发普通命令回退；GUI 不解析人类终端输出。

## 严格凭据和平台权限

- 读取必须区分 `Found`、确定 `Absent`、拒绝、系统不可用、超时和无效值。系统失败时不生成新私钥，也不静默选择可能陈旧的 fallback。
- 明确 file backend 时仅访问文件；自动模式只有系统明确缺失后才读取合法 fallback。缺失与不可访问不可合并。
- fallback 采用有界 nofollow 读取，检查目录/文件类型、链接及权限，拒绝 FIFO。私钥必须为规范 Base64 的 32 字节；无效现有值不能自动替换。
- macOS 使用现有 `security-framework` 能力写入私钥，避免 `security ... -w <secret>` 的 argv 暴露；Linux 凭据工具需要超时、输出上限与取消清理；Windows 使用原生凭据 API。
- Unix 新配置文件为 0600；Windows 在写入秘密前设置当前用户权限，并保留 LocalSystem 读取权限，优先原生 ACL API，不依赖用户名环境变量。

仅支持 Windows、macOS、Linux；FreeBSD 不在计划中。三平台均需原生权限、替换、取消和并发测试，交叉编译不能代替运行验证。普通协作进程使用共享锁；对同用户恶意进程在最后复验与替换之间更换路径，不宣称 `stat + rename` 提供跨平台原子 CAS 保证。

## 阶段、部分成功与未知结果

协议结果包含 `operation_id`、`home`、`context`、`output_path`、`complete`、`key_state`、`enrollment`、`publication`、稳定 `code` 和可空 `peer_count`；登记确认后还包含仅公开的 `peer_id` 与 `peer_version`。Go 只有在 peer identity/version 完整且状态组合一致时才接受确认回执；不返回私钥或 token。`operation_id` 由协议包装层按 request_id 补入共享 ConfigOutcome。

- `key_state`：`not_checked / reused / created / unknown`。
- `enrollment`：`not_attempted / rejected / confirmed / unknown`。
- `publication`：`not_attempted / published / failed`。
- `complete` 仅在登记已确认、文件已发布且该平台实现的同步步骤通过时为 true；Go 必须拒绝矛盾组合。

前置错误使用 `invalid_params`、`confirmation_required`、`configuration_changed`、`configuration_unavailable`、`authentication_required`、`output_invalid`、`output_unavailable`、`output_changed` 或 `operation_busy`。输出准备可能已创建目录或锁文件，再返回前置错误；不承诺这些准备工作回滚。密钥保存、登记和配置发布的已知阶段通过部分结果保留，丢失回执则保持未知。

| 情况 | 行为与错误码 |
| --- | --- |
| 严格凭据不可用或已有密钥无效 | `not_checked`；`credential_unavailable` 或 `wireguard_key_invalid`，不替换密钥 |
| 私钥保存失败或不确定 | `key_state=unknown`，不发送 enroll；`wireguard_key_store_incomplete` |
| enroll 明确 4xx | `rejected`，保留已存私钥，不发布 |
| enroll 超时、断连、5xx、成功体损坏或身份不符 | `unknown`；`wireguard_enrollment_unknown`，不继续 GET/发布 |
| enroll 已确认，GET 失败或数据非法 | 保留 `confirmed`；`wireguard_config_unavailable/invalid` |
| 密钥选定/保存后上下文变化 | `wireguard_context_changed`，保留密钥与已知登记阶段，不发布 |
| 密钥选定/保存后或发布前目标变化 | `output_changed`，保留密钥和登记阶段，不覆盖 |
| 发布失败或同步未确认 | `wireguard_publication_incomplete`，publication 为 failed 或 published；后者表示目标已经改变 |
| 进程丢失、超时或终态回执缺失 | 客户端合成 `wireguard_outcome_unknown` |

写操作失败均 `retryable: false`；CLI apply 执行预算 120 秒、桌面等待 150 秒。取消停止本地执行并清理工具进程，但不能证明 enroll 回滚。保留原私钥，禁止自动重试或重新生成。机器协议的有效部分回执仍以退出码 0 返回，并标记 complete=false；普通 CLI 则返回非零。精确状态组合见 [CLI WireGuard 契约](../../agent-remote-cli/docs/desktop-wireguard.md)。

现有 Server GET 不返回设备当前公钥，因此不能证明一次丢失回执的 enroll 是否提交。最小版本明确显示未知，后续重新登记需要用户再次确认；若提供“检查原操作/恢复”，应先补 Server 幂等键及查询契约，不能用 GET config 假装恢复已确认。该方案不承诺自动备份、自动密钥轮换或持久操作恢复。

## 测试与验收范围

CLI 和 desktop 新增测试、helper、fixture 均放各自仓库根 `tests/`：

- plan 零写入/零凭据/零 HTTP；默认、自定义、相对 CLI 输出、新建、覆盖与缺失父目录。
- 未确认、上下文/路径身份变化、等待锁期间变化均在 enroll 前阻断；发布前再变化不覆盖。
- 系统拒绝、超时、坏密钥、FIFO、链接及宽权限 fallback 不触发新密钥或 POST；所有输出无秘密。
- enroll 仅发送一次且只含公钥；4xx、5xx、断连、损坏回执、错误设备分别保留正确阶段；GET 失败和换行注入不覆盖旧文件。
- 文件写入、权限、同步和发布失败；Unix 权限、Windows ACL/LocalSystem、并发及失去回执。
- 普通 CLI、`init` 与桌面调用共享服务的 loopback 合同；真实 Go→Rust plan/apply 互通；Go 严格验证路径/阶段组合、确认前零提交、未知状态无自动重试、中英文切换。

原生凭据调用在持有锁期间同步完成，不能以异步取消提前释放锁。原生系统 API 没有可强制的内部截止期限，桌面仍可终止 CLI 并保留未知结果。Unix SIGTERM 会触发受控取消，清理独立凭据工具进程组；直接 SIGKILL 不运行清理逻辑，不能宣称它保证所有外部工具后代都被收回。

相关测试、平台模块交叉检查及完整门禁的实际结果记录在实施状态。本文不替代 Windows/macOS/Linux 的真实系统凭据、ACL 和运行验收。
