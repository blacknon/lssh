AGENTS.md
===

## 概要

- このリポジトリは `lssh` 向けのSSHライブラリです。

## 作業方針

- Go のバージョンは `go.mod` に合わせてください。古い Go への互換対応を勝手に追加しないでください。
- 変更範囲はできるだけ局所化し、無関係なリファクタは混ぜないでください。
- CLI フラグ、設定ファイル形式、既存のユーザー向け挙動を不必要に変えないでください。
- クロスプラットフォーム性を意識してください。特に Linux / macOS / Windows の差異がある処理では、対象 OS を明示して実装してください。
- 基本的に、コマンド実行をユーザが指定する機能以外ではOSコマンドを実行しての機能実装はしないでください。もしそれしか実現方法がない場合は都度聞いてください。
- Vendorへの修正はNGです。Vendorはgo mod以外で変化が加わることがないようにしてください。vendor配下が原因の可能性がある場合、まずはバージョンを上げることで対応できるかどうか検討してください。
- アーキテクチャの切り替えのような大きな変更を行う場合、勝手にやらず一旦ユーザへの確認を挟んでください。

## ビルドとテスト

- 変更後は、まず影響範囲に近いテストを優先して実行してください。
- テストやビルドが未実行なら、実行していないことと理由を最終報告に明記してください。

## ドキュメント更新ルール

- ユーザー向け挙動、CLI オプション、設定方法、インストール手順を変えた場合は、対応する README / docs も更新してください。

## 補足

### NFS / lsshfs compatibility patch

このリポジトリでは、`lsshfs` を macOS 上で NFS backend として利用した際に発生する以下の問題を回避するため、`go-nfs` と `go-billy` を `internal/third_party` 配下でローカル管理している。

発生していた問題:

- `failing create to indicate lack of support for 'exclusive' mode.`
- `Error applying attributes: Operation not supported: permission denied`
- mount 先に不要な一時ファイルや 0 byte ファイルが残ることがある

#### 現在の依存構成

`go.mod` では以下の `replace` を使っている。

- `github.com/go-git/go-billy/v5 => ./internal/third_party/github.com/go-git/go-billy/v5`
- `github.com/willscott/go-nfs => ./internal/third_party/github.com/willscott/go-nfs`

この構成は意図的なものであり、安易に upstream 版へ戻してはいけない。

#### 具体的な修正内容

##### 1. `nfs_sftpfs.go`
`NewChangeSFTPFS()` の返り値を、単純な `temporal.New(chroot.New(...))` ではなく、`billy.Change` を維持する wrapper に変更している。

変更意図:

- `chroot.New(...)` を通すと、下位の filesystem が持つ `billy.Change` が外側から見えなくなる
- そのままだと `go-nfs` 側の attribute apply (`Chmod`, `Chown`, `Lchown`, `Chtimes`) が失敗しやすい
- そのため、`changeChrootFS` で chroot 後の見かけの path を実 path に戻して、下位の `SFTPFS` に `Change` を forward している

加えて、`SFTPFS` 自体にも以下を実装している。

- `Chmod`
- `Chown`
- `Lchown`
- `Chtimes`

これらは `sftp.Client` へ forward する。

##### 2. `internal/third_party/github.com/willscott/go-nfs/nfs_oncreate.go`
`CREATE_EXCLUSIVE` を即 `NFSStatusNotSupp` で失敗させないようにしている。

変更意図:

- macOS クライアントが probe 的に `exclusive create` を使うことがある
- upstream のままだとこの時点で hard fail し、不要なエラーや挙動不良につながる
- 現在は `exclusive create` の場合、空の `SetFileAttributes` として続行する

さらに、attribute apply に失敗した場合は、作りかけファイルを `Remove()` して cleanup するようにしている。

##### 3. `internal/third_party/github.com/go-git/go-billy/v5`
このディレクトリは `replace` 先として内部保持している。
現時点では `go-sshlib` 側の wrapper で `Change` 問題を吸収しているが、依存整合性と将来の patch 管理のため、`go-billy` も `internal/third_party` 側で固定している。

#### 重要な注意

以下は勝手に変更しないこと。

- `go.mod` の `replace` の削除
- `internal/third_party` 配下の削除
- upstream 版への安易な差し戻し
- `nfs_sftpfs.go` の `changeChrootFS` wrapper の除去
- `nfs_oncreate.go` の `exclusive create` 回避処理の除去

これらを戻すと、`lsshfs` で以下の不具合が再発する可能性が高い。

- mount 後の謎ファイル生成
- `exclusive create` エラー
- attribute apply エラー
- macOS 上での不安定な file operation

#### 変更時のルール

この周辺を変更する場合は、少なくとも以下を確認すること。

- `go test ./...`
- `lsshfs` の実 mount 確認
- macOS 上での file create / edit / delete 確認
- 不要ファイルや 0 byte ファイルが発生しないこと
