// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/urfave/cli"
)

// outExecEnvrionment is struct for outexec environment
type outExecEnvrionment struct {
	Environment string
	Value       string
}

// localcmd_outexec
// example:
//   - %outexec -n [num] regist command...
func (ps *pShell) buildin_outexec(pline pipeLine, in *io.PipeReader, out *io.PipeWriter, ch chan<- bool, kill chan bool) (err error) {
	// set help text template
	pShellHelptext = `{{.Name}} - {{.Usage}}

	{{.HelpName}} {{if .VisibleFlags}}[options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{end}}
	{{range .VisibleFlags}}	{{.}}
	{{end}}

	Note:
	  This command is a built-in command for lssh.
	  Saves the output of the selected history in the \${LSSH_PSHELL_OUT_{md5(SERVER_NAME)_VALUE}} environment variable and executes the registration command passed in the argument.
	  and \${LSSH_PSHELL_OUT_{md5(SERVER_NAME)_NAME}} in server name.

	example:
	  outexec -n [num] regist command...
	`

	// create app
	app := cli.NewApp()

	// set help message
	app.CustomAppHelpTemplate = pShellHelptext

	// default number
	num := ps.Count - 1

	// set parameter
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "n", Usage: "set history number.", Value: strconv.Itoa(num)},
	}

	app.Name = "outexec"
	app.Usage = "lssh build-in command: outexec [-n num] regist command..."
	app.ArgsUsage = "perm path..."
	app.HideHelp = true
	app.HideVersion = true
	app.EnableBashCompletion = true

	// action
	app.Action = func(c *cli.Context) (aerr error) {
		// show help messages
		if c.Bool("help") {
			cli.ShowAppHelp(c)
			return nil
		}

		// check count args
		if len(c.Args()) < 1 {
			fmt.Fprintln(os.Stderr, "Too few arguments.")
			cli.ShowAppHelp(c)
			return nil
		}

		hnum, aerr := strconv.Atoi(c.String("n"))

		histories := ps.History[hnum]

		// get key
		keys := []string{}
		for k := range histories {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// get environment set
		appendEnv := []outExecEnvrionment{}
		for _, k := range keys {
			h := histories[k]

			key := md5.Sum([]byte(k))

			en := outExecEnvrionment{
				Environment: fmt.Sprintf("LSSH_PSHELL_OUT_%s_NAME", fmt.Sprintf("%x", key)),
				Value:       k,
			}
			appendEnv = append(appendEnv, en)

			ev := outExecEnvrionment{
				Environment: fmt.Sprintf("LSSH_PSHELL_OUT_%s_VALUE", fmt.Sprintf("%x", key)),
				Value:       h.Result,
			}
			appendEnv = append(appendEnv, ev)
		}
		childEnvrionment := genOutExecChildEnv(appendEnv)

		// create pline
		ppline := pipeLine{
			Args: c.Args(),
		}

		// run local command
		err = ps.executeLocalPipeLine(ppline, in, out, ch, kill, childEnvrionment)

		return err
	}

	app.Run(pline.Args)

	return
}

// genChildEnv generate child environment.
func genOutExecChildEnv(env []outExecEnvrionment) (result []string) {
	// If an environment variable is already set, retrieve the old value so you can rollback.
	rollbackEnvMap := map[string]string{}

	for _, e := range env {
		// get old env value
		oldValue := os.Getenv(e.Environment)
		rollbackEnvMap[e.Environment] = oldValue

		// set value
		os.Setenv(e.Environment, e.Value)
	}

	// get result
	result = os.Environ()

	// rollback
	for k, v := range rollbackEnvMap {
		if v == "" {
			_ = os.Unsetenv(k)
		} else {
			_ = os.Setenv(k, v)
		}
	}

	return result
}
