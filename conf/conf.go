package conf

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/blacknon/lssh/common"
)

type Config struct {
	Log     LogConfig
	Include map[string]IncludeConfig
	Common  ServerConfig
	Server  map[string]ServerConfig
	Proxy   map[string]HttpProxyConfig
}

type LogConfig struct {
	Enable    bool   `toml:"enable"`
	Timestamp bool   `toml:"timestamp"`
	Dir       string `toml:"dirpath"`
}

type IncludeConfig struct {
	Path string `toml:"path"`
}

type ServerConfig struct {
	Addr      string `toml:"addr"`
	Port      string `toml:"port"`
	User      string `toml:"user"`
	Pass      string `toml:"pass"`
	Key       string `toml:"key"`
	KeyPass   string `toml:"keypass"`
	PreCmd    string `toml:"pre_cmd"`
	PostCmd   string `toml:"post_cmd"`
	ProxyType string `toml:"proxy_type"`
	Proxy     string `toml:"proxy"`
	Note      string `toml:"note"`
}

type HttpProxyConfig struct {
	Addr string `toml:"addr"`
	Port string `toml:"port"`
	User string `toml:"user"`
	Pass string `toml:"pass"`
}

type ServerConfigMaps map[string]ServerConfig

type HttpProxyConfigMaps map[string]HttpProxyConfig

func ReadConf(confPath string) (checkConf Config) {
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

	// Read include files
	if checkConf.Include != nil {
		for _, v := range checkConf.Include {
			var includeConf Config

			// user path
			usr, _ := user.Current()
			path := strings.Replace(v.Path, "~", usr.HomeDir, 1)

			// Read include config file
			_, err := toml.DecodeFile(path, &includeConf)
			if err != nil {
				panic(err)
			}

			// reduce common setting
			setCommon := serverConfigReduct(checkConf.Common, includeConf.Common)

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

func checkFormatServerConf(c Config) (isFormat bool) {
	isFormat = true
	for k, v := range c.Server {
		// Address Input Check
		if v.Addr == "" {
			fmt.Printf("%s: 'addr' is not inserted.\n", k)
			isFormat = false
		}

		// User Input Check
		if v.User == "" {
			fmt.Printf("%s: 'user' is not inserted.\n", k)
			isFormat = false
		}

		// Password or Keyfile Input Check
		if v.Pass == "" && v.Key == "" {
			fmt.Printf("%s: Both Password and KeyPath are entered.Please enter either.\n", k)
			isFormat = false
		}
	}
	return
}

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
