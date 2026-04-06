tvxterm
===

<p align="center">
  <img src="./img/tvxterm.gif" width="77%" />
</p>

`tvxterm` is a terminal emulator widget for [tview](https://github.com/rivo/tview).

It gives you a small VT-style emulator plus a `tview.Primitive`, so you can embed an interactive terminal inside terminal UIs written in Go.

## Features

- `tview` primitive for rendering terminal output
- PTY backend for local shell and command execution
- stream backend for SSH sessions or custom transports
- UTF-8, wide rune, combining rune, and scrollback support
- terminal input handling for keyboard, paste, focus, and mouse reporting
- OSC title tracking and callback hooks

## Package Layout

The package name is `tvxterm`, and it can be imported directly from the repository root.

```go
import "github.com/blacknon/tvxterm"
```

## Installation

```bash
go get github.com/blacknon/tvxterm
```

## Quick Start

```go
package main

import (
	"log"
	"os"
	"os/exec"

	"github.com/rivo/tview"

	"github.com/blacknon/tvxterm"
)

func main() {
	app := tview.NewApplication()
	term := tvxterm.New(app)

	cmd := exec.Command(shell())
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	backend, err := tvxterm.NewPTYBackend(cmd, 80, 24)
	if err != nil {
		log.Fatal(err)
	}
	defer backend.Close()

	term.Attach(backend)

	if err := app.SetRoot(term, true).SetFocus(term).Run(); err != nil {
		log.Fatal(err)
	}
}

func shell() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/sh"
}
```

## Backends

### PTY backend

Use `NewPTYBackend` when you want to run a local shell or process inside a pseudo terminal.

```go
cmd := exec.Command("/bin/sh")
backend, err := tvxterm.NewPTYBackend(cmd, 80, 24)
```

### Stream backend

Use `NewStreamBackend` when another library already owns the session and only gives you `io.Reader` / `io.Writer`.

```go
backend := tvxterm.NewStreamBackend(reader, writer, resizeFunc, closeFunc)
term.Attach(backend)
```

This is useful for:

- SSH clients
- container exec sessions
- remote agent transports
- custom REPL bridges

## Example

A runnable sample is included:

```bash
go run ./sample/localpty
```

What the sample shows:

- local shell execution through `PTYBackend`
- window title updates from OSC sequences
- status line updates
- local scrollback control with `PageUp` / `PageDown`
- exit with `Ctrl+Q`

Additional samples:

```bash
go run ./sample/muxpty
go run ./sample/go-sshlib-shell
```

- `sample/muxpty`: terminal mux style sample with dynamic pane splitting
  `Ctrl+A c` creates a new page, `Ctrl+A %` / `Ctrl+A "` splits the current page, `Ctrl+A n/p` switches pages, `Ctrl+A o` moves pane focus, `Ctrl+A w` opens the page list
- `sample/go-sshlib-shell`: `go-sshlib` session rendered inside a `tvxterm` view via `StreamBackend`
  `Ctrl+A c` creates a new page, `Ctrl+A %` / `Ctrl+A "` splits a new SSH pane in the current page, `Ctrl+A n/p` switches pages, `Ctrl+A o` moves focus, `Ctrl+A w` opens the page list

For `sample/go-sshlib-shell`, you can use flags:

```bash
go run ./sample/go-sshlib-shell \
  -host example.com \
  -port 22 \
  -user myuser \
  -key-path $HOME/.ssh/id_ed25519 \
  -key-passphrase ''
```

Or password authentication:

```bash
go run ./sample/go-sshlib-shell \
  -host example.com \
  -port 22 \
  -user myuser \
  -password mypassword
```

Environment variables `TVXTERM_SSH_HOST`, `TVXTERM_SSH_PORT`, `TVXTERM_SSH_USER`, `TVXTERM_SSH_PASSWORD`, `TVXTERM_SSH_KEY_PATH`, and `TVXTERM_SSH_KEY_PASSPHRASE` can also be used as defaults.

## API Overview

The main exported pieces are:

- `View`: the `tview` widget
- `Backend`: transport abstraction for terminal I/O
- `PTYBackend`: local PTY-backed process launcher
- `StreamBackend`: adapter for generic streams
- `Emulator`: exported terminal state machine for testing or advanced use

Useful hooks on `View`:

- `Attach(backend)` to connect a session
- `SetBackendExitHandler(...)` to react when the backend closes
- `SetTitleHandler(...)` to receive title updates
- `ScrollbackUp`, `ScrollbackDown`, `ScrollbackPageUp`, `ScrollbackPageDown`

## Notes

- This library is focused on embedding terminal sessions into `tview` applications.
- The emulator is intentionally small rather than a full xterm clone.
- Alternate screen, colors, mouse reporting, bracketed paste, and title handling are supported, but some terminal features may still be incomplete.

## License

MIT. See [LICENSE](./LICENSE).
