package ssh

import (
	"fmt"
	"regexp"
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

	if connectKey != "" {
		//child, _ := gexpect.NewSubProcess("/usr/bin/ssh", "-i", connectKey, connectHost, "-p", connectPort)
		child, _ := gexpect.NewSubProcess("/usr/bin/ssh", "-o", "StrictHostKeyChecking=no", "-i", connectKey, connectHost, "-p", connectPort)
		if err := child.Start(); err != nil {
			fmt.Println(err)
		}
		defer child.Close()

		child.InteractTimeout(86400 * time.Second)
	} else {
		//child, _ := gexpect.NewSubProcess("/usr/bin/ssh", connectHost, "-p", connectPort)
		child, _ := gexpect.NewSubProcess("/usr/bin/ssh", "-o", "StrictHostKeyChecking=no", connectHost, "-p", connectPort)
		if err := child.Start(); err != nil {
			fmt.Println(err)
		}
		defer child.Close()
		if connectPass != "" {
			if idx, _ := child.ExpectTimeout(20*time.Second, regexp.MustCompile("word:")); idx >= 0 {
				child.SendLine(connectPass)
			}
		}
		child.InteractTimeout(86400 * time.Second)
	}
}
