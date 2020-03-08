# Gip

Manage your Git projects.

![CI Linux Mac](https://github.com/enr/gip/workflows/CI%20Linux%20Mac/badge.svg)
![CI Windows](https://github.com/enr/gip/workflows/CI%20Windows/badge.svg) https://enr.github.io/gip/

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

See https://enr.github.io/gip/ for more.

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

**Apache 2.0**

```
Copyright 2020 gip contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```
