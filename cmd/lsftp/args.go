package main

import (
	"os/user"

	"github.com/urfave/cli"
)

func Lsftp() (app *cli.App) {
	// Default config file path
	usr, _ := user.Current()
	defConf := usr.HomeDir + "/.lssh.conf"

	// TODO: 作成
}
