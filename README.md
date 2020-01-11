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
$ curl -sL https://github.com/enr/gip/releases/download/v0.4.7/gip-0.4.7_linux_amd64.zip -o gip-0.4.7_linux_amd64.zip
$ unzip gip-0.4.7_linux_amd64.zip
$ cp /tmp/gip-0.4.7_linux_amd64/gip ~/bin/gip
$ gip --version
gip version 0.4.7
Revision: 8ba918924e71c57303fa4679571ec639cc8ba001
Build date: 2020-01-11T17:03:34Z
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
