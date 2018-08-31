package common

import (
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
		if va != "" && map2[ia] == "" {
			map2[ia] = va
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
