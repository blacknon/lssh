package ssh

type pipeLine struct {
	Args    []string
	Oprator string
}

// //
// func (ps *pShell) parseCmdPipeLine() (pLine []string) {

// }

//
// func parseCallExpr(cmd *syntax.CallExpr) (pLine []string) {
// 	printer := syntax.NewPrinter()

// 	for _, arg := range cmd.Args {
// 		for _, part := range arg.Parts {
// 			buf := new(bytes.Buffer)
// 			printer.Print(buf, part)
// 			pLine = append(pLine, buf.String())

// 			switch p := part.(type) {
// 			case *syntax.ProcSubst:
// 				fmt.Printf("%s: %s\n", buf.String(), "*syntax.ProcSubst")
// 				fmt.Println(p.Op.String())
// 				fmt.Println(p.Stmts)
// 			case *syntax.CmdSubst:
// 				fmt.Printf("%s: %s\n", buf.String(), "*syntax.CmdSubst")
// 				fmt.Println(p.Stmts)
// 			}
// 		}
// 	}
// 	return
// }
