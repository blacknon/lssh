// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

/*
Package scplib is a library for exchanging data with scp in golang.
*/
package scplib

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// SCPClient save credentials and use scp from method.
type SCPClient struct {
	Connection *ssh.Client
	Session    *ssh.Session
	Permission bool
}

func unset(s []string, i int) []string {
	if i >= len(s) {
		return s
	}
	return append(s[:i], s[i+1:]...)
}

func getFullPath(path string) (fullPath string) {
	usr, _ := user.Current()
	fullPath = strings.Replace(path, "~", usr.HomeDir, 1)
	fullPath, _ = filepath.Abs(fullPath)
	return fullPath
}

func walkDir(dir string) (files []string, err error) {
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			path = path + "/"
		}
		files = append(files, path)
		return nil
	})
	return
}

// pushDirData is Write directory data to remote.
func pushDirData(w io.WriteCloser, baseDir string, paths []string, toName string, perm bool) {
	baseDirSlice := strings.Split(baseDir, "/")
	baseDirSlice = unset(baseDirSlice, len(baseDirSlice)-1)
	baseDir = strings.Join(baseDirSlice, "/")

	for _, path := range paths {
		relPath, _ := filepath.Rel(baseDir, path)
		dir := filepath.Dir(relPath)

		if len(dir) > 0 && dir != "." {
			dirList := strings.Split(dir, "/")
			dirpath := baseDir
			for _, dirName := range dirList {
				dirpath = dirpath + "/" + dirName
				dInfo, _ := os.Lstat(dirpath)
				dPerm := fmt.Sprintf("%04o", dInfo.Mode().Perm())

				// push directory information
				fmt.Fprintln(w, "D"+dPerm, 0, dirName)
			}
		}

		fInfo, _ := os.Lstat(path)

		if !fInfo.IsDir() {
			// check symlink
			if fInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
				fmt.Fprintf(os.Stderr, "'%v' is Symlink, Do not copy.\n", path)
			} else {
				pushFileData(w, []string{path}, toName, perm)
			}
		}

		if len(dir) > 0 && dir != "." {
			dirList := strings.Split(dir, "/")
			endStr := strings.Repeat("E\n", len(dirList))
			fmt.Fprintf(w, endStr)
		}
	}
	return
}

// pushFileData is exchange local file data, to scp format
func pushFileData(w io.WriteCloser, paths []string, toName string, perm bool) {
	for _, path := range paths {
		fInfo, _ := os.Lstat(path)

		content, err := os.Open(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		stat, _ := content.Stat()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		// default permission(0644)
		fPerm := "0644"
		if perm == true {
			fPerm = fmt.Sprintf("%04o", fInfo.Mode())
		}

		// push file information
		fmt.Fprintln(w, "C"+fPerm, stat.Size(), toName)
		io.Copy(w, content)
		fmt.Fprint(w, "\x00")
	}
	return
}

// writeData is write to local file, from scp data.
func writeData(data *bufio.Reader, path string, perm bool) {
	pwd := path
checkloop:
	for {
		// Get file or dir information (1st line)
		line, err := data.ReadString('\n')

		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Println(err)
		}

		line = strings.TrimRight(line, "\n")
		if line == "E" {
			pwdArray := strings.Split(pwd, "/")
			if len(pwdArray) > 0 {
				pwdArray = pwdArray[:len(pwdArray)-2]
			}
			pwd = strings.Join(pwdArray, "/") + "/"
			continue
		}

		lineSlice := strings.SplitN(line, " ", 3)

		scpType := lineSlice[0][:1]
		scpPerm := lineSlice[0][1:]
		scpPerm32, _ := strconv.ParseUint(scpPerm, 8, 32)
		scpSize, _ := strconv.Atoi(lineSlice[1])
		scpObjName := lineSlice[2]

		switch {
		case scpType == "C":
			scpPath := path
			// Check pwd
			check, _ := regexp.MatchString("/$", pwd)
			if check || pwd != path {
				scpPath = pwd + scpObjName
			}

			// set permission
			if perm == false {
				scpPerm32, _ = strconv.ParseUint("0644", 8, 32)
			}

			// 1st write to file
			file, err := os.OpenFile(scpPath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, os.FileMode(uint32(scpPerm32)))
			if err != nil {
				fmt.Println(err)
				break checkloop
			}
			file.WriteString("")
			file.Close()

			fileData := []byte{}
			readedSize := 0
			remainingSize := scpSize

			outFile, _ := os.OpenFile(scpPath, os.O_WRONLY|os.O_APPEND, os.FileMode(uint32(scpPerm32)))
			for {
				readBuffer := make([]byte, remainingSize)
				readSize, _ := data.Read(readBuffer)

				remainingSize -= readSize
				readedSize += readSize

				// check readedSize
				if readSize == 0 {
					break
				}

				readBuffer = readBuffer[:readSize]
				fileData = append(fileData, readBuffer...)

				if readedSize == scpSize {
					outFile.Write(fileData)
					fileData = []byte{}
					break
				} else if len(fileData) >= 10485760 {
					// write file over 10MB
					outFile.Write(fileData)
					fileData = []byte{}
				}
			}

			// write file to path
			os.Chmod(scpPath, os.FileMode(uint32(scpPerm32)))

			// read last nUll character
			_, _ = data.ReadByte()
		case scpType == "D":
			// Check pwd
			check, _ := regexp.MatchString("/$", pwd)
			if !check {
				pwd = pwd + "/"
			}

			if perm == false {
				scpPerm32, _ = strconv.ParseUint("0755", 8, 32)
			}

			pwd = pwd + scpObjName + "/"
			err := os.Mkdir(pwd, os.FileMode(uint32(scpPerm32)))
			if err != nil {
				fmt.Println(err)
				os.Chmod(pwd, os.FileMode(uint32(scpPerm32)))
			}
		default:
			fmt.Fprintln(os.Stderr, line)
			continue checkloop
			// break checkloop
		}
	}
	return
}

// GetFile get file data to file (remote to Local).
//
// example:
//    scp.GetFile("/From/Remote/Path","/To/Local/Path")
func (s *SCPClient) GetFile(fromPaths []string, toPath string) (err error) {
	session := s.Session
	if s.Connection != nil {
		session, err = s.Connection.NewSession()
		if err != nil {
			return
		}
	}
	defer session.Close()

	fin := make(chan bool)
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		// Null Characters(10,000 char)
		nc := strings.Repeat("\x00", 100000)
		fmt.Fprintf(w, nc)
	}()

	go func() {
		r, _ := session.StdoutPipe()
		b := bufio.NewReader(r)
		writeData(b, toPath, s.Permission)

		fin <- true
	}()

	// Create scp command
	fromPathList := []string{}
	for _, fromPath := range fromPaths {
		fromPathList = append(fromPathList, fromPath)
	}
	fromPathString := strings.Join(fromPathList, " ")
	// TODO(blacknon): scpしてる時点でセキュリティもクソもないのだが、OS Command Injectionへの対策を考える
	scpCmd := "/usr/bin/scp -rf " + fromPathString

	// Run scp
	err = session.Run(scpCmd)

	<-fin
	return
}

// PutFile is put file to remote path.
//
// example:
//    scp.PutFile("/From/Local/Path","/To/Remote/Path")
func (s *SCPClient) PutFile(fromPaths []string, toPath string) (err error) {
	session := s.Session
	if s.Connection != nil {
		session, _ = s.Connection.NewSession()
		if err != nil {
			return
		}
	}
	defer session.Close()

	// Read Dir or File
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		for _, fromPath := range fromPaths {
			// Get full path
			fromPath = getFullPath(fromPath)

			// File or Dir exits check
			pInfo, err := os.Lstat(fromPath)
			if err != nil {
				return
			}

			if pInfo.IsDir() {
				// Directory
				pList, _ := walkDir(fromPath)
				for _, i := range pList {
					pushDirData(w, fromPath, []string{i}, filepath.Base(i), s.Permission)
				}
			} else {
				// single files
				toFile := filepath.Base(toPath)
				if toFile == "." {
					toFile = filepath.Base(fromPath)
				}
				pushFileData(w, []string{fromPath}, toFile, s.Permission)
			}
		}
	}()

	// Create scp command
	// TODO(blacknon): scpしてる時点でセキュリティもクソもないのだが、OS Command Injectionへの対策を考える
	scpCmd := "/usr/bin/scp -tr '" + toPath + "'"
	if s.Permission == true {
		scpCmd = "/usr/bin/scp -ptr '" + toPath + "'"
	}

	// Run scp
	err = session.Run(scpCmd)

	return
}

// GetData get and return scp format data(remote to local).
//
// example:
//    scp.GetData("/path/remote/path")
func (s *SCPClient) GetData(fromPaths []string) (data *bytes.Buffer, err error) {
	session := s.Session
	if s.Connection != nil {
		session, _ = s.Connection.NewSession()
		if err != nil {
			return
		}
	}
	defer session.Close()

	fin := make(chan bool)
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		// Null Characters(10,000 char)
		nc := strings.Repeat("\x00", 100000)
		fmt.Fprintf(w, nc)
	}()

	buf := new(bytes.Buffer)
	go func() {
		r, _ := session.StdoutPipe()
		buf.ReadFrom(r)
		fin <- true
	}()

	// Create scp command
	fromPathList := []string{}
	for _, fromPath := range fromPaths {
		fromPathList = append(fromPathList, fromPath)
	}
	fromPathString := strings.Join(fromPathList, " ")
	// TODO(blacknon): scpしてる時点でセキュリティもクソもないのだが、OS Command Injectionへの対策を考える
	scpCmd := "/usr/bin/scp -fr " + fromPathString

	// Run scp
	err = session.Run(scpCmd)

	<-fin
	data = buf

	return data, err
}

// PutData put data of scp format as a file(local to remote).
//
// example:
//    scp.PutData(buffer(scp format data),"/path/remote/path")
func (s *SCPClient) PutData(fromData *bytes.Buffer, toPath string) (err error) {
	session := s.Session
	if s.Connection != nil {
		session, _ = s.Connection.NewSession()
		if err != nil {
			return
		}
	}
	defer session.Close()

	// Read Dir or File
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		w.Write(fromData.Bytes())
	}()

	// Create scp command
	// TODO(blacknon): scpしてる時点でセキュリティもクソもないのだが、OS Command Injectionへの対策を考える
	scpCmd := "/usr/bin/scp -tr '" + toPath + "'"
	if s.Permission == true {
		scpCmd = "/usr/bin/scp -ptr '" + toPath + "'"
	}

	err = session.Run(scpCmd)

	return
}
