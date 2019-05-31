package ssh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/blacknon/lssh/common"
)

var (
	cmdOPROMPT = "$s :: "
)

func (r *Run) cmd() {
	// make channel
	finished := make(chan bool)

	// print header
	r.printSelectServer()
	r.printRunCommand()
	r.printProxy()

	// print newline
	fmt.Println()

	// create input data channel
	input := make(chan []byte)
	inputWriter := make(chan io.Writer)
	exitInput := make(chan bool)
	defer close(input)

	// create ssh connect
	conns := r.createConn()

	// Create session, Get writer
	for i, conn := range conns {
		c := conn
		count := i

		// create Output
		o := &Output{
			Templete:   cmdOPROMPT,
			Count:      0,
			ServerList: r.ServerList,
			Conf:       r.Conf.Server,
			AutoColor:  true,
		}
		o.Create(c.Server)

		// craete output data channel
		outputChan := make(chan []byte)

		// create session, and run command
		go func() {
			r.cmdRun(c, count, inputWriter, outputChan)
			finished <- true
		}()

		// print command output
		if r.IsParallel || len(conns) == 1 {
			go func() {
				printOutput(o, outputChan)
			}()
		} else {
			// r.cmdPrintOutput(c, count, outputChan)
			printOutput(o, outputChan)
		}
	}

	// wait all finish
	if r.IsParallel || len(r.ServerList) == 1 {
		// create Input
		go func() {
			if len(r.StdinData) == 0 {
				// create MultipleWriter
				writers := []io.Writer{}
				for i := 0; i < len(r.ServerList); i++ {
					writer := <-inputWriter
					writers = append(writers, writer)
				}

				stdinWriter := io.MultiWriter(writers...)
				go pushInput(exitInput, stdinWriter)
			}
		}()

		for i := 0; i < len(r.ServerList); i++ {
			<-finished
		}
	}

	close(exitInput)

	return
}

func (r *Run) cmdRun(conn *Connect, serverListIndex int, inputWriter chan io.Writer, outputChan chan []byte) {
	// create session
	session, err := conn.CreateSession()

	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot connect session %v, %v\n", outColorStrings(serverListIndex, conn.Server), err)
		close(outputChan)
		return
	}

	// set stdin
	if len(r.StdinData) > 0 { // if stdin from pipe
		session.Stdin = bytes.NewReader(r.StdinData)
	} else { // if not stdin from pipe
		if r.IsParallel || len(r.ServerList) == 1 {
			writer, _ := session.StdinPipe()
			inputWriter <- writer
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

func (r *Run) cmdPrintOutput(conn *Connect, serverListIndex int, outputChan chan []byte) {
	serverNameMaxLength := common.GetMaxLength(r.ServerList)

	for data := range outputChan {
		dataStr := strings.TrimRight(string(data), "\n")

		if len(r.ServerList) > 1 {
			lineHeader := fmt.Sprintf("%-*s", serverNameMaxLength, conn.Server)
			fmt.Printf("%s :: %s\n", outColorStrings(serverListIndex, lineHeader), dataStr)
		} else {
			fmt.Printf("%s\n", dataStr)
		}
	}
}
