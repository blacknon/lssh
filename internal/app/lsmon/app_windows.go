//go:build windows

package lsmon

import (
	"fmt"
	"os"

	"github.com/blacknon/lssh/internal/version"
	"github.com/urfave/cli"
)

func Lsmon() (app *cli.App) {
	app = cli.NewApp()
	app.Name = "lsmon"
	app.Usage = "TUI list select and parallel ssh monitoring command."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = version.AppVersion("lsmon")
	app.HideHelp = true
	app.EnableBashCompletion = true

	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "help,h", Usage: "print this help"},
	}

	app.Action = func(c *cli.Context) error {
		if c.Bool("help") {
			cli.ShowAppHelp(c)
			return nil
		}

		fmt.Fprintln(os.Stderr, "lsmon is not supported on Windows.")
		return nil
	}

	return app
}
