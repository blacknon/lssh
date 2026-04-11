// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

// ServerConfig Structure for holding SSH connection information
type ServerConfig struct {
	// Connect basic Setting
	Addr string `toml:"addr" yaml:"addr"`
	Port string `toml:"port" yaml:"port"`
	User string `toml:"user" yaml:"user"`

	// Connect auth Setting
	Pass            string   `toml:"pass" yaml:"pass"`
	Passes          []string `toml:"passes" yaml:"passes"`
	Key             string   `toml:"key" yaml:"key"`
	KeyCommand      string   `toml:"keycmd" yaml:"keycmd"`
	KeyCommandPass  string   `toml:"keycmdpass" yaml:"keycmdpass"`
	KeyPass         string   `toml:"keypass" yaml:"keypass"`
	Keys            []string `toml:"keys" yaml:"keys"` // "keypath::passphrase"
	Cert            string   `toml:"cert" yaml:"cert"`
	Certs           []string `toml:"certs" yaml:"certs"` // "certpath::keypath::passphrase"
	CertKey         string   `toml:"certkey" yaml:"certkey"`
	CertKeyPass     string   `toml:"certkeypass" yaml:"certkeypass"`
	CertPKCS11      bool     `toml:"certpkcs11" yaml:"certpkcs11"`
	AgentAuth       bool     `toml:"agentauth" yaml:"agentauth"`
	SSHAgentUse     bool     `toml:"ssh_agent" yaml:"ssh_agent"`
	SSHAgentKeyPath []string `toml:"ssh_agent_key" yaml:"ssh_agent_key"` // "keypath::passphrase"
	PKCS11Use       bool     `toml:"pkcs11" yaml:"pkcs11"`
	PKCS11Provider  string   `toml:"pkcs11provider" yaml:"pkcs11provider"` // PKCS11 Provider PATH
	PKCS11PIN       string   `toml:"pkcs11pin" yaml:"pkcs11pin"`           // PKCS11 PIN code

	// pre execute command
	PreCmd string `toml:"pre_cmd" yaml:"pre_cmd"`

	// post execute command
	PostCmd string `toml:"post_cmd" yaml:"post_cmd"`

	// proxy setting
	ProxyType string `toml:"proxy_type" yaml:"proxy_type"`

	Proxy string `toml:"proxy" yaml:"proxy"`

	// OpenSSH type proxy setting
	ProxyCommand string `toml:"proxy_cmd" yaml:"proxy_cmd"`

	// local rcfile setting
	// yes|no (default: yes)
	LocalRcUse string `toml:"local_rc" yaml:"local_rc"`

	// LocalRcPath
	LocalRcPath []string `toml:"local_rc_file" yaml:"local_rc_file"`

	// If LocalRcCompress is true, gzip the localrc file to base64
	LocalRcCompress bool `toml:"local_rc_compress" yaml:"local_rc_compress"`

	// LocalRcDecodeCmd is localrc decode command. run remote machine.
	LocalRcDecodeCmd string `toml:"local_rc_decode_cmd" yaml:"local_rc_decode_cmd"`

	// LocalRcUncompressCmd is localrc un compress command. run remote machine.
	LocalRcUncompressCmd string `toml:"local_rc_uncompress_cmd" yaml:"local_rc_uncompress_cmd"`

	// local/remote port forwarding setting.
	// ex. [`L`,`l`,`LOCAL`,`local`]|[`R`,`r`,`REMOTE`,`remote`]
	PortForwardMode string `toml:"port_forward" yaml:"port_forward"`

	// port forward (local). "host:port"
	PortForwardLocal string `toml:"port_forward_local" yaml:"port_forward_local"`

	// port forward (remote). "host:port"
	PortForwardRemote string `toml:"port_forward_remote" yaml:"port_forward_remote"`

	// local/remote port forwarding settings
	// ex. {[`L`,`l`,`LOCAL`,`local`]|[`R`,`r`,`REMOTE`,`remote`]}:[localaddress]:[localport]:[remoteaddress]:[remoteport]
	PortForwards []string `toml:"port_forwards" yaml:"port_forwards"`

	// local/remote Port Forwarding slice.
	Forwards []*PortForward

	// Dynamic Port Forward setting
	// ex.) "11080"
	DynamicPortForward string `toml:"dynamic_port_forward" yaml:"dynamic_port_forward"`

	// Reverse Dynamic Port Forward setting
	// ex.) "11080"
	ReverseDynamicPortForward string `toml:"reverse_dynamic_port_forward" yaml:"reverse_dynamic_port_forward"`

	// HTTP Dynamic Port Forward setting
	// ex.) "11080"
	HTTPDynamicPortForward string `toml:"http_dynamic_port_forward" yaml:"http_dynamic_port_forward"`

	// HTTP Reverse Dynamic Port Forward setting
	// ex.) "11080"
	HTTPReverseDynamicPortForward string `toml:"http_reverse_dynamic_port_forward" yaml:"http_reverse_dynamic_port_forward"`

	// NFS Dynamic Forward port setting
	// ex.) "12049"
	NFSDynamicForwardPort string `toml:"nfs_dynamic_forward" yaml:"nfs_dynamic_forward"`

	// NFS Dynamic Forward path setting
	// ex.) "/path/to/remote"
	NFSDynamicForwardPath string `toml:"nfs_dynamic_forward_path" yaml:"nfs_dynamic_forward_path"`

	// NFS Reverse Dynamic Forward port setting
	// ex.) "12049"
	NFSReverseDynamicForwardPort string `toml:"nfs_reverse_dynamic_forward" yaml:"nfs_reverse_dynamic_forward"`

	// NFS Reverse Dynamic Forward path setting
	// ex.) "/path/to/local"
	NFSReverseDynamicForwardPath string `toml:"nfs_reverse_dynamic_forward_path" yaml:"nfs_reverse_dynamic_forward_path"`

	// SMB Dynamic Forward port setting
	// ex.) "12445"
	SMBDynamicForwardPort string `toml:"smb_dynamic_forward" yaml:"smb_dynamic_forward"`

	// SMB Dynamic Forward path setting
	// ex.) "/path/to/remote"
	SMBDynamicForwardPath string `toml:"smb_dynamic_forward_path" yaml:"smb_dynamic_forward_path"`

	// SMB Reverse Dynamic Forward port setting
	// ex.) "12445"
	SMBReverseDynamicForwardPort string `toml:"smb_reverse_dynamic_forward" yaml:"smb_reverse_dynamic_forward"`

	// SMB Reverse Dynamic Forward path setting
	// ex.) "/path/to/local"
	SMBReverseDynamicForwardPath string `toml:"smb_reverse_dynamic_forward_path" yaml:"smb_reverse_dynamic_forward_path"`

	// x11 forwarding setting
	X11 bool `toml:"x11" yaml:"x11"`

	// x11 trusted forwarding setting
	X11Trusted bool `toml:"x11_trusted" yaml:"x11_trusted"`

	// Connection Timeout second
	ConnectTimeout int `toml:"connect_timeout" yaml:"connect_timeout"`

	// Server Alive
	ServerAliveCountMax      int `toml:"alive_max" yaml:"alive_max"`
	ServerAliveCountInterval int `toml:"alive_interval" yaml:"alive_interval"`

	// Check KnownHosts
	CheckKnownHosts bool `toml:"check_known_hosts" yaml:"check_known_hosts"`

	// Check KnownHosts File
	KnownHostsFiles []string `toml:"known_hosts_files" yaml:"known_hosts_files"`

	// OpenSSH ControlMaster settings
	ControlMaster  bool                   `toml:"control_master" yaml:"control_master"`
	ControlPath    string                 `toml:"control_path" yaml:"control_path"`
	ControlPersist ControlPersistDuration `toml:"control_persist" yaml:"control_persist"`

	// note
	Note string `toml:"note" yaml:"note"`

	// ignore this server from selection / execution targets
	Ignore bool `toml:"ignore" yaml:"ignore"`

	// Conditional overrides under [server.<name>.match.<branch>]
	Match map[string]ServerMatchConfig `toml:"match" yaml:"match"`
}

// ServerMatchWhen stores match conditions for conditional server overrides.
type ServerMatchWhen struct {
	LocalIPIn     []string `toml:"local_ip_in" yaml:"local_ip_in"`
	LocalIPNotIn  []string `toml:"local_ip_not_in" yaml:"local_ip_not_in"`
	GatewayIn     []string `toml:"gateway_in" yaml:"gateway_in"`
	GatewayNotIn  []string `toml:"gateway_not_in" yaml:"gateway_not_in"`
	UsernameIn    []string `toml:"username_in" yaml:"username_in"`
	UsernameNotIn []string `toml:"username_not_in" yaml:"username_not_in"`
	HostnameIn    []string `toml:"hostname_in" yaml:"hostname_in"`
	HostnameNotIn []string `toml:"hostname_not_in" yaml:"hostname_not_in"`
	OSIn          []string `toml:"os_in" yaml:"os_in"`
	OSNotIn       []string `toml:"os_not_in" yaml:"os_not_in"`
	TermIn        []string `toml:"term_in" yaml:"term_in"`
	TermNotIn     []string `toml:"term_not_in" yaml:"term_not_in"`
	EnvIn         []string `toml:"env_in" yaml:"env_in"`
	EnvNotIn      []string `toml:"env_not_in" yaml:"env_not_in"`
	EnvValueIn    []string `toml:"env_value_in" yaml:"env_value_in"`
	EnvValueNotIn []string `toml:"env_value_not_in" yaml:"env_value_not_in"`
}

// Empty reports whether no match conditions are defined.
func (w ServerMatchWhen) Empty() bool {
	return len(w.LocalIPIn) == 0 &&
		len(w.LocalIPNotIn) == 0 &&
		len(w.GatewayIn) == 0 &&
		len(w.GatewayNotIn) == 0 &&
		len(w.UsernameIn) == 0 &&
		len(w.UsernameNotIn) == 0 &&
		len(w.HostnameIn) == 0 &&
		len(w.HostnameNotIn) == 0 &&
		len(w.OSIn) == 0 &&
		len(w.OSNotIn) == 0 &&
		len(w.TermIn) == 0 &&
		len(w.TermNotIn) == 0 &&
		len(w.EnvIn) == 0 &&
		len(w.EnvNotIn) == 0 &&
		len(w.EnvValueIn) == 0 &&
		len(w.EnvValueNotIn) == 0
}

// ServerMatchConfig stores a single conditional override branch.
//
// Keep override fields aligned with ServerConfig so branch tables can override
// the same keys as normal server definitions.
type ServerMatchConfig struct {
	Addr string `toml:"addr" yaml:"addr"`
	Port string `toml:"port" yaml:"port"`
	User string `toml:"user" yaml:"user"`

	Pass            string   `toml:"pass" yaml:"pass"`
	Passes          []string `toml:"passes" yaml:"passes"`
	Key             string   `toml:"key" yaml:"key"`
	KeyCommand      string   `toml:"keycmd" yaml:"keycmd"`
	KeyCommandPass  string   `toml:"keycmdpass" yaml:"keycmdpass"`
	KeyPass         string   `toml:"keypass" yaml:"keypass"`
	Keys            []string `toml:"keys" yaml:"keys"`
	Cert            string   `toml:"cert" yaml:"cert"`
	Certs           []string `toml:"certs" yaml:"certs"`
	CertKey         string   `toml:"certkey" yaml:"certkey"`
	CertKeyPass     string   `toml:"certkeypass" yaml:"certkeypass"`
	CertPKCS11      bool     `toml:"certpkcs11" yaml:"certpkcs11"`
	AgentAuth       bool     `toml:"agentauth" yaml:"agentauth"`
	SSHAgentUse     bool     `toml:"ssh_agent" yaml:"ssh_agent"`
	SSHAgentKeyPath []string `toml:"ssh_agent_key" yaml:"ssh_agent_key"`
	PKCS11Use       bool     `toml:"pkcs11" yaml:"pkcs11"`
	PKCS11Provider  string   `toml:"pkcs11provider" yaml:"pkcs11provider"`
	PKCS11PIN       string   `toml:"pkcs11pin" yaml:"pkcs11pin"`

	PreCmd       string `toml:"pre_cmd" yaml:"pre_cmd"`
	PostCmd      string `toml:"post_cmd" yaml:"post_cmd"`
	ProxyType    string `toml:"proxy_type" yaml:"proxy_type"`
	Proxy        string `toml:"proxy" yaml:"proxy"`
	ProxyCommand string `toml:"proxy_cmd" yaml:"proxy_cmd"`

	LocalRcUse           string   `toml:"local_rc" yaml:"local_rc"`
	LocalRcPath          []string `toml:"local_rc_file" yaml:"local_rc_file"`
	LocalRcCompress      bool     `toml:"local_rc_compress" yaml:"local_rc_compress"`
	LocalRcDecodeCmd     string   `toml:"local_rc_decode_cmd" yaml:"local_rc_decode_cmd"`
	LocalRcUncompressCmd string   `toml:"local_rc_uncompress_cmd" yaml:"local_rc_uncompress_cmd"`

	PortForwardMode               string   `toml:"port_forward" yaml:"port_forward"`
	PortForwardLocal              string   `toml:"port_forward_local" yaml:"port_forward_local"`
	PortForwardRemote             string   `toml:"port_forward_remote" yaml:"port_forward_remote"`
	PortForwards                  []string `toml:"port_forwards" yaml:"port_forwards"`
	Forwards                      []*PortForward
	DynamicPortForward            string `toml:"dynamic_port_forward" yaml:"dynamic_port_forward"`
	ReverseDynamicPortForward     string `toml:"reverse_dynamic_port_forward" yaml:"reverse_dynamic_port_forward"`
	HTTPDynamicPortForward        string `toml:"http_dynamic_port_forward" yaml:"http_dynamic_port_forward"`
	HTTPReverseDynamicPortForward string `toml:"http_reverse_dynamic_port_forward" yaml:"http_reverse_dynamic_port_forward"`
	NFSDynamicForwardPort         string `toml:"nfs_dynamic_forward" yaml:"nfs_dynamic_forward"`
	NFSDynamicForwardPath         string `toml:"nfs_dynamic_forward_path" yaml:"nfs_dynamic_forward_path"`
	NFSReverseDynamicForwardPort  string `toml:"nfs_reverse_dynamic_forward" yaml:"nfs_reverse_dynamic_forward"`
	NFSReverseDynamicForwardPath  string `toml:"nfs_reverse_dynamic_forward_path" yaml:"nfs_reverse_dynamic_forward_path"`
	SMBDynamicForwardPort         string `toml:"smb_dynamic_forward" yaml:"smb_dynamic_forward"`
	SMBDynamicForwardPath         string `toml:"smb_dynamic_forward_path" yaml:"smb_dynamic_forward_path"`
	SMBReverseDynamicForwardPort  string `toml:"smb_reverse_dynamic_forward" yaml:"smb_reverse_dynamic_forward"`
	SMBReverseDynamicForwardPath  string `toml:"smb_reverse_dynamic_forward_path" yaml:"smb_reverse_dynamic_forward_path"`

	X11        bool `toml:"x11" yaml:"x11"`
	X11Trusted bool `toml:"x11_trusted" yaml:"x11_trusted"`

	ConnectTimeout           int                    `toml:"connect_timeout" yaml:"connect_timeout"`
	ServerAliveCountMax      int                    `toml:"alive_max" yaml:"alive_max"`
	ServerAliveCountInterval int                    `toml:"alive_interval" yaml:"alive_interval"`
	CheckKnownHosts          bool                   `toml:"check_known_hosts" yaml:"check_known_hosts"`
	KnownHostsFiles          []string               `toml:"known_hosts_files" yaml:"known_hosts_files"`
	ControlMaster            bool                   `toml:"control_master" yaml:"control_master"`
	ControlPath              string                 `toml:"control_path" yaml:"control_path"`
	ControlPersist           ControlPersistDuration `toml:"control_persist" yaml:"control_persist"`
	Note                     string                 `toml:"note" yaml:"note"`
	Ignore                   bool                   `toml:"ignore" yaml:"ignore"`

	Priority int             `toml:"priority" yaml:"priority"`
	When     ServerMatchWhen `toml:"when" yaml:"when"`

	order           int
	priorityDefined bool
	definedKeys     map[string]bool
}

// EffectivePriority returns the branch priority, defaulting to 100 when omitted.
func (m ServerMatchConfig) EffectivePriority() int {
	if m.priorityDefined {
		return m.Priority
	}
	return 100
}

// OverrideConfig converts a branch override into a ServerConfig for merging.
func (m ServerMatchConfig) OverrideConfig() ServerConfig {
	return ServerConfig{
		Addr:                          m.Addr,
		Port:                          m.Port,
		User:                          m.User,
		Pass:                          m.Pass,
		Passes:                        m.Passes,
		Key:                           m.Key,
		KeyCommand:                    m.KeyCommand,
		KeyCommandPass:                m.KeyCommandPass,
		KeyPass:                       m.KeyPass,
		Keys:                          m.Keys,
		Cert:                          m.Cert,
		Certs:                         m.Certs,
		CertKey:                       m.CertKey,
		CertKeyPass:                   m.CertKeyPass,
		CertPKCS11:                    m.CertPKCS11,
		AgentAuth:                     m.AgentAuth,
		SSHAgentUse:                   m.SSHAgentUse,
		SSHAgentKeyPath:               m.SSHAgentKeyPath,
		PKCS11Use:                     m.PKCS11Use,
		PKCS11Provider:                m.PKCS11Provider,
		PKCS11PIN:                     m.PKCS11PIN,
		PreCmd:                        m.PreCmd,
		PostCmd:                       m.PostCmd,
		ProxyType:                     m.ProxyType,
		Proxy:                         m.Proxy,
		ProxyCommand:                  m.ProxyCommand,
		LocalRcUse:                    m.LocalRcUse,
		LocalRcPath:                   m.LocalRcPath,
		LocalRcCompress:               m.LocalRcCompress,
		LocalRcDecodeCmd:              m.LocalRcDecodeCmd,
		LocalRcUncompressCmd:          m.LocalRcUncompressCmd,
		PortForwardMode:               m.PortForwardMode,
		PortForwardLocal:              m.PortForwardLocal,
		PortForwardRemote:             m.PortForwardRemote,
		PortForwards:                  m.PortForwards,
		Forwards:                      m.Forwards,
		DynamicPortForward:            m.DynamicPortForward,
		ReverseDynamicPortForward:     m.ReverseDynamicPortForward,
		HTTPDynamicPortForward:        m.HTTPDynamicPortForward,
		HTTPReverseDynamicPortForward: m.HTTPReverseDynamicPortForward,
		NFSDynamicForwardPort:         m.NFSDynamicForwardPort,
		NFSDynamicForwardPath:         m.NFSDynamicForwardPath,
		NFSReverseDynamicForwardPort:  m.NFSReverseDynamicForwardPort,
		NFSReverseDynamicForwardPath:  m.NFSReverseDynamicForwardPath,
		SMBDynamicForwardPort:         m.SMBDynamicForwardPort,
		SMBDynamicForwardPath:         m.SMBDynamicForwardPath,
		SMBReverseDynamicForwardPort:  m.SMBReverseDynamicForwardPort,
		SMBReverseDynamicForwardPath:  m.SMBReverseDynamicForwardPath,
		X11:                           m.X11,
		X11Trusted:                    m.X11Trusted,
		ConnectTimeout:                m.ConnectTimeout,
		ServerAliveCountMax:           m.ServerAliveCountMax,
		ServerAliveCountInterval:      m.ServerAliveCountInterval,
		CheckKnownHosts:               m.CheckKnownHosts,
		KnownHostsFiles:               m.KnownHostsFiles,
		ControlMaster:                 m.ControlMaster,
		ControlPath:                   m.ControlPath,
		ControlPersist:                m.ControlPersist,
		Note:                          m.Note,
		Ignore:                        m.Ignore,
	}
}

// IsDefined reports whether the TOML key was explicitly set in the match branch.
func (m ServerMatchConfig) IsDefined(key string) bool {
	if len(m.definedKeys) == 0 {
		return false
	}
	return m.definedKeys[key]
}
