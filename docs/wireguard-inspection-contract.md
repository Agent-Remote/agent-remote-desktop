# WireGuard 检查与状态协议

状态：共享 Rust 服务、桌面协议和 Go 网络页已实现；完整验证结果见[实施状态](implementation-status.md)。本契约定义当前只读操作；隧道计划、启停和平台授权桥的现行合同见[隧道计划](wireguard-tunnel-plan.md)，真实三平台安装与授权仍是独立发布门禁。

两个方法均通过 capabilities 声明 `effect: read`、`credential: none`，不要求服务器配置、登录或设备登记。输入 home 必须与当前 CLI 的绝对 home 一致；不得创建 home、锁、数据库或凭据。读取配置或运行工具均由用户显式点击发起，页面进入不自动执行。

## wireguard.check

请求：`{"expected_home":"/absolute/home","config":null}`。config 为 null 时检查 CLI 默认 WireGuard 路径，否则为规范绝对路径。返回路径保留用户选择的词法绝对路径（例如 `/var`），不替换为文件系统解析后的别名。只在 Rust 内有界读取配置，返回以下结果；所有字段必须出现，可空字段显式为 null。

```json
{
  "home": "/absolute/home",
  "platform": "linux",
  "elevation": "standard",
  "config": {
    "path": "/absolute/home/wireguard/agent-remote.conf",
    "state": "ready",
    "addresses": ["10.77.0.2"],
    "dns": [],
    "mtu": 1000,
    "peer_count": 0,
    "unsupported_directives": []
  },
  "tools": [
    {"name":"wg","state":"available","path":"/usr/bin/wg"},
    {"name":"wg-quick","state":"available","path":"/usr/bin/wg-quick"}
  ]
}
```

config.state：ready / missing / unreadable / unsafe_path / insecure_permissions / invalid / unsupported。仅 ready、unsupported 可返回已解析摘要，其他状态的数组为空，mtu/peer_count 为 null。ready 的 unsupported_directives 为空；unsupported 必须有至少一项。peer_count 为必需可空整数，最多 1024；addresses 为 1–64 项，dns 最多 16，mtu 为 null 或 576–65535。

Interface 必需 PrivateKey 和至少一个 Address；每个 Peer 必需 PublicKey 和至少一个 AllowedIPs。Address、DNS、AllowedIPs 可重复，仍遵守总量上限；单值和重复 peer 公钥无效。支持生成器字段及常见 ListenPort、FwMark、PresharedKey 的严格语法校验；私钥和 PSK 不进入结果。PreUp、PostUp、PreDown、PostDown、SaveConfig、Table 仅报告这些固定指令名称，状态为 unsupported，不报告指令值、不执行。未知字段、重复单值、结构错误或超限为 invalid。允许 IP/CIDR 地址、IP DNS；域名 DNS/search 等暂不属于本受控子集，不能据此声称完整 wg-quick 语法支持。

工具顺序固定为 wg、平台隧道工具（Windows 为 wireguard，其他为 wg-quick）。工具状态为 available / missing / not_executable；missing 的 path 为 null，其他为规范绝对路径。工具发现不运行版本命令；用户指定的工具路径无效时不得悄悄换成另一个程序。

配置最多 1 MiB，nofollow 读取并拒绝链接、重解析点、硬链接、FIFO、目录及不安全权限。Windows 接受本项目生成配置的当前用户权限与 LocalSystem 只读权限，不借此放宽普通凭据文件 ACL。

## wireguard.status

请求：`{"expected_home":"/absolute/home"}`。通过共享 Rust 服务运行 `wg show all dump`，最多 1 MiB stdout、受限 stderr、10 秒执行预算；取消/超时清理临时工具进程树。不得调用 sudo/UAC、发起网络请求或返回原始工具输出。所有私钥与 PSK 在 Rust 解析边界丢弃。

```json
{
  "home":"/absolute/home", "platform":"linux", "elevation":"standard",
  "tool":{"name":"wg","state":"available","path":"/usr/bin/wg"},
  "state":"observed",
  "interfaces":[{
    "name":"wg0", "public_key":null, "listen_port":51820, "fwmark":0,
    "peers":[{
      "public_key":"BAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQ=",
      "endpoint":"127.0.0.1:51820", "allowed_ips":["10.77.0.1/32"],
      "latest_handshake":0, "received_bytes":0, "sent_bytes":0,
      "persistent_keepalive":25
    }]
  }]
}
```

state：observed / tool_missing / tool_not_executable / query_failed / timed_out / output_too_large / invalid_output。非 observed 的 interfaces 必须为空；observed 的空数组表示该次查询未观察到接口。tool_missing/tool_not_executable 与 tool.state 一致，其余状态要求 available。取消返回标准 canceled 错误，不以空状态假装成功。

最多 64 个不重名接口、总计 1024 peers、总计 4096 个 AllowedIPs；每 peer 最多 256 个。接口名称不含控制字符、最长 255 字节，同接口 peer 公钥不能重复。public_key 是规范 Base64 的非全零 32 字节公钥，未配置的接口为 null；endpoint 为 null 或有效 IP/域名与端口，最长 320 字节；IPv6 可带 1–64 字符的 ASCII 字母、数字、下划线、点或短横线 scope，域名不接受尾点。listen_port/keepalive 为 uint16，fwmark 为 uint32（off 归一为 0），收发量为 uint64，握手为 Unix 秒（0 表示从未握手，最大 253402300799）。字段或行结构无效、孤立 peer、截断、总量超限应整体拒绝，不静默丢行或截断为部分成功。

## 权限与桌面边界

platform 为 windows / macos / linux，必须与客户端主机相符。elevation 为 elevated / standard / unknown，只是当前进程权限观察，不证明 CAP_NET_ADMIN、接口访问、系统授权或远程授权；不得根据英文 stderr 推断权限失败。query_failed 与没有活动接口分开呈现。

desktop 在调用前校验方法、平台、read 与 credential:none；严格校验结果必需字段、枚举、整数范围及 home。配置输入在异步能力查询前冻结。每次只执行一次，无自动重试；跨页保留结果，取消、Close 和迟到响应有明确生命周期。文案使用官方中英文 catalog，全部测试与夹具位于根 tests/。
