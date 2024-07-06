// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

// ServerConfig Structure for holding SSH connection information
type ServerConfig struct {
	// Connect basic Setting
	Addr string `toml:"addr"`
	Port string `toml:"port"`
	User string `toml:"user"`

	// Connect auth Setting
	Pass            string   `toml:"pass"`
	Passes          []string `toml:"passes"`
	Key             string   `toml:"key"`
	KeyCommand      string   `toml:"keycmd"`
	KeyCommandPass  string   `toml:"keycmdpass"`
	KeyPass         string   `toml:"keypass"`
	Keys            []string `toml:"keys"` // "keypath::passphrase"
	Cert            string   `toml:"cert"`
	CertKey         string   `toml:"certkey"`
	CertKeyPass     string   `toml:"certkeypass"`
	CertPKCS11      bool     `toml:"certpkcs11"`
	AgentAuth       bool     `toml:"agentauth"`
	SSHAgentUse     bool     `toml:"ssh_agent"`
	SSHAgentKeyPath []string `toml:"ssh_agent_key"` // "keypath::passphrase"
	PKCS11Use       bool     `toml:"pkcs11"`
	PKCS11Provider  string   `toml:"pkcs11provider"` // PKCS11 Provider PATH
	PKCS11PIN       string   `toml:"pkcs11pin"`      // PKCS11 PIN code

	// pre execute command
	PreCmd string `toml:"pre_cmd"`

	// post execute command
	PostCmd string `toml:"post_cmd"`

	// proxy setting
	ProxyType string `toml:"proxy_type"`

	Proxy string `toml:"proxy"`

	// OpenSSH type proxy setting
	ProxyCommand string `toml:"proxy_cmd"`

	// local rcfile setting
	// yes|no (default: yes)
	LocalRcUse string `toml:"local_rc"`

	// LocalRcPath
	LocalRcPath []string `toml:"local_rc_file"`

	// If LocalRcCompress is true, gzip the localrc file to base64
	LocalRcCompress bool `toml:"local_rc_compress"`

	// LocalRcDecodeCmd is localrc decode command. run remote machine.
	LocalRcDecodeCmd string `toml:"local_rc_decode_cmd"`

	// LocalRcUncompressCmd is localrc un compress command. run remote machine.
	LocalRcUncompressCmd string `toml:"local_rc_uncompress_cmd"`

	// local/remote port forwarding setting.
	// ex. [`L`,`l`,`LOCAL`,`local`]|[`R`,`r`,`REMOTE`,`remote`]
	PortForwardMode string `toml:"port_forward"`

	// port forward (local). "host:port"
	PortForwardLocal string `toml:"port_forward_local"`

	// port forward (remote). "host:port"
	PortForwardRemote string `toml:"port_forward_remote"`

	// local/remote port forwarding settings
	// ex. {[`L`,`l`,`LOCAL`,`local`]|[`R`,`r`,`REMOTE`,`remote`]}:[localaddress]:[localport]:[remoteaddress]:[remoteport]
	PortForwards []string `toml:"port_forwards"`

	// local/remote Port Forwarding slice.
	Forwards []*PortForward

	// Dynamic Port Forward setting
	// ex.) "11080"
	DynamicPortForward string `toml:"dynamic_port_forward"`

	// Reverse Dynamic Port Forward setting
	// ex.) "11080"
	ReverseDynamicPortForward string `toml:"reverse_dynamic_port_forward"`

	// HTTP Dynamic Port Forward setting
	// ex.) "11080"
	HTTPDynamicPortForward string `toml:"http_dynamic_port_forward"`

	// HTTP Reverse Dynamic Port Forward setting
	// ex.) "11080"
	HTTPReverseDynamicPortForward string `toml:"http_reverse_dynamic_port_forward"`

	// x11 forwarding setting
	X11 bool `toml:"x11"`

	// x11 trusted forwarding setting
	X11Trusted bool `toml:"x11_trusted"`

	// Connection Timeout second
	ConnectTimeout int `toml:"connect_timeout"`

	// Server Alive
	ServerAliveCountMax      int `toml:"alive_max"`
	ServerAliveCountInterval int `toml:"alive_interval"`

	// note
	Note string `toml:"note"`
}
