---
title: Commands
description: Full reference for all gip commands with flags and examples
weight: 3
---

## status / statusfull

Run `git status --porcelain` on all managed projects. `statusfull` also shows untracked files.

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

List all registered projects in an aligned table showing name, local path, pull policy, detected provider, and tags.

```bash
gip list
gip list --tag personal
gip --json list
```

Example output:

```text
NAME        PATH                    POLICY    PROVIDER
frontend    ~/projects/frontend     default   github.com
dotfiles    ~/dotfiles              default   github.com
legacy      ~/projects/legacy       never     github.com
```

### Extended list (`--extended`)

Add `--extended` to include the current branch, sync status, and dirty indicators for every repository. This runs extra git calls per repo.

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

**STATUS** colour coding:

| Value | Meaning | Colour |
|-------|---------|--------|
| `synced` | Branch is up to date with upstream | Green |
| `↑N` | N commits ahead of upstream | Magenta |
| `↓N` | N commits behind upstream | Yellow |
| `↑N↓M` | Diverged — N ahead, M behind | Red |
| `no-remote` | No upstream tracking branch configured | White |

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

With `--json` and `--extended`, each project entry gains additional fields:

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
      "ahead": 0,
      "behind": 0,
      "dirty": "+*"
    }
  ],
  "warnings": []
}
```

## pull

Run `git pull` on managed projects.

- Projects whose `local_path` does not exist are **skipped** unless `pull_policy: always` is set, in which case a `git clone` is performed.
- Projects with `pull_policy: never` are always skipped.
- Projects with uncommitted changes are skipped unless `--force` is given; if the check itself fails the project is reported as an error and not pulled. For the same reason `--dirty` requires `--force`.

```bash
gip pull
gip pull --tag work -j 8
gip pull -a           # pull all; clone missing repos with pull_policy: always
gip --noop pull       # preview what would be done

# only pull repos that are behind their upstream
gip pull --behind
```

## fetch

Run `git fetch --all --prune` on all managed projects without merging into the working tree. Projects with `pull_policy: never` are skipped.

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

Repositories in detached HEAD state show `(detached)` as the branch name. Repositories whose directory does not exist show `(missing)`.

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

With `--json` and `--extended`, each project entry gains sync, dirty, and commit fields:

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
      "behind": 0,
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
- `--pending` keeps only projects with commits after the latest tag, or never tagged; the others are reported as skipped.
- Only local refs are read: run `gip fetch` first to see tags created elsewhere.
- The usual `--dirty`, `--behind`, `--ahead` and `--unsynced` filters are supported.

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

Execute an arbitrary command inside every project directory. Use `--` to separate gip flags from the command and its arguments.

```bash
gip exec -- git fetch --prune
gip exec -- git log --oneline -5
gip exec -j 8 -- make test
gip exec --tag work -- git remote -v
gip --noop exec -- git reset --hard origin/main

# run a command only in repos with uncommitted changes
gip exec --dirty -- git stash
```

The command is run in the `local_path` of each project. stdout and stderr of every invocation are collected and printed in a synchronised block so output from different projects does not interleave.

## init

Scan a directory recursively for Git repositories and generate a gip configuration file.

```bash
# scan ~/projects and write to ~/.gip (asks for confirmation if it exists)
gip init ~/projects

# write to a specific output file
gip init ~/projects --output ~/work/.gip

# overwrite without prompting, scan up to 3 levels deep
gip init . --force --depth 3
```

For each `.git/` directory found, gip detects:

- the directory name as `name`
- the `origin` remote URL as `repository` (via `git remote get-url origin`)
- the absolute path as `local_path`

Repositories without a configured `origin` remote are skipped with a warning. The generated file is in YAML format.

| Flag | Description |
|------|-------------|
| `-o, --output <path>` | Output file path (default: `~/.gip`) |
| `--force` | Overwrite existing file without prompting |
| `--depth <n>` | Maximum directory scan depth (default: 5) |
