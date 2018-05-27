package scp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blacknon/go-scplib"
	"github.com/blacknon/lssh/conf"
)

type RunInfoScp struct {
	CopyFromType   string
	CopyFromPath   string
	CopyFromServer []string
	CopyToType     string
	CopyToPath     string
	CopyToServer   []string
	CopyData       *bytes.Buffer
	ServrNameMax   int
	ConConfig      conf.Config
}

func (r *RunInfoScp) forScp(mode string) {
	finished := make(chan bool)
	x := 1

	targetServer := []string{}
	if mode == "push" {
		targetServer = r.CopyToServer
	} else {
		targetServer = r.CopyFromServer
	}
	for _, v := range targetServer {
		//y := x
		conServer := v
		go func() {
			c := new(scplib.SCPClient)
			//c.Server = conServer
			c.Addr = r.ConConfig.Server[conServer].Addr
			c.User = r.ConConfig.Server[conServer].User
			c.Port = "22"
			if r.ConConfig.Server[conServer].Port != "" {
				c.Port = r.ConConfig.Server[conServer].Port
			}
			c.Pass = ""
			if r.ConConfig.Server[conServer].Pass != "" {
				c.Pass = r.ConConfig.Server[conServer].Pass
			}
			c.KeyPath = ""
			if r.ConConfig.Server[conServer].Key != "" {
				c.KeyPath = r.ConConfig.Server[conServer].Key
			}

			err := c.CreateConnect()
			if err != nil {
				fmt.Fprintf(os.Stderr, "cannot connect %v:%v, %v \n", conServer, c.Port, err)
				finished <- true
				return
			}

			switch mode {
			case "push":
				// scp push
				if r.CopyToType == r.CopyFromType {
					err := c.PutData(r.CopyData, r.CopyToPath)
					if err != nil {
						fmt.Fprintln(os.Stderr, "Failed to run: "+err.Error())
					}
				} else {
					err := c.PutFile(r.CopyFromPath, r.CopyToPath)
					if err != nil {
						fmt.Fprintln(os.Stderr, "Failed to run: "+err.Error())
					}
					fmt.Println(conServer + " is exit.")
				}

			case "pull":
				toPath := r.CopyToPath

				// if multi server connect => path = /path/to/Dir/<ServerName>/Base
				if len(targetServer) > 1 {
					toDir := filepath.Dir(r.CopyToPath)
					toBase := filepath.Base(r.CopyToPath)
					serverDir := toDir + "/" + conServer

					err = os.Mkdir(serverDir, os.FileMode(uint32(0755)))
					if err != nil {
						fmt.Fprintln(os.Stderr, "Failed to run: "+err.Error())
					}

					if toDir != toBase {
						toPath = serverDir + "/" + toBase
					} else {
						toPath = serverDir + "/"
					}
				}

				// scp pull
				if r.CopyToType == r.CopyFromType {
					//buf := new(bytes.Buffer)
					r.CopyData, err = c.GetData(r.CopyFromPath)
					if err != nil {
						fmt.Fprintln(os.Stderr, "Failed to run: "+err.Error())
					}
				} else {
					err := c.GetFile(r.CopyFromPath, toPath)
					if err != nil {
						fmt.Fprintln(os.Stderr, "Failed to run: "+err.Error())
					}
					fmt.Println(conServer + " is exit.")
				}
			}

			finished <- true
		}()
		x++
	}

	for i := 1; i <= len(targetServer); i++ {
		<-finished
	}
}

func (r *RunInfoScp) ScpRun() {
	// get connect server name max length
	for _, conServerName := range append(r.CopyFromServer, r.CopyToServer...) {
		if r.ServrNameMax < len(conServerName) {
			r.ServrNameMax = len(conServerName)
		}
	}

	switch {
	case r.CopyFromType == "remote" && r.CopyToType == "remote":
		r.forScp("pull")
		r.forScp("push")
	case r.CopyFromType == "remote" && r.CopyToType == "local":
		r.forScp("pull")
	case r.CopyFromType == "local" && r.CopyToType == "remote":
		r.forScp("push")
	}
}
