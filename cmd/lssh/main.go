package main

import (
	"os"
)

func main() {
	app := Lssh()
	app.Run(os.Args)
}
