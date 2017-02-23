package check

import (
	"fmt"
	"os"
	"runtime"
)

type CommandOption struct {
	FilePath string `arg:"-f,help:config file path"`
	Exec     string `arg:"-e,help:exec_command"`
}

func isExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func OsCheck() {
	checkAlertFlag := 0
	execOS := runtime.GOOS
	if execOS == "windows" {
		fmt.Printf("This Program is not working %s.\n", execOS)
		checkAlertFlag = 1
	}

	if checkAlertFlag == 1 {
		os.Exit(1)
	}
}

func CommandExistCheck(cmd string) {
	if (isExist(cmd)) == false {
		fmt.Printf("%s:Not Found.\n", cmd)
	}
}

func DefCommandExistCheck() {
	commandPaths := []string{"/usr/bin/script", "/usr/bin/awk", "/usr/bin/ssh"}
	for _, v := range commandPaths {
		CommandExistCheck(v)
	}
}

func CheckInputServerExit(inputServer string, nameList []string) bool {
	for _, v := range nameList {
		if v == inputServer {
			return true
		}
	}
	return false
}
