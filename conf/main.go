// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

/*
conf is a package used to read configuration file (~/.lssh.conf).
*/

// TODO(blacknon): 各種クラウドの踏み台経由でのアクセスに対応する => pluginで処理させたいお気持ち
//                 - AWS SSM(セッションマネージャー)
//                 - Azure Bastion
//                 - GCP(gcloud compute ssh)

// TODO(blacknon): if/whenなどを使って、条件に応じて設定を追加するような仕組みを実装したい(v0.7.0)
//                 ex) 現在のipアドレスのrangeが192.168.10.0/24 => xxxのnwだからproxy serverが必要

// TODO(blacknon): 接続成功時に特定のコマンドを実行可能にする(接続前しか今はないので)

// TODO(blacknon): sshだけではなく、telnetやWinrmなどのプロトコルにも対応したい(v0.8.0)
//                 ※ たぶん、実現するならプラグインを追加できるようにするのがよさそう

package conf

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/blacknon/lssh/common"
)

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

// ReduceCommon reduce common setting (in .lssh.conf servers)
func (c *Config) ReduceCommon() {
	for key, value := range c.Server {
		setValue := serverConfigReduct(c.Common, value)
		c.Server[key] = setValue
	}
}

// ReadOpenSSHConfig read OpenSSH config file and append to Config.Server.
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

// ReadIncludeFiles read include files and append to Config.Server.
func (c *Config) ReadIncludeFiles() {
	if c.Includes.Path != nil {
		if c.Include == nil {
			c.Include = map[string]IncludeConfig{}
		}

		for _, includePath := range c.Includes.Path {
			// get abs path
			includePath = common.GetFullPath(includePath)

			unixTime := time.Now().Unix()
			keyString := strings.Join([]string{fmt.Sprint(unixTime), includePath}, "_")

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
				fmt.Fprintf(os.Stderr, "err: Read config file error: %s", err)
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
			log.Printf("%s: 'addr' is not set.\n", k)
			ok = false
		}

		// User Set Check
		if v.User == "" {
			log.Printf("%s: 'user' is not set.\n", k)
			ok = false
		}

		if !checkFormatServerConfAuth(v) {
			log.Printf("%s: Authentication information is not set.\n", k)
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
			log.Println(err)
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
