// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package conf

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/blacknon/lssh/internal/common"
	"github.com/kevinburke/ssh_config"
)

type openSSHConfigEntry struct {
	Host   string
	Config ServerConfig
}

// readOpenSSHConfig open the OpenSSH configuration file, return *ssh_config.Config.
func readOpenSSHConfig(path, command string) (cfg *ssh_config.Config, err error) {
	var rd io.Reader
	switch {
	case path != "":
		sshConfigFile := common.GetFullPath(path)
		rd, err = os.Open(sshConfigFile)
	case command != "":
		var data []byte
		cmd := exec.Command("sh", "-c", command)
		data, err = cmd.Output()
		rd = bytes.NewReader(data)
	}

	if err != nil {
		return
	}

	cfg, err = ssh_config.Decode(rd)
	return
}

// getOpenSSHConfig loads the specified OpenSSH configuration file and returns it in conf.ServerConfig format.
func getOpenSSHConfig(path, command string) (config map[string]ServerConfig, err error) {
	config = map[string]ServerConfig{}

	entries, err := loadOpenSSHConfigEntries(path, command)
	if err != nil {
		return config, err
	}

	ele := path
	if ele == "" {
		ele = "generate_sshconfig"
	}

	for _, entry := range entries {
		serverConfig := entry.Config
		serverConfig.Note = "from:" + ele
		serverName := ele + ":" + entry.Host
		config[serverName] = serverConfig
	}

	return config, err
}

func (c *Config) readConfiguredOpenSSHConfig(sc OpenSSHConfig) error {
	entries, err := loadOpenSSHConfigEntries(sc.Path, sc.Command)
	if err != nil {
		return err
	}

	matches, err := decodeOpenSSHImportMatches(sc.Match)
	if err != nil {
		return err
	}

	ele := sc.Path
	if ele == "" {
		ele = "generate_sshconfig"
	}

	base := serverConfigReduct(c.Common, sc.ServerConfig)
	for _, entry := range entries {
		value := entry.Config
		value.Note = "from:" + ele
		value = serverConfigReduct(base, value)
		value = applyOpenSSHImportMatches(entry.Host, value, matches)
		c.Server[ele+":"+entry.Host] = value
	}

	return nil
}

type openSSHImportMatch struct {
	Name      string
	Priority  int
	NameIn    []string
	NameNotIn []string
	UserIn    []string
	UserNotIn []string
	AddrIn    []string
	AddrNotIn []string
	PortIn    []string
	PortNotIn []string
	Config    ServerConfig
	order     int
}

func decodeOpenSSHImportMatches(raw map[string]map[string]interface{}) ([]openSSHImportMatch, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]openSSHImportMatch, 0, len(names))
	for idx, name := range names {
		branchMap := raw[name]

		var cfg ServerConfig
		if err := decodeTaggedMap(branchMap, &cfg, "toml"); err != nil {
			return nil, fmt.Errorf("match.%s: %w", name, err)
		}

		priority := 100
		if rawPriority, ok := branchMap["priority"]; ok {
			if p, ok := asInt64(rawPriority); ok {
				priority = int(p)
			} else {
				return nil, fmt.Errorf("match.%s.priority must be an integer", name)
			}
		}

		result = append(result, openSSHImportMatch{
			Name:      name,
			Priority:  priority,
			NameIn:    providerStringSlice(branchMap, "name_in"),
			NameNotIn: providerStringSlice(branchMap, "name_not_in"),
			UserIn:    providerStringSlice(branchMap, "user_in"),
			UserNotIn: providerStringSlice(branchMap, "user_not_in"),
			AddrIn:    providerStringSlice(branchMap, "addr_in"),
			AddrNotIn: providerStringSlice(branchMap, "addr_not_in"),
			PortIn:    providerStringSlice(branchMap, "port_in"),
			PortNotIn: providerStringSlice(branchMap, "port_not_in"),
			Config:    cfg,
			order:     idx + 1,
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].order < result[j].order
	})

	return result, nil
}

func applyOpenSSHImportMatches(host string, base ServerConfig, matches []openSSHImportMatch) ServerConfig {
	current := base
	for _, match := range matches {
		if openSSHImportMatchApplies(host, current, match) {
			current = serverConfigReduct(current, match.Config)
		}
	}
	return current
}

func openSSHImportMatchApplies(host string, current ServerConfig, match openSSHImportMatch) bool {
	if !openSSHMatchPatternList(host, match.NameIn, false) {
		return false
	}
	if !openSSHMatchPatternList(host, match.NameNotIn, true) {
		return false
	}
	if !openSSHMatchPatternList(current.User, match.UserIn, false) {
		return false
	}
	if !openSSHMatchPatternList(current.User, match.UserNotIn, true) {
		return false
	}
	if !openSSHMatchPatternList(current.Addr, match.AddrIn, false) {
		return false
	}
	if !openSSHMatchPatternList(current.Addr, match.AddrNotIn, true) {
		return false
	}
	if !openSSHMatchPatternList(current.Port, match.PortIn, false) {
		return false
	}
	if !openSSHMatchPatternList(current.Port, match.PortNotIn, true) {
		return false
	}
	return true
}

func openSSHMatchPatternList(value string, patterns []string, negative bool) bool {
	if len(patterns) == 0 {
		return true
	}

	matched := false
	for _, pattern := range patterns {
		ok, err := path.Match(pattern, value)
		if err == nil && ok {
			matched = true
			break
		}
	}

	if negative {
		return !matched
	}
	return matched
}

func loadOpenSSHConfigEntries(path, command string) ([]openSSHConfigEntry, error) {
	cfg, err := readOpenSSHConfig(path, command)
	if err != nil {
		return nil, err
	}

	hostList := []string{}
	for _, h := range cfg.Hosts {
		if isOpenSSHMatchBlock(h) {
			continue
		}

		re := regexp.MustCompile(`[\*\?]`)
		for _, pattern := range h.Patterns {
			patternString := pattern.String()
			if strings.HasPrefix(patternString, "!") || re.MatchString(patternString) {
				continue
			}
			hostList = append(hostList, patternString)
		}
	}

	hostList = removeDupString(hostList)

	entries := make([]openSSHConfigEntry, 0, len(hostList))
	for _, host := range hostList {
		serverConfig := ServerConfig{
			Addr:         getOpenSSHValue(cfg, host, "HostName"),
			Port:         getOpenSSHValue(cfg, host, "Port"),
			User:         getOpenSSHValue(cfg, host, "User"),
			ProxyCommand: getOpenSSHValue(cfg, host, "ProxyCommand"),
			PreCmd:       getOpenSSHValue(cfg, host, "LocalCommand"),
		}

		if serverConfig.Addr == "" {
			serverConfig.Addr = host
		}

		if serverConfig.User == "" {
			serverConfig.User = currentUsername()
		}

		keys := getOpenSSHIdentityFiles(cfg, host)
		certs := getOpenSSHValues(cfg, host, "CertificateFile")
		if len(certs) > 0 {
			serverConfig.Cert = certs[0]
			if len(keys) > 0 {
				serverConfig.CertKey = keys[0]
			}
			for i, cert := range certs {
				key := ""
				switch {
				case len(keys) > i:
					key = keys[i]
				case len(keys) > 0:
					key = keys[0]
				}
				if key == "" {
					continue
				}
				if i == 0 {
					continue
				}
				serverConfig.Certs = append(serverConfig.Certs, cert+"::"+key)
			}
		} else if len(keys) > 0 {
			serverConfig.Key = keys[0]
		}
		if len(keys) > 1 {
			serverConfig.Keys = append(serverConfig.Keys, keys[1:]...)
		}

		pkcs11Provider := getOpenSSHValue(cfg, host, "PKCS11Provider")
		if pkcs11Provider != "" {
			serverConfig.PKCS11Use = true
			serverConfig.PKCS11Provider = pkcs11Provider
		}

		x11 := getOpenSSHValue(cfg, host, "ForwardX11")
		if x11 == "yes" {
			serverConfig.X11 = true
		}

		cm := getOpenSSHValue(cfg, host, "ControlMaster")
		if cm != "" && cm != "no" {
			serverConfig.ControlMaster = true
		}

		cp := getOpenSSHValue(cfg, host, "ControlPath")
		if cp != "" {
			serverConfig.ControlPath = cp
		}

		cper := getOpenSSHValue(cfg, host, "ControlPersist")
		if cper != "" {
			if cperValue, err := parseControlPersist(cper); err == nil {
				serverConfig.ControlPersist = cperValue
			}
		}

		localForward := getOpenSSHValue(cfg, host, "LocalForward")
		if localForward != "" {
			array := strings.SplitN(localForward, " ", 2)
			if len(array) > 1 {
				var e error

				_, e = strconv.Atoi(array[0])
				if e != nil {
					serverConfig.PortForwardLocal = array[0]
				} else {
					serverConfig.PortForwardLocal = "localhost:" + array[0]
				}

				_, e = strconv.Atoi(array[1])
				if e != nil {
					serverConfig.PortForwardRemote = array[1]
				} else {
					serverConfig.PortForwardRemote = "localhost:" + array[1]
				}
			}
		}

		remoteForward := getOpenSSHValue(cfg, host, "RemoteForward")
		if remoteForward != "" {
			array := strings.SplitN(remoteForward, " ", 2)
			if len(array) > 1 {
				var e error

				_, e = strconv.Atoi(array[0])
				if e != nil {
					serverConfig.PortForwardLocal = array[0]
				} else {
					serverConfig.PortForwardLocal = "localhost:" + array[0]
				}

				_, e = strconv.Atoi(array[1])
				if e != nil {
					serverConfig.PortForwardRemote = array[1]
				} else {
					serverConfig.PortForwardRemote = "localhost:" + array[1]
				}
			}
		}

		dynamicForward := getOpenSSHValue(cfg, host, "DynamicForward")
		if dynamicForward != "" {
			serverConfig.DynamicPortForward = dynamicForward
		}

		entries = append(entries, openSSHConfigEntry{
			Host:   host,
			Config: serverConfig,
		})
	}

	return entries, nil
}

func currentUsername() string {
	if value := strings.TrimSpace(os.Getenv("USER")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("USERNAME")); value != "" {
		return value
	}
	if u, err := user.Current(); err == nil {
		if value := strings.TrimSpace(u.Username); value != "" {
			return value
		}
	}
	return ""
}

func normalizeOpenSSHIdentityFile(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	fullPath := expandOpenSSHPath(path)
	if fullPath == "" {
		return path
	}

	if _, err := os.Stat(fullPath); err == nil {
		return path
	}

	switch filepath.Base(fullPath) {
	case "identity", "id_rsa", "id_dsa", "id_ecdsa", "id_ecdsa_sk", "id_ed25519", "id_ed25519_sk", "id_xmss":
		return ""
	default:
		return path
	}
}

func expandOpenSSHPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, "~") {
		path = strings.Replace(path, "~", home, 1)
	}

	fullPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return fullPath
}

func isOpenSSHMatchBlock(h *ssh_config.Host) bool {
	if h == nil {
		return false
	}

	line, _, _ := strings.Cut(h.String(), "\n")
	line = strings.TrimSpace(line)
	return strings.HasPrefix(strings.ToLower(line), "match ")
}

func getOpenSSHValue(cfg *ssh_config.Config, host, key string) string {
	value, err := cfg.Get(host, key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func getOpenSSHValues(cfg *ssh_config.Config, host, key string) []string {
	values, err := cfg.GetAll(host, key)
	if err != nil {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		result = append(result, value)
	}

	return result
}

func getOpenSSHIdentityFiles(cfg *ssh_config.Config, host string) []string {
	values := getOpenSSHValues(cfg, host, "IdentityFile")
	if len(values) == 0 {
		return nil
	}

	keys := make([]string, 0, len(values))
	for _, value := range values {
		key := normalizeOpenSSHIdentityFile(value)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}

	return keys
}

func removeDupString(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}
