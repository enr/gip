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

The following flags are available on all parallel commands (`status`, `statusfull`, `pull`, `fetch`, `branch`, `exec`):

| Flag | Description |
|------|-------------|
| `-j, --jobs <n>` | Maximum number of repositories to process concurrently (default: 4) |
| `-t, --timeout <seconds>` | Per-operation timeout in seconds; 0 means no timeout (default: 0) |
| `--tag <tags>` | Filter projects by tag — comma-separated list with OR logic (e.g. `--tag work,js`) |
| `--errors-last` | Print a grouped error section after the summary instead of inline |

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

`status` values: `ok`, `error`, `skipped`. The `branch` command adds a `branch` field to each project entry. No ANSI codes appear in JSON output.

### Dry-run (`--noop`)

No git command is executed. Each project prints a `[DRY-RUN]` line describing what would happen. Exit code is always 0.

```bash
$ gip --noop pull
[DRY-RUN] frontend → git pull  (in ~/projects/frontend)
[DRY-RUN] legacy   → SKIPPED  (pull_policy: never)
```
