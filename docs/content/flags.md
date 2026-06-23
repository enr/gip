---
title: Flags & Output
description: Global flags, per-command flags, output modes, and dry-run behaviour
weight: 4
---

## Global flags

These flags are available on every command.

| Flag | Description |
|------|-------------|
| `-f, --file <path>` | Path to the configuration file (overrides all other lookup methods) |
| `-d, --debug` | Verbose output: logs every git command executed and the config file path |
| `-q, --quiet` | Suppress all output except the final summary line |
| `--json` | Emit a single JSON envelope to stdout instead of text output; takes precedence over `--quiet` |
| `--noop` | Dry-run: print what would be done without executing any git command |
| `-m, --ignore-missing` | Silently skip projects whose `local_path` does not exist; without this flag a warning is printed |

## Command flags

### Parallel command flags

The following flags are available on all parallel commands (`status`, `statusfull`, `pull`, `fetch`, `branch`, `exec`):

| Flag | Description |
|------|-------------|
| `-j, --jobs <n>` | Maximum number of repositories to process concurrently (default: 4) |
| `-t, --timeout <seconds>` | Per-operation timeout in seconds; 0 means no timeout (default: 0) |
| `--tag <tags>` | Filter projects by tag — comma-separated list with OR logic (e.g. `--tag work,js`) |
| `--errors-last` | Print a grouped error section after the summary instead of inline |
| `--dirty` | Only include repos with uncommitted changes (staged, unstaged, or untracked files) |
| `--behind` | Only include repos behind their upstream; requires up-to-date remote refs — run `gip fetch` first |
| `--ahead` | Only include repos ahead of their upstream; requires up-to-date remote refs — run `gip fetch` first |
| `--unsynced` | Only include repos out of sync with upstream (ahead or behind); alias for `--behind --ahead` |

### Extended view (`--extended`)

Available on `branch` and `list`. Runs additional git calls per repo to collect upstream sync state, working-tree indicators, and (for `branch`) last commit info.

| Flag | Available on | Description |
|------|-------------|-------------|
| `--extended` | `branch`, `list` | Show BRANCH, STATUS, DIRTY columns and (branch only) COMMIT and DATE |

**STATUS values and colours (TTY only):**

| Value | Meaning | Colour |
|-------|---------|--------|
| `synced` | Up to date with upstream | Green |
| `↑N` | N commits ahead of upstream | Magenta |
| `↓N` | N commits behind upstream | Yellow |
| `↑N↓M` | Diverged — N ahead, M behind | Red |
| `no-remote` | No upstream tracking branch | White |

**DIRTY symbols:**

| Symbol | Meaning |
|--------|---------|
| `+` | Staged changes |
| `*` | Unstaged changes |
| `?` | Untracked files |
| `$` | Stash present |
| `—` | Clean working tree |

### Git-state filter flags (`--dirty` / `--behind` / `--ahead` / `--unsynced`)

These flags skip repos that do not match the requested state before running the main operation. An extra `git status --porcelain=v2 --branch` call is made per repo to evaluate the condition. `--unsynced` is shorthand for `--behind --ahead` and matches any repo that is ahead of or behind its upstream.

```bash
# show status only for repos with uncommitted changes
gip status --dirty

# pull only repos that are behind their upstream
gip pull --behind

# list repos that have drifted from upstream in either direction
gip list --unsynced --extended

# run stash only in repos with local changes
gip exec --dirty -- git stash

# see which repos need attention
gip branch --dirty --extended
gip branch --behind --extended
```

> **Note:** `--behind`, `--ahead` and `--unsynced` compare against the last fetched remote tracking refs. If you have not run `gip fetch` recently the result may be stale.

## Output modes

### Text (default)

Each project prints its git output inline as it completes. A progress bar is shown on stderr while processing (TTY only, suppressed in `--quiet` and `--json` modes). At the end a summary line is printed:

```text
─────────────────────────────────────────
OK: 5   Errors: 1   Skipped: 2   Duration: 3.4s
```

### Quiet (`--quiet`)

All per-project output is suppressed; only the final summary line is printed.

### JSON (`--json`)

A single JSON envelope is written to stdout after all projects have been processed:

```json
{
  "command": "status",
  "timestamp": "2025-05-15T10:30:00Z",
  "projects": [
    {
      "name": "frontend",
      "local_path": "/home/user/projects/frontend",
      "status": "ok"
    },
    {
      "name": "legacy",
      "local_path": "/home/user/projects/legacy",
      "status": "skipped",
      "reason": "pull_policy: never"
    },
    {
      "name": "broken",
      "local_path": "/home/user/broken",
      "status": "error",
      "error": "exit status 128"
    }
  ],
  "summary": {
    "total": 3,
    "ok": 1,
    "errors": 1,
    "skipped": 1,
    "duration_ms": 1240
  },
  "warnings": []
}
```

`status` values: `ok`, `error`, `skipped`. The `reason` field is set for skipped entries (e.g. `"pull_policy: never"`, `"not dirty"`, `"not behind"`).

#### Extended JSON fields (`branch --extended`)

When `--extended` is used with `branch`, each project entry may include:

| Field | Type | Description |
|-------|------|-------------|
| `branch` | string | Current branch name or `"(detached)"` |
| `ahead` | int | Commits ahead of upstream (omitted when 0) |
| `behind` | int | Commits behind upstream (omitted when 0) |
| `no_remote` | bool | `true` when no upstream tracking branch is configured |
| `dirty` | string | Dirty symbols (e.g. `"+*"`); omitted when clean |
| `commit_msg` | string | Last commit subject line |
| `commit_date` | string | Relative date of last commit (e.g. `"2 hours ago"`) |

```json
{
  "command": "branch",
  "timestamp": "2025-05-15T10:30:00Z",
  "projects": [
    {
      "name": "backend",
      "local_path": "~/projects/backend",
      "status": "ok",
      "branch": "dev",
      "ahead": 2,
      "dirty": "+*",
      "commit_msg": "feat: add auth endpoint",
      "commit_date": "2 hours ago"
    }
  ],
  "summary": { "total": 1, "ok": 1, "errors": 0, "skipped": 0, "duration_ms": 45 },
  "warnings": []
}
```

#### Extended JSON fields (`list --extended`)

When `--extended` is used with `list`, each project entry may include `branch`, `ahead`, `behind`, `no_remote`, and `dirty` (same meaning as above; no commit fields).

No ANSI codes appear in any JSON output.

### Dry-run (`--noop`)

No git command is executed. Each project prints a `[DRY-RUN]` line describing what would happen. Exit code is always 0.

```bash
$ gip --noop pull
[DRY-RUN] frontend → git pull  (in ~/projects/frontend)
[DRY-RUN] legacy   → SKIPPED  (pull_policy: never)
```
