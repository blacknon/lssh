// Copyright (c) 2021 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/template"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type WriteInventory struct {
	Address     string
	RemoteAddr  string
	Fingerprint string
}

type OverwriteInventory struct {
	Address     string
	RemoteAddr  string
	Fingerprint string
	OldKeyText  string
}

// verifyAndAppendNew checks knownhosts from the files stored in c.KnownHostsFiles.
// If there is a problem with the known hosts check, it returns an error and the check content.
// If is no problem, error returns Nil.
//
// 【参考】: https://github.com/tatsushid/minssh/blob/57eae8c5bcf5d94639891f3267f05251f05face4/pkg/minssh/minssh.go#L190-L237
func (c *Connect) verifyAndAppendNew(hostname string, remote net.Addr, key ssh.PublicKey) (err error) {
	// set TextAskWriteKnownHosts default text
	if len(c.TextAskWriteKnownHosts) == 0 {
		c.TextAskWriteKnownHosts += "The authenticity of host '{{.Address}} ({{.RemoteAddr}})' can't be established.\n"
		c.TextAskWriteKnownHosts += "RSA key fingerprint is {{.Fingerprint}}\n"
		c.TextAskWriteKnownHosts += "Are you sure you want to continue connecting (yes/no)?"
	}

	// set TextAskOverwriteKnownHosts default text
	if len(c.TextAskOverwriteKnownHosts) == 0 {
		c.TextAskOverwriteKnownHosts += "The authenticity of host '{{.Address}} ({{.RemoteAddr}})' can't be established.\n"
		c.TextAskOverwriteKnownHosts += "Old key: {{.OldKeyText}}\n"
		c.TextAskOverwriteKnownHosts += "Are you sure you want to overwrite {{.Fingerprint}}, continue connecting (yes/no)?"
	}

	// check count KnownHostsFiles
	if len(c.KnownHostsFiles) == 0 {
		return fmt.Errorf("there is no knownhosts file")
	}
	knownHostsFiles := c.KnownHostsFiles

	// abspath
	for i, file := range knownHostsFiles {
		file = getAbsPath(file)
		knownHostsFiles[i] = file
	}

	// get hostKeyCallback
	hostKeyCallback, err := knownhosts.New(knownHostsFiles...)
	if err != nil {
		return
	}

	// check hostkey
	err = hostKeyCallback(hostname, remote, key)
	if err == nil {
		return nil
	}

	// var
	filepath := knownHostsFiles[0]
	var line int

	// check error
	keyErr, ok := err.(*knownhosts.KeyError)
	if !ok || len(keyErr.Want) > 0 {
		for _, w := range keyErr.Want {
			oldkey := w.String()
			line = w.Line

			if answer, err := askOverwriteKnownHostKey(c.TextAskOverwriteKnownHosts, hostname, remote, key, oldkey); err != nil || !answer {
				msg := "host key verification failed"
				if err != nil {
					msg += ": " + err.Error()
				}
				return fmt.Errorf(msg)
			}
		}
	} else {
		if answer, err := askAddingUnknownHostKey(c.TextAskWriteKnownHosts, hostname, remote, key); err != nil || !answer {
			msg := "host key verification failed"
			if err != nil {
				msg += ": " + err.Error()
			}
			return fmt.Errorf(msg)
		}
		line = 0
	}

	err = writeKnownHostsKey(filepath, line, hostname, remote, key)

	return nil
}

// askAddingUnknownHostKey
// 【参考】: https://github.com/tatsushid/minssh/blob/57eae8c5bcf5d94639891f3267f05251f05face4/pkg/minssh/minssh.go#L93-L128
func askAddingUnknownHostKey(text string, address string, remote net.Addr, key ssh.PublicKey) (bool, error) {
	// set template variable
	sweaters := WriteInventory{address, remote.String(), ssh.FingerprintSHA256(key)}

	// set template
	tmpl, err := template.New("test").Parse(text)
	if err != nil {
		return false, err
	}

	//
	stopC := make(chan struct{})
	defer func() {
		close(stopC)
	}()

	go func() {
		sigC := make(chan os.Signal, 1)
		signal.Notify(sigC, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		select {
		case <-sigC:
			os.Exit(1)
		case <-stopC:
		}
	}()

	err = tmpl.Execute(os.Stdout, sweaters)
	if err != nil {
		return false, err
	}

	b := bufio.NewReader(os.Stdin)
	for {
		answer, err := b.ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("failed to read answer: %s", err)
		}
		answer = string(strings.ToLower(strings.TrimSpace(answer)))
		if answer == "yes" {
			return true, nil
		} else if answer == "no" {
			return false, nil
		}
		fmt.Print("Please type 'yes' or 'no': ")
	}
}

// askOverwriteKnownHostKey
// 【参考】: https://github.com/tatsushid/minssh/blob/57eae8c5bcf5d94639891f3267f05251f05face4/pkg/minssh/minssh.go#L93-L128
func askOverwriteKnownHostKey(text string, address string, remote net.Addr, key ssh.PublicKey, oldkey string) (bool, error) {
	// set template variable
	sweaters := OverwriteInventory{address, remote.String(), ssh.FingerprintSHA256(key), oldkey}

	// set template
	tmpl, err := template.New("test").Parse(text)
	if err != nil {
		return false, err
	}

	//
	stopC := make(chan struct{})
	defer func() {
		close(stopC)
	}()

	go func() {
		sigC := make(chan os.Signal, 1)
		signal.Notify(sigC, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		select {
		case <-sigC:
			os.Exit(1)
		case <-stopC:
		}
	}()

	err = tmpl.Execute(os.Stdout, sweaters)
	if err != nil {
		return false, err
	}

	b := bufio.NewReader(os.Stdin)
	for {
		answer, err := b.ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("failed to read answer: %s", err)
		}
		answer = string(strings.ToLower(strings.TrimSpace(answer)))
		if answer == "yes" {
			return true, nil
		} else if answer == "no" {
			return false, nil
		}
		fmt.Print("Please type 'yes' or 'no': ")
	}
}

func writeKnownHostsKey(filepath string, linenum int, hostname string, remote net.Addr, key ssh.PublicKey) (err error) {
	//
	var addrs []string
	if remote.String() == hostname {
		addrs = []string{hostname}
	} else {
		addrs = []string{hostname, remote.String()}
	}

	// set string
	entry := knownhosts.Line(addrs, key)
	if linenum == 0 {
		// open file
		f, err := os.OpenFile(filepath, os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return fmt.Errorf("failed to add new host key: %s", err)
		}
		defer f.Close()

		if _, err = f.WriteString(entry + "\n"); err != nil {
			return fmt.Errorf("failed to add new host key: %s", err)
		}
	} else {
		// open file
		fr, err := os.Open(filepath)
		if err != nil {
			return fmt.Errorf("failed to add new host key: %s", err)
		}
		defer fr.Close()

		fw, err := os.OpenFile(filepath, os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("failed to add new host key: %s", err)
		}
		defer fw.Close()

		scanner := bufio.NewScanner(fr)
		writer := bufio.NewWriter(fw)
		var line string

		count := 1
		for scanner.Scan() {
			line = scanner.Text()

			if count == linenum {
				line = entry
			}

			writer.WriteString(line + "\n")
			count += 1
		}

		err = writer.Flush()
		if err != nil {
			fmt.Println(err)
		}
	}

	return
}
