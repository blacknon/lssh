// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

/*
Package sshlib is a library to easily connect with ssh by go.
You can perform multiple proxy, x11 forwarding, PKCS11 authentication, etc...

Example simple ssh shell

It is example code. simple connect ssh shell. You can also do tab completion, send sigint signal(Ctrl+C).

	package main

	import (
		"fmt"
		"os"

		sshlib "github.com/blacknon/go-sshlib"
		"golang.org/x/crypto/ssh"
	)

	var (
		host     = "target.com"
		port     = "22"
		user     = "user"
		password = "password"

		termlog = "./test_termlog"
	)

	func main() {
		// Create sshlib.Connect
		con := &sshlib.Connect{
			// If you use x11 forwarding, please set to true.
			ForwardX11: false,

			// If you use ssh-agent forwarding, please set to true.
			// And after, run `con.ConnectSshAgent()`.
			ForwardAgent: false,
		}

		// Create ssh.AuthMethod
		authMethod := sshlib.CreateAuthMethodPassword(password)

		// If you use ssh-agent forwarding, uncomment it.
		// con.ConnectSshAgent()

		// Connect ssh server
		err := con.CreateClient(host, port, user, []ssh.AuthMethod{authMethod})
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Set terminal log
		con.SetLog(termlog, false)

		// Start ssh shell
		con.Shell()
	}



Example simple ssh proxy shell

Multple proxy by ssh connection is also available. Please refer to the sample code for usage with http and socks5 proxy.

	package main

	import (
		"fmt"
		"os"

		sshlib "github.com/blacknon/go-sshlib"
		"golang.org/x/crypto/ssh"
	)

	var (
		// Proxy ssh server
		host1     = "proxy.com"
		port1     = "22"
		user1     = "user"
		password1 = "password"

		// Target ssh server
		host2     = "target.com"
		port2     = "22"
		user2     = "user"
		password2 = "password"

		termlog = "./test_termlog"
	)

	func main() {
		// ==========
		// proxy connect
		// ==========

		// Create proxy sshlib.Connect
		proxyCon := &sshlib.Connect{}

		// Create proxy ssh.AuthMethod
		proxyAuthMethod := sshlib.CreateAuthMethodPassword(password1)

		// Connect proxy server
		err := proxyCon.CreateClient(host1, port1, user1, []ssh.AuthMethod{proxyAuthMethod})
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// ==========
		// target connect
		// ==========

		// Create target sshlib.Connect
		targetCon := &sshlib.Connect{
			ProxyDialer: proxyCon.Client,
		}

		// Create target ssh.AuthMethod
		targetAuthMethod := sshlib.CreateAuthMethodPassword(password2)

		// Connect target server
		err = targetCon.CreateClient(host2, port2, user2, []ssh.AuthMethod{targetAuthMethod})
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Set terminal log
		targetCon.SetLog(termlog, false)

		// Start ssh shell
		targetCon.Shell()
	}


This library was created for my ssh client (https://github.com/blacknon/lssh)
*/
package sshlib
