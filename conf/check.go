package conf

import (
	"fmt"
	"os"
	"runtime"
)

func isExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func checkOS() {
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

func checkCommandExist(cmd string) {
	if (isExist(cmd)) == false {
		fmt.Printf("%s:Not Found.\n", cmd)
	}
}

func checkDefCommandExist() {
	commandPaths := []string{"/usr/bin/ssh"}
	for _, v := range commandPaths {
		checkCommandExist(v)
	}
}

func CheckBeforeStart() {
	checkOS()
	checkDefCommandExist()
}

func checkServerConf(c Config) (rFlg bool) {
	rFlg = true
	for k, v := range c.Server {
		// Address Input Check
		if v.Addr == "" {
			fmt.Printf("%s: 'addr' is not inserted.\n", k)
			rFlg = false
		}

		// User Input Check
		if v.User == "" {
			fmt.Printf("%s: 'user' is not inserted.\n", k)
			rFlg = false
		}

		// Password or Keyfile Input Check
		if v.Pass == "" && v.Key == "" {
			fmt.Printf("%s: Both Password and KeyPath are entered.Please enter either.\n", k)
			rFlg = false
		}
	}
	return
}

func CheckInputServerExit(inputServer []string, nameList []string) bool {
	for _, nv := range nameList {
		for _, iv := range inputServer {
			if nv == iv {
				return true
			}
		}
	}
	return false
}
