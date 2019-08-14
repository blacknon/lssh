package ssh

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/c-bata/go-prompt"
)

// Completer parallel-shell complete function
func (ps *pShell) Completer(t prompt.Document) []prompt.Suggest {
	// TODO(blacknon): とりあえず値を仮置き。後で以下の処理を追加する(優先度A)
	//        - compgen(confで補完用の結果を取得するためのコマンドは指定可能にする)での補完結果の定期取得処理(+補完の取得用ローカルコマンドの追加)
	//        - compgenの結果をStructに保持させる
	//        - Structに保持されている補完内容をベースにCompleteの結果を返す
	//        - 何も入力していない場合は非表示にさせたい
	//        - ファイルについても対応させたい
	//        - ファイルやコマンドなど、状況に応じて補完対象を変えるにはやはり構文解析が必要になってくる。Parserを実装するまではコマンドのみ対応。
	//        	参考: https://github.com/c-bata/kube-prompt/blob/2276d167e2e693164c5980427a6809058a235c95/kube/completer.go

	// TODO(blacknon):
	//        - t.GetWordBeforeCursor() などで前の文字までは取得できるので、その文字列に応じて補完を返すかどうかを対応する。
	//        - パイプラインを区切る際、複数のセパレータで区切れるか調査が必要(|や;の他、' 'や||、&&など)。
	//          (多分、位置情報と組み合わせてコマンドラインを取得して、位置より前の情報からセパレートして処理してやればどうにかなりそう。)

	// build in command suggest
	localCmdSuggest := []prompt.Suggest{
		{Text: "exit", Description: "exit lssh shell"},
		{Text: "quit", Description: "exit lssh shell"},
		{Text: "clear", Description: "clear screen"},
		{Text: "%history", Description: "show history"},
		{Text: "%out", Description: "%out [num], show history result."},
		// {Text: "%outlist", Description: "%outlist, show history result list."}, // outのリストを出力するためのローカルコマンド

		// outの出力でdiffをするためのローカルコマンド。すべての出力と比較するのはあまりに辛いと思われるため、最初の出力との比較、といった方式で対応するのが良いか？？
		// {Text: "%diff", Description: "%diff [num], show history result list."},

		// outの出力でユニークな出力だけを表示するコマンド
		// {Text: "%unique", Description: "%unique [num], show history result list."},

		// outの出力で重複している出力だけを表示するコマンド
		// {Text: "%duplicate", Description: "%duplicate [num], show history result list."},
	}

	// get complete data
	c := ps.Complete
	c = append(c, localCmdSuggest...)

	return prompt.FilterHasPrefix(c, t.GetWordBeforeCursor(), false)
}

// GetCompleteData get command list remote machine.
func (ps *pShell) GetCompleteData() {
	// TODO(blacknon):
	//   - 構文解析して、ファイルの補完処理も行わせる
	//     - 引数にコマンドorファイルの種別を渡すようにする
	//   - 補完コマンドをconfigでオプションとして指定できるようにする
	//     - あまり無いだろうけど、bash以外をリモートで使ってる場合など(ashとかzsh(レア)など)

	// bash complete command. use `compgen`.
	compCmd := []string{"compgen", "-c"}
	command := strings.Join(compCmd, " ")

	// command map
	cmdMap := map[string][]string{}

	// append command to cmdMap
	for _, c := range ps.Connects {
		// Create buffer
		buf := new(bytes.Buffer)

		// Create session, and output to buffer
		session, _ := c.CreateSession()
		session.Stdout = buf

		// Run get complete command
		session.Run(command)

		// Scan and put completed command to map.
		sc := bufio.NewScanner(buf)
		for sc.Scan() {
			cmdMap[sc.Text()] = append(cmdMap[sc.Text()], c.Name)
		}
	}

	// cmdMap to suggest
	for cmd, hosts := range cmdMap {
		// join hosts
		h := strings.Join(hosts, ",")

		// create suggest
		suggest := prompt.Suggest{
			Text:        cmd,
			Description: "Command. from:" + h,
		}

		// append ps.Complete
		ps.Complete = append(ps.Complete, suggest)
	}
}
