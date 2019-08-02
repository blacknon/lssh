package ssh

import (
	"io"
	"strings"
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
	parseCmd, _ := ps.parsePipeLine(command)
	if len(parseCmd) == 0 {
		return
	}

	// regist history
	ps.PutHistoryFile(command)

	// Check `build in` or `local machine` command.
	// If there are no built-in commands or local machine commands, pass the pipeline to the remote machine.
	switch {
	case !ps.checkBuildInCommand(parseCmd): // Execute the command as it is on the remote machine.
		ps.remoteRun(command)

	default: // if with build in or local machine command.

	}

	return
}

// parseExecuter assemble and execute the parsed command line.
// TODO(blacknon): ↓を参考にしてみる
//     - http://syohex.hatenablog.com/entry/20131016/1381935100
//     - https://stackoverflow.com/questions/10781516/how-to-pipe-several-commands-in-go
func (ps *pShell) parseExecuter(pmap map[int][]pipeLine) {
	// for pmap
	for _, plines := range pmap {
		// pipe stdin, stdout, stderr
		// TODO(blacknon):
		//   ローカル⇔リモート間で処理をする場合、stdinやstdout,stderrをMultiWriter,MultiReaderとしてforでつなげていき、stdinwriterやstdoutreaderとしてくっつけていくことで対処する。
		//   多分めんどくさいけどそれが確実。
		//   …というか、それをやる前にローカル⇔リモートで分離となるようにパースしてやるほうが良さそう？

		for _, pl := range plines {

		}
	}
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
