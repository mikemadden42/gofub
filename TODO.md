# TODO

Findings from a project review (2026-11-14), in suggested order of priority.

## Correctness and robustness (main.go)

- [x] **Stop parsing `file` output for architecture** (`getExecutableArchitecture` / `extractArchitecture`).
  - An executable whose `file` output contains none of `executable arm64`, `executable x86_64` or `universal` matches no case in the `switch` and is silently dropped (e.g. shell-script launchers). Add an "Other/Unknown" bucket.
  - `extractArchitecture` returns `"universal"` if any word of the output is `universal`, including words in the path.
  - Fat binaries holding only `x86_64` + `i386`, or `arm64` + `arm64e`, are reported as "Universal".
  - Fix: use the standard library's `debug/macho` (`macho.OpenFat`, falling back to `macho.Open`) to read CPU types directly. This removes the subprocess, the string parsing and gosec G204, and makes the logic unit-testable.
- [ ] **Widen the scan.** Only the top level of `/Applications` is checked. This misses `/Applications/Utilities`, apps in vendor subfolders, `~/Applications` and `/System/Applications`. `Readdir` uses `Lstat`, so symlinked `.app` bundles are skipped. Add a flag/argument for scan path(s) and an optional recursive walk.
- [x] **Error handling.**
  - Print errors to stderr instead of stdout, and exit non-zero on fatal errors such as failing to open the directory.
  - Remove the duplicated message: `parseInfoPlist` already says "error parsing X: …" and `main` adds "Error parsing X:" in front. Wrap with `%w` and add context in one place.
- [ ] **Style and cleanup.**
  - Flatten the six-level nesting in `main`: extract a `classify(appPath)` helper and use early `continue`s.
  - Replace `os.Open` + `Readdir(0)` + the deferred close closure with `os.ReadDir`.
  - Parse the plist into a struct with a `plist:"CFBundleExecutable"` tag instead of `map[string]interface{}`.
  - Delete the commented-out debug `Printf`s.
  - Make variable naming consistent (`x8664executables` vs `arm64Executables`).

## Tooling and config

- [ ] **Fix `.golangci.yml` for v2.** `golangci-lint config verify` currently fails.
  - `version: 2` should be the string `"2"`.
  - `linters.disable-all` should be `linters.default: none`.
  - `linters-settings` should be `linters.settings`.
  - `issues.exclude-dirs` should be `linters.exclusions.paths`.
  - Move the goimports settings under `formatters.settings`. `local-prefixes: github.com/mikemadden42` doesn't match the module path (`gofub`).
  - Resolve the current findings: G304 and the missing package comment (G204 fixed by the `debug/macho` switch).
- [ ] **Fix `.goreleaser.yaml`.** `goreleaser check` currently fails.
  - Add `version: 2` at the top.
  - Replace the deprecated `snapshot.name_template` with `version_template`.
  - Remove the unneeded `go generate` hook and the boilerplate template comments.
- [ ] **Match the build matrix to the platform.** The tool only works on macOS, but goreleaser builds Linux and Windows binaries and CI runs on Ubuntu and Windows. Restrict both to darwin, or have the tool exit with a clear "macOS only" error on other platforms.
- [ ] **Add lint to CI.** Add `golangci/golangci-lint-action` to the workflow. (Table tests for architecture classification now exist in `main_test.go`.)
- [ ] **Small items.**
  - Bump `go.mod` from `go 1.21` to at least 1.23 (CI's oldest version) and use newer stdlib such as `slices.Sort`.
  - Expand the README: what the tool does, that it's macOS only, and sample output.
