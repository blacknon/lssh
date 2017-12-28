package conf

import (
	"os"
	"os/user"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Log     LogConfig
	Include map[string]IncludeConfig
	Server  map[string]ServerConfig
}

type LogConfig struct {
	Enable bool   `toml:"enable"`
	Dir    string `toml:"dirpath"`
}

type IncludeConfig struct {
	Path string `toml:"path"`
}

type ServerConfig struct {
	Addr string `toml:"addr"`
	Port string `toml:"port"`
	User string `toml:"user"`
	Pass string `toml:"pass"`
	Key  string `toml:"key"`
	Note string `toml:"note"`
}

func ReadConf(confPath string) (checkConf Config) {
	// Read Config
	_, err := toml.DecodeFile(confPath, &checkConf)
	if err != nil {
		panic(err)
	}

	if checkConf.Include != nil {

		for _, v := range checkConf.Include {
			//var serverconf ServerConfig
			usr, _ := user.Current()
			path := strings.Replace(v.Path, "~", usr.HomeDir, 1)
			_, err := toml.DecodeFile(path, &checkConf)
			if err != nil {
				panic(err)
			}
			//fmt.Println(&checkConf)
		}
	}

	// Check Config Parameter
	checkAlertFlag := checkServerConf(checkConf)
	if checkAlertFlag == false {
		os.Exit(1)
	}

	return
}

func GetNameList(listConf Config) (nameList []string) {
	for k := range listConf.Server {
		nameList = append(nameList, k)
	}
	return
}
