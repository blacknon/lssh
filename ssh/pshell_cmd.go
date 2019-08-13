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

// checkBuildInCommand return bool, check is pshell build-in command or
// local machine command(%%command).
func checkBuildInCommand(cmd string) (isLocalCmd bool) {
	// check local command regex
	buildinRegex := regexp.MustCompile(`^!.*`)

	// check build-in command
	switch cmd {
	case "exit", "quit", "clear": // build-in command
		isLocalCmd = true

	case "%history", "%out": // parsent build-in command.
		isLocalCmd = true
	}

	// local command
	switch {
	case buildinRegex.MatchString(cmd):
		isLocalCmd = true
	}

	return
}

// checkLocalCommand return bool if each pipeline contains built-in commands or local machine commands.
func checkBuildInCommandInSlice(pslice [][]pipeLine) (isInLocalCmd bool) {
	for _, pipelines := range pslice {
		for _, p := range pipelines {
			// get pipeline command
			c := p.Args[0]

			if checkBuildInCommand(c) {
				isInLocalCmd = true
				return
			}
		}
	}

	return
}

// runBuildInCommand is run buildin or local machine command.
func (ps *pShell) run(pline pipeLine, in *io.PipeReader, out *io.PipeWriter, ch chan<- bool) (err error) {
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

	// Create History
	ps.History[ps.Count] = map[string]*pShellHistory{}

	// check and exec local command
	buildinRegex := regexp.MustCompile(`^!.*`)
	switch {
	case buildinRegex.MatchString(command):
		// exec local machine
		ps.executeLocalPipeLine(pline, in, out, ch)
	default:
		// exec remote machine
		ps.executeRemotePipeLine(pline, in, out, ch)
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
// TODO(blacknon):
//     - ひとまず動く状態にする
//     - Outputへの出力の引き渡しは後回しに(Writerに作り変える必要がありそう)
//     - HistoryResultについても同様とする
//     - Killの仕組みについても後回しにする
func (ps *pShell) executeRemotePipeLine(pline pipeLine, in *io.PipeReader, out *io.PipeWriter, ch chan<- bool) {
	// join command
	command := strings.Join(pline.Args, " ")

	// set stdin/stdout
	stdin := setInput(in)
	stdout := setOutput(out)

	// create channels
	exit := make(chan bool)
	exitInput := make(chan bool) // Input finish channel

	// create []io.WriteCloser
	var writers []io.WriteCloser
	var readers []io.Reader

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

		// set stdouts
		r, _ := s.StdoutPipe()
		readers = append(readers, r)

		// TODO(blacknon): 作業中！
		//     outputにうまいことbufのWriterとReaderを作って、それを出力先として利用させる
		//     stdoutの扱いが面倒になるので、そのあたりで調整が必要！
		//     https://stackoverflow.com/questions/23454940/getting-bytes-buffer-does-not-implement-io-writer-error-message

		// get and append stdin writer
		w, _ := s.StdinPipe()
		writers = append(writers, w)

		// append sessions
		sessions = append(sessions, s)
	}

	go pushStdoutPipe(readers, stdout)

	// multi input-writer
	switch stdin.(type) {
	case *os.File:
		// push input to pararell session (Only when input is os.Stdin and output is os.Stdout).
		if stdout == os.Stdout {
			go pushInput(exitInput, writers)
		}
	case *io.PipeReader:
		go pushPipeWriter(exitInput, writers, stdin)
	}

	for _, s := range sessions {
		session := s
		go func() {
			session.Run(command)
			session.Close()
			exit <- true
		}()
	}

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

	// TODO(blacknon): killのときだけ使う
	// exitInput <- true

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
func (ps *pShell) executeLocalPipeLine(pline pipeLine, in *io.PipeReader, out *io.PipeWriter, ch chan<- bool) (err error) {
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
	err = cmd.Run()

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

// TODO(blacknon):
//     - pShellのMethodにして、ホスト名等を付与可能なWriterを返すように作り変える
//     - local or remote の指定をさせるよう、引数を追加する必要がある
func setOutput(out io.WriteCloser) (stdout io.WriteCloser) {
	if reflect.ValueOf(out).IsNil() {
		stdout = os.Stdout
	} else {
		stdout = out
	}

	return
}
