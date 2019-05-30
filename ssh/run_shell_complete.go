package ssh

import (
	"github.com/c-bata/go-prompt"
)

// shell complete function
// @TODO: とりあえず値を仮置き。後で以下の処理を追加する(優先度A)
//        - compgen(confで補完用の結果を取得するためのコマンドは指定可能にする)での補完結果の定期取得処理(+補完の取得用ローカルコマンドの追加)
//        - compgenの結果をStructに保持させる
//        - Structに保持されている補完内容をベースにCompleteの結果を返す
//        - 何も入力していない場合は非表示にさせたい
//        - ファイルについても対応させたい
//        参考: https://github.com/c-bata/kube-prompt/blob/2276d167e2e693164c5980427a6809058a235c95/kube/completer.go
func (s *shell) Completer(t prompt.Document) []prompt.Suggest {
	ps := []prompt.Suggest{
		{Text: "test-suggest1"},
		{Text: "test-suggest2"},
		{Text: "test-suggest3"},
		{Text: "test-suggest4"},
		{Text: "test-suggest5"},
		{Text: "test-suggest6"},
		{Text: "test-suggest7"},
		{Text: "test-suggest8"},
		{Text: "test-suggest9"},
		{Text: "test-suggest10"},
		{Text: "test-suggest11"},
		{Text: "test-suggest12"},
		{Text: "test-suggest13"},
		{Text: "test-suggest14"},
		{Text: "test-suggest15"},
		{Text: "test-suggest16"},
		{Text: "test-suggest17"},
		{Text: "test-suggest18"},
		{Text: "test-suggest19"},
		{Text: "test-suggest20"},
	}
	return prompt.FilterHasPrefix(ps, t.GetWordBeforeCursor(), false)
}
