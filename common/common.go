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

// IsExist returns existence of file.
func IsExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// MapReduce sets map1 value to map2 if map1 and map2 have same key, and value
// of map2 is zero value. Available interface type is string or []string or
// bool.
//
// WARN: This function returns a map, but updates value of map2 argument too.
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

// StructToMap returns a map that converted struct to map.
// Keys of map are set from public field of struct.
//
// WARN: ok value is not used. Always returns false.
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

// MapToStruct sets value of mapVal to public field of val struct.
// Raises panic if mapVal has keys of private field of val struct or field that
// val struct doesn't have.
//
// WARN: ok value is not used. Always returns false.
func MapToStruct(mapVal map[string]interface{}, val interface{}) (ok bool) {
	structVal := reflect.Indirect(reflect.ValueOf(val))
	for name, elem := range mapVal {
		structVal.FieldByName(name).Set(reflect.ValueOf(elem))
	}

	return
}

// GetFullPath returns a fullpath of path.
// Expands `~` to user directory ($HOME environment variable).
func GetFullPath(path string) (fullPath string) {
	usr, _ := user.Current()
	fullPath = strings.Replace(path, "~", usr.HomeDir, 1)
	fullPath, _ = filepath.Abs(fullPath)
	return fullPath
}

// GetMaxLength returns a max length of list.
// Length is byte length.
func GetMaxLength(list []string) (MaxLength int) {
	MaxLength = 0
	for _, elem := range list {
		if MaxLength < len(elem) {
			MaxLength = len(elem)
		}
	}
	return
}

// GetFilesBase64 returns a base64 encoded string of file content of paths.
func GetFilesBase64(paths []string) (result string, err error) {
	var data []byte
	for _, path := range paths {

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
