// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

/*
conf is a package used to read configuration file (~/.lssh.conf).
*/

// TODO(blacknon): 1password managerなどの外部ツールと連携して、パスワードや秘密鍵の管理を行えるようにする(v0.7.X)
// TODO(blacknon): if/whenなどを使って、条件に応じて設定を追加するような仕組みを実装したい(v0.7.X)
//                 ex) 現在のipアドレスのrangeが192.168.10.0/24 => xxxのnwだからproxy serverが必要、という分岐機能の追加をする

// TODO(blacknon): 接続成功時に特定のコマンドを実行可能にする(接続前しか今はないので): (v0.7.X)

// TODO(blacknon): sshだけではなく、telnetやWinrmなどのプロトコルにも対応したい(v0.8.0)
//
//	※ たぶん、実現するならプラグインを追加できるようにするのがよさそう
//
// 　　　　　　　　　　　　　　　　　　　　　　　　　　　※ 上位コマンドを作成し、そちらで統合させる（その中から呼び出す1プログラムとしてlsshは残す）
// TODO(blacknon): 各種クラウドの踏み台経由でのアクセスに対応する => pluginで処理させたいお気持ち
//   - AWS SSM(セッションマネージャー)
//   - Azure Bastion
//   - GCP(gcloud compute ssh)

// TODO(blacknon): configの中に`plugin`structwを追加して、そこにプラグインの設定を記述できるようにする(v0.8.X)
//                  このとき、このstruct側でプラグインファイルのパスを指定するほか、どのようなプラグインなのかをこのstructもしくは対象化で持たせるようにすることで、ファイルの転送やコマンドの実行など、機能の制限を付けられるようにする。これにより、lsshや　lscpで実行する際に表示の制御や実行対象外を拾えるようにできる。
// 　　　　　　　　　　　　　　　　　　　　　　　　　　　　　例えば、winrmやtelnetの場合はファイル転送が難しいため、ターミナル操作だけを対象にすることで、lscpやlsftpでの実行を禁止するなどの制御ができるようにする。使えるバイナリや機能だけを使えるようにすれば、実現の難しいプロトコルにも対応できるようになるのではないかと考えている。
//                  このとき、server structをそのまま使うと面倒になるので、専用のstructを使わせるようにすれば混在を防げないだろうか？（型として継承するのは要検討）

package conf

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/blacknon/lssh/internal/common"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config is Struct that stores the entire configuration file.
type Config struct {
	Log       LogConfig                         `toml:"log" yaml:"log"`
	Mux       MuxConfig                         `toml:"mux" yaml:"mux"`
	Shell     ShellConfig                       `toml:"shell" yaml:"shell"`
	Lsshfs    LsshfsConfig                      `toml:"lsshfs" yaml:"lsshfs"`
	Providers ProvidersConfig                   `toml:"providers" yaml:"providers"`
	Include   map[string]IncludeConfig          `toml:"include" yaml:"include"`
	Includes  IncludesConfig                    `toml:"includes" yaml:"includes"`
	Common    ServerConfig                      `toml:"common" yaml:"common"`
	Server    map[string]ServerConfig           `toml:"server" yaml:"server"`
	Proxy     map[string]ProxyConfig            `toml:"proxy" yaml:"proxy"`
	Provider  map[string]map[string]interface{} `toml:"provider" yaml:"provider"`

	SSHConfig map[string]OpenSSHConfig `toml:"sshconfig" yaml:"sshconfig"`
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
		defaultPath := defaultOpenSSHConfigCandidate()
		if defaultPath == "" || !common.IsExist(defaultPath) {
			return
		}

		openSSHServerConfig, err := getOpenSSHConfig(defaultPath, "")
		if err == nil {
			// append data
			for key, value := range openSSHServerConfig {
				value := serverConfigReduct(c.Common, value)
				c.Server[key] = value
			}
		}
	} else {
		for _, sc := range c.activeOpenSSHConfigs() {
			if err := c.readConfiguredOpenSSHConfig(sc); err != nil {
				continue
			}
		}
	}
}

func defaultOpenSSHConfigCandidate() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".ssh", "config")
	}
	return "~/.ssh/config"
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
			err := decodeConfigFile(path, &includeConf)
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

			if len(includeConf.Provider) > 0 {
				if c.Provider == nil {
					c.Provider = map[string]map[string]interface{}{}
				}
				for key, value := range includeConf.Provider {
					c.Provider[key] = value
				}
			}

			c.Providers = mergeProvidersConfig(c.Providers, includeConf.Providers)
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
		if v.Ignore {
			continue
		}
		if c.ServerUsesConnector(k) {
			continue
		}

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
		err := decodeConfigFile(confPath, &c)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
	}

	// reduce default setting to common
	c.Common = serverConfigReduct(
		ServerConfig{
			Port:           "22",
			ControlPath:    "/tmp/lssh-control-%h-%p-%r",
			ControlPersist: 10,
		},
		c.Common,
	)
	c.Mux = c.Mux.ApplyDefaults()

	// reduce common setting (in .lssh.conf servers)
	c.ReduceCommon()

	// Read OpensSH configs
	c.ReadOpenSSHConfig()

	// for append includes to include.path
	c.ReadIncludeFiles()

	// Load inventory providers after includes and OpenSSH config are merged.
	if err := c.ReadInventoryProviders(); err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// resolve conditional server overrides after all sources have been merged
	if err := c.ResolveConditionalMatches(); err != nil {
		log.Println(err)
		os.Exit(1)
	}

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
	if c.Pass != "" || c.PassRef != "" || c.Key != "" || c.KeyRef != "" || c.Cert != "" || c.CertRef != "" {
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

	if len(c.Keys) > 0 || len(c.Passes) > 0 || c.CertKeyRef != "" || c.PKCS11PINRef != "" {
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
	result.ProviderConfig = mergeProviderConfigMaps(perConfig.ProviderConfig, childConfig.ProviderConfig)

	return result
}

// GetNameList return a list of server names from the Config structure.
func GetNameList(listConf Config) (nameList []string) {
	for k, v := range listConf.Server {
		if v.Ignore {
			continue
		}
		nameList = append(nameList, k)
	}
	return
}
