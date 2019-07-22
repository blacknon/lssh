package ssh

import (
	"fmt"
)

// localCmd_history is printout history (shell history)
// TODO(blacknon): 通番をつけて、bash等のように `!<N>` で実行できるようにする
func (ps *pShell) localCmd_history() {
	data, err := ps.GetHistoryFromFile()
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
	histories := ps.History[num]

	for _, h := range histories {
		fmt.Printf(h.Result)
	}
}
