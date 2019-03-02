package common

import (
	"encoding/base64"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
)

func IsExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func MapReduce(map1, map2 map[string]interface{}) map[string]interface{} {
	for ia, va := range map1 {
		switch value := va.(type) {
		case string:
			if value != "" && map2[ia] == "" {
				map2[ia] = value
			}
		case []string:
			map2Value := reflect.ValueOf(map2[ia])
			if len(value) > 0 && map2Value.Len() == 0 {
				map2[ia] = value
			}
		case bool:
			map2Value := reflect.ValueOf(map2[ia])
			if value == true && map2Value.Bool() == false {
				map2[ia] = value
			}
		}
	}

	return map2
}

func StructToMap(val interface{}) (mapVal map[string]interface{}, ok bool) {
	structVal := reflect.Indirect(reflect.ValueOf(val))
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

func MapToStruct(mapVal map[string]interface{}, val interface{}) (ok bool) {
	structVal := reflect.Indirect(reflect.ValueOf(val))
	for name, elem := range mapVal {
		structVal.FieldByName(name).Set(reflect.ValueOf(elem))
	}

	return
}

func GetFullPath(path string) (fullPath string) {
	usr, _ := user.Current()
	fullPath = strings.Replace(path, "~", usr.HomeDir, 1)
	fullPath, _ = filepath.Abs(fullPath)
	return fullPath
}

func GetMaxLength(list []string) (MaxLength int) {
	MaxLength = 0
	for _, elem := range list {
		if MaxLength < len(elem) {
			MaxLength = len(elem)
		}
	}
	return
}

func GetFilesBase64(list []string) (result string, err error) {
	var data []byte
	for _, path := range list {

		fullPath := GetFullPath(path)

		// open file
		file, err := os.Open(fullPath)
		if err != nil {
			return "", err
		}
		defer file.Close()

		file_data, err := ioutil.ReadAll(file)
		if err != nil {
			return "", err
		}

		data = append(data, file_data...)
		data = append(data, '\n')
	}

	result = base64.StdEncoding.EncodeToString(data)
	return result, err
}
