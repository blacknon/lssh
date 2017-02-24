package option

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mitchellh/go-ps"
)

func AddShellHistory() {
	// Get exec command line.
	cName := ""
	for i := 0; i < len(os.Args); i++ {
		if strings.Contains(os.Args[i], " ") {
			os.Args[i] = "\"" + os.Args[i] + "\""
		}
		cName = strings.Join(os.Args[:], " ") + " "
	}

	// Get shell type
	shellPid, _ := ps.FindProcess(os.Getppid())
	switch shellType := shellPid.Executable(); shellType {
	case "bash":
		err := exec.Command("history", "-s", cName).Start()
		if err != nil {
			panic(err)
		}
	case "zsh":
		err := exec.Command("print", "-s", cName).Start()
		if err != nil {
			panic(err)
		}
	default:
		fmt.Println("No")
	}
}
