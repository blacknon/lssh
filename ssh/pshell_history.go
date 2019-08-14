package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"
	"time"
)

type pShellHistory struct {
	Timestamp string
	Command   string
	Result    string
}

//
func (p *pShellHistory) NewWriter() io.PipeWriter {
	// create io.PipeReader, io.PipeWriter
	r, w := io.Pipe()

	// output Struct
	p.Print(r)

	// return io.PipeWriter
	return w
}

func (p *pShellHistory) Print(r io.PipeReader) {

}

// GetHistoryFromFile return []History from historyfile
func (ps *pShell) GetHistoryFromFile() (data []pShellHistory, err error) {
	// user path
	usr, _ := user.Current()
	histfile := strings.Replace(ps.HistoryFile, "~", usr.HomeDir, 1)

	// Open history file
	file, err := os.OpenFile(histfile, os.O_RDONLY, 0600)
	if err != nil {
		return
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := sc.Text()
		text := strings.SplitN(line, " ", 2)

		if len(text) < 2 {
			continue
		}

		d := pShellHistory{
			Timestamp: text[0],
			Command:   text[1],
			Result:    "",
		}

		data = append(data, d)
	}
	return
}

// PutHistoryFile put history text to s.HistoryFile
// ex.) write history(history file format)
//     YYYY-mm-dd_HH:MM:SS command...
//     YYYY-mm-dd_HH:MM:SS command...
//     ...
func (ps *pShell) PutHistoryFile(cmd string) (err error) {
	// user path
	usr, _ := user.Current()
	histfile := strings.Replace(ps.HistoryFile, "~", usr.HomeDir, 1)

	// Open history file
	file, err := os.OpenFile(histfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer file.Close()

	// Get Time
	timestamp := time.Now().Format("2006/01/02_15:04:05 ") // "yyyy/mm/dd_HH:MM:SS "

	fmt.Fprintln(file, timestamp+cmd)

	return
}

// PutHistoryResult is append history to []History and HistoryFile
// TODO(blacknon): Writerでやる場合にうまく動かないようなので、io.PipeWriterを利用した処理をすることで対処する。
//                 終わったらこれは削除！
func (ps *pShell) PutHistoryResult(server, command string, buf *bytes.Buffer, isExit chan bool) (err error) {
	// count
	count := ps.Count

	// Get Time
	timestamp := time.Now().Format("2006/01/02_15:04:05 ") // "yyyy/mm/dd_HH:MM:SS "

	// init result
	result := ""

loop:
	for {
		if buf.Len() > 0 {
			line, err := buf.ReadString('\n')
			result = result + line
			if err != io.EOF {
				continue
			}
		}

		select {
		case <-isExit:
			break loop
		case <-time.After(10 * time.Millisecond):
			continue loop
		}
	}

	// Add History
	ps.History[count][server] = &pShellHistory{
		Timestamp: timestamp,
		Command:   command,
		Result:    result,
	}

	fmt.Println("write history")

	fmt.Printf("Command   %s \n", ps.History[count][server].Command)
	fmt.Printf("Timestamp %s \n", ps.History[count][server].Timestamp)
	fmt.Printf("Result    %s \n", ps.History[count][server].Result)

	return
}
