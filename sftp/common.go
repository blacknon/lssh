// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

type FileInfo interface {
	fs.FileInfo
}

type sftpFileInfo struct {
	FileInfo
	Dir string
}

//
func DupPermutationsRecursive0(n, k int) [][]int {
	if k == 0 {
		pattern := []int{}
		return [][]int{pattern}
	}

	ans := [][]int{}
	for num := 0; num < n; num++ {
		childPatterns := DupPermutationsRecursive0(n, k-1)
		for _, childPattern := range childPatterns {
			pattern := append([]int{num}, childPattern...)
			ans = append(ans, pattern)
		}
	}

	return ans
}

// A function that returns the value of fs.Filemode from the permissions and umask passed in an array.
// ex)
//   defaultPerm ... [0,7,7,7]
//   umask ... [0,2,2]
func GeneratePermWithUmask(defaultPerm, umask []string) fs.FileMode {
	// set 1st char
	setPermStr := defaultPerm[0]

	// set 2nd char
	perm1char, _ := strconv.Atoi(defaultPerm[1])
	umask1char, _ := strconv.Atoi(umask[0])
	setPermStr = setPermStr + strconv.Itoa(perm1char-umask1char)

	// set 3rd char
	perm2char, _ := strconv.Atoi(defaultPerm[2])
	umask2char, _ := strconv.Atoi(umask[1])
	setPermStr = setPermStr + strconv.Itoa(perm2char-umask2char)

	// set 4th char
	perm3char, _ := strconv.Atoi(defaultPerm[3])
	umask3char, _ := strconv.Atoi(umask[2])
	setPermStr = setPermStr + strconv.Itoa(perm3char-umask3char)

	perm, _ := strconv.ParseUint(setPermStr, 8, 32)

	return os.FileMode(perm)
}

// Pass path including tilde etc., expand it as local machine PATH and return
func ExpandRemotePath(client *TargetConnectMap, path string) (expandPaths []string, err error) {
	// get home dir
	dir, err := client.Connect.Getwd()
	if err != nil {
		return
	}

	// expand tilde
	switch {
	case path == "~":
		path = dir

	case strings.HasPrefix(path, "~/"):
		path = filepath.Join(dir, path[2:])

	case !filepath.IsAbs(path):
		path = filepath.Join(client.Pwd, path)
	}

	// get glob
	expandPaths, err = client.Connect.Glob(path)
	if err != nil {
		return
	}

	if path != "" && len(expandPaths) == 0 {
		expandPaths = append(expandPaths, path)
	}

	return
}

// Pass path including tilde etc., expand it as local machine PATH and return
func ExpandLocalPath(path string) (expandPaths []string, err error) {
	// get home dir
	usr, _ := user.Current()
	dir := usr.HomeDir

	// expand tilde
	if path == "~" {
		path = dir
	} else if strings.HasPrefix(path, "~/") {
		path = filepath.Join(dir, path[2:])
	}

	// expand glob
	expandPaths, err = filepath.Glob(path)
	if path != "" && len(expandPaths) == 0 {
		expandPaths = append(expandPaths, path)
	}

	return
}

// CheckIsDirPath return path specifies a directory.
// This function working only local machine.
func CheckIsDirPath(path string) (result bool, dir, base string) {
	// strip spaces
	path = strings.TrimSpace(path)

	// check last character
	lastCharacter := path[len(path)-1:]
	if lastCharacter == "/" {
		result = true
		dir = filepath.Dir(path)
		base = ""
		return
	}

	// check local path
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		// If path is missing, assume that no directory is specified.
		result = true
		dir = path
		path = ""
		return
	} else if stat.IsDir() {
		// if path is directory.
		result = true
		dir = path
		path = ""
		return
	} else if !stat.IsDir() {
		// if path is file.
		result = false
		dir = filepath.Dir(path)
		path = filepath.Base(path)
		return
	}

	return
}
