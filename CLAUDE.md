# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build binary to bin/gip (uses vendored deps + ldflags for version info)
./.sdlc/build

# Run tests only (no linting)
go test -mod vendor ./...

# Run a single test
go test -mod vendor -run TestName ./cmd/gip/
go test -mod vendor -run TestName ./lib/core/

# Build without version injection (fast iteration)
go build -mod vendor ./...

# Full quality check (lint, fmt, vet, goimports, gocyclo, tests) — requires external tools
./.sdlc/check
```

All builds use `-mod vendor`; the `vendor/` directory is committed.

## Architecture

The binary lives in `cmd/gip/` (package `main`). The library lives in `lib/core/`.

### `lib/core/`

- **`runcmd.go`** — `runcmdWrapper` interface + `defaultGitWrapper` that executes the `git` binary. The interface exists solely for test injection: `stubs_test.go` provides `mockGitWrapper`.
- **`git.go`** — `GitCommands` struct. Every public method (Clone, Pull, Status, Fetch, CurrentBranch) calls `executor.exec(runcmdWrapperRequest{})` and synchronises output through `outputMu sync.Mutex` before calling `ui.Title`/`ui.Lifecycle`.
- **`exec.go`** — `CommandRunner` for arbitrary shell commands (used by `gip exec`). Same mutex-guarded output pattern as `GitCommands`.

### `cmd/gip/`

| File | Responsibility |
|---|---|
| `main.go` | App setup, global flags, `app.Before` hook that sets package-level vars |
| `commands.go` | Command definitions and `doX` handler functions |
| `utils.go` | Config file cascade, YAML/JSON parsing, project model, `filterByTag` |
| `tracker.go` | `tracker` struct, JSON types, progress bar, summary/JSON output |

**Package-level globals** (`main.go`): `ui *clui.Clui`, `ignoreMissingDirs`, `quietMode`, `noopMode`, `jsonMode`. All goroutines read these after `app.Before` sets them; they are never written after that.

### Command execution pattern

Every parallel command follows the same structure:

```go
t := newTracker(len(projects))
sem := make(chan struct{}, jobs)   // semaphore limits concurrency
var wg sync.WaitGroup
for _, project := range projects {
    project := project             // capture loop var
    wg.Add(1); sem <- struct{}{}
    go func() {
        defer wg.Done(); defer func() { <-sem }()
        // resolve path → check noopMode → opContext(c) → git call → t.record(opResult{...})
    }()
}
wg.Wait()
if jsonMode { t.printJSON("cmd", warnings) } else { t.printSummary(c.Bool("errors-last")) }
return buildError(t.errors())
```

`opContext(c)` returns a `context.WithTimeout` when `--timeout` is set.

### Output modes

- **Normal**: `clui` calls (Title/Lifecycle/Warnf/Errorf) go to stdout; progress bar overwrites stderr.
- **`--quiet`**: clui verbosity → Low; only the summary line is printed.
- **`--json`**: clui verbosity → Low (suppresses inline output); `tracker.printJSON` emits a single JSON envelope after `wg.Wait()`.
- **`--noop`**: skips all git calls; `tracker.printNoop` writes `[DRY-RUN]` lines.

### Config file cascade (`utils.go` → `defaultConfigurationFilePath`)

1. `-f/--file` flag
2. `GIP_FILE` environment variable
3. `.gip` in current working directory
4. `~/.gip`

### `gipProject` model

```go
type gipProject struct {
    Name       string
    Repository string
    LocalPath  string
    PullPolicy string   // "", "never", "always"
    Tags       []string // optional; used by --tag filter (OR logic, comma-separated)
}
```

`projectsList` returns `([]gipProject, []string, error)` — the second return is collected warnings (empty name, empty local_path, duplicate names, bad pull_policy, undetectable provider).

### Tracker / JSON output

`tracker` accumulates `opResult` values (one per project). Fields: `project`, `localPath`, `status` (opOK/opError/opSkipped), `err`, `reason`, `branch`. At the end:
- `printSummary(errorsLast bool)` — text table with ANSI color (TTY only).
- `printJSON(command, warnings)` — single JSON object: `command`, `timestamp`, `projects[]`, `summary{total,ok,errors,skipped,duration_ms}`, `warnings[]`.

`list` uses a separate `listOutputJSON` / `listProjectJSON` shape (includes repository, policy, provider, tags, missing).

## Testing

Tests live alongside source (`_test.go`). `lib/core/stubs_test.go` provides `mockGitWrapper` with a configurable `result runcmdResult` field; set it to override the default empty-success response.

`cmd/gip/tracker_test.go` and `cmd/gip/utils_test.go` test infrastructure independently of the CLI. Integration tests in `cmd/gip/main_test.go` use `cli.App.RunContext` or temp directories with real git repos.

Version variables (`core.Version`, `core.GitCommit`, `core.BuildTime`) are injected via `-ldflags` at build time; their zero values are empty strings, which is fine for tests.
