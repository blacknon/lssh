package lsshell

import (
	"fmt"
	"os"
)

// Run starts the lsshell placeholder command.
func Run() int {
	fmt.Fprintln(os.Stderr, "lsshell is not implemented yet.")
	return 1
}
