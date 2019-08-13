package ssh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// TODO(blacknon): ローカル・リモートでのコマンドの実行もここで処理する
//                 (sshlibでやるとちょっと辛そうなので)

// PipeSet is pipe in/out set struct.
type PipeSet struct {
	in  *io.PipeReader
	out *io.PipeWriter
}

// Executor run ssh command in parallel-shell.
func (ps *pShell) Executor(command string) {
	// trim space
	command = strings.TrimSpace(command)

	// parse command
	pslice, _ := parsePipeLine(command)
	if len(pslice) == 0 {
		return
	}

	// regist history
	ps.PutHistoryFile(command)

	// TODO(blacknon):
	//     パース処理したplineを、リモートマシンでの実行処理が連続している箇所はjoinしてまとめる(pipeを数える前に処理)。

	// exec pipeline
	ps.parseExecuter(pslice)

	return
}

// parseExecuter assemble and execute the parsed command line.
// TODO(blacknon): ↓を参考にしてみる
//     - http://syohex.hatenablog.com/entry/20131016/1381935100
//     - https://stackoverflow.com/questions/10781516/how-to-pipe-several-commands-in-go
func (ps *pShell) parseExecuter(pslice [][]pipeLine) {
	// TODO(blacknon): Add HistoryResult

	// for pslice
	for _, pline := range pslice {
		// pipe stdin, stdout, stderr
		// TODO(blacknon):
		//   ローカル⇔リモート間で処理をする場合、stdinやstdout,stderrをMultiWriter,MultiReaderとしてforでつなげていき、stdinwriterやstdoutreaderとしてくっつけていくことで対処する。
		//   多分めんどくさいけどそれが確実。
		//   …というか、それをやる前にローカル⇔リモートで分離となるようにパースしてやるほうが良さそう？

		// count pipe num
		pnum := countPipeSet(pline, "|")

		// create pipe set
		pipes := createPipeSet(pnum)

		var n int // pipe counter

		// create channel
		ch := make(chan bool)

		for i, p := range pline {
			// declare nextPipeLine
			var nextPipeLine pipeLine

			// @TODO(blacknon): Delete?
			// // set command
			// command := p.Args[0]

			// declare in,out
			var in *io.PipeReader
			var out *io.PipeWriter

			// get next pipe line
			if len(pline) > i+1 {
				nextPipeLine = pline[i+1]
			}

			// set stdin
			// If the delimiter is a pipe, set the stdin input source to
			// io.PipeReader and add 1 to the PipeSet counter.
			if p.Oprator == "|" {
				in = pipes[n-1].in
			}

			// set stdout
			// If the next delimiter is a pipe, make the output of stdout
			// a io.PipeWriter.
			if nextPipeLine.Oprator == "|" {
				out = pipes[n].out
			}

			// exec pipeline
			go ps.run(p, in, out, ch)

			// add pipe num
			n += 1
		}

		// wait channel
		ps.wait(len(pline), ch)
	}
}

// countPipeSet count delimiter in pslice.
func countPipeSet(pline []pipeLine, del string) (count int) {
	for _, p := range pline {
		if p.Oprator == del {
			count += 1
		}
	}

	return count
}

// createPipeSet return Returns []*PipeSet used by the process.
func createPipeSet(count int) (pipes []*PipeSet) {
	for i := 0; i < count; i++ {
		in, out := io.Pipe()
		pipe := &PipeSet{
			in:  in,
			out: out,
		}

		pipes = append(pipes, pipe)
	}

	return
}

// Run is exec command at remote machine.
// This function is not parse and run command.
// TODO(blacknon):
//     - 標準入出力をパイプ経由でやり取りできるよう、汎用性を考慮する
//     - 入出力の指定とoutputへのデータの送信処理を分離する必要がある？？
//     - 残すにしても名前変えないとわかりにくいし辛いことになりそう。
//         - `ExecuteRemoteCmd`とかかな
//         - 入出力含め、リファクタが必要
// TODO(blacknon): ちゃんと関数を移行したら削除する！！！
func (ps *pShell) executeRemoteCommand(command string, in io.Reader, out io.Writer) {
	// Create History
	ps.History[ps.Count] = map[string]*pShellHistory{}

	// create chanel
	finished := make(chan bool)    // Run Command finish channel
	input := make(chan io.Writer)  // Get io.Writer at input channel
	exitInput := make(chan bool)   // Input finish channel
	exitSignal := make(chan bool)  // Send kill signal finish channel
	exitHistory := make(chan bool) // Put History finish channel

	// create []io.Writer after in MultiWriter
	var writers []io.Writer

	// for connect and run
	m := new(sync.Mutex)
	for _, fc := range ps.Connects {
		// set variable c
		// NOTE: Variables need to be assigned separately for processing by goroutine.
		c := fc

		// Get output data channel
		output := make(chan []byte)
		// defer close(output)

		// Set count num
		c.Output.Count = ps.Count

		// Create output buffer, and MultiWriter
		buf := new(bytes.Buffer)
		omw := io.MultiWriter(os.Stdout, buf)
		c.Output.OutputWriter = omw

		// put result
		go func() {
			m.Lock()
			ps.PutHistoryResult(c.Name, command, buf, exitHistory)
			m.Unlock()
		}()

		// Run command
		go func() {
			c.CmdWriter(command, output, input)
			finished <- true
		}()

		// Get input(io.Writer), add MultiWriter
		w := <-input
		writers = append(writers, w)

		// run print Output
		go func() {
			printOutput(c.Output, output)
		}()
	}

	// create and run input writer
	mw := io.MultiWriter(writers...)
	go pushInput(exitInput, mw)

	// send kill signal function
	go ps.pushKillSignal(exitSignal, ps.Connects)

	// wait finished channel
	for i := 0; i < len(ps.Connects); i++ {
		select {
		case <-finished:
		}
	}

	// Exit check signal
	exitSignal <- true

	// wait time (0.500 sec)
	time.Sleep(500 * time.Millisecond)

	// Exit Messages
	// Because it is Blocking.IO, you can not finish Input without input from the user.
	fmt.Fprintf(os.Stderr, "\n---\n%s\n", "Command exit. Please input Enter.")

	// Exit input
	exitInput <- true

	// Exit history
	for i := 0; i < len(ps.Connects); i++ {
		exitHistory <- true
	}

	// Add count
	ps.Count += 1
}

// pushSignal is send kill signal to session.
func (ps *pShell) pushKillSignal(exitSig chan bool, conns []*psConnect) (err error) {
	i := 0
	for {
		select {
		case <-ps.Signal:
			if i == 0 {
				for _, c := range conns {
					// send kill
					c.Kill()
				}
				i = 1
			}
		case <-exitSig:
			return
		}
	}
	return
}
