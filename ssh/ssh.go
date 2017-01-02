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
		//connectPort = strconv.Itoa(serverList.Server[connectServer].Port)
		connectPort = serverList.Server[connectServer].Port
	}
	connectPass := serverList.Server[connectServer].Pass
	connectKey := serverList.Server[connectServer].Key

	connectHost := connectUser + "@" + connectAddr

	//var sshCommand []string
	//if connectKey != "" {
	//	sshCommand = {"/usr/bin/ssh", "-i", connectKey, connectHost, "-p", connectPort}
	//} else {
	//	sshCommand = {"/usr/bin/ssh", connectHost, "-p", connectPort}
	//}

	if connectKey != "" {
		child, _ := gexpect.NewSubProcess("/usr/bin/ssh", "-i", connectKey, connectHost, "-p", connectPort)
		if err := child.Start(); err != nil {
			fmt.Println(err)
		}
		defer child.Close()
		//if connectPass != "" {
		//	if idx, _ := child.ExpectTimeout(10*time.Second, regexp.MustCompile(":")); idx >= 0 {
		//		child.SendLine(connectPass)
		//	}
		//}
		child.InteractTimeout(20 * time.Second)
	} else {
		child, _ := gexpect.NewSubProcess("/usr/bin/ssh", connectHost, "-p", connectPort)
		if err := child.Start(); err != nil {
			fmt.Println(err)
		}
		defer child.Close()
		if connectPass != "" {
			if idx, _ := child.ExpectTimeout(10*time.Second, regexp.MustCompile(":")); idx >= 0 {
				child.SendLine(connectPass)
			}
		}
		child.InteractTimeout(20 * time.Second)
	}
	//child, _ := gexpect.NewSubProcess("/usr/bin/ssh", connectHost)
	//fmt.Println(connectPass)
	//child, _ := gexpect.NewSubProcess("/usr/bin/ssh", connectHost, "-p", connectPort)

	//child, _ := gexpect.NewSubProcess(sshCommand)

	//if err := child.Start(); err != nil {
	//	fmt.Println(err)
	//}
	//defer child.Close()
	//if connectPass != "" {
	//	if idx, _ := child.ExpectTimeout(10*time.Second, regexp.MustCompile(":")); idx >= 0 {
	//		child.SendLine(connectPass)
	//	}
	//}
	//child.InteractTimeout(20 * time.Second)
}

//fmt.Println(sshCommand)
//child, err := gexpect.Spawn(sshCommand)
//if err != nil {
//	panic(err)
//}
//if connectPass != "" {
//	child.Expect(":")
//	child.SendLine(connectPass)
//}
//child.Interact()
//child.Wait()
//child.Close()
//}
