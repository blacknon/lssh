package main

import (
	"os"

	"github.com/blacknon/lssh/args"
)

func main() {
	app := args.Lscp()
	app.Run(os.Args)
}
