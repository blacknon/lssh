package lssync

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/list"
	lsync "github.com/blacknon/lssh/internal/sync"
	"github.com/blacknon/lssh/internal/version"
	"github.com/urfave/cli"
)

func Lssync() (app *cli.App) {
	defConf := common.GetDefaultConfigPath()

	cli.AppHelpTemplate = `NAME:
    {{.Name}} - {{.Usage}}
USAGE:
    {{.HelpName}} {{if .VisibleFlags}}[options]{{end}} (local|remote):from_path... (local|remote):to_path
    {{if len .Authors}}
AUTHOR:
    {{range .Authors}}{{ . }}{{end}}
    {{end}}{{if .Commands}}
COMMANDS:
    {{range .Commands}}{{if not .HideHelp}}{{join .Names ", "}}{{ "\t"}}{{.Usage}}{{ "\n" }}{{end}}{{end}}{{end}}{{if .VisibleFlags}}
OPTIONS:
    {{range .VisibleFlags}}{{.}}
    {{end}}{{end}}{{if .Copyright }}
COPYRIGHT:
    {{.Copyright}}
    {{end}}{{if .Version}}
VERSION:
    {{.Version}}
    {{end}}
USAGE:
    # local to remote sync
    {{.Name}} /path/to/local... remote:/path/to/remote

    # remote to local sync
    {{.Name}} remote:/path/to/remote... /path/to/local

    # remote to remote sync
    {{.Name}} remote:/path/to/remote... remote:/path/to/local
`

	app = cli.NewApp()
	app.Name = "lssync"
	app.Usage = "TUI list select and parallel one-way sync command over SSH/SFTP."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = version.AppVersion(app.Name)
	app.Flags = []cli.Flag{
		cli.StringSliceFlag{Name: "host,H", Usage: "connect servernames"},
		cli.BoolFlag{Name: "list,l", Usage: "print server list from config"},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config file path"},
		cli.IntFlag{Name: "parallel,P", Value: 1, Usage: "parallel file sync count per host"},
		cli.BoolFlag{Name: "permission,p", Usage: "copy file permission"},
		cli.BoolFlag{Name: "delete", Usage: "delete destination entries that do not exist in source"},
		cli.BoolFlag{Name: "help,h", Usage: "print this help"},
	}
	app.EnableBashCompletion = true
	app.HideHelp = true

	app.Action = func(c *cli.Context) error {
		if c.Bool("help") {
			cli.ShowAppHelp(c)
			os.Exit(0)
		}

		hosts := c.StringSlice("host")
		confpath := c.String("file")
		if len(c.Args()) < 2 {
			fmt.Fprintln(os.Stderr, "Too few arguments.")
			cli.ShowAppHelp(c)
			os.Exit(1)
		}

		fromArgs := c.Args()[:c.NArg()-1]
		toArg := c.Args()[c.NArg()-1]

		isFromInRemote := false
		isFromInLocal := false
		for _, from := range fromArgs {
			isFromRemote, _ := check.ParseScpPath(from)
			if isFromRemote {
				isFromInRemote = true
			} else {
				isFromInLocal = true
			}
		}
		isToRemote, _ := check.ParseScpPath(toArg)
		check.CheckTypeError(isFromInRemote, isFromInLocal, isToRemote, len(hosts))

		data := conf.Read(confpath)
		names := conf.GetNameList(data)
		sort.Strings(names)

		if c.Bool("list") {
			fmt.Fprintf(os.Stdout, "lssh Server List:\n")
			for _, name := range names {
				fmt.Fprintf(os.Stdout, "  %s\n", name)
			}
			os.Exit(0)
		}

		selected := []string{}
		toServer := []string{}
		fromServer := []string{}
		switch {
		case len(hosts) != 0:
			if !check.ExistServer(hosts, names) {
				fmt.Fprintln(os.Stderr, "Input Server not found from list.")
				os.Exit(1)
			}
			toServer = hosts
		case isFromInRemote && isToRemote:
			fromList := new(list.ListInfo)
			fromList.Prompt = "lssync(from)>>"
			fromList.NameList = names
			fromList.DataList = data
			fromList.MultiFlag = false
			fromList.View()
			fromServer = fromList.SelectName
			if len(fromServer) == 0 || fromServer[0] == "ServerName" {
				fmt.Fprintln(os.Stderr, "Server not selected.")
				os.Exit(1)
			}

			toList := new(list.ListInfo)
			toList.Prompt = "lssync(to)>>"
			toList.NameList = names
			toList.DataList = data
			toList.MultiFlag = true
			toList.View()
			toServer = toList.SelectName
			if len(toServer) == 0 || toServer[0] == "ServerName" {
				fmt.Fprintln(os.Stderr, "Server not selected.")
				os.Exit(1)
			}
		default:
			l := new(list.ListInfo)
			l.Prompt = "lssync>>"
			l.NameList = names
			l.DataList = data
			l.MultiFlag = true
			l.View()
			selected = l.SelectName
			if len(selected) == 0 || selected[0] == "ServerName" {
				fmt.Fprintln(os.Stderr, "Server not selected.")
				os.Exit(1)
			}

			if isFromInRemote {
				fromServer = selected
			} else {
				toServer = selected
			}
		}

		s := new(lsync.Sync)
		for _, from := range fromArgs {
			isFromRemote, fromPath := check.ParseScpPath(from)
			if !isFromRemote {
				if _, err := os.Stat(common.GetFullPath(fromPath)); err != nil {
					fmt.Fprintf(os.Stderr, "not found path %s \n", fromPath)
					os.Exit(1)
				}
				fromPath = common.GetFullPath(fromPath)
			} else {
				fromPath = check.EscapePath(fromPath)
			}

			s.From.IsRemote = isFromRemote
			s.From.Path = append(s.From.Path, fromPath)
		}
		s.From.Server = fromServer

		isToRemote, toPath := check.ParseScpPath(toArg)
		if isToRemote {
			toPath = check.EscapePath(toPath)
		}
		s.To.IsRemote = isToRemote
		s.To.Path = []string{toPath}
		s.To.Server = toServer
		s.Permission = c.Bool("permission")
		s.Delete = c.Bool("delete")
		s.ParallelNum = c.Int("parallel")
		s.Config = data

		if !isFromInRemote {
			fmt.Fprintf(os.Stderr, "From local:%s\n", s.From.Path)
		} else {
			fmt.Fprintf(os.Stderr, "From remote(%s):%s\n", strings.Join(s.From.Server, ","), s.From.Path)
		}
		if !isToRemote {
			fmt.Fprintf(os.Stderr, "To   local:%s\n", s.To.Path)
		} else {
			fmt.Fprintf(os.Stderr, "To   remote(%s):%s\n", strings.Join(s.To.Server, ","), s.To.Path)
		}

		s.Start()
		return nil
	}

	return app
}
