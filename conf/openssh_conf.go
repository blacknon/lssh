// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

import (
	"os"
	"regexp"
	"strings"

	"github.com/blacknon/lssh/common"
	"github.com/kevinburke/ssh_config"
)

// openOpenSshConfig open the OpenSsh configuration file, return *ssh_config.Config.
func openOpenSshConfig(path string) (cfg *ssh_config.Config, err error) {
	// Read Openssh Config
	sshConfigFile := common.GetFullPath(path)
	f, err := os.Open(sshConfigFile)
	if err != nil {
		return
	}

	cfg, err = ssh_config.Decode(f)
	return
}

// getOpenSshConfig loads the specified OpenSsh configuration file and returns it in conf.ServerConfig format
func getOpenSshConfig(path string) (config map[string]ServerConfig, err error) {
	config = map[string]ServerConfig{}

	// open openssh config
	cfg, err := openOpenSshConfig(path)
	if err != nil {
		return
	}

	// Get Node names
	hostList := []string{}
	for _, h := range cfg.Hosts {
		// not supported wildcard host
		re := regexp.MustCompile("\\*")
		for _, pattern := range h.Patterns {
			if !re.MatchString(pattern.String()) {
				hostList = append(hostList, pattern.String())
			}
		}
	}

	// append ServerConfig
	// TODO(blacknon): port forwardingとx11の設定も読み込むよう処理を追加！！
	for _, host := range hostList {
		serverConfig := ServerConfig{
			Addr:         ssh_config.Get(host, "HostName"),
			Port:         ssh_config.Get(host, "Port"),
			User:         ssh_config.Get(host, "User"),
			ProxyCommand: ssh_config.Get(host, "ProxyCommand"),
			PreCmd:       ssh_config.Get(host, "LocalCommand"),
			Note:         "from :" + path,
		}

		// TODO(blacknon): OpenSshの設定ファイルだと、Certificateは複数指定可能な模様。ただ、あまり一般的な使い方ではないようなので、現状は複数のファイルを受け付けるように作っていない。
		key := ssh_config.Get(host, "IdentityFile")
		cert := ssh_config.Get(host, "Certificate")
		if cert != "" {
			serverConfig.Cert = cert
			serverConfig.CertKey = key
		} else {
			serverConfig.Key = key
		}

		// PKCS11 provider
		pkcs11Provider := ssh_config.Get(host, "PKCS11Provider")
		if pkcs11Provider != "" {
			serverConfig.PKCS11Use = true
			serverConfig.PKCS11Provider = pkcs11Provider
		}

		// x11 forwarding
		x11 := ssh_config.Get(host, "ForwardX11")
		if x11 == "yes" {
			serverConfig.X11 = true
		}

		// Port forwarding (Local forward)
		localForward := ssh_config.Get(host, "LocalForward")
		if localForward != "" {
			array := strings.SplitN(localForward, " ", 2)
			if len(array) > 1 {
				serverConfig.PortForwardLocal = "localhost:" + array[0]
				serverConfig.PortForwardRemote = array[1]
			}
		}

		// Port forwarding (Remote forward)
		remoteForward := ssh_config.Get(host, "RemoteForward")
		if remoteForward != "" {
			array := strings.SplitN(remoteForward, " ", 2)
			if len(array) > 1 {
				serverConfig.PortForwardLocal = array[1]
				serverConfig.PortForwardRemote = "localhost:" + array[0]
			}
		}

		// Port forwarding (Dynamic forward)
		dynamicForward := ssh_config.Get(host, "DynamicForward")
		if dynamicForward != "" {
			serverConfig.DynamicPortForward = dynamicForward
		}

		serverName := path + ":" + host
		config[serverName] = serverConfig
	}

	return config, err
}
