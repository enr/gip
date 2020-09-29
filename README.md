# Gip

Manage your Git projects.

![CI Linux Mac](https://github.com/enr/gip/workflows/CI%20Linux%20Mac/badge.svg)
![CI Windows](https://github.com/enr/gip/workflows/CI%20Windows/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/enr/gip)](https://goreportcard.com/report/github.com/enr/gip)
[![Documentation](https://img.shields.io/badge/Website-Documentation-orange)](https://enr.github.io/gip/)

Gip run Git commands on a set of repositories.

Repositories can be registered in a YAML or JSON file, eg:

```json
[
  {
    "name": "gip",
    "repository": "https://github.com/enr/gip.git",
    "local_path": "/tmp/gip"
  },
  {
    "name": "nowhere",
    "repository": "https://nowhere.git",
    "local_path": "/nowhere/not/found",
    "pull_policy": "never"
  }
]
```

```yaml
- name: gip
  repository: "https://github.com/enr/gip.git"
  local_path: /tmp/gip
- name: nowhere
  repository: "https://nowhere.git"
  local_path: /nowhere/not/found
  pull_policy: never
```

Available commands:

- status
- pull
- list

See [website](https://enr.github.io/gip/) for more.

## Development

Download or clone repository.

Build (binaries will be created in `bin/`):

```
./.sdlc/build
```

Check (code quality and tests);

```
./.sdlc/check
```

## License

Apache 2.0 - see LICENSE file.

Copyright 2020 runcmd contributors
