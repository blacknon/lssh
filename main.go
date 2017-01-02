package main

import (
	"os/user"
	"sort"
	"strings"

	"github.com/blacknon/lssh/conf"
	"github.com/blacknon/lssh/list"
	"github.com/blacknon/lssh/ssh"
)

func main() {
	// Get ConfigFile Path
	usr, _ := user.Current()
	configFile := strings.Replace("~/.lssh.conf", "~", usr.HomeDir, 1)

	// Get List
	listConf := conf.ConfigCheckRead(configFile)

	// Get Server Name List (and sort List)
	nameList := conf.GetNameList(listConf)
	sort.Strings(nameList)

	selectServer := nameList[list.DrawList(nameList, listConf)]
	ssh.ConnectSsh(selectServer, listConf)
	//fmt.Println(selectServer)

}
