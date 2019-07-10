/*
ssh package is that describes the whole process of connecting to ssh with lssh.

TODO(blacknon): 以下の機能について、汎用ライブラリとして外出ししてしまう
    - 認証系(AuthMap系？)
        - 鍵認証
        - パスワード認証
        - 証明書認証
        - PKCS11認証
        - ssh-agent認証
    - プロキシ系
        - http/httpsプロキシ
        - socks5プロキシ
        - ssh多段プロキシ
    - ターミナル接続周り(キー入力をちゃんといけるように)
*/
package ssh
