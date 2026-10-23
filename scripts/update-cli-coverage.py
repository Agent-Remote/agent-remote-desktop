#!/usr/bin/env python3
"""Generate a reviewable syntax inventory from the actual CLI discovery output."""
import json
import pathlib
import sys

schemas = json.loads(pathlib.Path(sys.argv[1]).read_text())
destination = pathlib.Path(__file__).resolve().parents[1] / "docs" / "cli-coverage.md"

# Keep the authored workflow notes in cli-coverage.md.  Only the inventory
# between these markers is generated, so refreshing a schema cannot erase
# safety and acceptance notes maintained by the desktop team.
begin_marker = "<!-- BEGIN GENERATED CLI INVENTORY -->"
end_marker = "<!-- END GENERATED CLI INVENTORY -->"
if destination.exists():
    authored = destination.read_text()
    if begin_marker in authored and end_marker in authored:
        preamble = authored.split(begin_marker, 1)[0].rstrip()
    else:
        table_start = authored.find("| 程序 | 版本 | 命令节点 | 参数绑定 |")
        preamble = (authored[:table_start] if table_start >= 0 else authored).rstrip()
else:
    preamble = "\n".join([
        "# CLI 图形化覆盖清单", "",
        "由 `scripts/update-cli-coverage.py` 从实际二进制的命令定义生成。",
        "发现命令不等于已实现 GUI。下表为语法基线；执行适配、表单、确认、错误与恢复均需分别验收。",
        "当前每个已发现的叶子命令均可从能力页进入专用业务页、受管终端 workflow 或 schema 驱动的通用表单；专用业务流程、错误恢复和三平台原生验收仍需分别核对。",
        "已实现流程及验证边界见 [实施状态](implementation-status.md)；本表不将其计为全量命令验收完成。", "",
        "平台声明仅针对当前命令在客户端操作系统上的实现；未声明表示未知，不继承父命令，也不代表权限、组件或 GUI 已就绪。",
    ])

lines = [begin_marker, "", "| 程序 | 版本 | 命令节点 | 参数绑定 |", "| --- | --- | ---: | ---: |"]
for schema in schemas:
    if schema["schema_version"] != 1:
        raise ValueError("unsupported command schema")
    commands = schema["commands"]
    lines.append(f"| {schema['executable']} | {schema['cli_version']} | {len(commands)} | {sum(len(c['arguments']) for c in commands)} |")
for schema in schemas:
    lines += ["", f"## {schema['executable']}", "", "| 命令路径 | 类型 | 参数数 | 客户端平台声明 |", "| --- | --- | ---: | --- |"]
    for command in schema["commands"]:
        path = " ".join(command["path"]) or "（默认入口）"
        kind = "命令组 / 默认入口" if command["has_subcommands"] else "操作"
        platforms = ", ".join(command.get("platforms") or []) or "未声明"
        lines.append(f"| `{path}` | {kind} | {len(command['arguments'])} | {platforms} |")
destination.parent.mkdir(parents=True, exist_ok=True)
lines += ["", end_marker]
destination.write_text(preamble + "\n\n" + "\n".join(lines) + "\n")
print(destination)
