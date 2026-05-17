---
title: Getting Started
description: Install gip and run your first bulk git operation
weight: 1
---

## Installation

Download the latest binary from [GitHub Releases](https://github.com/enr/gip/releases/latest) and place it on your `$PATH`.

```bash
# Linux / macOS — make it executable
chmod +x gip
sudo mv gip /usr/local/bin/gip

# Verify
gip --version
```

Or build from source (requires Go 1.21+):

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

List all registered projects:

```bash
gip list
```

## Filter by tag

Run any command against a tagged subset:

```bash
# only repos tagged "work"
gip status --tag work

# OR logic — repos tagged "js" or "personal"
gip pull --tag js,personal
```

## Scan existing repos

If you already have repos cloned and want to generate a config file from them:

```bash
gip init ~/projects
# writes to ~/.gip by default
```
