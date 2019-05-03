package ssh

import (
	"bytes"
	"fmt"
	"os"

	"github.com/blacknon/lssh/common"
)

func (r *Run) cmd() {
	// make channel
	finished := make(chan bool)

	// if IsTerminal, Get input data
	// @CommentOut 20190503
	// if terminal.IsTerminal(syscall.Stdin) {
	// 	go r.getInputFromStdin()
	// }

	// print header
	r.printSelectServer()
	r.printRunCommand()
	r.printProxy()

	// print newline
	fmt.Println()

	// @TODO: add get input in buffer

	for i, server := range r.ServerList {
		count := i

		c := new(Connect)
		c.Server = server
		c.Conf = r.Conf
		c.IsTerm = r.IsTerm
		c.IsParallel = r.IsParallel

		// run command
		outputChan := make(chan string)
		go r.cmdRun(c, i, outputChan)

		// print command output
		if r.IsParallel {
			go func() {
				r.cmdPrintOutput(c, count, outputChan)
				finished <- true
			}()
		} else {
			r.cmdPrintOutput(c, count, outputChan)
		}

	}

	// wait all finish
	if r.IsParallel {
		for i := 1; i <= len(r.ServerList); i++ {
			<-finished
		}
	}

	return
}

func (r *Run) cmdRun(conn *Connect, serverListIndex int, outputChan chan string) {
	// create session
	session, err := conn.CreateSession()

	if err != nil {
		go func() {
			fmt.Fprintf(os.Stderr, "cannot connect session %v, %v\n", outColorStrings(serverListIndex, conn.Server), err)
		}()
		close(outputChan)

		return
	}

	// set stdin
	if len(r.StdinData) > 0 { // if stdin from pipe
		session.Stdin = bytes.NewReader(r.StdinData)
	} else { // if not stdin from pipe
		// @TODO:
		//     os.Stdinをそのまま渡すのだとだめなので、一度bufferに書き出してから各Sessionに書き出す必要がある。
		//     なので、Structとかもっと上位に入力を受け付ける代物を入れる必要があるので注意。もしくはchannelかな？？
		// @CommentOut 20190503
		// go r.putInputToSession(session)

		if len(r.ServerList) == 1 {
			session.Stdin = os.Stdin
		}
	}

	// run command and get output data to outputChan
	isExit := make(chan bool)
	go func() {
		conn.RunCmdWithOutput(session, r.ExecCmd, outputChan)
		isExit <- true
	}()

	select {
	case <-isExit:
		close(outputChan)
	}
}

func (r *Run) cmdPrintOutput(conn *Connect, serverListIndex int, outputChan chan string) {
	serverNameMaxLength := common.GetMaxLength(r.ServerList)

	for outputLine := range outputChan {
		if len(r.ServerList) > 1 {
			lineHeader := fmt.Sprintf("%-*s", serverNameMaxLength, conn.Server)
			fmt.Println(outColorStrings(serverListIndex, lineHeader)+" :: ", outputLine)
		} else {
			fmt.Println(outputLine)
		}
	}

}

// get input data
// @CommentOut 20190503
// func (r *Run) getInputFromStdin() {
// 	stdin := bufio.NewScanner(os.Stdin)

// 	for {
// 		for stdin.Scan() {
// 			data := stdin.Bytes()
// 			r.InputData = append(r.InputData, data...)
// 		}
// 		time.Sleep(10 * time.Millisecond)
// 	}
// }

// @CommentOut 20190503
// func (r *Run) putInputToSession(session *ssh.Session) {
// 	stdin, _ := session.StdinPipe()
// 	i := 0
// 	for {
// 		size := len(r.InputData)
// 		if len(r.InputData) > i {
// 			// fmt.Println(string(r.InputData[i:size]))
// 			fmt.Fprint(stdin, string(r.InputData[i:size]))
// 			// session.Stdin.Read(r.InputData[i:size])
// 			i = size
// 		}
// 		time.Sleep(10 * time.Millisecond)
// 	}
// }

func outColorStrings(num int, inStrings string) (str string) {
	// 1=Red,2=Yellow,3=Blue,4=Magenta,0=Cyan
	color := 31 + num%5

	str = fmt.Sprintf("\x1b[%dm%s\x1b[0m", color, inStrings)
	return
}
