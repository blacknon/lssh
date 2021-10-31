// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"bytes"
	"strings"

	"mvdan.cc/sh/syntax"
)

type pipeLine struct {
	Args    []string
	Oprator string
}

// pipeLine return string of join
func (p *pipeLine) String() string {
	result := strings.Join(p.Args, " ")
	result = result + " " + p.Oprator

	return result
}

// joinPipeLineSlice
func joinPipeLineSlice(pslice []pipeLine) string {
	var result string
	for _, pline := range pslice {
		result = result + pline.String()
	}

	return result
}

// joinPipeLine is concatenates a pipe without a built-in command or
// local command as a command to be executed on a remote machine as a string.
func joinPipeLine(pslice []pipeLine) []pipeLine {
	beforeLocal := false
	var bpline pipeLine // before pipeLine
	result := []pipeLine{}

	for _, pline := range pslice {
		// get command
		cmd := pline.Args[0]

		// check in local or build-in command
		isLocal := checkLocalBuildInCommand(cmd)
		switch {
		case isLocal:
			if len(bpline.Args) > 0 {
				result = append(result, bpline)
			}
			bpline = pline
			beforeLocal = true
		case !isLocal && beforeLocal: // RemoteCommand で前がLocalの場合
			if len(bpline.Args) > 0 {
				result = append(result, bpline)
			}
			bpline = pline
			beforeLocal = false
		case !isLocal && !beforeLocal: // RemoteCommandで前がRemoteの場合
			// append bpline
			bpline.Args = append(bpline.Args, bpline.Oprator)
			bpline.Args = append(bpline.Args, pline.Args...)
			bpline.Oprator = pline.Oprator
			beforeLocal = false
		}
	}

	result = append(result, bpline)
	return result
}

// parseCmdPipeLine return [][]pipeLine.
func parsePipeLine(command string) (pslice [][]pipeLine, err error) {
	// Create result pipeLineSlice
	pslice = [][]pipeLine{}

	// Create parser
	in := strings.NewReader(command)
	f, err := syntax.NewParser().Parse(in, " ")
	if err != nil {
		return
	}

	// parse stmt
	for _, stmt := range f.Stmts {
		// create slice
		var cmdLine []pipeLine

		// create stmtCmd, stmtRedirs
		var stmtCmd syntax.Command
		var stmtRedirs []*syntax.Redirect
		stmtCmd = stmt.Cmd
		stmtRedirs = stmt.Redirs

		// parse stmt loop
	stmtCmdLoop:
		for {
			switch c := stmtCmd.(type) {
			case *syntax.CallExpr:
				args := parseCallExpr(c)

				args = append(args, parseRedirect(stmtRedirs)...)
				pLine := pipeLine{
					Args: args,
				}
				cmdLine = append(cmdLine, pLine)

				break stmtCmdLoop
			case *syntax.BinaryCmd:
				switch c.X.Cmd.(type) {
				case *syntax.CallExpr:
					cx := c.X.Cmd.(*syntax.CallExpr)
					cxr := c.X.Redirs

					args := parseCallExpr(cx)
					args = append(args, parseRedirect(cxr)...)

					pLine := pipeLine{
						Args:    args,
						Oprator: c.Op.String(),
					}
					cmdLine = append(cmdLine, pLine)
					stmtCmd = c.Y.Cmd
					stmtRedirs = c.Y.Redirs

				case *syntax.BinaryCmd: // TODO(blacknon): &&や||に対応させる(対処方法がわからん…)
					stmtCmd = c.X.Cmd
					stmtRedirs = c.X.Redirs
				}
			}
		}

		pslice = append(pslice, cmdLine)
	}

	return
}

// parseCallExpr return pipeline element ([]string).
func parseCallExpr(cmd *syntax.CallExpr) (pLine []string) {
	printer := syntax.NewPrinter()

	for _, arg := range cmd.Args {
		for _, part := range arg.Parts {
			buf := new(bytes.Buffer)
			printer.Print(buf, part)
			pLine = append(pLine, buf.String())

		}
	}
	return
}

// parseRedirect return pipeline redirect element ([]string)
func parseRedirect(redir []*syntax.Redirect) (rs []string) {
	printer := syntax.NewPrinter()

	for _, r := range redir {
		var rr string
		if r.N != nil {
			rr = rr + r.N.Value
		}
		rr = rr + r.Op.String()
		for _, part := range r.Word.Parts {
			buf := new(bytes.Buffer)
			printer.Print(buf, part)
			rr = rr + buf.String()
		}

		rs = append(rs, rr)
	}

	return
}
