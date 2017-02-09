package ssh

import (
	"fmt"
	"regexp"
	"runtime"
	"time"

	"github.com/blacknon/lssh/conf"
	"github.com/shavac/gexpect"
)

// OS ssh command Rapper
func ConnectSsh(connectServer string, confList conf.Config) {
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
		logFile := "$(date +%Y%m%d_%H%M%S)_" + connectServer + ".log"
		logFilePATH := logDirPath + "/" + logFile
		awkCmd := ">(awk '{print strftime(\"%F %T \") $0}{fflush() }'>>" + logFilePATH + ")"

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
		if idx, _ := child.ExpectTimeout(20*time.Second, regexp.MustCompile("word:")); idx >= 0 {
			child.SendLine(connectPass)
		}
	}

	// timeout
	child.InteractTimeout(86400 * time.Second)
}
