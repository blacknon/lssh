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
	buildinRegex := regexp.MustCompile(`^%%.*`)

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
		ps.buildin_history(out)
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
	buildinRegex := regexp.MustCompile(`^%%.*`)
	switch {
	case buildinRegex.MatchString(command):
		// exec local machine
		ps.executePipeLineLocal(pline, in, out, ch)
	default:
		// exec remote machine
		ps.executePipeLineRemote(pline, in, out, ch)
	}

	return
}

// localCmd_history is printout history (shell history)
// TODO(blacknon): 通番をつけて、bash等のように `!<N>` で実行できるようにする
func (ps *pShell) buildin_history(out io.Writer) {
	data, err := ps.GetHistoryFromFile()
	if err != nil {
		return
	}

	for _, h := range data {
		fmt.Fprintf(out, "%s: %s\n", h.Timestamp, h.Command)
	}
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
func (ps *pShell) executePipeLineRemote(pline pipeLine, in *io.PipeReader, out *io.PipeWriter, ch chan<- bool) {
	// join command
	command := strings.Join(pline.Args, " ")

	// set stdin/stdout
	stdin := setInput(in)
	stdout := setOutput(out)

	// create channels
	// exit := make(chan bool)
	exitInput := make(chan bool) // Input finish channel
	// checkExitInput := make(chan bool)
	// receiveExitInput := make(chan bool)
	// exitSignal := make(chan bool) // Send kill signal finish channel

	// create []io.PipeWriter, this slice after in MultiWriter.
	var writers []io.WriteCloser

	// create []ssh.Session
	var sessions []*ssh.Session

	// TODO(blacknon): HistoryResultの書き込みをパラレルで行うため、mutexを使用する必要がある(今は使わない)
	// m := new(sync.Mutex)

	// TODO(blacknon): MultiReaderにしないとpanicになるので、stdoutに直接書き込むのではなくMultiReaderからio.Copyに変更する

	// go multiInput(exitInput, mw, stdin)

	// create session and writers
	for _, c := range ps.Connects {
		// create session
		s, err := c.CreateSession()
		if err != nil {
			continue
		}

		// Request tty
		sshlib.RequestTty(s)

		// set stdouts
		s.Stdout = stdout

		// get and append stdin writer
		w, _ := s.StdinPipe()
		writers = append(writers, w)

		// append sessions
		sessions = append(sessions, s)
	}

	// multi input-writer
	switch stdin.(type) {
	case *os.File:
		// TODO(blacknon): 仮置きの値。後でwritersをまとめられるか検証する
		var ws []io.Writer
		for _, wc := range writers {
			ws = append(ws, wc)
		}
		mw := io.MultiWriter(ws...)
		go pushInput(exitInput, mw)
	case *io.PipeReader:
		go multiPipeReadWriter(exitInput, writers, stdin)
	}

	for _, s := range sessions {
		session := s
		go func() {
			session.Start(command)
		}()
	}

	// wait
	for _, s := range sessions {
		s.Wait()
	}

	// wait time (0.500 sec)
	time.Sleep(500 * time.Millisecond)

	//
	switch stdin.(type) {
	case *os.File:
		fmt.Fprintf(os.Stderr, "\n---\n%s\n", "Command exit. Please input Enter.")
		exitInput <- true
	}

	// TODO(blacknon): killのときだけ使う
	// exitInput <- true

	// send exit
	ch <- true

	// close in
	if !reflect.ValueOf(in).IsNil() {
		in.CloseWithError(io.ErrClosedPipe)
	} else

	// close out
	if !reflect.ValueOf(out).IsNil() {
		out.CloseWithError(io.ErrClosedPipe)
	}

	return
}

// executePipeLineLocal is exec command in local machine.
func (ps *pShell) executePipeLineLocal(pline pipeLine, in *io.PipeReader, out *io.PipeWriter, ch chan<- bool) (err error) {
	// set stdin/stdout
	stdin := setInput(in)
	stdout := setOutput(out)

	// delete command prefix(`%%`)
	rep := regexp.MustCompile(`^%%`)
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

	// close in
	if !reflect.ValueOf(in).IsNil() {
		in.CloseWithError(io.ErrClosedPipe)
	}

	// close out
	if !reflect.ValueOf(out).IsNil() {
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
