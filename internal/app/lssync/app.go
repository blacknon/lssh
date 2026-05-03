package lssync

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/blacknon/lssh/internal/app/apputil"
	"github.com/blacknon/lssh/internal/common"
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
		cli.StringFlag{Name: "generate-lssh-conf", Usage: "print generated lssh config from OpenSSH config to stdout (`~/.ssh/config` by default)."},
		cli.BoolFlag{Name: "daemon,D", Usage: "run as a daemon and repeat sync at each interval"},
		cli.DurationFlag{Name: "daemon-interval", Value: 5 * time.Second, Usage: "daemon sync interval"},
		cli.BoolFlag{Name: "bidirectional,B", Usage: "sync both sides and copy newer changes in either direction"},
		cli.IntFlag{Name: "parallel,P", Value: 1, Usage: "parallel file sync count per host"},
		cli.BoolFlag{Name: "permission,p", Usage: "copy file permission"},
		cli.BoolFlag{Name: "dry-run", Usage: "show sync actions without modifying files"},
		cli.BoolFlag{Name: "delete", Usage: "delete destination entries that do not exist in source"},
		cli.BoolFlag{Name: "help,h", Usage: "print this help"},
	}
	app.Flags = append(app.Flags, common.ControlMasterOverrideFlags()...)
	app.EnableBashCompletion = true
	app.HideHelp = true

	app.Action = func(c *cli.Context) error {
		if c.Bool("help") {
			cli.ShowAppHelp(c)
			return nil
		}

		hosts := c.StringSlice("host")
		controlMasterOverride, controlMasterErr := common.GetControlMasterOverride(c)
		if controlMasterErr != nil {
			return controlMasterErr
		}
		data, handled, err := apputil.LoadConfigWithGenerateMode(c, os.Stdout, os.Stderr)
		if handled {
			return err
		}
		if err != nil {
			return err
		}
		allNames, names, err := apputil.SortedServerNames(data, "sftp_transport")
		if err != nil {
			return err
		}

		if c.Bool("list") {
			apputil.PrintServerList(os.Stdout, names)
			return nil
		}

		if len(c.Args()) < 2 {
			return fmt.Errorf("Too few arguments.")
		}

		fromArgs := c.Args()[:c.NArg()-1]
		toArg := c.Args()[c.NArg()-1]

		sourceSpecs, targetSpec, isFromInRemote, isFromInLocal, explicitSourceHosts, err := parseSyncSpecs(fromArgs, toArg, allNames)
		if err != nil {
			return err
		}
		isToRemote := targetSpec.IsRemote
		if err := apputil.ValidateTransferCopyTypes(isFromInRemote, isFromInLocal, isToRemote, len(hosts)); err != nil {
			return err
		}

		fromServer, toServer, err := selectSyncServers(hosts, names, allNames, data, explicitSourceHosts, targetSpec, isFromInRemote, isToRemote, apputil.PromptServerSelection)
		if err != nil {
			return err
		}

		s, err := buildSync(c, data, sourceSpecs, targetSpec, fromServer, toServer, isToRemote, controlMasterOverride)
		if err != nil {
			return err
		}

		if !isFromInRemote {
			fmt.Fprintf(os.Stderr, "From local:%s\n", s.From.DisplayPath)
		} else {
			fmt.Fprintf(os.Stderr, "From remote(%s):%s\n", strings.Join(s.From.Server, ","), s.From.Path)
		}
		if !isToRemote {
			fmt.Fprintf(os.Stderr, "To   local:%s\n", s.To.DisplayPath)
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
