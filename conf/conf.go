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

type IncludesConfig struct {
	Path []string `toml:"path"`
}

type IncludeConfig struct {
	Path string `toml:"path"`
}

type ServerConfig struct {
	Addr              string   `toml:"addr"`
	Port              string   `toml:"port"`
	User              string   `toml:"user"`
	Pass              string   `toml:"pass"`
	Key               string   `toml:"key"`
	KeyPass           string   `toml:"keypass"`
	PreCmd            string   `toml:"pre_cmd"`
	PostCmd           string   `toml:"post_cmd"`
	ProxyType         string   `toml:"proxy_type"`
	Proxy             string   `toml:"proxy"`
	LocalRcUse        string   `toml:"local_rc"` // yes|no
	LocalRcPath       []string `toml:"local_rc_file"`
	LocalRcDecodeCmd  string   `toml:"local_rc_decode_cmd"`
	PortForwardLocal  string   `toml:"port_forward_local"`  // host:port
	PortForwardRemote string   `toml:"port_forward_remote"` // host:port
	Note              string   `toml:"note"`
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
