---
title: Flags & Output
description: Global flags, per-command flags, output modes, and dry-run behaviour
weight: 4
---

## Global flags

These flags are available on every command and can be written before or after the command name: `gip --json status` and `gip status --json` are the same. After the command they are recognized up to the first argument that is not a flag, or `--`: in `gip exec -- git log -q` the `-q` belongs to `git log`.

| Flag | Description |
|------|-------------|
| `-f, --file <path>` | Path to the configuration file (overrides all other lookup methods) |
| `-d, --debug` | Verbose output: logs every git command executed, the config file path and informational notices (e.g. repositories whose provider cannot be detected) |
| `-q, --quiet` | Suppress per-project output and the progress bar; tables, warnings (on stderr) and the summary line are still printed |
| `--json` | Emit a single JSON envelope to stdout instead of text output; takes precedence over `--quiet` |
| `--noop` | Dry-run: print what would be done without executing any git command |
| `-m, --ignore-missing` | Do not print the warning for projects whose `local_path` does not exist (they are skipped either way) |
| `--strict` | A missing `local_path` (or a missing extra branch) is an error instead of a skipped project: the command exits with status `1`. Meant for CI, e.g. `gip --strict pull` |
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
| `--min <n>` | Only include projects with at least `n` commits after the latest tag; never-tagged projects count all their commits unless `--tagged` is given (default 0: no minimum) |
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
OK: 5   Errors: 1   Skipped: 3 (missing 1, filtered 2)   Duration: 3.4s
```

The skipped projects are split by kind (only the non-zero kinds are shown): `missing` (the configured directory or branch does not exist: usually a configuration to fix), `protected` (gip refused to touch the repository: `pull_policy: never`, uncommitted changes without `--force`) and `filtered` (nothing to do: clean, in sync, excluded by a filter). `--show-skipped` lists them with their reason.

### Quiet (`--quiet`)

Per-project output and the progress bar are suppressed, as is the `gip: using configuration` line (printed only for a `.gip` found in the current directory). Tables (`list`, `branch`, `releases`), the final summary line and warnings and errors (on stderr) are still printed.

### JSON (`--json`)

A single JSON envelope is written to stdout after all projects have been processed; warnings and the `gip: using configuration` line go to stderr, so stdout can be piped to `jq`. The shape below is shared by `status`, `statusfull`, `pull`, `fetch`, `branch`, `releases` and `exec`; `list` uses its own shape without `summary` (see [list](/gip/commands/#list)) and so does `check` (see [check](/gip/commands/#check)). Extra branches appear as separate entries named `name@branch`.

Every envelope starts with `schema_version` (currently `2`): it changes only when a field is removed or changes meaning, while new fields can be added without changing it. Projects are sorted by name, so two runs on the same repositories give the same order. `command` is the command that ran (`statusfull`, not `status`, for `gip statusfull`).

```json
{
  "schema_version": 2,
  "command": "statusfull",
  "timestamp": "2025-05-15T10:30:00Z",
  "projects": [
    {
      "name": "broken",
      "local_path": "/home/user/broken",
      "status": "error",
      "error": "exit status 128: fatal: not a git repository"
    },
    {
      "name": "frontend",
      "local_path": "/home/user/projects/frontend",
      "status": "ok",
      "state": {
        "upstream": true, "ahead": 0, "behind": 0,
        "dirty": true, "staged": false, "unstaged": true, "untracked": true, "stashed": false
      },
      "files": [
        { "path": "src/app.ts", "index": ".", "worktree": "M" },
        { "path": "notes.md", "index": "?", "worktree": "?" }
      ]
    },
    {
      "name": "legacy",
      "local_path": "/home/user/projects/legacy",
      "status": "skipped",
      "reason": "clean and in sync",
      "skip": "filtered"
    }
  ],
  "summary": {
    "total": 3,
    "ok": 1,
    "errors": 1,
    "skipped": 1,
    "skipped_by": { "missing": 0, "protected": 0, "filtered": 1 },
    "duration_ms": 1240
  },
  "warnings": []
}
```

`status` values: `ok`, `error`, `skipped`. `error` holds the error message of failed entries; skipped entries have a `reason` and a `skip` kind (`missing`, `protected`, `filtered`, see [Text](#text-default)), counted in `summary.skipped_by` (all three keys always present):

| Reason | Kind | When |
|--------|------|------|
| `local dir missing`, `not a project dir` | `missing` | the directory does not exist or has no `.git` |
| `branch not found locally` | `missing` | an extra `branches` entry that does not exist |
| `pull_policy: never` | `protected` | `pull` / `fetch` on a project with `pull_policy: never` |
| `uncommitted changes (use --force)` | `protected` | `pull` on a dirty repo |
| `not dirty`, `not ahead or behind`, `not behind`, `not ahead` | `filtered` | excluded by a git-state filter |
| `clean and in sync` | `filtered` | `statusfull` on a repo with nothing to report |
| `no commits`, `no tags`, `no commits after latest tag`, `fewer than <n> commits after latest tag` | `filtered` | excluded by `releases --tagged` / `--pending` / `--min` |
| `in sync`, `no upstream configured`, `ahead of upstream (nothing to pull)` | `filtered` | extra `branches` entries |

With `--strict` the `missing` entries become errors (`"status": "error"`, `"error": "local dir missing (--strict)"`).

#### Command results

Each entry carries the data of the command that ran, so a script or an agent never has to run git again to read it. A field is omitted when the data was not read (skipped or failed project, `--noop`); once present, it is complete: `false`, `0` and empty values are written.

| Field | Commands | Description |
|-------|----------|-------------|
| `files` | `status`, `statusfull` | Changed files of `git status --porcelain` (untracked ones only with `statusfull`): `path`, `orig_path` (source of a rename or copy), `index` and `worktree` (the X and Y status letters, `.` when unchanged, `?` for untracked files). `[]` for a clean tree |
| `exec` | `exec` | `exit_code` (`-1` when the command did not start or was stopped by `--timeout`), `stdout` and `stderr` of the command, also when it failed |
| `pull` | `pull` | `cloned` (`true` for a repository cloned by `--all` or `pull_policy: always`), `updated`, `from` and `to` (`HEAD` before and after, full hashes; `from` is omitted for a clone), `commits` (number of commits between them) |
| `branch` | `branch`, `releases`, extra branches | Current branch name or `"(detached)"` |
| `state` | `statusfull`, `branch --extended`, git-state filters | Upstream delta and working tree, described below |
| `commit_msg`, `commit_date` | `branch --extended` | Last commit subject and relative date (e.g. `"2 hours ago"`) |

#### The `state` object

| Field | Type | Description |
|-------|------|-------------|
| `upstream` | bool | An upstream tracking branch is configured |
| `ahead`, `behind` | int | Commits ahead of and behind the upstream |
| `dirty` | bool | Uncommitted changes: `staged`, `unstaged` or `untracked` (a stash alone does not count) |
| `staged`, `unstaged`, `untracked`, `stashed` | bool | Each kind of change; `stashed` is `true` when the stash is not empty |

```json
{
  "schema_version": 2,
  "command": "branch",
  "timestamp": "2025-05-15T10:30:00Z",
  "projects": [
    {
      "name": "backend",
      "local_path": "/home/user/projects/backend",
      "status": "ok",
      "branch": "dev",
      "state": {
        "upstream": true, "ahead": 2, "behind": 0,
        "dirty": true, "staged": true, "unstaged": true, "untracked": false, "stashed": false
      },
      "commit_msg": "feat: add auth endpoint",
      "commit_date": "2 hours ago"
    }
  ],
  "summary": { "total": 1, "ok": 1, "errors": 0, "skipped": 0, "skipped_by": { "missing": 0, "protected": 0, "filtered": 0 }, "duration_ms": 45 },
  "warnings": []
}
```

#### Changes from schema version 1

Version 1 envelopes had no `schema_version`. Version 2 replaces the top-level `ahead`, `behind` (omitted when 0), `no_remote` and `dirty` (a string of symbols such as `"+*?"`) with the `state` object, sorts the projects by name, reports `statusfull` as its own command and adds `files`, `exec` and `pull`.

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

`list` entries include `branch` and the `state` object whenever the repository could be read, with or without `--extended` (same meaning as above; no commit fields).

No ANSI codes appear in any JSON output.

### Dry-run (`--noop`)

No git command (or `exec` command) is executed. Each project prints a `[DRY-RUN]` line describing what would happen, and the summary is followed by a reminder. Projects that would run count as OK. The exit status is 0 unless a `local_path` cannot be resolved (e.g. unset environment variable). Git-state filters are not evaluated in dry-run mode. `--noop` has no effect on `list`, `init` and `completion`; `check` prints a `[DRY-RUN]` line per project and checks nothing.

```bash
$ gip --noop pull
[DRY-RUN] frontend → git pull  (in /home/user/projects/frontend)
[DRY-RUN] legacy → SKIPPED  (pull_policy: never)
─────────────────────────────────────────
OK: 1   Errors: 0   Skipped: 1 (protected 1)   Duration: 0.0s
DRY-RUN — no operations performed. Remove --noop to proceed.
```

## Exit status

| Status | Meaning |
|--------|---------|
| `0` | Every project succeeded or was skipped |
| `1` | At least one project failed, or is missing with `--strict`; also when gip cannot run at all (git not installed, `init` cannot write its file) |
| `2` | Usage or configuration error, nothing was done: configuration not found, unreadable or invalid (bad YAML, duplicate names), unknown command or flag, bad flag value (`--jobs 0`, `--min -1`), missing argument (`gip exec` without a command) |

So a script can tell "some repositories need attention" from "gip was called or configured wrongly":

```bash
gip --strict --json pull > result.json
case $? in
  0) echo "all good" ;;
  1) jq -r '.projects[] | select(.status == "error") | .name' result.json ;;
  2) echo "fix the gip invocation or configuration" >&2; exit 2 ;;
esac
```

The failed projects are listed on stderr, one line each: the most telling line of the git output (`fatal: ...`), followed by a hint for well-known causes (missing credentials, rejected SSH key, unknown host, repository not found, diverged branches). The full git output is printed inline above it. With `--errors-last` the list appears only in the summary's error section. An unknown command suggests the closest one.

## Colors

Colors are used only when the output stream is a terminal. They are disabled by `NO_COLOR` (any non-empty value, see [no-color.org](https://no-color.org)) and by `TERM=dumb`, and forced with `FORCE_COLOR` (any non-empty value other than `0`), e.g. in CI systems that render ANSI in their logs. `NO_COLOR` wins over `FORCE_COLOR`. Piped or redirected output never contains escape sequences unless `FORCE_COLOR` is set.
