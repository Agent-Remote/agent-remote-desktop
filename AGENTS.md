# Desktop development rules

- Follow the [product plan](../agent-remote/docs/agent-remote-desktop-design.zh-CN.md) in P0–P4 order. The first formal release requires all existing CLI operations, with platform and permission conditions. Do not count terminal launch or command discovery as completed GUI coverage.
- UI and desktop business code are Go with native mygo. Reuse the existing Rust CLI for shared remote data and credentials. Target Windows, macOS and Linux; FreeBSD is not planned.
- Formal releases are community builds: macOS uses the project's community self-signing identity; Windows/Linux do not require publisher code signing. Do not introduce Apple Developer ID or notarization as release prerequisites.
- All desktop-owned user-visible text uses the official mygo catalog in `locales/`. Add English and Simplified Chinese together, use typed placeholders, and regenerate `locales/catalog_gen.go` with the pinned mygo version. Keep stable protocol/error identifiers and original Agent/user content unchanged.
- Put every test, test helper, fixture and test-only executable under repository-root `tests/`. Use the root Go module and test production behavior via imports. Never add `_test.go` beside production code or expose internal state solely to accommodate tests.
- Follow [development standards](docs/development-standards.md). Run `bash scripts/check.sh`; for terminal changes, enable `DESKTOP_TERMINAL_TESTS=1` and run the native probe where supported. Cross-compilation is not runtime validation.
- Keep [implementation status](docs/implementation-status.md) and both READMEs accurate. Record failures and platform gaps; do not describe planned or partially verified features as complete.
