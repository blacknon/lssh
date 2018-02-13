package main

import (
	"fmt"
	"os/user"

	arg "github.com/alexflint/go-arg"
	"github.com/blacknon/lssh/conf"
)

// Command Option
type CommandOption struct {
	Host []string `arg:"-H,help:Connect servername"`
	List bool     `arg:"-l,help:print server list"`
	File string   `arg:"-f,help:config file path"`
	From string   `arg:"positional,help:Copy from path"`
	To   string   `arg:"positional,help:Copy to path"`
}

// Version Setting
func (CommandOption) Version() string {
	return "lscp v0.4.3"
}

func main() {
	// Exec Before Check
	conf.CheckBeforeStart()

	// Set default value
	usr, _ := user.Current()
	defaultConfPath := usr.HomeDir + "/.lssh.conf"

	// get Command Option
	var args struct {
		CommandOption
	}

	// Default Value
	args.File = defaultConfPath
	arg.MustParse(&args)

	// set option value
	connectHost := args.Host
	listFlag := args.List
	configFile := args.File
	cpFrom := args.From
	cpTo := args.To

	fmt.Println(connectHost)
	fmt.Println(listFlag)
	fmt.Println(configFile)
	fmt.Println(from)
	fmt.Println(to)

}
