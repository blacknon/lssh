// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package ssh

import (
	"fmt"
	"os"
	"strings"
)

// PrintSelectServer is printout select server.
// use ssh login header.
func (r *Run) PrintSelectServer() {
	serverListStr := strings.Join(r.ServerList, ",")
	fmt.Fprintf(os.Stderr, "Select Server :%s\n", serverListStr)
}

// printRunCommand is printout run command.
// use ssh command run header.
func (r *Run) printRunCommand() {
	runCmdStr := strings.Join(r.ExecCmd, " ")
	fmt.Fprintf(os.Stderr, "Run Command   :%s\n", runCmdStr)
}

// printPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printPortForward(m, forwardLocal, forwardRemote string) {
	if forwardLocal != "" && forwardRemote != "" {
		var mode, arrow string
		switch m {
		case "L", "":
			mode = "LOCAL "
			arrow = " =>"
		case "R":
			mode = "REMOTE"
			arrow = "<= "
		}

		fmt.Fprintf(os.Stderr, "Port Forward  :%s\n", mode)
		fmt.Fprintf(os.Stderr, "               local[%s] %s remote[%s]\n", forwardLocal, arrow, forwardRemote)
	}
}

// printDynamicPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printDynamicPortForward(port string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "DynamicForward:%s\n", port)
		fmt.Fprintf(os.Stderr, "               %s\n", "connect Socks5.")
	}
}

// printReverseDynamicPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printReverseDynamicPortForward(port string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "ReverseDynamicForward:%s\n", port)
		fmt.Fprintf(os.Stderr, "                      %s\n", "connect Socks5.")
	}
}

// printPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printHTTPDynamicPortForward(port string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "HTTPDynamicForward:%s\n", port)
		fmt.Fprintf(os.Stderr, "                   %s\n", "connect http.")
	}
}

// printHTTPReverseDynamicPortForward is printout port forwarding.
// use ssh command run header. only use shell().
func (r *Run) printHTTPReverseDynamicPortForward(port string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "HTTPReverseDynamicForward:%s\n", port)
		fmt.Fprintf(os.Stderr, "                        %s\n", "connect http.")
	}
}

// printNFSDynamicForward is printout forwarding.
// use ssh command run header. only use shell().
func (r *Run) printNFSDynamicForward(port, path string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "NFSDynamicForward:%s:%s\n", port, path)
		fmt.Fprintf(os.Stderr, "                 %s\n", "connect NFS.")
	}
}

// printNFSReverseDynamicForward is printout forwarding.
// use ssh command run header. only use shell().
func (r *Run) printNFSReverseDynamicForward(port, path string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "NFSReverseDynamicForward:%s:%s\n", port, path)
		fmt.Fprintf(os.Stderr, "                      %s\n", "connect NFS.")
	}
}

func (r *Run) printSMBDynamicForward(port, path string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "SMBDynamicForward:%s:%s\n", port, path)
		fmt.Fprintf(os.Stderr, "                 %s\n", "connect SMB.")
	}
}

func (r *Run) printSMBReverseDynamicForward(port, path string) {
	if port != "" {
		fmt.Fprintf(os.Stderr, "SMBReverseDynamicForward:%s:%s\n", port, path)
		fmt.Fprintf(os.Stderr, "                      %s\n", "connect SMB.")
	}
}

// printProxy is printout proxy route.
// use ssh command run header. only use shell().
func (r *Run) printProxy(server string) {
	array := []string{}

	proxyRoute, err := getProxyRoute(server, r.Conf)
	if err != nil || len(proxyRoute) == 0 {
		return
	}

	localhost := "localhost"
	targethost := server
	array = append(array, localhost)

	for _, pxy := range proxyRoute {
		var sep string
		if pxy.Type == "command" {
			sep = ":"
		} else {
			sep = "://"
		}

		str := "[" + pxy.Type + sep + pxy.Name
		if pxy.Port != "" {
			str = str + ":" + pxy.Port
		}
		str = str + "]"

		array = append(array, str)
	}

	array = append(array, targethost)
	header := strings.Join(array, " => ")
	fmt.Fprintf(os.Stderr, "Proxy         :%s\n", header)
}
