// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/blacknon/lssh/internal/common"
	"golang.org/x/crypto/ssh/terminal"
)

const defaultOpenSSHConfigPath = "~/.ssh/config"

var (
	generateConfigFromOpenSSHFn = GenerateLSSHConfigFromOpenSSH
	isInteractivePromptFn       = isInteractivePrompt
	promptYesNoFn               = promptYesNo
	readConfigFn                = Read
)

// HandleGenerateConfigMode writes a generated lssh config to stdout and reports
// whether the caller should exit without continuing normal command execution.
func HandleGenerateConfigMode(path string, out io.Writer) (bool, error) {
	if path == "" {
		return false, nil
	}

	data, err := generateConfigFromOpenSSHFn(path, "")
	if err != nil {
		return true, err
	}

	_, err = out.Write(data)
	return true, err
}

// ReadWithFallback loads the lssh config, falls back to OpenSSH import mode
// when the file does not exist, and optionally offers to create the missing
// file in interactive sessions.
func ReadWithFallback(confPath string, stderr io.Writer) (Config, error) {
	confExists := common.IsExist(confPath)
	data, err := readConfigFn(confPath)
	if err != nil {
		return Config{}, err
	}
	if confExists {
		return data, nil
	}

	fmt.Fprintf(stderr, "Information   : config file %s was not found.\n", confPath)
	fmt.Fprintf(stderr, "Information   : falling back to OpenSSH config import mode (%s).\n", defaultOpenSSHConfigPath)

	created, err := maybeCreateConfigFromOpenSSH(confPath, stderr)
	if err != nil {
		return Config{}, err
	}
	if created {
		data, err = readConfigFn(confPath)
		if err != nil {
			return Config{}, err
		}
	}

	return data, nil
}

func maybeCreateConfigFromOpenSSH(confPath string, stderr io.Writer) (bool, error) {
	defaultSSHConfig := common.GetFullPath(defaultOpenSSHConfigPath)
	if !common.IsExist(defaultSSHConfig) {
		return false, nil
	}

	if !isInteractivePromptFn() {
		fmt.Fprintf(stderr, "Information   : run `lssh --generate-lssh-conf > %s` if you want to create it.\n", confPath)
		return false, nil
	}

	answer, err := promptYesNoFn(
		stderr,
		fmt.Sprintf("Create %s from %s now? (Y/n): ", confPath, defaultSSHConfig),
		true,
	)
	if err != nil {
		return false, err
	}
	if !answer {
		return false, nil
	}

	data, err := generateConfigFromOpenSSHFn(defaultSSHConfig, "")
	if err != nil {
		return false, err
	}

	fullConfPath := common.GetFullPath(confPath)
	if err := os.MkdirAll(filepath.Dir(fullConfPath), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(fullConfPath, data, 0o600); err != nil {
		return false, err
	}

	fmt.Fprintf(stderr, "Information   : created %s from %s.\n", fullConfPath, defaultSSHConfig)
	return true, nil
}

func isInteractivePrompt() bool {
	stdin := int(os.Stdin.Fd())
	stdout := int(os.Stdout.Fd())
	return terminal.IsTerminal(stdin) && terminal.IsTerminal(stdout)
}

func promptYesNo(stderr io.Writer, message string, defaultYes bool) (bool, error) {
	fmt.Fprint(stderr, message)

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}

	line = regexp.MustCompile(`\s+`).ReplaceAllString(line, "")
	if line == "" {
		return defaultYes, nil
	}

	switch line {
	case "y", "Y", "yes", "Yes", "YES":
		return true, nil
	case "n", "N", "no", "No", "NO":
		return false, nil
	default:
		return defaultYes, nil
	}
}
