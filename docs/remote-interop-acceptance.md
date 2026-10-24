# 远程闭环联调验收

`remote-interop-acceptance.sh` 是 Desktop、Rust CLI、Server、Node 和本机
Mutagen/fclaude 边界的统一验收入口。它把三种证据分开，避免把 fixture 或
交叉编译误报成真实远程成功：

```sh
# 协议/适配器合同（不连接真实主机）
scripts/remote-interop-acceptance.sh --contracts

# 当前主机的 Go→Rust desktop-control、agent-remote/fclaude 及能力目录探针
# 输出 remote_execution.status=not_run
scripts/remote-interop-acceptance.sh --host-only

# 真实一次性 Server/Node/SSH exact attachment；仅对 disposable session 执行
AGENT_REMOTE_HOME=/path/to/home \
AGENT_REMOTE_EXACT_ATTACHMENT_TEST_SESSION=<session-uuid> \
AGENT_REMOTE_EXACT_ATTACHMENT_TEST_DEVICE=<device-uuid> \
scripts/remote-interop-acceptance.sh --live
```

真实同步链路另有显式入口，避免一次性 attachment 验收意外创建同步资源：

```sh
SYNC_CLI_BINARY=/absolute/build/agent-remote \
SYNC_MUTAGEN_SOURCE=/absolute/reviewed-mutagen-v0.18.1 \
SYNC_SERVER_REPO=/absolute/agent-remote-server \
AGENT_REMOTE_NODE_REPO=/absolute/agent-remote-node \
scripts/remote-interop-acceptance.sh --sync-chain
```

`--sync-chain` 只接受 macOS Docker Desktop 控制端，并调用 Node 仓库的
`tests/linux_sync_chain_test.sh`。它在一次性 Linux 容器中启动当前 Server、非 root
Node worker、root helper、SSH forced-command、Bubblewrap 和 Mutagen agent，验证工作区
准备与 UID/ACL、双向文件、pause/resume、冲突两侧保留、人工 resolve、reset，以及未知
和已撤销 SSH key（包括 stale `authorized_keys`）拒绝，并在同一隔离环境中停止并重启
真实 Node worker poll 生命周期和 Mutagen daemon，确认 Mutagen session 清单持久、worker
重新启动后 pause/resume 仍能完成文件传输。`SYNC_CLI_BINARY` 旁边必须存在
匹配的 `agent-remote-ssh` 与 `agent-remote-scp`；`SYNC_MUTAGEN_SOURCE` 必须是审阅过的
Mutagen checkout。缺少任一条件或 Docker/Server/Node 失败都会写入 `status=failed` 证据
并返回非零，合同/fixture 结果不会作为 fallback。证据默认写入临时目录，也可通过
`SYNC_CHAIN_EVIDENCE_DIR` 指定；只有日志中出现精确的
`RESTART_RECOVERY_STATUS=passed` marker，证据才会写入
`remote_execution.restart_recovery=passed` 及
`restart_recovery_scope=post_restart_persistence_and_mutation`。marker 缺失时即使
普通同步链路返回零，入口也会 fail closed，并将 restart recovery 保持为 `not_run`；
它不宣称任意 in-flight mutation 崩溃后的恢复，后者需要独立的受控 lease/interruption
fixture。

`--contracts` 固定运行 Server 的 session attach/verify、attachment transfer、
Node attach、port-forward 测试，Node 的 attach/attachment 包测试，Rust
`desktop_control_contract`、Mutagen environment 和 exact attachment 合同测试，
以及 Desktop 的 remote/cliadapter 测试。Rust 合同目标会实际执行 fixture 测试，
而不是只编译测试二进制；exact attachment 的真实上传测试仍由 Cargo 标记为
ignored，仅在 `--live` 提供隔离身份后运行。这些测试证明协议和 fail-closed 边界，
不证明网络、SSH、Server 授权或 Node 进程真实运行。

Desktop 对 `accounts.list`、`sessions.list` 和 `ssh.inspect` 只接受有界的公开
投影：列表有数量上限，ID/状态/路径/任务字段会检查长度和控制字符，SSH 授权
状态必须同时带有合法的 session、Node、授权任务标识和正的有效期。畸形投影会
报告为协议错误并保留原有查询错误，不会降级为空列表或继续启动终端。
Server 的 exact attachment 对运行态同时接受 `running` 与 `active`；Desktop
只在这两个状态下打开连接确认，`starting` 会继续等待运行时发布稳定的终端目标。

`--host-only` 要求当前进程没有 `GOOS`/`GOARCH`，检查不可替代的普通可执行文件、
`--version`、实际 GOHOSTOS/GOHOSTARCH，并向真实 CLI 发出 `capabilities` 请求。
它必须看到 `sessions.list/create`、`nodes.list`、`ssh.inspect`、Mutagen 环境方法
和 `sessions.attach` terminal capability；缺少任何一项就 fail closed。证据写到
`AGENT_REMOTE_INTEROP_EVIDENCE_DIR`（默认系统临时目录），只保存版本、能力数量和
状态，不保存 home 内容、凭据、会话输出或 payload。

`--live` 还要求已登录的 Desktop CLI home、精确 session UUID 和 device UUID。它
调用现有 exact attachment live contract，一次性上传临时文件并要求 Server/Node
授权任务、SSH forced-command gateway 和清理回执全部绑定原始 session/device；
失败或响应丢失不会重试或猜测。设置 `AGENT_REMOTE_TEST_MUTAGEN` 后，另运行隔离
的原生 Mutagen daemon/protocol probe；未设置时证据明确标记 `mutagen=not_run`。
该 Rust probe 当前只在 Unix 编译；Windows 即使提供 Mutagen 二进制也会明确保留
`mutagen=not_run`，不会把 Cargo 的零测试成功状态当作通过。
Live 测试必须使用可删除的测试 session，不能用于生产会话。

2026-10-24 的一次性 Docker sync-chain 已通过真实 Server、Node、SSH、Mutagen
和 worker/daemon restart recovery；该记录只覆盖同步链路，不能替代 `--live` 所需的
准确 session/device、exact attachment、停止/删除和 cleanup receipt。未提供这些真实
身份时，`--live` 仍应保留 `remote_execution.status=not_run`，并在发布记录中列为待验收项。
即使 `--live` 通过，仍需目标主机独立记录 broker 或其他服务的重启/重连；任何
host-only 或 contract 通过结果都不能单独宣称完整远程闭环已完成。
