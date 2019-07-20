package ssh

import (
	"bufio"
	"fmt"
	"strings"
	"time"
)

// localCmd_history is printout history (shell history)
// TODO(blacknon): 通番をつけて、bash等のように `!<N>` で実行できるようにする
func (ps *pshell) localCmd_history() {
	data, err := ps.GetHistory()
	if err != nil {
		return
	}

	for _, h := range data {
		fmt.Printf("%s: %s\n", h.Timestamp, h.Command)
	}
}

// localCmd_out is print exec history at number
// example:
//     %out 3
//
// TODO(blacknon): 引数がない場合、直前の処理の出力を表示させる
func (ps *pShell) localCmd_out(num int) {
	cmd := ps.ExecHistory[num]
	fmt.Printf("%d :%s \n", num, cmd)

	for _, c := range ps.Connects {
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
