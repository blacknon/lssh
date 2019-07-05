package ssh

import (
	"bufio"
	"fmt"
	"strings"
	"time"
)

// localCmd_history is printout history (shell history)
// TODO(blacknon): 通番をつけて、bash等のように `!<N>` で実行できるようにする
func (s *shell) localCmd_history() {
	data, err := s.GetHistory()
	if err != nil {
		return
	}

	for _, hist := range data {
		fmt.Printf("%s: %s\n", hist.Timestamp, hist.Command)
	}
}

// localCmd_out is print exec history at number
// example:
//     %out 3
func (s *shell) localCmd_out(num int) {
	cmd := s.ExecHistory[num]
	fmt.Printf("%d :%s \n", num, cmd)

	for _, c := range s.Connects {
		// Create Output
		o := &Output{
			Templete:   c.OutputPrompt,
			Count:      num,
			ServerList: c.ServerList,
			Conf:       c.Conf.Server[c.Server],
			AutoColor:  true,
		}
		o.Create(c.Server)

		// craete output data channel
		outputChan := make(chan []byte)

		go printOutput(o, outputChan)

		data := c.ExecHistory[num].OutputData.Bytes()
		sc := bufio.NewScanner(strings.NewReader(string(data)))
		for sc.Scan() {
			outputChan <- sc.Bytes()
		}

		close(outputChan)
		time.Sleep(10 * time.Millisecond)
	}
}
