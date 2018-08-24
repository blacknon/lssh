package ssh

import (
	"bytes"
	"fmt"
	"os"

	"github.com/blacknon/lssh/common"
)

func (r *Run) cmd() {
	serverNameMaxLength := common.GetMaxLength(r.ServerList)
	finished := make(chan bool)

	// print header
	r.printSelectServer()
	r.printRunCommand()
	r.printProxy()
	fmt.Println() // print newline

	for i, server := range r.ServerList {
		c := new(Connect)
		c.Server = server
		c.Conf = r.Conf
		c.IsTerm = r.IsTerm
		c.IsParallel = r.IsParallel
		serverListIndex := i

		// run command
		outputChan := make(chan string)
		go func(outputChan chan string) {
			// create session
			session, err := c.CreateSession()
			if err != nil {
				go func() {
					fmt.Fprintf(os.Stderr, "cannot connect session %v, %v\n", outColorStrings(serverListIndex, c.Server), err)
				}()
				close(outputChan)
				return
			}

			session.Stdin = bytes.NewReader(r.StdinData)

			c.RunCmdGetOutput(session, r.ExecCmd, outputChan)
			close(outputChan)
		}(outputChan)

		// get command output
		go func(outputChan chan string) {
			for outputLine := range outputChan {
				if len(r.ServerList) > 1 {
					lineHeader := fmt.Sprintf("%-*s", serverNameMaxLength, c.Server)
					fmt.Println(outColorStrings(serverListIndex, lineHeader)+" :: ", outputLine)
				} else {
					fmt.Println(outputLine)
				}
			}
			finished <- true
		}(outputChan)
	}

	// wait all finish
	for i := 1; i <= len(r.ServerList); i++ {
		<-finished
	}

	return
}

func outColorStrings(num int, inStrings string) (str string) {
	// 1=Red,2=Yellow,3=Blue,4=Magenta,0=Cyan
	color := 31 + num%5
	str = fmt.Sprintf("\x1b[%dm%s\x1b[0m", color, inStrings)
	return
}
