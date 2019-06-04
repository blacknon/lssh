package ssh

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"
)

// @NOTE:
//     [history file format]
//         YYYY-mm-dd_HH:MM:SS command...
//         YYYY-mm-dd_HH:MM:SS command...
//         ...

type History struct {
	Timestamp string
	Command   string
}

func (s *shell) GetHistory() (data []History, err error) {
	// user path
	usr, _ := user.Current()
	histfile := strings.Replace(s.HistoryFile, "~", usr.HomeDir, 1)

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

		d := History{
			Timestamp: text[0],
			Command:   text[1],
		}

		data = append(data, d)
	}
	return
}

func (s *shell) PutHistory(cmd string) (err error) {
	// user path
	usr, _ := user.Current()
	histfile := strings.Replace(s.HistoryFile, "~", usr.HomeDir, 1)

	// Open history file
	file, err := os.OpenFile(histfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer file.Close()

	// Get Time
	timestamp := time.Now().Format("2006/01/02_15:04:05 ") // "yyyy-mm-dd_HH:MM:SS "

	// write history
	fmt.Fprintln(file, timestamp+cmd)

	return
}
