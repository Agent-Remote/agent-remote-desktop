# WireGuard 权限桥状态合同

`wireguard.bridge.status` 是版本 1 的只读 desktop-control 方法。它只观察当前 CLI home
所绑定的本机权限桥，不读取凭据，不联系服务器，不启动授权对话框，也不执行隧道操作。
请求严格为：

```json
{"expected_home":"/absolute/cli/home"}
```

`expected_home` 必须是当前 CLI 的规范绝对 home。返回对象严格包含以下字段：

```json
{
  "version": 1,
  "home": "/absolute/cli/home",
  "platform": "linux",
  "state": "unavailable",
  "installation": "not_installed",
  "approval": "not_requested",
  "protocol": "unsupported",
  "authentication": "unavailable",
  "execution": "unavailable"
}
```

`platform` 为 `windows`、`macos` 或 `linux`，并且必须与调用方主机一致。`state` 只能为
`unavailable`、`unsupported` 或 `observed`。安装、系统批准、协议兼容和调用方认证是彼此
独立的观察：组件存在或进程具有 elevation 都不能推导 `execution=ready`。当状态不是
`observed` 时，`execution` 仍必须由 broker service 明确返回 `ready` 才可用。当前 CLI 已携带
三平台 broker client、受保护 service 实现和严格 caller/permit 合同已进入仓库，但正式构建
仍不会自动安装受保护 broker service：
macOS/Windows 的未安装观察返回 `state=unsupported`，Linux 的系统总线缺失返回
`state=unavailable`，服务缺失、身份不匹配或协议错误均保持 `execution=unavailable`；这些状态都不是权限桥安装或运行验收。

desktop 在调用前检查 capability 中准确的 `effect=read`、`credential=none` 声明，并严格
校验返回字段、枚举、home 和平台。未知字段、缺失字段、平台不匹配或诱导性的执行状态均
按协议错误处理。UI 只在用户显式点击后查询，并把不可用/不支持与已观察状态分开显示。

CLI 会在跨平台实现边界把原生传输和进程状态投影到上述固定枚举：Linux 的
`dbus-system`、macOS 的 `unix_socket` 和 Windows 的 `named_pipe` 只有在对应受保护
通道通过自身检查后才映射为 `protocol=compatible`，服务的 `running`/`ready` 等内部状态
映射为 `state=observed`。macOS 当前的 socket/peer 观察仍只能映射为
`execution=not_ready`，因为它不代表每次操作都已获系统批准；Windows 在客户端完成服务
进程身份的独立 status 握手前，哪怕固定 pipe 存在也继续报告 `unsupported/not_installed`。
UI 不消费这些平台私有字符串，也不会把发现 endpoint 当作安装或授权证据。

desktop 另外把上述独立事实投影为一个本地生命周期门槛，供界面和当前写操作复用；该字段
不是 CLI 返回值，也不授予权限。`WireGuardBridgeStatus.Lifecycle()` 按固定顺序返回第一个
未满足的条件：`unavailable`、`unsupported`、`installation_required`、
`approval_pending`、`approval_required`、`protocol_unavailable`、
`authentication_required`、`authorization_required`、`not_ready` 或 `ready`。
只有所有前置条件均已观察为安装、批准、协议兼容、调用方已认证且执行状态为 `ready` 时，
`CanExecute()` 才为真；普通权限、工具存在或单独的安装事实不会跳过门槛。未知和部分观察
一律停在阻断状态，避免 UI 将生命周期中的中间状态显示为可执行。

测试使用隔离 home 和 CLI fixture，位于桌面仓库根 `tests/`；不安装系统组件、不请求提权、
不创建真实隧道。真实 broker 的安装、认证、授权和三平台运行验收仍遵循
[权限桥实施设计](wireguard-bridge-design.md)。
