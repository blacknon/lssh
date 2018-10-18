package check

import (
	"fmt"
	"os"
)

// @brief
//
func ExistServer(inputServer []string, nameList []string) bool {
	for _, nv := range nameList {
		for _, iv := range inputServer {
			if nv == iv {
				return true
			}
		}
	}
	return false
}

// @brief
//
func CheckScpPathResult(fromResult bool, toResult bool) {
	if fromResult == false || toResult == false {
		fmt.Fprintln(os.Stderr, "The format of the specified argument is incorrect.")
		os.Exit(1)
	}
}

// @brief
//
func CheckScpPathType(fromType string, toType string, countHosts int) {
	// Check HostType local only
	if fromType == "local" && toType == "local" {
		fmt.Fprintln(os.Stderr, "It does not correspond local to local copy.")
		os.Exit(1)
	}

	// Check HostType remote only and Host flag
	if fromType == "remote" && toType == "remote" && countHosts != 0 {
		fmt.Fprintln(os.Stderr, "In the case of remote to remote copy, it does not correspond to Host option.")
		os.Exit(1)
	}
}
