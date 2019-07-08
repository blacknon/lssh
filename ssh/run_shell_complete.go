package ssh

import (
	"bufio"
	"bytes"

	"github.com/c-bata/go-prompt"
)

// Completer lssh-shell complete function
func (s *shell) Completer(t prompt.Document) []prompt.Suggest {
	// TODO(blacknon): とりあえず値を仮置き。後で以下の処理を追加する(優先度A)
	//        - compgen(confで補完用の結果を取得するためのコマンドは指定可能にする)での補完結果の定期取得処理(+補完の取得用ローカルコマンドの追加)
	//        - compgenの結果をStructに保持させる
	//        - Structに保持されている補完内容をベースにCompleteの結果を返す
	//        - 何も入力していない場合は非表示にさせたい
	//        - ファイルについても対応させたい
	//        - ファイルやコマンドなど、状況に応じて補完対象を変えるにはやはり構文解析が必要になってくる。Parserを実装するまではコマンドのみ対応。
	//        	参考: https://github.com/c-bata/kube-prompt/blob/2276d167e2e693164c5980427a6809058a235c95/kube/completer.go

	// local command suggest
	localCmdSuggest := []prompt.Suggest{
		{Text: "exit", Description: "exit lssh shell"},
		{Text: "quit", Description: "exit lssh shell"},
		{Text: "clear", Description: "clear screen"},
		{Text: "history", Description: "show history"},
		{Text: "%out", Description: "%out [num], show history result."},

		// outのリストを出力ためのローカルコマンド
		// {Text: "%outlist", Description: "%outlist, show history result list."},

		// outの出力でdiffをするためのローカルコマンド。すべての出力と比較するのはあまりに辛いと思われるため、最初の出力との比較、といった方式で対応するのが良いか？？
		// {Text: "%diff", Description: "%diff [num], show history result list."},

		// outの出力でユニークな出力だけを表示するコマンド
		// {Text: "%unique", Description: "%unique [num], show history result list."},

		// outの出力で重複している出力だけを表示するコマンド
		// {Text: "%duplicate", Description: "%duplicate [num], show history result list."},

	}

	// get complete data
	ps := s.Complete
	ps = append(ps, localCmdSuggest...)

	return prompt.FilterHasPrefix(ps, t.GetWordBeforeCursor(), false)
}

// GetCompleteData get command list remote machine.
func (s *shell) GetCompleteData() {
	// bash complete command
	compCmd := []string{"compgen", "-c"}

	// TODO(blacknon):
	// - 重複データの排除
	// - 構文解析して、ファイルの補完処理も行わせる
	//   - 引数にコマンドorファイルの種別を渡すようにする
	// - 補完コマンドをconfigでオプションとして指定できるようにする
	//   - あまり無いだろうけど、zshをリモートで使ってる場合なんかには指定(zshとかはデフォルトでcompgen使えないし)

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
