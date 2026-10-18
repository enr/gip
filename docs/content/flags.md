---
title: Flags & Output
description: Global flags, per-command flags, output modes, and dry-run behaviour
weight: 4
---

## Global flags

These flags are available on every command and go **before** the command name (`gip --json status`, not `gip status --json`).

| Flag | Description |
|------|-------------|
| `-f, --file <path>` | Path to the configuration file (overrides all other lookup methods) |
| `-d, --debug` | Verbose output: logs every git command executed and the config file path |
| `-q, --quiet` | Suppress per-project output and the progress bar; tables, warnings (on stderr) and the summary line are still printed |
| `--json` | Emit a single JSON envelope to stdout instead of text output; takes precedence over `--quiet` |
| `--noop` | Dry-run: print what would be done without executing any git command |
| `-m, --ignore-missing` | Do not print the warning for projects whose `local_path` does not exist (they are skipped either way) |
| `-v, --version` | Print version, revision and build date |
| `-h, --help` | Show help; `gip <command> --help` shows the command's flags |

## Command flags

### Parallel command flags

The following flags are available on all parallel commands (`status`, `statusfull`, `pull`, `fetch`, `branch`, `releases`, `exec`):

| Flag | Description |
|------|-------------|
| `-j, --jobs <n>` | Maximum number of repositories to process concurrently, 1–32 (default: 4) |
| `-t, --timeout <seconds>` | Per-operation timeout in seconds; 0 means no timeout (default: 0) |
| `--tag <tags>` | Filter projects by tag — comma-separated and/or repeated, OR logic (e.g. `--tag work,js` or `--tag work --tag js`) |
| `--errors-last` | Also print a grouped error section after the summary (errors are still shown inline) |
| `--show-skipped` | Also print the skipped projects, each with the reason it was skipped, after the summary |
| `--dirty` | Only include repos with uncommitted changes (staged, unstaged or untracked files; a stash alone does not count) |
| `--behind` | Only include repos behind their upstream (diverged included); requires up-to-date remote refs — run `gip fetch` first |
| `--ahead` | Only include repos ahead of their upstream (diverged included); requires up-to-date remote refs — run `gip fetch` first |
| `--unsynced` | Only include repos out of sync with upstream (ahead or behind); alias for `--behind --ahead` |

`list` accepts `--tag`, `-j`, `-t` and the git-state filters, but not `--errors-last` or `--show-skipped` (see [list](/gip/commands/#list)). `pull` adds `-a/--all` and `--force` (see [pull](/gip/commands/#pull)).

### Releases flags

Available on `releases`, in addition to the parallel command flags.

| Flag | Description |
|------|-------------|
| `-p, --pending` | Only include projects with commits after the latest tag, or never tagged |
| `--tagged` | Only include projects with at least one tag (hides `(none)` and `(no commits)`) |
| `--match <glob>` | Only consider tags matching the glob (`git describe --match`), e.g. `'v*'` |

### Extended view (`--extended`)

Available on `branch` and `list`. On `branch` it runs additional git calls per repo to collect upstream sync state, working-tree indicators and last commit info; `list` always shows branch and sync status, and `--extended` only adds the DIRTY column.

| Flag | Available on | Description |
|------|-------------|-------------|
| `--extended` | `branch` | Add STATUS, DIRTY, COMMIT and DATE columns |
| `--extended` | `list` | Add the DIRTY column |

**STATUS values and colours (TTY only):**

| Value | Meaning | Colour |
|-------|---------|--------|
| `synced` | Up to date with upstream | Green |
| `↑N` | N commits ahead of upstream | Magenta |
| `↓N` | N commits behind upstream | Yellow |
| `↑N↓M` | Diverged — N ahead, M behind | Red |
| `no-remote` | No upstream tracking branch | White |

`list` shows `synced*` for a repo in sync with its upstream but with uncommitted changes.

**DIRTY symbols:**

| Symbol | Meaning |
|--------|---------|
| `+` | Staged changes |
| `*` | Unstaged changes |
| `?` | Untracked files |
| `$` | Stash present |
| `—` | Clean working tree |

### Git-state filter flags (`--dirty` / `--behind` / `--ahead` / `--unsynced`)

These flags skip repos that do not match the requested state before running the main operation. An extra `git status --porcelain=v2 --branch` call (plus `git stash list`) is made per repo to evaluate the condition. When several are given, `--dirty` must match **and** at least one of the sync filters must match. `--unsynced` is shorthand for `--behind --ahead` and matches any repo that is ahead of or behind its upstream. A repo without upstream never matches the sync filters. If the state of a repo cannot be read, it is reported as an error instead of being treated as matching.

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

Each project prints its git output inline as it completes. A progress bar is shown on stderr while processing (TTY only, suppressed in `--quiet` and `--json` modes). `branch` and `releases` print their table after all projects are processed. At the end a summary line is printed (not by `list`):

```text
─────────────────────────────────────────
OK: 5   Errors: 1   Skipped: 2   Duration: 3.4s
```

### Quiet (`--quiet`)

Per-project output and the progress bar are suppressed, as is the `gip: using configuration` line. Tables (`list`, `branch`, `releases`), the final summary line and warnings and errors (on stderr) are still printed.

### JSON (`--json`)

A single JSON envelope is written to stdout after all projects have been processed; warnings and the `gip: using configuration` line go to stderr, so stdout can be piped to `jq`. The shape below is shared by `status`, `statusfull`, `pull`, `fetch`, `branch`, `releases` and `exec`; `list` uses its own shape without `summary` (see [list](/gip/commands/#list)) and so does `check` (see [check](/gip/commands/#check)). Extra branches appear as separate entries named `name@branch`.

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

`status` values: `ok`, `error`, `skipped`. `error` holds the error message of failed entries; `reason` is set for skipped entries, for example:

| Reason | When |
|--------|------|
| `pull_policy: never` | `pull` / `fetch` on a project with `pull_policy: never` |
| `local dir missing`, `not a project dir` | the directory does not exist or has no `.git` |
| `uncommitted changes (use --force)` | `pull` on a dirty repo |
| `not dirty`, `not ahead or behind`, `not behind`, `not ahead` | excluded by a git-state filter |
| `clean and in sync` | `statusfull` on a repo with nothing to report |
| `no commits`, `no tags`, `no commits after latest tag` | excluded by `releases --tagged` / `--pending` |
| `in sync`, `branch not found locally`, `no upstream configured`, `ahead of upstream (nothing to pull)` | extra `branches` entries |

#### Extended JSON fields (`branch --extended`)

`branch` always sets `branch`. When `--extended` (or a git-state filter) is used, each project entry may also include:

| Field | Type | Description |
|-------|------|-------------|
| `branch` | string | Current branch name or `"(detached)"` |
| `ahead` | int | Commits ahead of upstream (omitted when 0) |
| `behind` | int | Commits behind upstream (omitted when 0) |
| `no_remote` | bool | `true` when no upstream tracking branch is configured |
| `dirty` | string | Dirty symbols (e.g. `"+*"`); omitted when clean |
| `commit_msg` | string | Last commit subject line (`--extended` only) |
| `commit_date` | string | Relative date of last commit (e.g. `"2 hours ago"`; `--extended` only) |

```json
{
  "command": "branch",
  "timestamp": "2025-05-15T10:30:00Z",
  "projects": [
    {
      "name": "backend",
      "local_path": "/home/user/projects/backend",
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

#### Tag JSON fields (`releases`)

The `releases` command adds to each project entry with at least one commit:

| Field | Type | Description |
|-------|------|-------------|
| `branch` | string | Checked-out branch the tag is looked up from |
| `last_tag` | string | Latest tag reachable from `HEAD`; omitted when the project was never tagged |
| `commits_since_tag` | int | Commits after `last_tag` (total commits when never tagged); always present, `0` included |
| `tag_date` | string | Relative creation date of the tag (e.g. `"3 weeks ago"`) |
| `unreleased` | bool | `true` when there are commits after the tag, or the project was never tagged; omitted when `false` |

#### Git fields in `list` JSON

`list` entries include `branch`, `ahead`, `behind`, `no_remote` and `dirty` whenever they are known and non-empty, with or without `--extended` (same meaning as above; no commit fields).

No ANSI codes appear in any JSON output.

### Dry-run (`--noop`)

No git command (or `exec` command) is executed. Each project prints a `[DRY-RUN]` line describing what would happen, and the summary is followed by a reminder. Projects that would run count as OK. The exit status is 0 unless a `local_path` cannot be resolved (e.g. unset environment variable). Git-state filters are not evaluated in dry-run mode. `--noop` has no effect on `list`, `init` and `completion`; `check` prints a `[DRY-RUN]` line per project and checks nothing.

```bash
$ gip --noop pull
[DRY-RUN] frontend → git pull  (in /home/user/projects/frontend)
[DRY-RUN] legacy → SKIPPED  (pull_policy: never)
─────────────────────────────────────────
OK: 1   Errors: 0   Skipped: 1   Duration: 0.0s
DRY-RUN — no operations performed. Remove --noop to proceed.
```

## Exit status

gip exits with status `0` when every project succeeded or was skipped, and `1` when at least one project failed or the command could not start (configuration not found or invalid, bad flag values such as `--jobs 0`). The failed projects are listed on stderr.
