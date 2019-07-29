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

// parseCmdPipeLine
func (ps *pShell) parsePipeLine(command string) (pmap map[int][]pipeLine, err error) {
	// Create result map
	pmap = map[int][]pipeLine{}

	// Create parser
	in := strings.NewReader(command)
	f, err := syntax.NewParser().Parse(in, " ")
	if err != nil {
		return
	}

	// parse stmt
	for i, stmt := range f.Stmts {
		// create slice
		var cmdLine []pipeLine

		// パースするコマンドを変数につっこむ
		var stmtCmd syntax.Command
		stmtCmd = stmt.Cmd

		// ひとまず、ワンライナーのパイプラインの識別はできそう？？
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

		pmap[i] = cmdLine
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
