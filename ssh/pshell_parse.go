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

// joinPipeLine is concatenates a pipe without a built-in command or
// local command as a command to be executed on a remote machine as a string.
// TODO(blacknon): 作成する。
//
// func joinPipeLine(pslice [][]pipeline) ([][]pipeLine, error) {
// }

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

		// create stmtCmd
		var stmtCmd syntax.Command
		stmtCmd = stmt.Cmd

		// parse stmt loop
	stmtCmdLoop:
		for {
			switch c := stmtCmd.(type) {
			case *syntax.CallExpr:
				pLine := pipeLine{
					Args: parseCallExpr(c),
				}
				cmdLine = append([]pipeLine{pLine}, cmdLine...)

				break stmtCmdLoop
			case *syntax.BinaryCmd:
				pLine := pipeLine{
					Args:    parseCallExpr(c.Y.Cmd.(*syntax.CallExpr)),
					Oprator: c.Op.String(),
				}
				cmdLine = append([]pipeLine{pLine}, cmdLine...)

				stmtCmd = c.X.Cmd
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
