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
NAME        LOCAL_PATH              POLICY    PROVIDER     TAGS
frontend    ~/projects/frontend     default   github.com   work, js
dotfiles    ~/dotfiles              default   github.com   personal
legacy      ~/projects/legacy       never     github.com   —
```

With `--json`:

```json
{
  "command": "list",
  "timestamp": "2025-05-15T10:30:00Z",
  "projects": [
    {
      "name": "frontend",
      "local_path": "~/projects/frontend",
      "repository": "https://github.com/org/frontend.git",
      "policy": "default",
      "provider": "github.com",
      "tags": ["work", "js"],
      "missing": false
    }
  ],
  "warnings": []
}
```

## pull

Run `git pull` on managed projects.

- Projects whose `local_path` does not exist are **skipped** unless `pull_policy: always` is set, in which case a `git clone` is performed.
- Projects with `pull_policy: never` are always skipped.

```bash
gip pull
gip pull --tag work -j 8
gip pull -a           # pull all; clone missing repos with pull_policy: always
gip --noop pull       # preview what would be done
```

## fetch

Run `git fetch --all --prune` on all managed projects without merging into the working tree. Projects with `pull_policy: never` are skipped.

```bash
gip fetch
gip fetch -j 8 -t 60
gip fetch --tag work
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
NAME        BRANCH         LOCAL_PATH
frontend    main           ~/projects/frontend
backend     feature/auth   ~/projects/backend
infra       (detached)     ~/work/infra
```

Repositories in detached HEAD state show `(detached)` as the branch name.

## exec

Execute an arbitrary command inside every project directory. Use `--` to separate gip flags from the command and its arguments.

```bash
gip exec -- git fetch --prune
gip exec -- git log --oneline -5
gip exec -j 8 -- make test
gip exec --tag work -- git remote -v
gip --noop exec -- git reset --hard origin/main
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
