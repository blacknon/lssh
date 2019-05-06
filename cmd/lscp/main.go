package main

import (
	"os"
)

func main() {
	app := Lscp()
	app.Run(os.Args)
}
