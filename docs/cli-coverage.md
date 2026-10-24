# CLI coverage

Updated 2026-10-24. This inventory is generated from the pinned binaries
using `--desktop-schema`; it is a syntax and routing inventory, not proof of
server authorization or native platform acceptance. The current catalog contains
`agent-remote` 106 command nodes and `fclaude` 12 command nodes.

Every discovered leaf command is reachable from the desktop either through a
dedicated structured page, a reviewed terminal workflow, or the schema-driven
form. The form validates defaults, required values, choices, boolean/count and
append options, positional arguments, NUL values and the frozen argv. Sensitive
credentials, tokens, private keys and device codes use dedicated flows and never
enter generic argv or desktop persistence. Unknown platform or permission
metadata remains unknown; it is not treated as authorization.

The desktop keeps the following boundaries: CLI credentials and operation
journals stay in the Rust CLI; Agent and CLI output stays in the managed terminal;
mutations require explicit confirmation; and an uncertain result can only be
queried with its original request key. `CallWithProgress` may project bounded
progress events, but only the final CLI receipt completes an operation.

The inventory is refreshed with:

```sh
go run ./cmd/command-catalog \
  --cli ../agent-remote-cli/target/debug/agent-remote \
  --launcher ../agent-remote-cli/target/debug/fclaude > /tmp/desktop-catalog.json
python3 scripts/update-cli-coverage.py /tmp/desktop-catalog.json
```

The generated table below records command paths, parameter counts and declared
platforms. It must be regenerated when the CLI schema changes. It does not mark
remote Server/Node/SSH behavior, broker permissions, or Windows/Linux runtime
checks as complete.

## agent-remote

| 命令路径 | 类型 | 参数数 | 客户端平台声明 |
| --- | --- | ---: | --- |
| `（默认入口）` | 命令组 / 默认入口 | 3 | 未声明 |
| `skill` | 命令组 / 默认入口 | 3 | 未声明 |
| `skill state` | 命令组 / 默认入口 | 3 | 未声明 |
| `skill state prune` | 操作 | 11 | 未声明 |
| `skill state reset` | 操作 | 10 | 未声明 |
| `skill state migrate` | 操作 | 11 | 未声明 |
| `skill state resolve` | 操作 | 12 | 未声明 |
| `skill state restore` | 操作 | 11 | 未声明 |
| `skill state list` | 操作 | 9 | 未声明 |
| `skill state conflicts` | 操作 | 9 | 未声明 |
| `skill state info` | 操作 | 7 | 未声明 |
| `skill state diff` | 操作 | 10 | 未声明 |
| `skill state export` | 操作 | 9 | 未声明 |
| `skill add` | 操作 | 15 | 未声明 |
| `skill check` | 操作 | 4 | 未声明 |
| `skill update` | 操作 | 12 | 未声明 |
| `skill enable` | 操作 | 10 | 未声明 |
| `skill disable` | 操作 | 11 | 未声明 |
| `skill pin` | 操作 | 11 | 未声明 |
| `skill unpin` | 操作 | 10 | 未声明 |
| `skill inherit` | 操作 | 11 | 未声明 |
| `skill remove` | 操作 | 8 | 未声明 |
| `skill rollback` | 操作 | 9 | 未声明 |
| `skill list` | 操作 | 10 | 未声明 |
| `skill info` | 操作 | 6 | 未声明 |
| `skill status` | 操作 | 8 | 未声明 |
| `skill retry` | 操作 | 8 | 未声明 |
| `init` | 操作 | 12 | 未声明 |
| `login` | 操作 | 12 | 未声明 |
| `logout` | 操作 | 4 | 未声明 |
| `status` | 操作 | 4 | 未声明 |
| `doctor` | 操作 | 4 | 未声明 |
| `deps` | 命令组 / 默认入口 | 3 | 未声明 |
| `deps status` | 操作 | 4 | 未声明 |
| `deps mutagen-environment` | 操作 | 4 | linux, macos, windows |
| `wireguard` | 命令组 / 默认入口 | 3 | 未声明 |
| `wireguard config` | 操作 | 4 | linux, macos, windows |
| `wireguard check` | 操作 | 5 | linux, macos, windows |
| `wireguard status` | 操作 | 3 | linux, macos, windows |
| `wireguard up` | 操作 | 5 | linux, macos, windows |
| `wireguard down` | 操作 | 5 | linux, macos, windows |
| `ssh` | 命令组 / 默认入口 | 3 | 未声明 |
| `ssh check` | 操作 | 4 | 未声明 |
| `sync` | 命令组 / 默认入口 | 3 | 未声明 |
| `sync ensure` | 操作 | 6 | 未声明 |
| `sync status` | 操作 | 5 | 未声明 |
| `sync pause` | 操作 | 5 | 未声明 |
| `sync resume` | 操作 | 5 | 未声明 |
| `sync resolve` | 操作 | 5 | 未声明 |
| `sync reset` | 操作 | 5 | 未声明 |
| `account` | 命令组 / 默认入口 | 3 | 未声明 |
| `account recover-runtime` | 操作 | 8 | 未声明 |
| `account recovery-status` | 操作 | 7 | 未声明 |
| `account list` | 操作 | 4 | 未声明 |
| `account create` | 操作 | 9 | 未声明 |
| `account bind` | 操作 | 4 | 未声明 |
| `account attach` | 操作 | 7 | 未声明 |
| `account import-config` | 操作 | 12 | 未声明 |
| `account verify` | 操作 | 4 | 未声明 |
| `account status` | 操作 | 4 | 未声明 |
| `account disable` | 操作 | 4 | 未声明 |
| `account default` | 命令组 / 默认入口 | 3 | 未声明 |
| `account default set` | 操作 | 5 | 未声明 |
| `account default get` | 操作 | 4 | 未声明 |
| `account default clear` | 操作 | 4 | 未声明 |
| `credentials` | 命令组 / 默认入口 | 3 | 未声明 |
| `credentials list` | 操作 | 4 | 未声明 |
| `credentials create` | 操作 | 8 | 未声明 |
| `credentials bind` | 操作 | 5 | 未声明 |
| `credentials unbind` | 操作 | 4 | 未声明 |
| `device` | 命令组 / 默认入口 | 3 | 未声明 |
| `device install` | 操作 | 4 | macos |
| `device uninstall` | 操作 | 4 | macos |
| `device status` | 操作 | 3 | macos |
| `device launch` | 操作 | 3 | macos |
| `device diagnose` | 操作 | 3 | macos |
| `device revoke` | 操作 | 5 | 未声明 |
| `device rotate-token` | 操作 | 4 | 未声明 |
| `node` | 命令组 / 默认入口 | 3 | 未声明 |
| `node install` | 操作 | 6 | 未声明 |
| `ego-browser` | 命令组 / 默认入口 | 3 | 未声明 |
| `ego-browser setup` | 操作 | 4 | 未声明 |
| `ego-browser connect` | 操作 | 5 | 未声明 |
| `ego-browser repair` | 操作 | 4 | 未声明 |
| `ego-browser upgrade` | 操作 | 4 | macos |
| `ego-browser remove` | 操作 | 5 | 未声明 |
| `ego-browser forget-this-mac` | 操作 | 5 | 未声明 |
| `ego-browser re-enroll` | 操作 | 4 | 未声明 |
| `ego-browser device-rotate` | 操作 | 4 | 未声明 |
| `ego-browser switch-server` | 操作 | 5 | 未声明 |
| `ego-browser register` | 操作 | 5 | 未声明 |
| `ego-browser status` | 操作 | 5 | 未声明 |
| `ego-browser list` | 操作 | 4 | 未声明 |
| `ego-browser requests` | 操作 | 5 | 未声明 |
| `ego-browser cancel-request` | 操作 | 6 | 未声明 |
| `ego-browser claim` | 操作 | 5 | 未声明 |
| `ego-browser pause` | 操作 | 6 | 未声明 |
| `ego-browser resume` | 操作 | 6 | 未声明 |
| `ego-browser stop` | 操作 | 6 | 未声明 |
| `ego-browser revoke` | 操作 | 6 | 未声明 |
| `ego-browser delete-device` | 操作 | 5 | 未声明 |
| `ego-browser delete-binding` | 操作 | 5 | 未声明 |
| `attach` | 操作 | 6 | 未声明 |
| `forward` | 命令组 / 默认入口 | 8 | 未声明 |
| `forward list` | 操作 | 3 | 未声明 |
| `forward stop` | 操作 | 6 | 未声明 |

## fclaude

| 命令路径 | 类型 | 参数数 | 客户端平台声明 |
| --- | --- | ---: | --- |
| `（默认入口）` | 命令组 / 默认入口 | 6 | 未声明 |
| `run` | 操作 | 6 | 未声明 |
| `new` | 操作 | 6 | 未声明 |
| `list` | 操作 | 14 | 未声明 |
| `attach` | 操作 | 9 | 未声明 |
| `stop` | 操作 | 7 | 未声明 |
| `stop-status` | 操作 | 8 | 未声明 |
| `delete` | 操作 | 7 | 未声明 |
| `delete-status` | 操作 | 8 | 未声明 |
| `forward` | 命令组 / 默认入口 | 10 | 未声明 |
| `forward list` | 操作 | 5 | 未声明 |
| `forward stop` | 操作 | 8 | 未声明 |

<!-- END GENERATED CLI INVENTORY -->
