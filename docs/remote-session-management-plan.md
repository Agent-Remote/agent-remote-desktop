# 远程会话生命周期状态与发布门禁

本文只记录当前 desktop、Rust CLI、Server 和 Node 代码已经提供的生命周期合同，以及正式发布前仍需补齐的证据。历史设计、废弃接口和实施前假设不属于当前合同。desktop 的总体发布状态以同目录 [implementation-status.md](implementation-status.md) 为准。

## 当前代码边界

- desktop 负责选择上下文、显示确认、聚焦准确 pane，以及展示 operation receipt；不保存凭据、原始 argv、终端输出或 Server 任务正文。
- Rust CLI 是 caller key、journal、凭据、SSH/Mutagen/WireGuard 操作和恢复查询的持久所有者。未知结果始终查询原 key，不生成新 key 重放。
- Server 负责 owner-scoped 受理、任务和回执、生命周期状态保护、删除范围和 attachment cleanup receipt。原始受理身份与历史成功回执不可被后续观察改写。
- Node 只执行已授权的准确 session、node、runtime backend 操作；无法证明停止或清理时返回 pending，并保留收据。

## 已接入的操作

| 操作 | 当前语义 | 仍需的发布证据 |
| --- | --- | --- |
| 创建 | `sessions.create` 使用规范非零 caller key 和冻结的 tool/account/workspace/project/sync/replacement 上下文；同 key 同意图返回原回执，冲突明确失败；响应丢失只查询原 key。 | 真实 Server/Node/SSH/tmux/fclaude 创建、丢响应恢复和 exact attach 闭环。 |
| 连接 | desktop 和 CLI 按完整 session UUID、device、node、runtime backend 查询并 attach；创建受理不代表已连接。 | 三平台真实终端、权限和重启后的准确 attach 证据。 |
| 停止 | `sessions.stop` 绑定准确 session 和 caller key；Server/Node 使用持久 stop fence、生命周期锁和结果守卫，迟到创建不能被历史回执重新授权。 | 启动中停止、旧 worker 晚回报、跨端恢复和物理终止证明。 |
| 保存 | Skill snapshot/finalization 使用独立 operation ID 和状态观察。 | 真实 Skill 保存与发布结果，且与进程终止、项目同步分别验收。 |
| 删除 | Server/CLI/desktop 绑定用户确认时冻结的 session UUID 集合；保留检查和 cleanup receipt 独立记录，集合变化不能扩大范围。 | 并发集合变化、保留阻塞、删除后原 key 查询和 attachment 清理。 |
| 转发 | Rust CLI 持有 caller-key journal 和 loopback listener；支持准确 forward ID、TTL、bounded reconnect、受管停止和一次性连接授权。 | Server/Node 实网 tunnel、权限撤销、跨重启恢复和三平台安装证据。 |
| 同步 | verified flush 只复验已经准备好的 Mutagen 关系并报告 flush 观察；不初始化同步，也不代替创建或保存授权。一次性 Docker sync-chain 已验证双向传输、冲突处理、暂停/恢复和 Mutagen/Node 重启恢复。 | 将该记录绑定到最终发布组合，并补齐真实目标主机与完整 session 生命周期证据。 |

## 状态显示规则

`prepared`、`admission_pending`、`accepted`、`running`、`stopping`、`saved`、`failed`、`unknown` 和 `deleted` 表示不同观察阶段。一个阶段成功不能推导另一个阶段成功：进程终止、Skill 保存/发布、项目同步、终端连接和附件清理必须分别显示。

`accepted` 只表示 Server 已保存受理记录；`running` 需要任务或会话观察；`unknown` 保留原 key 并提供 query-only 恢复入口；`deleted` 表示删除观察与原回执仍可查询。desktop 不按 latest session、默认账户或本地项目猜测未知结果。

## 发布前门禁

以下项目是当前未完成的正式发布门禁：

1. Windows amd64 真实主机：Win32、ConPTY、IME、托盘、安装升级/回滚和 workbench probe。
2. Linux amd64 真实主机：X11 与 Wayland、GTK、AppIndicator、PTY、托盘和安装行为。
3. 三个平台受保护 WireGuard broker：caller authorization、replay rejection、隧道存活和重启恢复。
4. 一次性临时 Server/Node/SSH/tmux/fclaude 真实闭环：创建、丢响应恢复、exact attach、停止/删除、attachment transfer 和 cleanup receipt。
5. Agent TUI、IME、附件、输入反馈和 GPU/帧率的完整实测；Mutagen 重启恢复已由一次性 Docker sync-chain 验收通过，但仍需绑定最终发布组合。
6. 每个目标主机安装并验证与发布组合完全一致的候选 artifacts。

macOS arm64 当前有本机 workbench、安装和性能证据；Windows/Linux 交叉编译只证明可构建，不能替代真实主机证据。候选构建可以是 ad-hoc/smoke；正式发布仍需用户决定，并且在所有门禁完成前不得发布。

## 验证入口

```sh
# desktop 本地质量与 workbench
bash scripts/check.sh
DESKTOP_TERMINAL_TESTS=1 go run ./tests/workbench-probe

# 候选构建与策略验证
scripts/build-release.sh --out dist/release --clean
scripts/verify-release.sh dist/release --require-targets darwin/arm64,windows/amd64,linux/amd64
```

验收必须使用临时 CLI home、临时凭据、一次性 Server/Node 数据、临时 socket 和测试 workspace。完成后删除临时 home、容器、镜像、证据目录和测试进程；生产凭据与真实 workspace 不得用于验收。
