// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

/*
common is a package that summarizes the common processing of lssh package.
*/
package common

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
)

var characterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// enum
const (
	ARCHIVE_NONE = iota
	ARCHIVE_GZIP
)

// IsExist returns existence of file.
func IsExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func Contains(list interface{}, elem interface{}) bool {
	listV := reflect.ValueOf(list)

	if listV.Kind() == reflect.Slice {
		for i := 0; i < listV.Len(); i++ {
			item := listV.Index(i).Interface()
			// check conver
			if !reflect.TypeOf(elem).ConvertibleTo(reflect.TypeOf(item)) {
				continue
			}
			// convert type
			target := reflect.ValueOf(elem).Convert(reflect.TypeOf(item)).Interface()
			// check
			if ok := reflect.DeepEqual(item, target); ok {
				return true
			}
		}
	}
	return false
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
		case int:
			map2Value := reflect.ValueOf(map2[ia])
			if value != 0 && map2Value.Int() == 0 {
				map2[ia] = value
			}
		}
	}

	return map2
}

// MapMerge merges multiple Maps
func MapMerge(m ...map[string]interface{}) map[string]interface{} {
	ans := make(map[string]interface{}, 0)

	for _, c := range m {
		for k, v := range c {
			ans[k] = v
		}
	}
	return ans
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

// Get order num in array
func GetOrderNumber(value string, array []string) int {
	for i, v := range array {
		if v == value {
			return i
		}
	}

	return 0
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
func GetFilesBase64(paths []string, iscompress int) (result string, err error) {
	var data []byte
	for _, path := range paths {

		fullPath := GetFullPath(path)

		// open file
		file, err := os.Open(fullPath)
		if err != nil {
			return "", err
		}
		defer file.Close()

		filedata, err := ioutil.ReadAll(file)
		if err != nil {
			return "", err
		}

		data = append(data, filedata...)
		data = append(data, '\n')
	}

	switch iscompress {
	case ARCHIVE_NONE:
		result = base64.StdEncoding.EncodeToString(data)

	case ARCHIVE_GZIP:
		data, err = StringCompression(ARCHIVE_GZIP, data)
		result = base64.StdEncoding.EncodeToString(data)
	}

	return result, err
}

// GetPassPhrase gets the passphrase from virtual terminal input and returns the result. Works only on UNIX-based OS.
func GetPassPhrase(msg string) (input string, err error) {
	fmt.Printf(msg)

	// Open /dev/tty
	tty, err := os.Open("/dev/tty")
	if err != nil {
		log.Fatal(err)
	}
	defer tty.Close()

	// get input
	result, err := terminal.ReadPassword(int(tty.Fd()))

	if len(result) == 0 {
		err = fmt.Errorf("err: input is empty")
		return
	}

	input = string(result)
	fmt.Println()
	return
}

// NewSHA1Hash generates a new SHA1 hash based on
// a random number of characters.
func NewSHA1Hash(n ...int) string {
	noRandomCharacters := 32

	if len(n) > 0 {
		noRandomCharacters = n[0]
	}

	randString := RandomString(noRandomCharacters)

	hash := sha1.New()
	hash.Write([]byte(randString))
	bs := hash.Sum(nil)

	return fmt.Sprintf("%02x", bs)
}

// RandomString generates a random string of n length
func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = characterRunes[rand.Intn(len(characterRunes))]
	}
	return string(b)
}

// GetUniqueSlice return slice, removes duplicate values ​​from data(slice).
func GetUniqueSlice(data []string) (result []string) {
	m := make(map[string]bool)

	for _, ele := range data {
		if !m[ele] {
			m[ele] = true
			result = append(result, ele)
		}
	}

	return
}

// WalkDir return file path list ([]string).
func WalkDir(dir string) (files []string, err error) {
	_, err = os.Lstat(dir)
	if err != nil {
		return
	}

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			path = path + "/"
		}
		files = append(files, path)
		return nil
	})
	return
}

// GetUserName return user name from /etc/passwd and uid.
func GetIdFromName(file string, name string) (id uint32, err error) {
	rd := strings.NewReader(file)
	sc := bufio.NewScanner(rd)

	for sc.Scan() {
		l := sc.Text()
		line := strings.Split(l, ":")
		if line[0] == name {
			idstr := line[2]
			u64, _ := strconv.ParseUint(idstr, 10, 32)
			id = uint32(u64)
			return
		}
	}

	err = errors.New(fmt.Sprintf("Error: %s", "name not found"))

	return
}

// GetUserName return user name from /etc/passwd and uid.
func GetNameFromId(file string, id uint32) (name string, err error) {
	rd := strings.NewReader(file)
	sc := bufio.NewScanner(rd)

	idstr := strconv.FormatUint(uint64(id), 10)
	for sc.Scan() {
		l := sc.Text()
		line := strings.Split(l, ":")
		if line[2] == idstr {
			name = line[0]
			return
		}
	}

	err = errors.New(fmt.Sprintf("Error: %s", "name not found"))

	return
}

// ParseForwardPort return forward address and port from string.
//
// ex.)
//   - `localhost:8000:localhost:18000` => local: "localhost:8000", remote: "localhost:18000"
//   - `8080:localhost:18080` => local: "localhost:8080", remote: "localhost:18080"
//   - `localhost:2222:12222` => local: "localhost:2222", remote: "localhost:12222"
func ParseForwardPort(value string) (local, remote string, err error) {
	// count column
	count := strings.Count(value, ":")
	data := strings.Split(value, ":")

	// switch count
	switch count {
	case 3: // `localhost:8000:localhost:18000`
		local = data[0] + ":" + data[1]
		remote = data[2] + ":" + data[3]
	case 2:
		// check 1st column is int
		_, e := strconv.Atoi(data[0])
		if e == nil { // 1st column is port (int)
			local = "localhost:" + data[0]
			remote = data[1] + ":" + data[2]
		} else { // 1st column is not port (int)
			local = data[0] + ":" + data[1]
			remote = "localhost:" + data[2]
		}

	default:
		err = errors.New("Could not parse.")
	}

	return
}

func ParseNFSForwardPortPath(value string) (port, path string, err error) {
	data := strings.Split(value, ":")
	if len(data) != 2 {
		err = errors.New("Could not parse.")
		return
	}

	port = data[0]
	path = data[1]

	return
}

// ParseHostPath return host and path, from host:/path/to/dir/file.
func ParseHostPath(value string) (host []string, path string) {
	if !strings.Contains(value, ":") {
		path = value
		return
	}

	parseValue := strings.SplitN(value, ":", 2)
	host = strings.Split(parseValue[0], ",")
	path = parseValue[1]

	return
}

// ParseArgs return os.Args parse short options (ex.) [-la] => [-l,-a] )
//
// TODO(blacknon): Migrate to github.com/urfave/cli version 1.22.
func ParseArgs(options []cli.Flag, args []string) []string {
	// create cli.Flag map
	optionMap := map[string]cli.Flag{}
	for _, op := range options {
		name := op.GetName()
		names := strings.Split(name, ",")

		for _, n := range names {
			// add hyphen
			if len(n) == 1 {
				n = "-" + n
			} else {
				n = "--" + n
			}
			optionMap[n] = op
		}
	}

	var result []string
	result = append(result, args[0])

	// command flag
	isOptionArgs := false

	optionReg := regexp.MustCompile("^-")
	parseReg := regexp.MustCompile("^-[^-]{2,}")

parseloop:
	for i, arg := range args[1:] {
		// delete space
		arg = strings.TrimSpace(arg)

		switch {
		case !optionReg.MatchString(arg) && !isOptionArgs:
			// not option arg, and sOptinArgs flag false
			result = append(result, args[i+1:]...)
			break parseloop

		case !optionReg.MatchString(arg) && isOptionArgs:
			result = append(result, arg)

		case parseReg.MatchString(arg): // combine short option -la)
			slice := strings.Split(arg[1:], "")
			for _, s := range slice {
				s = "-" + s
				result = append(result, s)

				if val, ok := optionMap[s]; ok {
					switch val.(type) {
					case cli.StringSliceFlag:
						isOptionArgs = true
					case cli.StringFlag:
						isOptionArgs = true
					}
				}
			}

		default: // options (-a,--all)
			result = append(result, arg)

			if val, ok := optionMap[arg]; ok {
				switch val.(type) {
				case cli.StringSliceFlag:
					isOptionArgs = true
				case cli.StringFlag:
					isOptionArgs = true
				}
			}
		}
	}
	return result
}

// IsDirPath identifies is the directory from the PATH string.
func IsDirPath(path string) (isDir bool) {
	dir := filepath.Dir(path)
	path = filepath.Clean(path)

	if dir == path {
		isDir = true
	}

	return
}

// StringCompression compresses bytes in the specified mode.
func StringCompression(mode int, data []byte) (result []byte, err error) {
	// create buffer
	buf := new(bytes.Buffer)

	switch mode {
	case ARCHIVE_GZIP:
		zw := gzip.NewWriter(buf)
		defer zw.Close()

		r := bytes.NewReader(data)

		_, err = io.Copy(zw, r)
		zw.Flush()
	}

	result = buf.Bytes()

	return
}

func GetDefaultConfigPath() (path string) {
	// get XDG_CONFIG_HOME
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")

	// get user
	usr, _ := user.Current()

	// get home dir
	home := usr.HomeDir

	// get config path
	homeConfigPath := filepath.Join(home, ".lssh.conf")
	xdgConfigPath := filepath.Join(xdgConfigHome, "lssh", "lssh.conf")

	if _, err := os.Stat(homeConfigPath); os.IsExist(err) {
		return homeConfigPath
	}

	if _, err := os.Stat(xdgConfigPath); os.IsExist(err) {
		return xdgConfigPath
	}

	return homeConfigPath
}
