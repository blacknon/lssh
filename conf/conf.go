package conf

import (
	"fmt"
	"os"
	"os/user"
	r "reflect"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Log     LogConfig
	Include map[string]IncludeConfig
	Common  ServerConfig
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
	// Basic
	Addr string `toml:"addr"`
	Port string `toml:"port"`
	User string `toml:"user"`
	Pass string `toml:"pass"`
	Key  string `toml:"key"`

	// Local command
	BeforeCmd string `toml:"before_cmd"`
	AfterCmd  string `toml:"after_cmd"`

	// Proxy
	ProxyAddr string `toml:"proxy_user"`
	ProxyPort string `toml:"proxy_user"`
	ProxyUser string `toml:"proxy_user"`
	ProxyPass string `toml:"proxy_pass"`
	ProxyCmd  string `toml:"proxy_cmd"`

	Note string `toml:"note"`
}

type ServerConfigMaps map[string]ServerConfig

type structMap map[string]interface{}

func ReadConf(confPath string) (checkConf Config) {
	if isExist(confPath) == false {
		fmt.Printf("Config file(%s) Not Found.\nPlease create file.\n\n", confPath)
		fmt.Printf("sample: %s\n", "https://raw.githubusercontent.com/blacknon/lssh/master/example/config.tml")
		os.Exit(1)
	}

	// Read config file
	_, err := toml.DecodeFile(confPath, &checkConf)
	if err != nil {
		panic(err)
	}

	// main conf file common
	mainCommon, _ := structToMap(&checkConf.Common)

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

			// common setting
			setCommon := &ServerConfig{}
			includeCommon, _ := structToMap(&includeConf.Common)
			setCommonMap := commonConfigReduce(mainCommon, includeCommon)
			_ = mapToStruct(setCommonMap, setCommon)

			// add include file serverconf
			for key, value := range includeConf.Server {
				setValue := ServerConfig{}
				mapValue, _ := structToMap(&value)
				setValueMap := commonConfigReduce(setCommonMap, mapValue)
				_ = mapToStruct(setValueMap, &setValue)
				value = setValue

				checkConf.Server[key] = value
			}

		}
	}

	// Check Config Parameter
	checkAlertFlag := checkServerConf(checkConf)
	if checkAlertFlag == false {
		os.Exit(1)
	}

	return
}

func commonConfigReduce(map1, map2 structMap) structMap {
	for ia, va := range map1 {
		if va != "" && map2[ia] == "" {
			map2[ia] = va
		}
	}
	return map2
}

func structToMap(val interface{}) (mapVal map[string]interface{}, ok bool) {
	structVal := r.Indirect(r.ValueOf(val))
	typ := structVal.Type()

	mapVal = make(map[string]interface{})

	for i := 0; i < typ.NumField(); i++ {
		field := structVal.Field(i)

		if field.CanSet() {
			mapVal[typ.Field(i).Name] = field.Interface()
		}
	}

	return
}

func mapToStruct(mapVal map[string]interface{}, val interface{}) (ok bool) {
	structVal := r.Indirect(r.ValueOf(val))
	for name, elem := range mapVal {
		structVal.FieldByName(name).Set(r.ValueOf(elem))
	}

	return
}

func GetNameList(listConf Config) (nameList []string) {
	for k := range listConf.Server {
		nameList = append(nameList, k)
	}
	return
}
