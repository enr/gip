---
title: Commands
description: Full reference for all gip commands with flags and examples
weight: 3
---

| Command | Aliases | Description |
|---------|---------|-------------|
| [`status`](#status--statusfull) | `s` | Modified files in each project |
| [`statusfull`](#status--statusfull) | `sf` | Modified and untracked files, plus repos out of sync |
| [`list`](#list) | `ls` | Registered projects with branch and sync status |
| [`pull`](#pull) | | `git pull` (or `git clone` for missing repos) |
| [`fetch`](#fetch) | | `git fetch --all --prune` |
| [`branch`](#branch) | `br` | Current branch of each project |
| [`releases`](#releases) | `r`, `rel` | Latest tag and commits made after it |
| [`exec`](#exec) | | Arbitrary command in each project directory |
| [`init`](#init) | | Generate a configuration file from existing clones |
| [`completion`](#completion) | | Shell completion script |

Projects with `disabled: true` are excluded from every command (`list --all` shows them). See [Flags & Output](/gip/flags/) for the flags shared by all commands.

## status / statusfull

`status` runs `git status --porcelain --untracked-files=no` on all managed projects and prints the modified files of each one.

`statusfull` also shows untracked files and reports the repositories that are out of sync with their upstream: a project that is clean but ahead, behind or without upstream gets a line with its sync state (`<path>  ↑2`), and a project that is clean and in sync is counted as skipped. When a git-state filter (`--dirty`, `--behind`, `--ahead`, `--unsynced`) is given, `statusfull` behaves like `status` plus untracked files.

Both commands also report the extra `branches` configured for a project (see [Extra branches](/gip/configuration/#extra-branches)).

```bash
gip status
gip statusfull

# only work-tagged repos, errors grouped at the bottom
gip status --tag work --errors-last

# only repos with uncommitted changes
gip status --dirty

# ignore repos whose directory does not exist
gip -m status
```

## list

List all registered projects in an aligned table showing name, current branch, sync status, local path, pull policy, detected provider and, when at least one project has tags, tags. Branch and sync status are read with one `git status` call per repository (in parallel, `-j`/`-t` apply).

```bash
gip list
gip list --tag personal
gip list --all        # include projects with disabled: true
gip --json list
```

Example output:

```text
NAME      BRANCH  STATUS   PATH                         POLICY   PROVIDER    TAGS
frontend  main    synced   ~/projects/frontend          default  github.com  work, js
dotfiles  main    synced*  ~/dotfiles                   default  github.com  personal
legacy    —       —        ~/projects/legacy (missing)  never    github.com  —
```

- `synced*` means in sync with upstream but with uncommitted changes.
- Missing directories are marked `(missing)`, paths that cannot be resolved `(ERROR)`, disabled projects `(disabled)`.
- A repository whose state cannot be read shows `ERROR` in STATUS; the error is printed on stderr and gip exits with status 1.
- `list` has no summary line and does not accept `--errors-last`.

| Flag | Description |
|------|-------------|
| `-a, --all` | Include projects with `disabled: true` |
| `--extended` | Add the DIRTY column |
| `--tag`, `--dirty`, `--behind`, `--ahead`, `--unsynced`, `-j`, `-t` | As for the [parallel commands](/gip/flags/#parallel-command-flags) |

### Extended list (`--extended`)

Add `--extended` to include the working-tree dirty indicators as well.

```bash
gip list --extended
gip list --extended --tag work
```

Example output:

```text
NAME        BRANCH    STATUS    DIRTY  PATH                    POLICY    PROVIDER
frontend    main      synced    —      ~/projects/frontend     default   github.com
backend     dev       ↑2        +*     ~/projects/backend      default   github.com
dotfiles    main      ↓1        —      ~/dotfiles              default   github.com
infra       main      ↑1↓3      *      ~/work/infra            default   github.com
```

**STATUS** colour coding (TTY only):

| Value | Meaning | Colour |
|-------|---------|--------|
| `synced` | Branch is up to date with upstream | Green |
| `↑N` | N commits ahead of upstream | Magenta |
| `↓N` | N commits behind upstream | Yellow |
| `↑N↓M` | Diverged — N ahead, M behind | Red |
| `no-remote` | No upstream tracking branch configured | White |

In `list`, `synced*` marks a repo in sync with its upstream but with uncommitted changes.

**DIRTY** symbols: `+` staged · `*` unstaged · `?` untracked · `$` stash present · `—` clean

### Filtering by git state

```bash
# only repos with uncommitted changes
gip list --dirty

# only repos behind their upstream (run gip fetch first)
gip list --behind

# combine
gip list --dirty --extended
```

When a filter is active, missing directories and repositories whose state cannot be read are left out of the table; the latter are still reported as errors.

With `--json` the output has no `summary`. Each entry has `name`, `local_path` (resolved), `repository`, `policy`, `provider` and `tags`; `missing` and `disabled` appear when `true`, `error` when the path or the repository state cannot be read; `provider` is empty when it cannot be detected; `branch`, `ahead`, `behind`, `no_remote` and `dirty` appear when known and non-zero/non-empty, with or without `--extended`:

```json
{
  "command": "list",
  "timestamp": "2025-05-15T10:30:00Z",
  "projects": [
    {
      "name": "frontend",
      "local_path": "/home/user/projects/frontend",
      "repository": "https://github.com/org/frontend.git",
      "policy": "default",
      "provider": "github.com",
      "tags": ["work", "js"],
      "branch": "main",
      "ahead": 2,
      "dirty": "+*"
    }
  ],
  "warnings": []
}
```

## pull

Run `git pull` on managed projects.

- Projects whose `local_path` does not exist are **skipped**, unless `pull_policy: always` is set or `-a/--all` is given, in which case `git clone <repository> <local_path>` is performed.
- Projects with `pull_policy: never` are always skipped, even with `--all`.
- Projects with uncommitted changes are skipped unless `--force` is given; if the check itself fails the project is reported as an error and not pulled. For the same reason `--dirty` requires `--force`.
- Extra `branches` are fast-forwarded too (see [Extra branches](/gip/configuration/#extra-branches)).
- `pull` takes no positional arguments.

| Flag | Description |
|------|-------------|
| `-a, --all` | Also clone the projects whose directory is missing (except `pull_policy: never`) |
| `--force` | Pull even when the working tree has uncommitted changes |

```bash
gip pull
gip pull --tag work -j 8
gip pull -a           # also clone every missing repo
gip --noop pull       # preview what would be done

# only pull repos that are behind their upstream
gip pull --behind
```

## fetch

Run `git fetch --all --prune` on all managed projects without merging into the working tree. Projects with `pull_policy: never` and missing directories are skipped; missing repositories are never cloned.

```bash
gip fetch
gip fetch -j 8 -t 60
gip fetch --tag work

# fetch only repos with local changes (unusual, but supported)
gip fetch --dirty
```

## branch

Show the current branch for every project in an aligned table.

```bash
gip branch
gip branch --tag work
gip --json branch
```

Example output:

```text
NAME        BRANCH         PATH
frontend    main           ~/projects/frontend
backend     feature/auth   ~/projects/backend
infra       (detached)     ~/work/infra
```

Repositories in detached HEAD state show `(detached)` as the branch name. Repositories whose directory does not exist show `(missing)` (omitted when a git-state filter is active). The table is followed by the usual summary line.

### Extended branch view (`--extended`)

Add `--extended` for a richer table that includes upstream sync status, working-tree dirty indicators, and the last commit subject and relative date.

```bash
gip branch --extended
gip branch --extended --tag work
```

Example output:

```text
NAME        BRANCH    STATUS    DIRTY  COMMIT                       DATE        PATH
frontend    main      synced    —      chore: bump deps             3 days ago  ~/projects/frontend
backend     dev       ↑2        +*     feat: add auth endpoint      2 hours ago ~/projects/backend
dotfiles    main      ↓1        —      docs: update README          1 week ago  ~/dotfiles
infra       main      ↑1↓3      *      fix: terraform state         5 days ago  ~/work/infra
```

See the [list --extended](#extended-list---extended) section above for the STATUS colour legend and DIRTY symbol reference.

### Filtering by git state

```bash
# only repos with uncommitted changes
gip branch --dirty

# only repos behind their upstream (run gip fetch first)
gip branch --behind

# combine filters with extended view
gip branch --dirty --extended
gip branch --behind --extended
```

With `--json`, each entry has a `branch` field; with `--extended` (or a git-state filter) it gains sync and dirty fields, and with `--extended` the commit fields. Zero, `false` and empty values are omitted:

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

## releases

Show, for every project, the latest tag reachable from the checked-out branch and how many commits were made after it. Projects with commits after their latest tag are candidates for a new release.

```bash
gip releases
gip releases --pending          # only projects with unreleased commits
gip r --pending                 # same, using the short alias (also: rel)
gip releases --tagged           # hide projects never tagged
gip releases --tagged --pending # tagged projects with unreleased commits
gip releases --match 'v*'       # only consider tags matching a glob
gip releases --pending --tag work
gip --json releases --pending
```

Example output:

```text
NAME        TAG           COMMITS  TAG DATE      BRANCH  PATH
backend     v1.4.0        7        3 weeks ago   main    ~/projects/backend
dotfiles    (none)        42       —             main    ~/dotfiles
frontend    v2.0.1        0        2 days ago    main    ~/projects/frontend
scratch     (no commits)  —        —             —       ~/projects/scratch
```

- Both annotated and lightweight tags are considered. The tag is looked up from `HEAD`, so the result depends on the checked-out branch, shown in the BRANCH column.
- `COMMITS` is the number of commits after the tag; for a project that was never tagged (`(none)`) it is the total number of commits.
- `(no commits)` marks a repository without any commit; `(missing)` a missing directory.
- `--pending` keeps only projects with commits after the latest tag, or never tagged; the others are reported as skipped.
- `--tagged` keeps only projects with at least one tag (hides `(none)` and `(no commits)`); combined with `--pending` it lists the tagged projects with commits after their latest tag.
- Only local refs are read: run `gip fetch` first to see tags created elsewhere.
- The usual `--dirty`, `--behind`, `--ahead` and `--unsynced` filters are supported. With any filter (including `--pending`/`--tagged`) missing directories are left out of the table.

### How the latest tag is chosen

gip relies on `git describe --tags --long`: the "latest" tag is the **closest tag in the history of `HEAD`**, not the most recent by date or the highest version number.

- Only tags reachable from `HEAD` count. A tag created on a branch not merged into the checked-out one (e.g. `v1.0.1` on a maintenance branch) is ignored until that branch is merged.
- Among reachable tags, the one with the fewest commits between it and `HEAD` wins. Version numbers are not compared: if `v9.0.0` is further back in history than `v1.0.0`, `v1.0.0` is reported.
- When several tags point to the same commit, annotated tags win over lightweight ones, and the most recent annotated tag (by tagger date) wins among them.
- `COMMITS` is the number of commits in `HEAD` not reachable from the tag (`git rev-list --count <tag>..HEAD`), merge commits included.
- `--match <glob>` restricts the candidates, e.g. `--match 'v*'` to ignore non-release tags such as `nightly` or `deploy-*`.

With `--json`, each project entry gains tag fields (`commits_since_tag` is always present, `0` included, when the repository has commits):

```json
{
  "name": "backend",
  "local_path": "/home/user/projects/backend",
  "status": "ok",
  "branch": "main",
  "last_tag": "v1.4.0",
  "commits_since_tag": 7,
  "tag_date": "3 weeks ago",
  "unreleased": true
}
```

## exec

Execute an arbitrary command inside every project directory. Use `--` to separate gip flags from the command and its arguments. The command is executed directly, not through a shell: to use pipes, redirections or variables wrap it, e.g. `gip exec -- sh -c 'git log -1 | cat'`.

```bash
gip exec -- git fetch --prune
gip exec -- git log --oneline -5
gip exec -j 8 -- make test
gip exec --tag work -- git remote -v
gip --noop exec -- git reset --hard origin/main

# run a command only in repos with uncommitted changes
gip exec --dirty -- git stash
```

The command is run in the `local_path` of each project (missing directories are skipped). stdout and stderr of every invocation are collected and printed in a synchronised block so output from different projects does not interleave. A non-zero exit status is reported as an error for that project, and gip exits with status 1.

## init

Scan a directory recursively for Git repositories and generate a gip configuration file.

```bash
# scan ~/projects and write .gip in the current directory (asks for confirmation if it exists)
gip init ~/projects

# write the global configuration
gip init ~/projects --output ~/.gip

# write to a specific output file
gip init ~/projects --output ~/work/.gip

# overwrite without prompting, scan up to 3 levels deep
gip init . --force --depth 3
```

The directory to scan defaults to the current one. For each directory containing a `.git` entry (directory or file, so worktrees and submodules are found too), gip detects:

- the directory name as `name` (when two repositories share it, their path relative to the scanned directory is used instead, since names must be unique)
- the `origin` remote URL as `repository` (via `git remote get-url origin`)
- the absolute path as `local_path`

gip does not descend into a repository it found, so nested repositories are not listed. Repositories without a configured `origin` remote are skipped with a warning. The generated file is in YAML format, written with `0600` permissions; edit it to add `tags`, `pull_policy` and the other [fields](/gip/configuration/#fields). Nothing is written when no repository is found.

| Flag | Description |
|------|-------------|
| `-o, --output <path>` | Output file path (default: `.gip` in the current directory) |
| `--force` | Overwrite existing file without prompting |
| `--depth <n>` | Maximum directory scan depth (default: 5) |

## completion

Print a shell completion script for `bash`, `zsh` or `fish`.

```bash
source <(gip completion bash)
source <(gip completion zsh)
gip completion fish > ~/.config/fish/completions/gip.fish
```

Add the `source` line to `~/.bashrc` / `~/.zshrc` to enable it permanently.
