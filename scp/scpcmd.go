package scp

import (
	"fmt"
	"os"

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

	ServrNameMax int
	ConConfig    conf.Config
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
				err := c.PutFile(r.CopyFromPath, r.CopyToPath)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Failed to run: "+err.Error())
				}
				fmt.Println(conServer + " is exit.")

			case "pull":
				// scp pull
				err := c.GetFile(r.CopyFromPath, r.CopyToPath)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Failed to run: "+err.Error())
				}
				fmt.Println(conServer + " is exit.")
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
		fmt.Println("remote to remote")
	case r.CopyFromType == "remote" && r.CopyToType == "local":
		r.forScp("pull")
	case r.CopyFromType == "local" && r.CopyToType == "remote":
		r.forScp("push")
	}
}
