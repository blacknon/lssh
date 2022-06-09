go-sshlib
====

[![GoDoc](https://godoc.org/github.com/blacknon/go-sshlib?status.svg)](https://godoc.org/github.com/blacknon/go-sshlib)

## About

A library to handle ssh easily with Golang.It can do multiple proxy, x11 forwarding, etc.
Supported on Linux, macOS and Windows.

If use **pkcs11** authentication, cgo must be enabled.

* This program refactors the processing performed by lssh(https://github.com/blacknon/lssh) so that it can be treated as a library.

## Usage

[See GoDoc reference.](https://godoc.org/github.com/blacknon/go-sshlib)

## Download

    GO111MODULE=on go get github.com/blacknon/go-sshlib

## Example

    // Copyright (c) 2022 Blacknon. All rights reserved.
    // Use of this source code is governed by an MIT license
    // that can be found in the LICENSE file.

    // Shell connection Example file.
    // Change the value of the variable and compile to make sure that you can actually connect.
    //
    // This file uses password authentication. Please replace as appropriate.

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

        // Create Session
        session, err := con.CreateSession()
        if err != nil {
            fmt.Println(err)
            os.Exit(1)
        }

        // Start ssh shell
        con.Shell(session)
    }
