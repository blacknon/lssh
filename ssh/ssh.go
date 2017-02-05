package ssh

import (
	"fmt"
	"regexp"
	"time"

	"github.com/blacknon/lssh/conf"
	"github.com/shavac/gexpect"
)

// OS ssh command Rapper
func ConnectSsh(connectServer string, confList conf.Config) {
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
	connectCmd := ""
	//connectArgStr := ""
	if connectKey != "" {
		//connectArgStr = "/usr/bin/ssh" + "-i " + connectKey + " " + connectHost + " -p " + connectPort
		connectCmd = "/usr/bin/ssh -i " + connectKey + " " + connectHost + " -p " + connectPort
	} else {
		//connectArgStr = "/usr/bin/ssh" + connectHost + " -p " + connectPort
		connectCmd = "/usr/bin/ssh " + connectHost + " -p " + connectPort
	}
	fmt.Println(connectCmd)
	//connectArgMap := strings.Split(connectArgStr, " ")

	// exec ssh command
	//child, _ := gexpect.NewSubProcess("/usr/bin/ssh", connectArgMap...)
	//child, _ := gexpect.NewSubProcess("/bin/bash", "-c", connectCmd)
	child, _ := gexpect.NewSubProcess("/usr/bin/script", "-c", connectCmd)

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
