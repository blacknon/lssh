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
		}

		data = append(data, d)
	}
	return
}

// PutHistoryFile put history to s.HistoryFile
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

	// write history
	// history file format
	//     YYYY-mm-dd_HH:MM:SS command...
	//     YYYY-mm-dd_HH:MM:SS command...
	//     ...
	fmt.Fprintln(file, timestamp+cmd)

	return
}

// PutHistoryResult is append history to []History and HistoryFile
func (ps *pShell) PutHistoryResult(server, command string, buf *bytes.Buffer, isExit chan bool) (err error) {
	// Get Time
	timestamp := time.Now().Format("2006/01/02_15:04:05 ") // "yyyy/mm/dd_HH:MM:SS "

	result := ""

	// append result
	l := 0
loop:
	for {
		len := buf.Len()
		if l != len {
			line, err := buf.ReadString('\n')
			result = result + line
			if err == io.EOF {
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
	ps.History[ps.Count][server] = pShellHistory{
		Timestamp: timestamp,
		Command:   command,
		Result:    result,
	}

	return
}
