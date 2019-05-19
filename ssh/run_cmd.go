package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/blacknon/lssh/common"
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
	exitInputGet := make(chan bool)
	exitInputPut := make(chan bool)
	defer close(input)

	// conns := []*Connect{}
	conns := r.createConn()

	// Create session, Get writer
	for i, conn := range conns {
		c := conn
		// for i, server := range r.ServerList {
		count := i

		// c := new(Connect)
		// c.Server = server
		// c.Conf = r.Conf
		// c.IsTerm = r.IsTerm
		// c.IsParallel = r.IsParallel
		// conns = append(conns, c)

		// craete output data channel
		outputChan := make(chan []byte)

		// create session, and run command
		go r.cmdRun(c, count, inputWriter, outputChan)

		// print command output
		if r.IsParallel || len(conns) == 1 {
			go func() {
				r.cmdPrintOutput(c, count, outputChan)
				finished <- true
			}()
		} else {
			r.cmdPrintOutput(c, count, outputChan)
		}
	}

	// wait all finish
	if r.IsParallel || len(r.ServerList) == 1 {
		// create Input
		if len(r.StdinData) == 0 {
			// create MultipleWriter
			writers := []io.Writer{}
			for i := 0; i < len(r.ServerList); i++ {
				writer := <-inputWriter
				writers = append(writers, writer)
			}

			stdinWriter := io.MultiWriter(writers...)
			go getInputFromStdin(input, exitInputGet)
			go putInputToSession(stdinWriter, input, exitInputPut)
		}

		for i := 0; i < len(r.ServerList); i++ {
			<-finished
		}
	}

	close(exitInputGet)
	close(exitInputPut)
	return
}

func (r *Run) cmdRun(conn *Connect, serverListIndex int, inputWriter chan io.Writer, outputChan chan []byte) {
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

func getInputFromStdin(ch chan<- []byte, isExit <-chan bool) {
	// stdin scanner
	sc := bufio.NewScanner(os.Stdin)

scan:
	for sc.Scan() {
		select {
		case <-isExit:
			break scan

		case <-time.After(10 * time.Millisecond):
			data := sc.Bytes()
			ch <- data
		}
	}
}

func putInputToSession(writer io.Writer, ch <-chan []byte, isExit <-chan bool) {
pull:
	for {
		select {
		case <-isExit:
			break pull

		case data := <-ch:
			writer.Write(data)
			writer.Write([]byte("\n"))

		case <-time.After(10 * time.Millisecond):
			continue
		}
	}
}

func outColorStrings(num int, inStrings string) (str string) {
	// 1=Red,2=Yellow,3=Blue,4=Magenta,0=Cyan
	color := 31 + num%5

	str = fmt.Sprintf("\x1b[%dm%s\x1b[0m", color, inStrings)
	return
}
