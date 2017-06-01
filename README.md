# Gip

Manage your Git projects.

Gip reads a JSON file named `.gip` in your home directory.

```json
[
  {
    "Name": "gip",
    "Repository": "https://github.com/enr/gip.git",
    "LocalPath": "~/Projects/gip"
  },
  {
    "Name": "...",
    "Repository": "...",
    "LocalPath": "..."
  }
]
```

## Install

Put `gip` in your `$PATH` and make it executable:

```
$ curl -sL https://github.com/enr/gip/releases/download/v0.4.1/gip-linux-amd64 -o ~/bin/gip
$ chmod +x ~/bin/gip
$ gip --version
gip version 0.4.1
Revision: d0fd984ff7f3140f5f68bb3c6217c337d071d80f
Build date: 2017-05-27T09:36:12Z
```

## License

**Apache 2.0**

```
Copyright 2014 gip contributors

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
