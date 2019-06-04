package ssh

import (
	"bufio"
	"bytes"

	"github.com/c-bata/go-prompt"
)

// shell complete function
// @TODO: とりあえず値を仮置き。後で以下の処理を追加する(優先度A)
//        - compgen(confで補完用の結果を取得するためのコマンドは指定可能にする)での補完結果の定期取得処理(+補完の取得用ローカルコマンドの追加)
//        - compgenの結果をStructに保持させる
//        - Structに保持されている補完内容をベースにCompleteの結果を返す
//        - 何も入力していない場合は非表示にさせたい
//        - ファイルについても対応させたい
//        - ファイルやコマンドなど、状況に応じて補完対象を変えるにはやはり構文解析が必要になってくる。Parserを実装するまではコマンドのみに対応。
//        参考: https://github.com/c-bata/kube-prompt/blob/2276d167e2e693164c5980427a6809058a235c95/kube/completer.go
func (s *shell) Completer(t prompt.Document) []prompt.Suggest {
	// local command suggest
	localCmdSuggest := []prompt.Suggest{
		{Text: "exit", Description: "exit lssh shell"},
		{Text: "quit", Description: "exit lssh shell"},
		{Text: "clear", Description: "clear screen"},
		{Text: "history", Description: "show history"},
		{Text: "!out", Description: "!out [num],show history result."},
	}

	// get complete data
	ps := s.Complete

	ps = append(ps, localCmdSuggest...)

	return prompt.FilterHasPrefix(ps, t.GetWordBeforeCursor(), false)
}

// get complete (command)
func (s *shell) GetCompleteData() {
	// bash complete command
	compCmd := []string{"compgen", "-c"}

	for _, c := range s.Connects {
		buf := new(bytes.Buffer)
		session, _ := c.CreateSession()
		session.Stdout = buf
		c.RunCmd(session, compCmd)
		sc := bufio.NewScanner(buf)
		for sc.Scan() {
			suggest := prompt.Suggest{
				Text:        sc.Text(),
				Description: "Command. from:" + c.Server,
			}
			s.Complete = append(s.Complete, suggest)
		}
	}
}
