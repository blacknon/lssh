package main

import (
	"fmt"
	"os/user"

	arg "github.com/alexflint/go-arg"
	"github.com/blacknon/lssh/conf"
)

// Command Option
type CommandOption struct {
	Host        []string `arg:"-H,help:Connect servername"`
	List        bool     `arg:"-l,help:print server list"`
	File        string   `arg:"-f,help:config file path"`
	Recursively bool     `arg:"-r,help:Recursively copy entire directories."`
	From        string   `arg:"positional,required,help:Copy from path"`
	To          string   `arg:"positional,required,help:Copy to path"`
}

// Version Setting
func (CommandOption) Version() string {
	return "lscp v0.5.0"
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
	copyFrom := args.From
	copyTo := args.To

	fmt.Println(connectHost)
	fmt.Println(listFlag)
	fmt.Println(configFile)
	fmt.Println(copyFrom)
	fmt.Println(copyTo)
}
