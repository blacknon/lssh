package scp

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blacknon/lssh/conf"
	lssh_ssh "github.com/blacknon/lssh/ssh"
	"golang.org/x/crypto/ssh"
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

func printScpWord(baseDir string, path string, toName string) (scpWord string) {
	dPerm := "0755"
	fPerm := "0644"

	buf := []string{}
	relPath, _ := filepath.Rel(baseDir, path)
	dir := filepath.Dir(relPath)

	if len(dir) > 0 && dir != "." {
		dirList := strings.Split(dir, "/")
		for _, dirName := range dirList {
			buf = append(buf, fmt.Sprintln("D"+dPerm, 0, dirName))
		}
	}

	fInfo, _ := os.Stat(path)

	if !fInfo.IsDir() {
		content, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		buf = append(buf, fmt.Sprintln("C"+fPerm, len(content), toName))
		buf = append(buf, fmt.Sprintf(string(content)))
		buf = append(buf, fmt.Sprintf("\x00"))
	}

	if len(dir) > 0 && dir != "." {
		buf = append(buf, fmt.Sprintln("E"))
	}
	scpWord = strings.Join(buf, "")
	return
}

//func (r *RunInfoScp) scpPull(conn *ssh.Client) {
//
//}

func (r *RunInfoScp) scpPush(conn *ssh.Client, toDir string, toName string) {
	defer conn.Close()

	// New Session
	session, err := conn.NewSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect erro %v,cannot open new session: %v \n", err)
	}
	defer session.Close()

	pInfo, _ := os.Stat(r.CopyFromPath)
	if pInfo.IsDir() {
		toDir = r.CopyToPath
	}

	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		if pInfo.IsDir() {
			pList, _ := conf.PathWalkDir(r.CopyFromPath)
			for _, i := range pList {
				scpData := printScpWord(r.CopyFromPath, i, filepath.Base(i))
				if len(scpData) > 0 {
					fmt.Fprintf(w, scpData)
				}
			}
		} else {
			fPerm := "0644"
			content, err := ioutil.ReadFile(r.CopyFromPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			fmt.Fprintln(w, "C"+fPerm, len(content), toName)
			fmt.Fprint(w, string(content))
			fmt.Fprint(w, "\x00")
		}
	}()

	if err := session.Run("/usr/bin/scp -pqtr " + toDir); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to run: "+err.Error())
	}
}

func (r *RunInfoScp) forScpPull() {

}

func (r *RunInfoScp) forScpPush() {
	finished := make(chan bool)
	x := 1
	for _, v := range r.CopyToServer {
		y := x
		c := new(lssh_ssh.ConInfoCmd)
		conServer := v
		go func() {
			c.Index = y
			c.Server = conServer
			c.Addr = r.ConConfig.Server[c.Server].Addr
			c.User = r.ConConfig.Server[c.Server].User
			c.Port = "22"
			if r.ConConfig.Server[c.Server].Port != "" {
				c.Port = r.ConConfig.Server[c.Server].Port
			}
			c.Pass = ""
			if r.ConConfig.Server[c.Server].Pass != "" {
				c.Pass = r.ConConfig.Server[c.Server].Pass
			}
			c.KeyPath = ""
			if r.ConConfig.Server[c.Server].Key != "" {
				c.KeyPath = r.ConConfig.Server[c.Server].Key
			}

			connect, err := c.CreateConnect()
			if err != nil {
				fmt.Fprintf(os.Stderr, "cannot connect %v, %v: %v \n", c.Server, c.Port, err)
			}
			toName := filepath.Base(r.CopyToPath)
			toDir := filepath.Dir(r.CopyToPath)

			match, _ := regexp.MatchString("/$", r.CopyToPath)
			if toName == toDir || match {
				toName = filepath.Base(r.CopyFromPath)
			}

			r.scpPush(connect, toDir, toName)

			finished <- true
		}()
		x++
	}

	for i := 1; i <= len(r.CopyToServer); i++ {
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
		//r.forScpPull()
		fmt.Println("remote to local")
	case r.CopyFromType == "local" && r.CopyToType == "remote":
		r.forScpPush()
	}
}
