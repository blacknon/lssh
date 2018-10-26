package ssh

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	scplib "github.com/blacknon/go-scplib"
	"github.com/blacknon/lssh/conf"
)

type CopyConInfo struct {
	IsRemote bool
	Path     []string
	Server   []string
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
	// remote to remote
	case r.From.IsRemote && r.To.IsRemote:
		r.run("pull")
		r.run("push")

	// remote to local
	case r.From.IsRemote && !r.To.IsRemote:
		r.run("pull")

	// local to remote
	case !r.From.IsRemote && r.To.IsRemote:
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
	if r.From.IsRemote && r.To.IsRemote {
		err = scp.PutData(r.CopyData, r.To.Path[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to run %v \n", err)
		}
	} else {
		err = scp.PutFile(r.From.Path, r.To.Path[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to run %v \n", err)
		}
	}
}

// @brief:
//    pull scp
func (r *RunScp) pull(target string, scp *scplib.SCPClient) {
	var err error

	// scp pull
	if r.From.IsRemote && r.To.IsRemote {
		r.CopyData, err = scp.GetData(r.From.Path)
	} else {
		toPath := createServersDir(target, r.From.Server, r.To.Path[0])
		err = scp.GetFile(r.From.Path, toPath)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run %v \n", err.Error())
	}
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
