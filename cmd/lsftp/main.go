package main

import (
	"os"
)

func main() {
	app := Lsftp()
	app.Run(os.Args)
}
