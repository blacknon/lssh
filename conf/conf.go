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

type Config struct {
	Log      LogConfig
	Shell    ShellConfig
	Include  map[string]IncludeConfig
	Includes IncludesConfig
	Common   ServerConfig
	Server   map[string]ServerConfig
	Proxy    map[string]ProxyConfig
}

type LogConfig struct {
	Enable    bool   `toml:"enable"`
	Timestamp bool   `toml:"timestamp"`
	Dir       string `toml:"dirpath"`
}

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

type IncludeConfig struct {
	Path string `toml:"path"`
}

// multiple include path
type IncludesConfig struct {
	Path []string `toml:"path"`
}

type ServerConfig struct {
	// Connect Setting
	Addr string `toml:"addr"`
	Port string `toml:"port"`
	User string `toml:"user"`

	// Connect auth Setting
	Pass            string   `toml:"pass"`
	Passes          []string `toml:"passes"`
	Key             string   `toml:"key"`
	KeyPass         string   `toml:"keypass"`
	Keys            []string `toml:"keys"` // "keypath::passphase"
	Cert            string   `toml:"cert"`
	CertKey         string   `toml:"certkey"`
	CertKeyPass     string   `toml:"certkeypass"`
	AgentAuth       bool     `toml:"agentauth"`
	SSHAgentUse     bool     `toml:"ssh_agent"`
	SSHAgentKeyPath []string `toml:"ssh_agent_key"` // "keypath::passphase"
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

	// port forwarding setting
	PortForwardLocal  string `toml:"port_forward_local"`  // port forward (local). "host:port"
	PortForwardRemote string `toml:"port_forward_remote"` // port forward (remote). "host:port"
	Note              string `toml:"note"`
}

type ProxyConfig struct {
	Addr string `toml:"addr"`
	Port string `toml:"port"`
	User string `toml:"user"`
	Pass string `toml:"pass"`
	Note string `toml:"note"`
}

func ReadConf(confPath string) (checkConf Config) {
	// user path
	usr, _ := user.Current()

	if !common.IsExist(confPath) {
		fmt.Printf("Config file(%s) Not Found.\nPlease create file.\n\n", confPath)
		fmt.Printf("sample: %s\n", "https://raw.githubusercontent.com/blacknon/lssh/master/example/config.tml")
		os.Exit(1)
	}

	// Read config file
	_, err := toml.DecodeFile(confPath, &checkConf)
	if err != nil {
		panic(err)
	}

	// reduce common setting (in .lssh.conf servers)
	for key, value := range checkConf.Server {
		setValue := serverConfigReduct(checkConf.Common, value)
		checkConf.Server[key] = setValue
	}

	// for append includes to include.path
	if checkConf.Includes.Path != nil {
		if checkConf.Include == nil {
			checkConf.Include = map[string]IncludeConfig{}
		}

		for _, includePath := range checkConf.Includes.Path {
			unixTime := time.Now().Unix()
			keyString := strings.Join([]string{string(unixTime), includePath}, "_")

			// key to md5
			hasher := md5.New()
			hasher.Write([]byte(keyString))
			key := string(hex.EncodeToString(hasher.Sum(nil)))

			// append checkConf.Include[key]
			checkConf.Include[key] = IncludeConfig{strings.Replace(includePath, "~", usr.HomeDir, 1)}
		}
	}

	// Read include files
	if checkConf.Include != nil {
		for _, v := range checkConf.Include {
			var includeConf Config

			// user path
			path := strings.Replace(v.Path, "~", usr.HomeDir, 1)

			// Read include config file
			_, err := toml.DecodeFile(path, &includeConf)
			if err != nil {
				panic(err)
			}

			// reduce common setting
			setCommon := serverConfigReduct(checkConf.Common, includeConf.Common)

			// map init
			if len(checkConf.Server) == 0 {
				checkConf.Server = map[string]ServerConfig{}
			}

			// add include file serverconf
			for key, value := range includeConf.Server {
				// reduce common setting
				setValue := serverConfigReduct(setCommon, value)
				checkConf.Server[key] = setValue
			}
		}
	}

	// Check Config Parameter
	checkAlertFlag := checkFormatServerConf(checkConf)
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

func GetNameList(listConf Config) (nameList []string) {
	for k := range listConf.Server {
		nameList = append(nameList, k)
	}
	return
}
