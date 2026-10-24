# agent-remote-desktop

[English](README.md) | 中文

`agent-remote-desktop` 是基于 Go/mygo 的原生工作台，用于管理本地 Codex、Claude 终端和远程 `fclaude` 会话。它共用现有 Rust CLI 的配置、凭据和结构化操作合同。

## 当前状态

Zen 工作区、Agent/项目层、控制中心、本地会话管理器、受管终端、持久化、i18n 和 schema 驱动的 CLI 入口已经实现。当前实施记录把 UI 重构 R0–R6 记为完成。

当前仍是开发版本。正式发布还需要真实 Windows amd64 和 Linux amd64 主机证据、Linux X11/Wayland 覆盖、受保护 WireGuard broker 证据、真实 Server/Node/SSH/fclaude 联调、完整 Agent TUI/输入法/附件检查和端到端性能测量。FreeBSD 不在计划中。

## 架构

- Go 和原生 mygo 负责窗口、UI、i18n、主题和生命周期。
- `terminalhost` 负责各平台 PTY/ConPTY 会话和终端渲染。
- `session` 独立管理本地 Agent 进程，不依赖可见面板。
- `remote` 和 `cliadapter` 通过版本化结构化合同调用 Rust CLI。
- SQLite 只保存 desktop 自有的布局、主题、项目和会话元数据。
- CLI 凭据、服务器状态、账户状态和远程操作 journal 仍由 Rust CLI 管理。

所有 desktop 自有文案使用 mygo 官方 i18n catalog。所有测试、辅助代码、夹具和测试程序均位于仓库根目录 `tests/` 下。

## 运行

需要 Go 1.27.1 或更高版本：

```sh
go run .
go run . --home /path/to/agent-remote-home
```

安装需要使用的外部工具（`agent-remote`、`fclaude`、`codex` 和/或 `claude`）。打开项目后创建本地或远程会话，再使用 Agent 抽屉、项目圆环或控制中心导航。关闭窗口会隐藏窗口并保持受管会话运行；显式退出时会询问是否停止本地会话。

## 安装候选版本

先构建完整发布目录，再在目标主机验证并安装。随包提供的
`agent-remote-desktop-installer` 只会把已验证目录中的文件安装到用户可写目录，
不会下载、提权、签名或安装 Agent CLI。

```sh
scripts/build-release.sh --out dist/release --clean
scripts/verify-release.sh dist/release --require-targets darwin/arm64,windows/amd64,linux/amd64
scripts/release-native-smoke.sh dist/release
```

安装器支持 `--status`、普通升级和 `--rollback`。原生 smoke 入口必须在目标主机运行；
交叉构建不属于安装证据。

## 检查

```sh
bash scripts/check.sh
go test ./...
go run ./tests/native-probe
go run -race ./tests/workbench-probe
go run ./tests/performance-probe -frames 120
```

终端相关改动需要设置 `DESKTOP_TERMINAL_TESTS=1`。原生验收脚本必须在目标主机运行；交叉编译不属于运行证据。远程合同夹具和主机预检不能证明真实 Server/Node 会话。

## 发布

正式目标为 `darwin/arm64`、`windows/amd64` 和 `linux/amd64`。macOS 沿用项目社区自签。Windows/Linux 不要求发行者代码签名，Apple Developer ID 和公证也不是前置条件。FreeBSD 不在计划中。

构建并验证候选版本：

```sh
scripts/build-release.sh --out dist/release --clean
scripts/verify-release.sh dist/release --require-targets darwin/arm64,windows/amd64,linux/amd64
```

只有原生证据、broker 证据、远程联调和性能门禁全部完成后，发布流程才可以发布。发布流程本身不能替代这些门禁。

## 验收与发布脚本

仓库把合同、主机和制品入口分开：

```sh
# 构建并验证候选制品目录。
scripts/build-release.sh --out dist/release --clean
scripts/verify-release.sh dist/release --require-targets darwin/arm64,windows/amd64,linux/amd64
# 原生主机验收和打包 smoke。
scripts/native-acceptance.sh
scripts/release-native-smoke.sh dist/release
# 远程、附件和同步合同。
scripts/remote-interop-acceptance.sh --contracts
scripts/remote-interop-acceptance.sh --host-only
scripts/exact-attachment-acceptance.sh --contracts
scripts/sync-chain-acceptance.sh
# 刷新生成的 CLI 覆盖目录。
go run ./cmd/command-catalog > /tmp/desktop-catalog.json
python3 scripts/update-cli-coverage.py /tmp/desktop-catalog.json
# 目标主机生成记录后合并主机证据。
scripts/verify-native-evidence.sh <macos-dir> <windows-dir> <linux-dir>
```

Windows 主机使用 PowerShell 等价入口：

```powershell
pwsh -File scripts/native-acceptance.ps1
pwsh -File scripts/release-native-smoke.ps1 -ReleaseDir dist/release
```

`verify-native-evidence.sh` 是最终证据合并器，不会生成主机证据。验收脚本使用临时 home 和一次性
夹具；评审后请删除证据目录和测试工作区。

## 文档

- 产品方案：`../agent-remote/docs/agent-remote-desktop-design.zh-CN.md`
- 实施状态：`docs/implementation-status.md`
- CLI 覆盖：`docs/cli-coverage.md`
- 平台验证：`docs/platform-validation.md`
- 开发规范：`docs/development-standards.md`
- 发布构建：`docs/release-build.md`
- 远程联调：`docs/remote-interop-acceptance.md`
- WireGuard 合同：`docs/wireguard-*.md`
- 会话合同：`docs/remote-session-*.md`、`docs/desktop-ego-browser-requests.md`

实施状态文件记录当前证据；旧条目只是历史快照，不能覆盖最新发布门禁结论。

## 许可证

GPL-3.0-only。详见 `LICENSE` 和 `THIRD_PARTY_NOTICES.md`。
