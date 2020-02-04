# Gip

Manage your Git projects.

![CI Linux Mac](https://github.com/enr/gip/workflows/CI%20Linux%20Mac/badge.svg)
![CI Windows](https://github.com/enr/zipp/workflows/CI%20Windows/badge.svg) https://enr.github.io/gip/

Gip reads a JSON file named declaring all the repositories you want to manage.

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
You can set the path using the `-f` flag:

```
gip -f examples/linux.json -d pull -a
```

Otherwise it defaults to the file `.gip` in your home directory.

## Install

The latest release is in: https://github.com/enr/gip/releases/latest

Put `gip` in your `$PATH` and make it executable:

```
$ curl -sL https://github.com/enr/gip/releases/download/v0.4.7/gip-0.4.7_linux_amd64.zip -o gip-0.4.7_linux_amd64.zip
$ unzip gip-0.4.7_linux_amd64.zip
$ cp /tmp/gip-0.4.7_linux_amd64/gip ~/bin/gip
$ gip --version
gip version 0.4.8-dev
Revision: a475d370b57496460b19c8f254cf312d0a5cb979+CHANGES
Build date: 2020-01-18T00:08:51Z

$ gip --help
NAME:
   gip - Keep tracks of your Git projects

USAGE:
   gip [global options] command [command options] [arguments...]

VERSION:
   0.4.8-dev
Revision: a475d370b57496460b19c8f254cf312d0a5cb979+CHANGES
Build date: 2020-01-18T00:08:51Z


COMMANDS:
   status, s       
   statusfull, sf  
   list, ls        
   pull            
   help, h         Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --file value, -f value  path to the configuration file to use (if not set will be picked ${HOME}/.gip)
   --debug, -d             operates in debug mode: lot of output (default: false)
   --quiet, -q             operates in quiet mode (default: false)
   --ignore-missing, -m    ignores missing local directories, otherwise prints a warn (default: false)
   --help, -h              show help (default: false)
   --version, -v           print the version (default: false)

```

List managed projects:

```
$ gip ls
```

Status:

```
$ gip sf
```

Status hiding the warn about missing local directory:

```
$ gip -m sf
```

Verbose output:

```
$ gip -d sf
```

Choose a path other than `~/.gip` for configuration file:

```
$ gip -f ~/.config/local/gip.json -d sf
```

Pull:

```
$ gip pull
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
