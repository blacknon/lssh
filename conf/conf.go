// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

/*
conf is a package used to read configuration file (~/.lssh.conf).
*/

// TODO(blacknon): 各種クラウドの踏み台経由でのアクセスに対応する
//                 - AWS SSM(セッションマネージャー)
//                 - Azure Bastion
//                 - GCP(gcloud compute ssh)

// TODO(blacknon): if/whenなどを使って、条件に応じて設定を追加するような仕組みを実装したい
//                 ex) 現在のipアドレスのrangeが192.168.10.0/24 => xxxのnwだからproxy serverが必要

package conf

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/blacknon/lssh/common"
)

// LogConfig store the contents about the terminal log.
// The log file name is created in "YYYYmmdd_HHMMSS_servername.log" of the specified directory.
type LogConfig struct {
	// Enable terminal logging.
	Enable bool `toml:"enable"`

	// Add a timestamp at the beginning of the terminal log line.
	Timestamp bool `toml:"timestamp"`

	// Specifies the directory for creating terminal logs.
	Dir string `toml:"dirpath"`

	// Logging with remove ANSI code.
	RemoveAnsiCode bool `toml:"remove_ansi_code"`
}

// ShellConfig Structure for storing lssh-shell settings.
type ShellConfig struct {
	// prompt
	Prompt  string `toml:"PROMPT"`  // lssh shell prompt
	OPrompt string `toml:"OPROMPT"` // lssh shell output prompt

	// message,title etc...
	Title string `toml:"title"`

	// history file
	HistoryFile string `toml:"histfile"`

	// pre | post command setting
	PreCmd  string `toml:"pre_cmd"`
	PostCmd string `toml:"post_cmd"`
}

// IncludeConfig specify the configuration file to include (ServerConfig only).
type IncludeConfig struct {
	Path string `toml:"path"`
}

// IncludesConfig specify the configuration file to include (ServerConfig only).
// Struct that can specify multiple files in array.
// TODO: ワイルドカード指定可能にする
type IncludesConfig struct {
	// example:
	// 	path = [
	// 		 "~/.lssh.d/home.conf"
	// 		,"~/.lssh.d/cloud.conf"
	// 	]
	Path []string `toml:"path"`
}

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

// ProxyConfig is that stores Proxy server settings connected via http and socks5.
type ProxyConfig struct {
	Addr      string `toml:"addr"`
	Port      string `toml:"port"`
	User      string `toml:"user"`
	Pass      string `toml:"pass"`
	Proxy     string `toml:"proxy"`
	ProxyType string `toml:"proxy_type"`
	Note      string `toml:"note"`
}

// OpenSSHConfig is read OpenSSH configuration file.
type OpenSSHConfig struct {
	Path    string `toml:"path"` // This is preferred
	Command string `toml:"command"`
	ServerConfig
}

// PortForward
type PortForward struct {
	Mode   string // L or R.
	Local  string // localhost:8080
	Remote string // localhost:80
}

// Config is Struct that stores the entire configuration file.
type Config struct {
	Log      LogConfig
	Shell    ShellConfig
	Include  map[string]IncludeConfig
	Includes IncludesConfig
	Common   ServerConfig
	Server   map[string]ServerConfig
	Proxy    map[string]ProxyConfig

	SSHConfig map[string]OpenSSHConfig
}

func (c *Config) ReduceCommon() {
	// reduce common setting (in .lssh.conf servers)
	for key, value := range c.Server {
		setValue := serverConfigReduct(c.Common, value)
		c.Server[key] = setValue
	}
}

func (c *Config) ReadOpenSSHConfig() {
	if len(c.SSHConfig) == 0 {
		openSSHServerConfig, err := getOpenSSHConfig("~/.ssh/config", "")
		if err == nil {
			// append data
			for key, value := range openSSHServerConfig {
				value := serverConfigReduct(c.Common, value)
				c.Server[key] = value
			}
		}
	} else {
		for _, sc := range c.SSHConfig {
			openSSHServerConfig, err := getOpenSSHConfig(sc.Path, sc.Command)
			if err == nil {
				// append data
				for key, value := range openSSHServerConfig {
					setCommon := serverConfigReduct(c.Common, sc.ServerConfig)
					value = serverConfigReduct(setCommon, value)
					c.Server[key] = value
				}
			}
		}
	}
}

func (c *Config) ReadIncludeFiles() {
	if c.Includes.Path != nil {
		if c.Include == nil {
			c.Include = map[string]IncludeConfig{}
		}

		for _, includePath := range c.Includes.Path {
			// get abs path
			includePath = common.GetFullPath(includePath)

			unixTime := time.Now().Unix()
			keyString := strings.Join([]string{string(unixTime), includePath}, "_")

			// key to md5
			hasher := md5.New()
			hasher.Write([]byte(keyString))
			key := string(hex.EncodeToString(hasher.Sum(nil)))

			// append c.Include[key]
			c.Include[key] = IncludeConfig{includePath}
		}
	}

	// Read include files
	if c.Include != nil {
		for _, v := range c.Include {
			var includeConf Config

			// user path
			path := common.GetFullPath(v.Path)

			// Read include config file
			_, err := toml.DecodeFile(path, &includeConf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "err: Read config file error: ", err)
				os.Exit(1)
			}

			// reduce common setting
			setCommon := serverConfigReduct(c.Common, includeConf.Common)

			// add include file serverconf
			for key, value := range includeConf.Server {
				// reduce common setting
				setValue := serverConfigReduct(setCommon, value)
				c.Server[key] = setValue
			}
		}
	}
}

// checkFormatServerConf checkes format of server config.
//
// Note: Checking Addr, User and authentications
// having a value. No checking a validity of each fields.
//
// See also: checkFormatServerConfAuth function.
func (c *Config) checkFormatServerConf() (ok bool) {
	ok = true
	for k, v := range c.Server {
		// Address Set Check
		if v.Addr == "" {
			fmt.Printf("%s: 'addr' is not set.\n", k)
			ok = false
		}

		// User Set Check
		if v.User == "" {
			fmt.Printf("%s: 'user' is not set.\n", k)
			ok = false
		}

		if !checkFormatServerConfAuth(v) {
			fmt.Printf("%s: Authentication information is not set.\n", k)
			ok = false
		}
	}
	return
}

// ReadConf load configuration file and return Config structure
// TODO(blacknon): リファクタリング！(v0.6.5) 外出しや処理のまとめなど
func Read(confPath string) (c Config) {
	c.Server = map[string]ServerConfig{}
	c.SSHConfig = map[string]OpenSSHConfig{}

	// TODO(blacknon): ~/.lssh.confがなくても、openssh用のファイルがアレばそれをみるように処理
	if common.IsExist(confPath) {
		// Read config file
		_, err := toml.DecodeFile(confPath, &c)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	// reduce common setting (in .lssh.conf servers)
	c.ReduceCommon()

	// Read OpensSH configs
	c.ReadOpenSSHConfig()

	// for append includes to include.path
	c.ReadIncludeFiles()

	// Check Config Parameter
	ok := c.checkFormatServerConf()
	if !ok {
		os.Exit(1)
	}

	return
}

// checkFormatServerConfAuth checkes format of server config authentication.
//
// Note: Checking Pass, Key, Cert, AgentAuth, PKCS11Use, PKCS11Provider, Keys or
// Passes having a value. No checking a validity of each fields.
func checkFormatServerConfAuth(c ServerConfig) (ok bool) {
	ok = false
	if c.Pass != "" || c.Key != "" || c.Cert != "" {
		ok = true
	}

	if c.AgentAuth == true {
		ok = true
	}

	if c.PKCS11Use == true {
		_, err := os.Stat(c.PKCS11Provider)
		if err == nil {
			ok = true
		}
	}

	if len(c.Keys) > 0 || len(c.Passes) > 0 {
		ok = true
	}

	return
}

// serverConfigReduct returns a new server config that set perConfig field to
// childConfig empty filed.
func serverConfigReduct(perConfig, childConfig ServerConfig) ServerConfig {
	result := ServerConfig{}

	// struct to map
	perConfigMap, _ := common.StructToMap(&perConfig)
	childConfigMap, _ := common.StructToMap(&childConfig)

	resultMap := common.MapReduce(perConfigMap, childConfigMap)
	_ = common.MapToStruct(resultMap, &result)

	return result
}

// GetNameList return a list of server names from the Config structure.
func GetNameList(listConf Config) (nameList []string) {
	for k := range listConf.Server {
		nameList = append(nameList, k)
	}
	return
}
