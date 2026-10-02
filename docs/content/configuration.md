---
title: Configuration
description: Config file format, project fields, tags, file lookup cascade, and validation warnings
weight: 2
---

## File format

The config file is a list of project entries in YAML or JSON. Each entry describes one repository.

### YAML example

```yaml
- name: frontend
  repository: "https://github.com/org/frontend.git"
  local_path: ~/projects/frontend
  tags: [work, js]

- name: dotfiles
  repository: "git@github.com:user/dotfiles.git"
  local_path: ~/dotfiles
  tags: [personal]

- name: legacy
  repository: "https://github.com/org/legacy.git"
  local_path: ~/projects/legacy
  pull_policy: never
```

### JSON example

```json
[
  {
    "name": "frontend",
    "repository": "https://github.com/org/frontend.git",
    "local_path": "~/projects/frontend",
    "tags": ["work", "js"]
  },
  {
    "name": "dotfiles",
    "repository": "git@github.com:user/dotfiles.git",
    "local_path": "~/dotfiles",
    "tags": ["personal"]
  }
]
```

## Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Descriptive name for the project. Must be unique across the file. |
| `repository` | yes | Remote URL (HTTPS or SSH). Used by `pull_policy: always` to clone missing repos. |
| `local_path` | yes | Local directory path. `~` and environment variables are expanded. |
| `pull_policy` | no | `never` — always skip this project for `pull` and `fetch`. `always` — clone via `git clone` if `local_path` does not exist. Omit for default behaviour: operate only on already-present directories. |
| `branches` | no | Extra local branches reported by `status` and fast-forwarded by `pull` (see [Extra branches](#extra-branches)). |
| `tags` | no | List of free-form labels. Used by `--tag` filtering (see [Tag filtering](#tag-filtering)). |

## Extra branches

By default gip only looks at the branch currently checked out in each repo. List more with `branches`:

```yaml
- name: api
  repository: git@github.com:acme/api.git
  local_path: ~/work/api
  branches: [main, release/1.x]
```

- `gip status` / `statusfull` print one extra line per listed branch that is ahead, behind or without upstream (`repo [release/1.x]  ↓2`). It reads local refs only: run `gip fetch` first to refresh them. Results appear as `name@branch` in the summary and JSON output.
- `gip pull` fast-forwards each listed branch that is not checked out (`git fetch <remote> b:b`). Branches that cannot fast-forward (diverged) are reported as errors; local-only or ahead-only branches, missing branches and branches without upstream are skipped. Nothing is ever rewritten and the checked-out branch is never switched.
- `pull_policy: never` also disables branch updates. The checked-out branch keeps being handled by the normal pull.
- Without `branches` nothing changes.

## Tag filtering

Tags allow you to run any gip command against a named subset of projects without maintaining separate config files.

Both YAML list syntaxes are accepted:

```yaml
# inline (flow) syntax
- name: frontend
  repository: "https://github.com/org/frontend.git"
  local_path: ~/projects/frontend
  tags: [work, js, client-acme]

# block (expanded) syntax
- name: infra
  repository: "git@github.com:org/infra.git"
  local_path: ~/work/infra
  tags:
    - work
    - ops

# no tags — never selected by --tag
- name: dotfiles
  repository: "git@github.com:user/dotfiles.git"
  local_path: ~/dotfiles
```

A project with no `tags` key is **never matched** when `--tag` is active.

Use `--tag` with a comma-separated list for OR logic:

```bash
# only repos tagged "work"
gip status --tag work

# OR: repos tagged "js" or "personal"
gip pull --tag js,personal

# combine with other flags
gip exec --tag work -j 8 -- make build
```

Matching is case-sensitive.

## Config file lookup

gip searches for the config file in this order:

1. The path given by `-f / --file` flag
2. The `GIP_FILE` environment variable
3. `.gip` in the current working directory
4. `~/.gip` (home directory)

The first match wins. Run with `--debug` to see which file is being used.

```bash
# explicit path
gip -f ~/work/gip.yaml status

# per-workspace config picked up automatically
cd ~/work && gip status

# override via environment variable
GIP_FILE=/tmp/test.yaml gip list
```

## Validation warnings

When the config file is loaded, gip validates every entry and prints a warning for each anomaly before executing the command:

- missing or empty `name`
- missing or empty `local_path`
- duplicate `name` values
- unknown `pull_policy` (accepted values: `never`, `always`)
- missing `repository` URL
- repository URL from which the provider cannot be detected

Warnings are printed in yellow and do not block execution. With `--quiet` they are suppressed; with `--json` they appear in the `warnings` array of the output envelope.
