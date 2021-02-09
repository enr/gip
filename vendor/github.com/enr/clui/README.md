Go Clui
=======

Opinionated, minimalistic and cross platform Go library to manage output of command line apps.

Clui concepts:

- Layout: the output style, eg. plain or machine readable
- VerbosityLevel: how the output is filtered
- Interactivity: if wait for user's answers

Import the library:

```Go
import (
    "github.com/enr/clui"
)
```

Creation of a default `Clui`:

```Go
ui, err := clui.NewClui()
```

Creation with configuration:

```Go
verbosity := func(ui *clui.Clui) {
    ui.VerbosityLevel = clui.VerbosityLevelHigh
}
ui, _ := clui.NewClui(verbosity)
```

See `examples` directory for more.

License
-------

A lot of code of this library was taken from Packer UI released under the same license (Mozilla Public License Version 2.0).

Mozilla Public License Version 2.0 - see LICENSE file.

Copyright 2015 clui contributors
