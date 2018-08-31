package conf

import (
	"fmt"
	"os"
	"os/user"

	"regexp"

	"github.com/blacknon/lssh/common"
	"github.com/kevinburke/ssh_config"
)

func ReadSshConfig() (cfg *ssh_config.Config, err error) {
	// Open ~/.ssh/config
	sshConfigFile := common.GetFullPath("~/.ssh/config")
	f, err := os.Open(sshConfigFile)
	if err != nil {
		return
	}

	cfg, err = ssh_config.Decode(f)
	return
}

func GenConf() {
	// get user infomation
	usr, _ := user.Current()

	// Read .ssh/config
	configData, err := ReadSshConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error)
		os.Exit(1)
	}

	// Get Node names
	hostList := []string{}
	for _, host := range configData.Hosts {
		re := regexp.MustCompile("\\*")
		for _, pattern := range host.Patterns {
			if !re.MatchString(pattern.String()) {
				hostList = append(hostList, pattern.String())
			}
		}
	}

	// print conf [log]
	fmt.Println("[log]")
	fmt.Println("enable = false")
	fmt.Println("dirpath = \"/path/to/dir/<Date>/<Hostname>\"")
	fmt.Println("")

	// print conf [server]
	for _, server := range hostList {
		// Get ssh/config value
		addr := ssh_config.Get(server, "Hostname")
		user := ssh_config.Get(server, "User")
		key := ssh_config.Get(server, "IdentityFile")
		port := ssh_config.Get(server, "Port")
		note := "from ~/.ssh/config"

		fmt.Printf("[server.%s]\n", server)
		fmt.Printf("addr = \"%s\"\n", addr)
		if user == "" {
			fmt.Printf("user = \"%s\"\n", usr.Username)
		} else {
			fmt.Printf("user = \"%s\"\n", user)
		}
		if key == "" {
			fmt.Printf("key = \"%s\"\n", "~/.ssh/id_rsa")
		} else {
			fmt.Printf("key = \"%s\"\n", key)
		}
		if port != "" {
			fmt.Printf("port = \"%s\"\n", port)
		}
		fmt.Printf("note = \"%s\"\n", note)
		fmt.Println("")

	}
}
