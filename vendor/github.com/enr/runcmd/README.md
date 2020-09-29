Runcmd
======

![CI Linux](https://github.com/enr/runcmd/workflows/CI%20Nix/badge.svg)
![CI Windows](https://github.com/enr/runcmd/workflows/CI%20Windows/badge.svg)

Should be a Go library to execute external commands.

Import the library:

```Go
import (
    "github.com/enr/runcmd"
)
```

You can use this library in two ways:

- Run
- Start

`Run` starts the specified command, waits for it to complete and returns a result complete of `stdout` and `stderr`:

```Go
executable := "/usr/bin/ls"
args := []string{"-al"}
command := &runcmd.Command{
    Exe:  executable,
    Args: args,
}
res := command.Run()
if res.Success() {
    fmt.Printf("standard output: %s", res.Stdout().String())
} else {
    fmt.Printf("error executing %s. Exit code %d", command, res.ExitStatus())
    fmt.Printf("error output: %s", res.Stderr().String())
    fmt.Printf("the error: %v", res.Error())
}
```

`Start` starts the specified command but does not wait for it to complete.

```Go
executable := "/usr/local/bin/start-server"
command := &runcmd.Command{
    Exe:  executable,
}
logFile := command.GetLogfile()
// maybe you want to follow logs...
t, _ := tail.TailFile(logFile, tail.Config{Follow: true})
command.Start()
runningProcess := command.Process
```

License
-------

Apache 2.0 - see LICENSE file.

Copyright 2020 runcmd contributors
