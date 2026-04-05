go-sshproc
===

[![GoDoc](https://godoc.org/github.com/blacknon/go-sshproc?status.svg)](https://godoc.org/github.com/blacknon/go-sshproc)

## About

This library is adds the functionality to get performance information from procfs to [go-sshlib](https://github.com/blacknon/go-sshlib).
Use an sftp client to obtain performance information by retrieving information such as `/proc`.
For this reason, the ssh connection destination must be a UNIX-like OS such as Linux.

Some of the references and code have been changed in order to have almost the same usability as [goprocinfo](https://github.com/c9s/goprocinfo).

## Usage

[See GoDoc reference.](https://godoc.org/github.com/blacknon/go-sshproc)

## Example

    // Copyright(c) 2024 Blacknon. All rights reserved.
    // Use of this source code is governed by an MIT license
    // that can be found in the LICENSE file.

    package main

    import (
        "fmt"
        "os"

        "github.com/blacknon/go-sshlib"
        "github.com/blacknon/go-sshproc"

        "golang.org/x/crypto/ssh"
    )

    var (
        host     = "server1.example.com"
        port     = "22"
        user     = "user"
        key      = "~/.ssh/id_rsa"
        password = ""
    )

    func main() {
        // Create sshlib.Connect
        con := &sshlib.Connect{}

        // Create ssh.AuthMethod
        authMethod, _ := sshlib.CreateAuthMethodPublicKey(key, password)

        // Connect ssh server
        con.HostKeyCallback = ssh.InsecureIgnoreHostKey()

        err := con.CreateClient(host, port, user, []ssh.AuthMethod{authMethod})
        if err != nil {
            fmt.Println(err)
            os.Exit(1)
        }

     // Create sshproc
        proc := &sshproc.ConnectWithProc{Connect: con}
        err = proc.CreateSftpClient()
        if err != nil {
            fmt.Println(err)
            os.Exit(1)
        }
        defer proc.CloseSftpClient()

        fmt.Println("")
        fmt.Println("Read CPUInfo")
        cpuinfo, err := proc.ReadCPUInfo("/proc/cpuinfo")
        if err != nil {
            fmt.Println(err)
            os.Exit(1)
        }
        fmt.Println(cpuinfo.NumCPU())
        fmt.Println(cpuinfo.NumCore())
        fmt.Println(cpuinfo.NumPhysicalCPU())
    }
