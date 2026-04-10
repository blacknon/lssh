package lssync

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

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
	app.Usage = "TUI list select and parallel sync command over SSH/SFTP."
	app.Copyright = "blacknon(blacknon@orebibou.com)"
	app.Version = version.AppVersion(app.Name)
	app.Flags = []cli.Flag{
		cli.StringSliceFlag{Name: "host,H", Usage: "connect servernames"},
		cli.BoolFlag{Name: "list,l", Usage: "print server list from config"},
		cli.StringFlag{Name: "file,F", Value: defConf, Usage: "config file path"},
		cli.BoolFlag{Name: "daemon,D", Usage: "run as a daemon and repeat sync at each interval"},
		cli.DurationFlag{Name: "daemon-interval", Value: 5 * time.Second, Usage: "daemon sync interval"},
		cli.BoolFlag{Name: "bidirectional,B", Usage: "sync both sides and copy newer changes in either direction"},
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

		sourceSpecs := make([]lsync.PathSpec, 0, len(fromArgs))
		isFromInRemote := false
		isFromInLocal := false
		explicitSourceHosts := []string{}
		for _, from := range fromArgs {
			spec, err := lsync.ParsePathSpec(from)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
			if spec.IsRemote {
				isFromInRemote = true
				explicitSourceHosts = appendUniqueStrings(explicitSourceHosts, spec.Hosts...)
			} else {
				isFromInLocal = true
			}
			sourceSpecs = append(sourceSpecs, spec)
		}

		targetSpec, err := lsync.ParsePathSpec(toArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		isToRemote := targetSpec.IsRemote
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
			if isFromInRemote {
				fromServer = append(fromServer, hosts...)
			} else {
				toServer = append(toServer, hosts...)
			}
		case len(explicitSourceHosts) > 0 || (targetSpec.IsRemote && len(targetSpec.Hosts) > 0):
			fromServer = append(fromServer, explicitSourceHosts...)
			toServer = append(toServer, targetSpec.Hosts...)
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
		for _, spec := range sourceSpecs {
			fromPath := spec.Path
			if !spec.IsRemote {
				if _, err := os.Stat(common.GetFullPath(fromPath)); err != nil {
					fmt.Fprintf(os.Stderr, "not found path %s \n", fromPath)
					os.Exit(1)
				}
				fromPath = common.GetFullPath(fromPath)
			} else {
				fromPath = check.EscapePath(fromPath)
			}

			s.From.IsRemote = spec.IsRemote
			s.From.Path = append(s.From.Path, fromPath)
		}
		s.From.Server = fromServer

		if isToRemote {
			targetSpec.Path = check.EscapePath(targetSpec.Path)
		}
		s.To.IsRemote = isToRemote
		s.To.Path = []string{targetSpec.Path}
		s.To.Server = toServer
		s.Daemon = c.Bool("daemon")
		s.DaemonInterval = c.Duration("daemon-interval")
		s.Bidirectional = c.Bool("bidirectional")
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

func appendUniqueStrings(dst []string, values ...string) []string {
	for _, value := range values {
		exists := false
		for _, current := range dst {
			if current == value {
				exists = true
				break
			}
		}
		if !exists {
			dst = append(dst, value)
		}
	}

	return dst
}
