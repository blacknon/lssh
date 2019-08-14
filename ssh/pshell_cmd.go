package ssh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/blacknon/go-sshlib"
	"golang.org/x/crypto/ssh"
)

func checkBuildInCommand(cmd string) (isBuildInCmd bool) {
	// check build-in command
	switch cmd {
	case "exit", "quit", "clear": // build-in command
		isBuildInCmd = true

	case "%history", "%out": // parsent build-in command.
		isBuildInCmd = true
	}

	return
}

// checkLocalCommand return bool, check is pshell build-in command or
// local machine command(%%command).
func checkLocalCommand(cmd string) (isLocalCmd bool) {
	// check build-in command
	isLocalCmd = checkBuildInCommand(cmd)

	// check local command regex
	buildinRegex := regexp.MustCompile(`^!.*`)

	// local command
	switch {
	case buildinRegex.MatchString(cmd):
		isLocalCmd = true
	}

	return
}

// checkLocalCommand return bool if each pipeline contains built-in commands or local machine commands.
// func checkLocalCommandInSlice(pslice [][]pipeLine) (isInLocalCmd bool) {
// 	for _, pipelines := range pslice {
// 		for _, p := range pipelines {
// 			// get pipeline command
// 			c := p.Args[0]

// 			if checkLocalCommand(c) {
// 				isInLocalCmd = true
// 				return
// 			}
// 		}
// 	}

// 	return
// }

// runBuildInCommand is run buildin or local machine command.
func (ps *pShell) run(pline pipeLine, in *io.PipeReader, out *io.PipeWriter, ch chan<- bool, kill chan bool) (err error) {
	// get 1st element
	command := pline.Args[0]

	// check and exec build-in command
	switch command {
	// exit or quit
	case "exit", "quit":
		os.Exit(0)

	// clear
	case "clear":
		fmt.Printf("\033[H\033[2J")
		return

	// %history
	case "%history":
		ps.buildin_history(out, ch)
		return

	// %out [num]
	case "%out":
		num := 0
		if len(pline.Args) > 1 {
			num, err = strconv.Atoi(pline.Args[1])
			if err != nil {
				return
			}
		}

		ps.buildin_out(num, out)
		return
	}

	// check and exec local command
	buildinRegex := regexp.MustCompile(`^!.*`)
	switch {
	case buildinRegex.MatchString(command):
		// exec local machine
		ps.executeLocalPipeLine(pline, in, out, ch, kill)
	default:
		// exec remote machine
		ps.executeRemotePipeLine(pline, in, out, ch, kill)
	}

	return
}

// localCmd_history is printout history (shell history)
func (ps *pShell) buildin_history(out *io.PipeWriter, ch chan<- bool) {
	stdout := setOutput(out)

	// read history file
	data, err := ps.GetHistoryFromFile()
	if err != nil {
		return
	}

	// print out history
	for _, h := range data {
		fmt.Fprintf(stdout, "%s: %s\n", h.Timestamp, h.Command)
	}

	// close out
	switch stdout.(type) {
	case *io.PipeWriter:
		out.CloseWithError(io.ErrClosedPipe)
	}

	// send exit
	ch <- true
}

// localCmd_out is print exec history at number
// example:
//     - %out
//     - %out <num>
func (ps *pShell) buildin_out(num int, out io.Writer) {
	histories := ps.History[num]

	i := 0
	for _, h := range histories {
		// if first, print out command
		if i == 0 {
			fmt.Fprintf(out, "Command: %s\n", h.Command)
		}
		i += 1

		// print out result
		fmt.Fprintf(out, h.Result)
	}
}

// executePipeLineRemote is exec command in remote machine.
// Didn't know how to send data from Writer to Channel, so switch the function if * io.PipeWriter is Nil.
// TODO(blacknon):
//     - HistoryResultへの書き込みの実装(writerを受け付ける？)
func (ps *pShell) executeRemotePipeLine(pline pipeLine, in *io.PipeReader, out *io.PipeWriter, ch chan<- bool, kill chan bool) {
	// join command
	command := strings.Join(pline.Args, " ")

	// set stdin/stdout
	stdin := setInput(in)
	stdout := setOutput(out)

	// create channels
	exit := make(chan bool)
	exitInput := make(chan bool) // Input finish channel
	exitOutput := make(chan bool)

	// create []io.WriteCloser
	var writers []io.WriteCloser

	// create []ssh.Session
	var sessions []*ssh.Session

	// create session and writers
	for _, c := range ps.Connects {
		// create session
		s, err := c.CreateSession()
		if err != nil {
			continue
		}

		// Request tty (Only when input is os.Stdin and output is os.Stdout).
		if stdin == os.Stdin && stdout == os.Stdout {
			sshlib.RequestTty(s)
		}

		// set stdout
		var ow io.WriteCloser
		ow = stdout
		if ow == os.Stdout {
			c.Output.Count = ps.Count
			w := c.Output.NewWriter()
			defer w.CloseWithError(io.ErrClosedPipe)
			ow = w
		}
		s.Stdout = ow

		// get and append stdin writer
		w, _ := s.StdinPipe()
		writers = append(writers, w)

		// append sessions
		sessions = append(sessions, s)
	}

	// multi input-writer
	switch stdin.(type) {
	case *os.File:
		// push input to pararell session
		// (Only when input is os.Stdin and output is os.Stdout).
		if stdout == os.Stdout {
			go pushInput(exitInput, writers)
		}
	case *io.PipeReader:
		go pushPipeWriter(exitInput, writers, stdin)
	}

	// run command
	for _, s := range sessions {
		session := s
		go func() {
			session.Run(command)
			session.Close()
			exit <- true
			if stdout == os.Stdout {
				exitOutput <- true
			}
		}()
	}

	// kill
	go func() {
		select {
		case <-kill:
			for _, s := range sessions {
				s.Signal(ssh.SIGINT)
				s.Close()
			}
		}
	}()

	// wait
	ps.wait(len(sessions), exit)

	// wait time (0.500 sec)
	time.Sleep(500 * time.Millisecond)

	// Print message `Please input enter` (Only when input is os.Stdin and output is os.Stdout).
	// Note: This necessary for using Blocking.IO.
	if stdin == os.Stdin && stdout == os.Stdout {
		fmt.Fprintf(os.Stderr, "\n---\n%s\n", "Command exit. Please input Enter.")
		exitInput <- true
	}

	// send exit
	ch <- true

	// close out
	switch stdout.(type) {
	case *io.PipeWriter:
		out.CloseWithError(io.ErrClosedPipe)
	}

	return
}

// executePipeLineLocal is exec command in local machine.
// TODO(blacknon):
//     - HistoryResultへの書き込みの実装(writerを受け付ける？)
func (ps *pShell) executeLocalPipeLine(pline pipeLine, in *io.PipeReader, out *io.PipeWriter, ch chan<- bool, kill chan bool) (err error) {
	// set stdin/stdout
	stdin := setInput(in)
	stdout := setOutput(out)

	// delete command prefix(`%%`)
	rep := regexp.MustCompile(`^!`)
	pline.Args[0] = rep.ReplaceAllString(pline.Args[0], "")

	// join command
	command := strings.Join(pline.Args, " ")

	// execute command
	cmd := exec.Command("sh", "-c", command)

	// set stdin, stdout, stderr
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr

	// run command
	err = cmd.Start()

	// get signal and kill
	p := cmd.Process
	go func() {
		select {
		case <-kill:
			p.Kill()
		}
	}()

	// wait command
	cmd.Wait()

	// close out
	switch stdout.(type) {
	case *io.PipeWriter:
		out.CloseWithError(io.ErrClosedPipe)
	}

	// send exit
	ch <- true

	return
}

// ps.wait
func (ps *pShell) wait(num int, ch <-chan bool) {
	for i := 0; i < num; i++ {
		<-ch
	}
}

// setInput
func setInput(in io.ReadCloser) (stdin io.ReadCloser) {
	if reflect.ValueOf(in).IsNil() {
		stdin = os.Stdin
	} else {
		stdin = in
	}

	return
}

// setOutput
func setOutput(out io.WriteCloser) (stdout io.WriteCloser) {
	if reflect.ValueOf(out).IsNil() {
		stdout = os.Stdout
	} else {
		stdout = out
	}

	return
}
