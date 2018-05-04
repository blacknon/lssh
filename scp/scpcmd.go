package scp

import (
	"fmt"

	"github.com/blacknon/lssh/conf"
)

type RunInfoScp struct {
	CopyFromType   string
	CopyFromPath   string
	CopyToType     string
	CopyToPath     string
	CopyFromServer []string
	CopyToServer   []string
	ConConfig      conf.Config
}

type ConInfoScp struct {
	Index           int
	Count           int
	Server          string
	ServerMaxLength int
	Addr            string
	Port            string
	User            string
	Pass            string
	KeyPath         string
	CopyToPath      string
}

func (r *RunInfoScp) ScpRun() {
	fmt.Println(r.CopyFromType)
	fmt.Println(r.CopyToType)

	fmt.Println("-----")
	fmt.Println(r.CopyFromPath)
	fmt.Println(r.CopyToPath)

	fmt.Println("-----")
	fmt.Println(r.CopyFromServer)
	fmt.Println(r.CopyToServer)
}
