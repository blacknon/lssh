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
//        参考: https://github.com/c-bata/kube-prompt/blob/2276d167e2e693164c5980427a6809058a235c95/kube/completer.go
func (s *shell) Completer(t prompt.Document) []prompt.Suggest {
	ps := []prompt.Suggest{
		{Text: "test-suggest"},
	}
	return prompt.FilterHasPrefix(ps, t.GetWordBeforeCursor(), true)
}
