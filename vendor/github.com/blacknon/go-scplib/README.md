go-scplib
====

[![GoDoc](https://godoc.org/github.com/blacknon/go-scplib?status.svg)](https://godoc.org/github.com/blacknon/go-scplib)
[![TravisCI](https://travis-ci.org/blacknon/go-scplib.svg?branch=master)](https://travis-ci.org/blacknon/go-scplib)
[![Go Report Card](https://goreportcard.com/badge/github.com/blacknon/go-scplib)](https://goreportcard.com/report/github.com/blacknon/go-scplib)

scp library for golang

## Usage

[See GoDoc reference.](https://godoc.org/github.com/blacknon/go-scplib)

## Projects using go-scplib

* [blacknon/lssh : List selection type alternative ssh/scp client.Pure Go.](https://github.com/blacknon/lssh)

## Example

    package main

    import (
        "fmt"
        "io/ioutil"
        "os"
        "time"

        "github.com/blacknon/go-scplib"
        "golang.org/x/crypto/ssh"
    )

    func main() {
        // Read Private key
        key, err := ioutil.ReadFile(os.Getenv("HOME") + "/.ssh/id_rsa")
        if err != nil {
            fmt.Fprintf(os.Stderr, "Unable to read private key: %v\n", err)
            os.Exit(1)
        }

        // Parse Private key
        signer, err := ssh.ParsePrivateKey(key)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Unable to parse private key: %v\n", err)
            os.Exit(1)
        }

        // Create ssh client config
        config := &ssh.ClientConfig{
            User: "user",
            Auth: []ssh.AuthMethod{
                ssh.PublicKeys(signer),
            },
            HostKeyCallback: ssh.InsecureIgnoreHostKey(),
            Timeout:         60 * time.Second,
        }

        // Create ssh connection
        connection, err := ssh.Dial("tcp", "test-node:22", config)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Failed to dial: %s\n", err)
            os.Exit(1)
        }
        defer connection.Close()

        // Create scp client
        scp := new(scplib.SCPClient)
        scp.Permission = false // copy permission with scp flag
        scp.Connection = connection

        // scp get file
        // scp.GetFile([]strint{"/From/Remote/Path1","/From/Remote/Path2"],"/To/Local/Path")
        err = scp.GetFile([]string{"/etc/passwd"}, "./passwd")
        if err != nil {
            fmt.Fprintf(os.Stderr, "Failed to scp get: %s\n", err)
            os.Exit(1)
        }

        // // Dir pattern (Snip)
        // err := scp.GetFile("/path/from/remote/dir", "./path/to/local/dir")
        // if err != nil {
        //  fmt.Fprintf(os.Stderr, "Failed to create session: %s\n", err)
        //  os.Exit(1)
        // }

        // scp put file
        // scp.PutFile("/From/Local/Path","/To/Remote/Path")
        err = scp.PutFile([]string{"./passwd"}, "./passwd_scp")
        if err != nil {
            fmt.Fprintf(os.Stderr, "Failed to scp put: %s\n", err)
            os.Exit(1)
        }

        // scp get file (to scp format data)
        // scp.GetData("/path/remote/path")
        getData, err := scp.GetData([]string{"/etc/passwd"})
        if err != nil {
            fmt.Fprintf(os.Stderr, "Failed to scp put: %s\n", err)
            os.Exit(1)
        }

        fmt.Println(getData)

        // scp put file (to scp format data)
        // scp.GetData(Data,"/path/remote/path")
        err = scp.PutData(getData, "./passwd_data")
        if err != nil {
            fmt.Fprintf(os.Stderr, "Failed to scp put: %s\n", err)
            os.Exit(1)
        }
    }
