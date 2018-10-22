package main

import (
	"os"

	"github.com/blacknon/lssh/args"
)

func main() {
	app := args.Lssh()
	app.Run(os.Args)
}
