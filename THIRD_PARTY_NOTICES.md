# Third-party dependencies

The application source is GPL-3.0-only. Dependency versions and integrity hashes
are recorded in `go.mod` and `go.sum`.

| Component | Purpose | License |
| --- | --- | --- |
| egoist/mygo | Native windows, UI, terminal rendering | MIT |
| Ghostty libghostty-vt | Terminal emulation, pinned by mygo's native manifest | MIT |
| charmbracelet/x/xpty and related packages | Unix PTY and Windows ConPTY | MIT |
| creack/pty | Unix pseudoterminal primitives | MIT |
| modernc.org/sqlite and its dependencies | Desktop-only SQLite state | See upstream component licenses (SQLite public domain; Go wrapper BSD-3-Clause) |
| Go x/* packages | Platform and rendering support | BSD-3-Clause |

The release pipeline must include the exact native-library source, version,
checksum and licenses, and the CLI's notices when bundling it. This development
notice is not a substitute for the release artifact's complete dependency inventory.
