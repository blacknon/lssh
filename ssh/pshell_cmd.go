package ssh

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

// checkLocalCommand return bool  if each pipeline contains built-in commands or local machine commands.
func (ps *pShell) checkBuildInCommand(pmap map[int][]pipeLine) (isInLocalCmd bool) {
	// build in command regex
	buildinRegex := regexp.MustCompile(`^%.*`)

	for _, pipelines := range pmap {
		for _, p := range pipelines {
			// get pipeline command
			c := p.Args[0]

			// check command
			switch {
			case c == "exit", c == "quit", c == "clear":
				isInLocalCmd = true

			case buildinRegex.MatchString(c):
				isInLocalCmd = true
			}
		}
	}

	return
}

// runBuildInCommand
func (ps *pShell) runBuildInCommand(pLine pipeLine) (isLocal bool, err error) {
	// get 1st element
	command := pLine.Args[0]

	// check local command
	switch command {
	// exit or quit
	case "exit", "quit":
		isLocal = true

		os.Exit(0)

	// clear
	case "clear":
		isLocal = true

		fmt.Printf("\033[H\033[2J")

	// %history
	case "%history":
		isLocal = true

		ps.buildin_history()

	// %out [num]
	case "%out":
		isLocal = true

		num := 0
		if len(pLine.Args) > 1 {
			num, err = strconv.Atoi(pLine.Args[1])
			if err != nil {
				return
			}
		}

		ps.buildin_out(num)
	}

	return
}

// localCmd_history is printout history (shell history)
// TODO(blacknon): 通番をつけて、bash等のように `!<N>` で実行できるようにする
func (ps *pShell) buildin_history() {
	data, err := ps.GetHistoryFromFile()
	if err != nil {
		return
	}

	for _, h := range data {
		fmt.Printf("%s: %s\n", h.Timestamp, h.Command)
	}
}

// localCmd_out is print exec history at number
// example:
//     - %out
//     - %out <num>
func (ps *pShell) buildin_out(num int) {
	histories := ps.History[num]

	i := 0
	for _, h := range histories {
		// if first, print out command
		if i == 0 {
			fmt.Printf("Command: %s\n", h.Command)
		}
		i += 1
		fmt.Printf(h.Result)
	}
}
