// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

/*
conf is a package used to read configuration file (~/.lssh.conf).
*/
package conf

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/blacknon/lssh/common"
)

// Config is Struct that stores the entire configuration file
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

// LogConfig store the contents about the terminal log.
// The log file name is created in "YYYYmmdd_HHMMSS_servername.log" of the specified directory.
type LogConfig struct {
	// Enable terminal logging.
	Enable bool `toml:"enable"`

	// Add a timestamp at the beginning of the terminal log line.
	Timestamp bool `toml:"timestamp"`

	// Specifies the directory for creating terminal logs.
	Dir string `toml:"dirpath"`
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

	// pre | post command setting
	PreCmd  string `toml:"pre_cmd"`
	PostCmd string `toml:"post_cmd"`

	// proxy setting
	ProxyType    string `toml:"proxy_type"`
	Proxy        string `toml:"proxy"`
	ProxyCommand string `toml:"proxy_cmd"` // OpenSSH type proxy setting

	// local rcfile setting
	LocalRcUse       string   `toml:"local_rc"` // yes|no (default: yes)
	LocalRcPath      []string `toml:"local_rc_file"`
	LocalRcDecodeCmd string   `toml:"local_rc_decode_cmd"`

	// local/remote port forwarding setting
	PortForwardMode   string `toml:"port_forward"`        // [`L`,`l`,`LOCAL`,`local`]|[`R`,`r`,`REMOTE`,`remote`]
	PortForwardLocal  string `toml:"port_forward_local"`  // port forward (local). "host:port"
	PortForwardRemote string `toml:"port_forward_remote"` // port forward (remote). "host:port"

	// local/remote port forwarding settings
	// {[`L`,`l`,`LOCAL`,`local`]|[`R`,`r`,`REMOTE`,`remote`]}:[localaddress]:[localport]:[remoteaddress]:[remoteport]
	PortForwards []string `toml:"port_forwards"`

	// local/remote Port Forwarding slice.
	Forwards []*PortForward

	// Dynamic Port Forwarding setting
	DynamicPortForward string `toml:"dynamic_port_forward"` // ex.) "11080"

	// x11 forwarding setting
	X11 bool `toml:"x11"`

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

// ReadConf load configuration file and return Config structure
// TODO(blacknon): リファクタリング！(v0.6.1) 外出しや処理のまとめなど
func ReadConf(confPath string) (config Config) {
	// user path
	usr, _ := user.Current()

	if !common.IsExist(confPath) {
		fmt.Printf("Config file(%s) Not Found.\nPlease create file.\n\n", confPath)
		fmt.Printf("sample: %s\n", "https://raw.githubusercontent.com/blacknon/lssh/master/example/config.tml")
		os.Exit(1)
	}

	config.Server = map[string]ServerConfig{}
	config.SSHConfig = map[string]OpenSSHConfig{}

	// Read config file
	_, err := toml.DecodeFile(confPath, &config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// reduce common setting (in .lssh.conf servers)
	for key, value := range config.Server {
		setValue := serverConfigReduct(config.Common, value)
		config.Server[key] = setValue
	}

	// Read OpensSH configs
	if len(config.SSHConfig) == 0 {
		openSSHServerConfig, err := getOpenSSHConfig("~/.ssh/config", "")
		if err == nil {
			// append data
			for key, value := range openSSHServerConfig {
				value := serverConfigReduct(config.Common, value)
				config.Server[key] = value
			}
		}
	} else {
		for _, sshConfig := range config.SSHConfig {
			openSSHServerConfig, err := getOpenSSHConfig(sshConfig.Path, sshConfig.Command)
			if err == nil {
				// append data
				for key, value := range openSSHServerConfig {
					setCommon := serverConfigReduct(config.Common, sshConfig.ServerConfig)
					value = serverConfigReduct(setCommon, value)
					config.Server[key] = value
				}
			}
		}
	}

	// for append includes to include.path
	if config.Includes.Path != nil {
		if config.Include == nil {
			config.Include = map[string]IncludeConfig{}
		}

		for _, includePath := range config.Includes.Path {
			unixTime := time.Now().Unix()
			keyString := strings.Join([]string{string(unixTime), includePath}, "_")

			// key to md5
			hasher := md5.New()
			hasher.Write([]byte(keyString))
			key := string(hex.EncodeToString(hasher.Sum(nil)))

			// append config.Include[key]
			config.Include[key] = IncludeConfig{strings.Replace(includePath, "~", usr.HomeDir, 1)}
		}
	}

	// Read include files
	if config.Include != nil {
		for _, v := range config.Include {
			var includeConf Config

			// user path
			path := strings.Replace(v.Path, "~", usr.HomeDir, 1)

			// Read include config file
			_, err := toml.DecodeFile(path, &includeConf)
			if err != nil {
				fmt.Fprintf(os.Stderr, "err: Read config file error: ", err)
				os.Exit(1)
			}

			// reduce common setting
			setCommon := serverConfigReduct(config.Common, includeConf.Common)

			// map init
			if len(config.Server) == 0 {
				config.Server = map[string]ServerConfig{}
			}

			// add include file serverconf
			for key, value := range includeConf.Server {
				// reduce common setting
				setValue := serverConfigReduct(setCommon, value)
				config.Server[key] = setValue
			}
		}
	}

	// Check Config Parameter
	checkAlertFlag := checkFormatServerConf(config)
	if !checkAlertFlag {
		os.Exit(1)
	}

	return
}

// checkFormatServerConf checkes format of server config.
//
// Note: Checking Addr, User and authentications
// having a value. No checking a validity of each fields.
//
// See also: checkFormatServerConfAuth function.
func checkFormatServerConf(c Config) (isFormat bool) {
	isFormat = true
	for k, v := range c.Server {
		// Address Set Check
		if v.Addr == "" {
			fmt.Printf("%s: 'addr' is not set.\n", k)
			isFormat = false
		}

		// User Set Check
		if v.User == "" {
			fmt.Printf("%s: 'user' is not set.\n", k)
			isFormat = false
		}

		if !checkFormatServerConfAuth(v) {
			fmt.Printf("%s: Authentication information is not set.\n", k)
			isFormat = false
		}
	}
	return
}

// checkFormatServerConfAuth checkes format of server config authentication.
//
// Note: Checking Pass, Key, Cert, AgentAuth, PKCS11Use, PKCS11Provider, Keys or
// Passes having a value. No checking a validity of each fields.
func checkFormatServerConfAuth(c ServerConfig) (isFormat bool) {
	isFormat = false
	if c.Pass != "" || c.Key != "" || c.Cert != "" {
		isFormat = true
	}

	if c.AgentAuth == true {
		isFormat = true
	}

	if c.PKCS11Use == true {
		_, err := os.Stat(c.PKCS11Provider)
		if err == nil {
			isFormat = true
		}
	}

	if len(c.Keys) > 0 || len(c.Passes) > 0 {
		isFormat = true
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
