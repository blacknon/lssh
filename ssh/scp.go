package ssh

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	scplib "github.com/blacknon/go-scplib"
	"github.com/blacknon/lssh/conf"
)

type CopyConInfo struct {
	Type   string
	Path   string
	Server []string
}

type RunScp struct {
	From       CopyConInfo
	To         CopyConInfo
	CopyData   *bytes.Buffer
	Permission bool
	Config     conf.Config
}

// @brief:
//    start
func (r *RunScp) Start() {
	switch {
	case r.From.Type == "remote" && r.To.Type == "remote":
		r.run("pull")
		r.run("push")
	case r.From.Type == "remote" && r.To.Type == "local":
		r.run("pull")
	case r.From.Type == "local" && r.To.Type == "remote":
		r.run("push")
	}
}

// @brief:
//     run scp
func (r *RunScp) run(mode string) {
	finished := make(chan bool)

	// set target list
	targetList := []string{}
	switch mode {
	case "push":
		targetList = r.To.Server
	case "pull":
		targetList = r.From.Server
	}

	for _, value := range targetList {
		target := value

		go func() {
			// create ssh connect
			con := new(Connect)
			con.Server = target
			con.Conf = r.Config

			// create ssh session
			session, err := con.CreateSession()
			if err != nil {
				fmt.Fprintf(os.Stderr, "cannot connect %v, %v \n", target, err)
				finished <- true
				return
			}
			defer session.Close()

			// create scp client
			scp := new(scplib.SCPClient)
			scp.Permission = r.Permission
			scp.Session = session

			switch mode {
			case "push":
				r.push(target, scp)
			case "pull":
				r.pull(target, scp)
			}

			fmt.Fprintf(os.Stderr, "%v(%v) is finished.\n", target, mode)
			finished <- true
		}()
	}

	for i := 1; i <= len(targetList); i++ {
		<-finished
	}
}

// @brief:
//    push scp
func (r *RunScp) push(target string, scp *scplib.SCPClient) {
	var err error
	if r.From.Type == "remote" && r.To.Type == "remote" {
		err = scp.PutData(r.CopyData, r.To.Path)
	} else {
		err = scp.PutFile(r.From.Path, r.To.Path)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run %v \n", err)
	}
}

// @brief:
//    pull scp
func (r *RunScp) pull(target string, scp *scplib.SCPClient) {
	var err error
	// scp pull
	if r.From.Type == "remote" && r.To.Type == "remote" {
		r.CopyData, err = scp.GetData(r.From.Path)
	} else {
		toPath := createServersDir(target, r.To.Server, r.To.Path)
		err = scp.GetFile(r.From.Path, toPath)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run %v \n", err.Error())
	}
}

// @brief:
//    parse lscp args path
func ParseScpPath(arg string) (hostType string, path string, result bool) {
	argArray := strings.SplitN(arg, ":", 2)

	// check split count
	if len(argArray) != 2 {
		hostType = ""
		path = ""
		result = false
		return
	}

	pathType := strings.ToLower(argArray[0])
	switch pathType {
	case "local", "l":
		hostType = "local"
		path = argArray[1]
	case "remote", "r":
		hostType = "remote"
		path = argArray[1]
	default:
		hostType = ""
		path = ""
		result = false
		return
	}
	result = true
	return
}

func createServersDir(target string, serverList []string, toPath string) (path string) {
	if len(serverList) > 1 {
		toDir := filepath.Dir(toPath)
		toBase := filepath.Base(toPath)
		serverDir := toDir + "/" + target

		err := os.Mkdir(serverDir, os.FileMode(uint32(0755)))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to run: "+err.Error())
		}

		if toDir != toBase {
			toPath = serverDir + "/" + toBase
		} else {
			toPath = serverDir + "/"
		}
	}

	path = toPath
	return
}
