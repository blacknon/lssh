/*
ssh package is that describes the whole process of connecting to ssh with lssh.

TODO(blacknon): 以下の機能について、汎用ライブラリとして外出しする
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
    - フォワード系
        - PortForward
        - X11Forward
    - シェルへの接続周り(local bashrcについては組み込まない)
*/
package ssh
