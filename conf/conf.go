package conf

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Log    LogConfig
	Server map[string]ReadConfig
}

type LogConfig struct {
	Enable bool   `toml:"enable"`
	Dir    string `toml:"dirpath"`
}

type ReadConfig struct {
	Addr string `toml:"addr"`
	Port string `toml:"port"`
	User string `toml:"user"`
	Pass string `toml:"pass"`
	Key  string `toml:"key"`
	Note string `toml:"note"`
}

func ConfigCheckRead(confPath string) (checkConf Config) {
	var checkAlertFlag int = 0

	// Read Config
	_, err := toml.DecodeFile(confPath, &checkConf)
	if err != nil {
		panic(err)
	}

	// Config Value Check
	for k, v := range checkConf.Server {
		// Address Input Check
		if v.Addr == "" {
			fmt.Printf("%s: 'addr' is not inserted.\n", k)
			checkAlertFlag = 1
		}

		// User Input Check
		if v.User == "" {
			fmt.Printf("%s: 'user' is not inserted.\n", k)
			checkAlertFlag = 1
		}

		// Password or Keyfile Input Check
		if v.Pass == "" && v.Key == "" {
			fmt.Printf("%s: Both Password and KeyPath are entered.Please enter either.\n", k)
			checkAlertFlag = 1
		}
	}

	if checkAlertFlag == 1 {
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
