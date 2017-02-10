package ssh

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/blacknon/lssh/conf"
	"github.com/shavac/gexpect"
)

// OS ssh command Rapper
func ConnectSsh(connectServer string, confList conf.Config, execRemoteCmd string) {
	// Get log config value
	logEnable := confList.Log.Enable
	logDirPath := confList.Log.Dir

	// Get ssh config value
	connectUser := confList.Server[connectServer].User
	connectAddr := confList.Server[connectServer].Addr
	var connectPort string
	if confList.Server[connectServer].Port == "" {
		connectPort = "22"
	} else {
		connectPort = confList.Server[connectServer].Port
	}
	connectPass := confList.Server[connectServer].Pass
	connectKey := confList.Server[connectServer].Key
	connectHost := connectUser + "@" + connectAddr

	// ssh command Args
	sshCmd := ""
	if connectKey != "" {
		// "/usr/bin/ssh -o 'StrictHostKeyChecking no' -o 'NumberOfPasswordPrompts 1' -i connectKey connectUser@connectAddr -p connectPort"
		sshCmd = "/usr/bin/ssh -o 'StrictHostKeyChecking no' -o 'NumberOfPasswordPrompts 1' -i " + connectKey + " " + connectHost + " -p " + connectPort
	} else {
		// "/usr/bin/ssh -o 'StrictHostKeyChecking no' -o 'NumberOfPasswordPrompts 1' connectUser@connectAddr -p connectPort"
		sshCmd = "/usr/bin/ssh -o 'StrictHostKeyChecking no' -o 'NumberOfPasswordPrompts 1' " + connectHost + " -p " + connectPort
	}

	// log Enable
	execCmd := ""
	if logEnable == true {
		execOS := runtime.GOOS
		execCmd = "/usr/bin/script"

		// ~ replace User current Directory
		usr, _ := user.Current()
		logDirPath := strings.Replace(logDirPath, "~", usr.HomeDir, 1)

		// mkdir logDIr
		if err := os.MkdirAll(logDirPath, 0755); err != nil {
			fmt.Println(err)
		}

		// Golang time.format (https://golang.org/src/time/format.go)
		logFile := time.Now().Format("20060102_150405") + "_" + connectServer + ".log"
		logFilePATH := logDirPath + "/" + logFile
		awkCmd := ">(awk '{print strftime(\"%F %T \") $0}{fflush() }'>>" + logFilePATH + ")"

		// exec_command option check
		if execRemoteCmd != "" {
			sshCmd = sshCmd + " " + execRemoteCmd
			logHeadContent := []byte("Exec command: " + execRemoteCmd + "\n\n" +
				"=============================\n")
			ioutil.WriteFile(logFilePATH, logHeadContent, os.ModePerm)
		}

		if execOS == "linux" || execOS == "android" {
			execCmd = "/usr/bin/script -qf -c \"" + sshCmd + "\" " + awkCmd
		} else {
			execCmd = "/usr/bin/script -qF " + awkCmd + " " + sshCmd
		}

	} else {
		execCmd = sshCmd
	}

	// exec ssh command
	child, _ := gexpect.NewSubProcess("/bin/bash", "-c", execCmd)

	if err := child.Start(); err != nil {
		fmt.Println(err)
	}
	defer child.Close()

	// Password Input
	if connectPass != "" {
		passwordPrompt := "word:"
		if idx, _ := child.ExpectTimeout(20*time.Second, regexp.MustCompile(passwordPrompt)); idx >= 0 {
			child.SendLine(connectPass)
		}
	}

	// timeout
	child.InteractTimeout(86400 * time.Second)
}
