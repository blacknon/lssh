package ssh

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/blacknon/lssh/conf"
	"github.com/shavac/gexpect"
)

// OS ssh command Rapper
func ConnectSsh(connectServer string, serverList conf.Config) {
	connectUser := serverList.Server[connectServer].User
	connectAddr := serverList.Server[connectServer].Addr
	var connectPort string
	if serverList.Server[connectServer].Port == "" {
		connectPort = "22"
	} else {
		connectPort = serverList.Server[connectServer].Port
	}
	connectPass := serverList.Server[connectServer].Pass
	connectKey := serverList.Server[connectServer].Key
	connectHost := connectUser + "@" + connectAddr

	// ssh command Args
	connectArgStr := ""
	if connectKey != "" {
		connectArgStr = "-i " + connectKey + " " + connectHost + " -p " + connectPort
	} else {
		connectArgStr = connectHost + " -p " + connectPort
	}
	connectArgMap := strings.Split(connectArgStr, " ")

	// exec ssh command
	child, _ := gexpect.NewSubProcess("/usr/bin/ssh", connectArgMap...)
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
