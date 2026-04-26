package ssh

import (
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

func runNativeInteractiveSession(run func() error) error {
	fd := int(os.Stdin.Fd())
	if !terminal.IsTerminal(fd) {
		return run()
	}

	state, err := terminal.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer func() {
		_ = terminal.Restore(fd, state)
	}()

	return run()
}
