package ssh

import (
	"os"
	"time"
)

type history struct {
	historyFile *os.File
	timestamp   time.Time
	command     string
}

// @bref:
func (h *history) SetHistoryFile(historyFile string) (err error) {
	h.historyFile, err = os.OpenFile(historyFile, os.O_WRONLY|os.O_CREATE, 0600)
	return
}

// @NOTE: jsonか否かでのチェック処理が必要(↓参考)
//     https://stackoverflow.com/questions/22128282/how-to-check-string-is-in-json-format

// func (s *shell) GetHistory() (data []string) {}

// func (s *shell) PutHistory(cmd string) {}
