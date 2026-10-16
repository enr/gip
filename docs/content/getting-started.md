---
title: Getting Started
description: Install gip and run your first bulk git operation
weight: 1
---

## Installation

Download the archive for your platform (`gip-<version>_<os>_<arch>.zip`) from [GitHub Releases](https://github.com/enr/gip/releases/latest), extract it and place the `gip` binary (`gip.exe` on Windows) on your `$PATH`.

```bash
# Linux / macOS
unzip gip-*_linux_amd64.zip
chmod +x gip-*/gip
sudo mv gip-*/gip /usr/local/bin/gip

# Verify
gip --version
```

Or build from source (requires Go 1.25+):

```bash
git clone https://github.com/enr/gip
cd gip
./.sdlc/build
# binary is placed in ./bin/gip
```

## Create a config file

Create `~/.gip` (YAML or JSON) listing the repositories you want to manage:

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

See [Configuration](/gip/configuration/) for all available fields.

## First commands

Check the status of all repos:

```bash
gip status
```

Pull the latest changes:

```bash
gip pull
```

See which branch each repo is on:

```bash
gip branch
```

List all registered projects with their branch and sync status:

```bash
gip list
```

## Get a richer overview

Add `--extended` to `branch` for a dashboard view with sync status, dirty indicators and last commit info (on `list`, `--extended` adds the DIRTY column):

```bash
gip branch --extended
gip list --extended
```

```text
NAME        BRANCH    STATUS    DIRTY  COMMIT                    DATE        PATH
frontend    main      synced    —      chore: bump deps          3 days ago  ~/projects/frontend
backend     dev       ↑2        +*     feat: add auth endpoint   2 hours ago ~/projects/backend
dotfiles    main      ↓1        —      docs: update README       1 week ago  ~/dotfiles
```

STATUS colour (TTY only): green = synced · magenta = ahead · yellow = behind · red = diverged · white = no remote  
DIRTY symbols: `+` staged · `*` unstaged · `?` untracked · `$` stash

## Focus on repos that need attention

Use `--dirty`, `--behind`, `--ahead` or `--unsynced` to limit a command to the repos that match:

```bash
# only repos with uncommitted changes
gip status --dirty
gip branch --dirty --extended

# only repos behind their upstream (run fetch first)
gip fetch && gip pull --behind
```

`pull` skips repos with uncommitted changes unless `--force` is given, so `gip pull --dirty` requires `--force`.

## Find projects to release

List the projects with commits after their latest tag (or never tagged):

```bash
gip fetch && gip releases --pending
```

```text
NAME      TAG     COMMITS  TAG DATE     BRANCH  PATH
backend   v1.4.0  7        3 weeks ago  main    ~/projects/backend
dotfiles  (none)  42       —            main    ~/dotfiles
```

See [releases](/gip/commands/#releases) for how the latest tag is chosen.

## Filter by tag

Run any command against a tagged subset:

```bash
# only repos tagged "work"
gip status --tag work

# OR logic — repos tagged "js" or "personal"
gip pull --tag js,personal
gip pull --tag js --tag personal   # same
```

## Scan existing repos

If you already have repos cloned and want to generate a config file from them:

```bash
gip init ~/projects
# writes .gip in the current directory by default
gip init ~/projects --output ~/.gip
```

See [init](/gip/commands/#init) for details.

## Shell completion

```bash
source <(gip completion bash)   # or: zsh
gip completion fish > ~/.config/fish/completions/gip.fish
```
