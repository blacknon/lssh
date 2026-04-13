package conf

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

type matchContext struct {
	LocalIPs    []netip.Addr
	Gateways    []netip.Addr
	Username    string
	Hostname    string
	OS          string
	Terms       []string
	EnvNames    []string
	EnvValues   map[string]string
	LocalIPErr  error
	GatewayErr  error
	UsernameErr error
	HostnameErr error
}

type matchRequirements struct {
	needLocalIP  bool
	needGateway  bool
	needUsername bool
	needHostname bool
	needOS       bool
	needTerm     bool
	needEnv      bool
}

type namedMatch struct {
	name   string
	config ServerMatchConfig
}

type namedOpenSSHConfig struct {
	name   string
	config OpenSSHConfig
}

var detectMatchContext = buildMatchContext

func decodeConfigFile(path string, c *Config) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return decodeYAMLConfigFile(path, c)
	}

	md, err := toml.DecodeFile(path, c)
	if err != nil {
		return err
	}

	applyMatchMetadata(c, md)
	return nil
}

func decodeYAMLConfigFile(path string, c *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return err
	}

	return applyYAMLMatchMetadata(data, c)
}

func applyMatchMetadata(c *Config, md toml.MetaData) {
	order := 0
	for _, key := range md.Keys() {
		if len(key) < 4 || key[0] != "server" || key[2] != "match" {
			continue
		}

		serverName := key[1]
		branchName := key[3]

		serverConf, ok := c.Server[serverName]
		if !ok || serverConf.Match == nil {
			continue
		}

		matchConf, ok := serverConf.Match[branchName]
		if !ok {
			continue
		}

		if matchConf.order == 0 {
			order++
			matchConf.order = order
		}
		if md.IsDefined("server", serverName, "match", branchName, "priority") {
			matchConf.priorityDefined = true
		}
		matchConf.definedKeys = collectDefinedMatchKeys(md, serverName, branchName)

		serverConf.Match[branchName] = matchConf
		c.Server[serverName] = serverConf
	}
}

func collectDefinedMatchKeys(md toml.MetaData, serverName, branchName string) map[string]bool {
	keys := []string{
		"addr", "port", "user", "pass", "pass_ref", "passes", "key", "key_ref", "keycmd", "keycmdpass", "keycmdpass_ref", "keypass", "keypass_ref",
		"keys", "cert", "cert_ref", "certs", "certkey", "certkey_ref", "certkeypass", "certkeypass_ref", "certpkcs11", "agentauth",
		"ssh_agent", "ssh_agent_key", "pkcs11", "pkcs11provider", "pkcs11pin", "pkcs11pin_ref", "pre_cmd",
		"post_cmd", "proxy_type", "proxy", "proxy_cmd", "local_rc", "local_rc_file",
		"local_rc_compress", "local_rc_decode_cmd", "local_rc_uncompress_cmd", "port_forward",
		"port_forward_local", "port_forward_remote", "port_forwards", "dynamic_port_forward",
		"reverse_dynamic_port_forward", "http_dynamic_port_forward",
		"http_reverse_dynamic_port_forward", "nfs_dynamic_forward", "nfs_dynamic_forward_path",
		"nfs_reverse_dynamic_forward", "nfs_reverse_dynamic_forward_path",
		"smb_dynamic_forward", "smb_dynamic_forward_path",
		"smb_reverse_dynamic_forward", "smb_reverse_dynamic_forward_path",
		"x11", "x11_trusted",
		"connect_timeout", "alive_max", "alive_interval", "check_known_hosts",
		"known_hosts_files", "control_master", "control_path", "control_persist", "note", "ignore",
	}

	defined := make(map[string]bool, len(keys))
	for _, key := range keys {
		if md.IsDefined("server", serverName, "match", branchName, key) {
			defined[key] = true
		}
	}

	return defined
}

func applyYAMLMatchMetadata(data []byte, c *Config) error {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return err
	}

	if len(node.Content) == 0 {
		return nil
	}

	root := node.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}

	serversNode := yamlMapValue(root, "server")
	if serversNode == nil || serversNode.Kind != yaml.MappingNode {
		return nil
	}

	order := 0
	for i := 0; i+1 < len(serversNode.Content); i += 2 {
		serverName := serversNode.Content[i].Value
		serverNode := serversNode.Content[i+1]
		if serverNode.Kind != yaml.MappingNode {
			continue
		}

		matchesNode := yamlMapValue(serverNode, "match")
		if matchesNode == nil || matchesNode.Kind != yaml.MappingNode {
			continue
		}

		serverConf, ok := c.Server[serverName]
		if !ok || serverConf.Match == nil {
			continue
		}

		for j := 0; j+1 < len(matchesNode.Content); j += 2 {
			branchName := matchesNode.Content[j].Value
			branchNode := matchesNode.Content[j+1]

			matchConf, ok := serverConf.Match[branchName]
			if !ok {
				continue
			}

			if matchConf.order == 0 {
				order++
				matchConf.order = order
			}
			matchConf.priorityDefined = yamlMapHasKey(branchNode, "priority")
			matchConf.definedKeys = collectDefinedYAMLMatchKeys(branchNode)

			serverConf.Match[branchName] = matchConf
		}

		c.Server[serverName] = serverConf
	}

	return nil
}

func collectDefinedYAMLMatchKeys(branchNode *yaml.Node) map[string]bool {
	keys := []string{
		"addr", "port", "user", "pass", "pass_ref", "passes", "key", "key_ref", "keycmd", "keycmdpass", "keycmdpass_ref", "keypass", "keypass_ref",
		"keys", "cert", "cert_ref", "certs", "certkey", "certkey_ref", "certkeypass", "certkeypass_ref", "certpkcs11", "agentauth",
		"ssh_agent", "ssh_agent_key", "pkcs11", "pkcs11provider", "pkcs11pin", "pkcs11pin_ref", "pre_cmd",
		"post_cmd", "proxy_type", "proxy", "proxy_cmd", "local_rc", "local_rc_file",
		"local_rc_compress", "local_rc_decode_cmd", "local_rc_uncompress_cmd", "port_forward",
		"port_forward_local", "port_forward_remote", "port_forwards", "dynamic_port_forward",
		"reverse_dynamic_port_forward", "http_dynamic_port_forward",
		"http_reverse_dynamic_port_forward", "nfs_dynamic_forward", "nfs_dynamic_forward_path",
		"nfs_reverse_dynamic_forward", "nfs_reverse_dynamic_forward_path",
		"smb_dynamic_forward", "smb_dynamic_forward_path",
		"smb_reverse_dynamic_forward", "smb_reverse_dynamic_forward_path",
		"x11", "x11_trusted",
		"connect_timeout", "alive_max", "alive_interval", "check_known_hosts",
		"known_hosts_files", "control_master", "control_path", "control_persist", "note", "ignore",
	}

	defined := make(map[string]bool, len(keys))
	for _, key := range keys {
		if yamlMapHasKey(branchNode, key) {
			defined[key] = true
		}
	}

	return defined
}

func yamlMapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}

	return nil
}

func yamlMapHasKey(node *yaml.Node, key string) bool {
	return yamlMapValue(node, key) != nil
}

func (c *Config) ResolveConditionalMatches() error {
	reqs, err := c.validateMatchConfigs()
	if err != nil {
		return err
	}
	if !reqs.needLocalIP && !reqs.needGateway && !reqs.needUsername && !reqs.needHostname && !reqs.needOS && !reqs.needTerm && !reqs.needEnv {
		return nil
	}

	ctx := detectMatchContext(reqs)

	for serverName, serverConf := range c.Server {
		if len(serverConf.Match) == 0 {
			continue
		}

		matches := sortedMatches(serverConf.Match)
		merged := serverConf
		for _, branch := range matches {
			ok := branchMatches(serverName, branch.name, branch.config, ctx)
			if !ok {
				continue
			}

			merged = mergeServerMatchConfig(merged, branch.config)
		}
		merged.Match = serverConf.Match
		c.Server[serverName] = merged
	}

	return nil
}

func (c *Config) validateMatchConfigs() (matchRequirements, error) {
	reqs := matchRequirements{}

	for serverName, serverConf := range c.Server {
		for branchName, branch := range serverConf.Match {
			if branch.When.Empty() {
				return reqs, fmt.Errorf("server.%s.match.%s: at least one when.* condition is required", serverName, branchName)
			}

			if err := validateMatchNetworkList(branch.When.LocalIPIn, "local_ip_in", serverName, branchName); err != nil {
				return reqs, err
			}
			if err := validateMatchNetworkList(branch.When.LocalIPNotIn, "local_ip_not_in", serverName, branchName); err != nil {
				return reqs, err
			}
			if err := validateMatchNetworkList(branch.When.GatewayIn, "gateway_in", serverName, branchName); err != nil {
				return reqs, err
			}
			if err := validateMatchNetworkList(branch.When.GatewayNotIn, "gateway_not_in", serverName, branchName); err != nil {
				return reqs, err
			}

			reqs.needLocalIP = reqs.needLocalIP || len(branch.When.LocalIPIn) > 0 || len(branch.When.LocalIPNotIn) > 0
			reqs.needGateway = reqs.needGateway || len(branch.When.GatewayIn) > 0 || len(branch.When.GatewayNotIn) > 0
			reqs.needUsername = reqs.needUsername || len(branch.When.UsernameIn) > 0 || len(branch.When.UsernameNotIn) > 0
			reqs.needHostname = reqs.needHostname || len(branch.When.HostnameIn) > 0 || len(branch.When.HostnameNotIn) > 0
			reqs.needOS = reqs.needOS || len(branch.When.OSIn) > 0 || len(branch.When.OSNotIn) > 0
			reqs.needTerm = reqs.needTerm || len(branch.When.TermIn) > 0 || len(branch.When.TermNotIn) > 0
			reqs.needEnv = reqs.needEnv || len(branch.When.EnvIn) > 0 || len(branch.When.EnvNotIn) > 0 || len(branch.When.EnvValueIn) > 0 || len(branch.When.EnvValueNotIn) > 0
		}
	}

	return reqs, nil
}

func validateMatchNetworkList(values []string, key, serverName, branchName string) error {
	for _, value := range values {
		if _, _, err := parseIPorPrefix(value); err != nil {
			return fmt.Errorf("server.%s.match.%s.when.%s: %w", serverName, branchName, key, err)
		}
	}
	return nil
}

func sortedMatches(matches map[string]ServerMatchConfig) []namedMatch {
	result := make([]namedMatch, 0, len(matches))
	for name, matchConf := range matches {
		result = append(result, namedMatch{name: name, config: matchConf})
	}

	sort.SliceStable(result, func(i, j int) bool {
		left := result[i].config
		right := result[j].config
		if left.EffectivePriority() != right.EffectivePriority() {
			return left.EffectivePriority() < right.EffectivePriority()
		}
		return left.order < right.order
	})

	return result
}

func branchMatches(serverName, branchName string, branch ServerMatchConfig, ctx matchContext) bool {
	return whenMatches(branch.When, serverName, branchName, ctx)
}

func whenMatches(when ServerMatchWhen, serverName, branchName string, ctx matchContext) bool {

	if len(when.LocalIPIn) > 0 && !matchIPList(ctx.LocalIPs, when.LocalIPIn) {
		return false
	}
	if len(when.LocalIPNotIn) > 0 && matchIPList(ctx.LocalIPs, when.LocalIPNotIn) {
		return false
	}

	if len(when.GatewayIn) > 0 {
		if ctx.GatewayErr != nil {
			log.Printf("server.%s.match.%s: gateway lookup failed: %v", serverName, branchName, ctx.GatewayErr)
		}
		if !matchIPList(ctx.Gateways, when.GatewayIn) {
			return false
		}
	}
	if len(when.GatewayNotIn) > 0 {
		if ctx.GatewayErr != nil {
			log.Printf("server.%s.match.%s: gateway lookup failed: %v", serverName, branchName, ctx.GatewayErr)
		}
		if matchIPList(ctx.Gateways, when.GatewayNotIn) {
			return false
		}
	}

	if len(when.UsernameIn) > 0 && !matchStringList(ctx.Username, when.UsernameIn) {
		return false
	}
	if len(when.UsernameNotIn) > 0 && matchStringList(ctx.Username, when.UsernameNotIn) {
		return false
	}

	if len(when.HostnameIn) > 0 && !matchStringList(ctx.Hostname, when.HostnameIn) {
		return false
	}
	if len(when.HostnameNotIn) > 0 && matchStringList(ctx.Hostname, when.HostnameNotIn) {
		return false
	}

	if len(when.OSIn) > 0 && !matchStringList(ctx.OS, when.OSIn) {
		return false
	}
	if len(when.OSNotIn) > 0 && matchStringList(ctx.OS, when.OSNotIn) {
		return false
	}

	if len(when.TermIn) > 0 && !matchAnyStringList(ctx.Terms, when.TermIn) {
		return false
	}
	if len(when.TermNotIn) > 0 && matchAnyStringList(ctx.Terms, when.TermNotIn) {
		return false
	}

	if len(when.EnvIn) > 0 && !matchAnyStringList(ctx.EnvNames, when.EnvIn) {
		return false
	}
	if len(when.EnvNotIn) > 0 && matchAnyStringList(ctx.EnvNames, when.EnvNotIn) {
		return false
	}
	if len(when.EnvValueIn) > 0 && !matchEnvValueList(ctx.EnvValues, when.EnvValueIn) {
		return false
	}
	if len(when.EnvValueNotIn) > 0 && matchEnvValueList(ctx.EnvValues, when.EnvValueNotIn) {
		return false
	}

	return true
}

func (c *Config) activeOpenSSHConfigs() []OpenSSHConfig {
	if len(c.SSHConfig) == 0 {
		return nil
	}

	reqs, err := c.validateOpenSSHConfigWhens()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	ctx := matchContext{}
	if reqs.needLocalIP || reqs.needGateway || reqs.needUsername || reqs.needHostname || reqs.needOS || reqs.needTerm || reqs.needEnv {
		ctx = detectMatchContext(reqs)
	}

	configs := sortedOpenSSHConfigs(c.SSHConfig)
	result := make([]OpenSSHConfig, 0, len(configs))
	for _, item := range configs {
		if item.config.When.Empty() || whenMatches(item.config.When, "sshconfig", item.name, ctx) {
			result = append(result, item.config)
		}
	}

	return result
}

func sortedOpenSSHConfigs(configs map[string]OpenSSHConfig) []namedOpenSSHConfig {
	result := make([]namedOpenSSHConfig, 0, len(configs))
	for name, config := range configs {
		result = append(result, namedOpenSSHConfig{name: name, config: config})
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].name < result[j].name
	})

	return result
}

func (c *Config) validateOpenSSHConfigWhens() (matchRequirements, error) {
	reqs := matchRequirements{}

	for name, sc := range c.SSHConfig {
		if sc.When.Empty() {
			continue
		}

		if err := validateMatchNetworkList(sc.When.LocalIPIn, "local_ip_in", "sshconfig", name); err != nil {
			return reqs, err
		}
		if err := validateMatchNetworkList(sc.When.LocalIPNotIn, "local_ip_not_in", "sshconfig", name); err != nil {
			return reqs, err
		}
		if err := validateMatchNetworkList(sc.When.GatewayIn, "gateway_in", "sshconfig", name); err != nil {
			return reqs, err
		}
		if err := validateMatchNetworkList(sc.When.GatewayNotIn, "gateway_not_in", "sshconfig", name); err != nil {
			return reqs, err
		}

		reqs.needLocalIP = reqs.needLocalIP || len(sc.When.LocalIPIn) > 0 || len(sc.When.LocalIPNotIn) > 0
		reqs.needGateway = reqs.needGateway || len(sc.When.GatewayIn) > 0 || len(sc.When.GatewayNotIn) > 0
		reqs.needUsername = reqs.needUsername || len(sc.When.UsernameIn) > 0 || len(sc.When.UsernameNotIn) > 0
		reqs.needHostname = reqs.needHostname || len(sc.When.HostnameIn) > 0 || len(sc.When.HostnameNotIn) > 0
		reqs.needOS = reqs.needOS || len(sc.When.OSIn) > 0 || len(sc.When.OSNotIn) > 0
		reqs.needTerm = reqs.needTerm || len(sc.When.TermIn) > 0 || len(sc.When.TermNotIn) > 0
		reqs.needEnv = reqs.needEnv || len(sc.When.EnvIn) > 0 || len(sc.When.EnvNotIn) > 0 || len(sc.When.EnvValueIn) > 0 || len(sc.When.EnvValueNotIn) > 0
	}

	return reqs, nil
}

func matchStringList(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func matchAnyStringList(actual []string, expected []string) bool {
	if len(actual) == 0 {
		return false
	}

	set := make(map[string]struct{}, len(actual))
	for _, value := range actual {
		set[strings.ToLower(value)] = struct{}{}
	}

	for _, candidate := range expected {
		if _, ok := set[strings.ToLower(candidate)]; ok {
			return true
		}
	}

	return false
}

func matchEnvValueList(actual map[string]string, expected []string) bool {
	if len(actual) == 0 {
		return false
	}

	for _, candidate := range expected {
		name, value, ok := strings.Cut(candidate, "=")
		if !ok {
			continue
		}

		actualValue, exists := actual[name]
		if exists && actualValue == value {
			return true
		}
	}

	return false
}

func matchIPList(actual []netip.Addr, expected []string) bool {
	if len(actual) == 0 {
		return false
	}

	for _, exp := range expected {
		addr, prefix, err := parseIPorPrefix(exp)
		if err != nil {
			return false
		}

		for _, actualAddr := range actual {
			if addr.IsValid() && actualAddr == addr {
				return true
			}
			if prefix.IsValid() && prefix.Contains(actualAddr) {
				return true
			}
		}
	}

	return false
}

func parseIPorPrefix(value string) (netip.Addr, netip.Prefix, error) {
	if addr, err := netip.ParseAddr(value); err == nil {
		return addr.Unmap(), netip.Prefix{}, nil
	}

	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return netip.Addr{}, netip.Prefix{}, fmt.Errorf("invalid IP/CIDR %q", value)
	}

	return netip.Addr{}, prefix.Masked(), nil
}

func buildMatchContext(reqs matchRequirements) matchContext {
	ctx := matchContext{}

	if reqs.needLocalIP {
		ctx.LocalIPs, ctx.LocalIPErr = getLocalIPs()
	}
	if reqs.needGateway {
		ctx.Gateways, ctx.GatewayErr = getDefaultGateways()
	}
	if reqs.needUsername {
		ctx.Username, ctx.UsernameErr = getCurrentUsername()
	}
	if reqs.needHostname {
		ctx.Hostname, ctx.HostnameErr = os.Hostname()
	}
	if reqs.needOS {
		ctx.OS = runtime.GOOS
	}
	if reqs.needTerm {
		ctx.Terms = getTerminalKinds()
	}
	if reqs.needEnv {
		ctx.EnvNames = getEnvNames()
		ctx.EnvValues = getEnvValues()
	}

	return ctx
}

func getTerminalKinds() []string {
	var values []string

	if termProgram := strings.TrimSpace(os.Getenv("TERM_PROGRAM")); termProgram != "" {
		normalized := strings.ToLower(termProgram)
		values = append(values, normalized)

		switch {
		case strings.Contains(normalized, "iterm"):
			values = append(values, "iterm2")
		case strings.Contains(normalized, "apple_terminal"), strings.Contains(normalized, "terminal.app"):
			values = append(values, "terminal")
		}
	}

	if os.Getenv("WT_SESSION") != "" {
		values = append(values, "windows-terminal")
	}

	if term := strings.TrimSpace(os.Getenv("TERM")); term != "" {
		normalized := strings.ToLower(term)
		values = append(values, normalized)

		if idx := strings.IndexByte(normalized, '-'); idx > 0 {
			values = append(values, normalized[:idx])
		}
	}

	return uniqueStrings(values)
}

func getEnvNames() []string {
	envs := os.Environ()
	result := make([]string, 0, len(envs))

	for _, env := range envs {
		name, _, ok := strings.Cut(env, "=")
		if !ok || name == "" {
			continue
		}
		result = append(result, name)
	}

	return uniqueStrings(result)
}

func getEnvValues() map[string]string {
	envs := os.Environ()
	result := make(map[string]string, len(envs))

	for _, env := range envs {
		name, value, ok := strings.Cut(env, "=")
		if !ok || name == "" {
			continue
		}
		result[name] = value
	}

	return result
}

func getCurrentUsername() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

func getLocalIPs() ([]netip.Addr, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	result := []netip.Addr{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			prefix, err := netip.ParsePrefix(addr.String())
			if err != nil {
				continue
			}
			result = append(result, prefix.Addr().Unmap())
		}
	}

	return uniqueAddrs(result), nil
}

func getDefaultGateways() ([]netip.Addr, error) {
	switch runtime.GOOS {
	case "linux":
		return getLinuxDefaultGateways()
	case "darwin":
		return getDarwinDefaultGateways()
	case "windows":
		return getWindowsDefaultGateways()
	default:
		return nil, fmt.Errorf("gateway lookup is not supported on %s", runtime.GOOS)
	}
}

func getLinuxDefaultGateways() ([]netip.Addr, error) {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var gateways []netip.Addr
	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}

		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}

		value, err := strconv.ParseUint(fields[2], 16, 32)
		if err != nil {
			continue
		}

		ip := net.IPv4(byte(value), byte(value>>8), byte(value>>16), byte(value>>24))
		addr, ok := netip.AddrFromSlice(ip)
		if ok {
			gateways = append(gateways, addr.Unmap())
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(gateways) == 0 {
		return nil, fmt.Errorf("default gateway not found")
	}

	return uniqueAddrs(gateways), nil
}

func getDarwinDefaultGateways() ([]netip.Addr, error) {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return nil, err
	}

	var gateways []netip.Addr
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "gateway:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "gateway:"))
		addr, err := netip.ParseAddr(value)
		if err == nil {
			gateways = append(gateways, addr.Unmap())
		}
	}

	if len(gateways) == 0 {
		return nil, fmt.Errorf("default gateway not found")
	}

	return uniqueAddrs(gateways), nil
}

func getWindowsDefaultGateways() ([]netip.Addr, error) {
	out, err := exec.Command("route", "print", "0.0.0.0").Output()
	if err != nil {
		return nil, err
	}

	var gateways []netip.Addr
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || fields[0] != "0.0.0.0" || fields[1] != "0.0.0.0" {
			continue
		}
		addr, err := netip.ParseAddr(fields[2])
		if err == nil {
			gateways = append(gateways, addr.Unmap())
		}
	}

	if len(gateways) == 0 {
		return nil, fmt.Errorf("default gateway not found")
	}

	return uniqueAddrs(gateways), nil
}

func uniqueAddrs(values []netip.Addr) []netip.Addr {
	if len(values) == 0 {
		return nil
	}

	seen := map[netip.Addr]struct{}{}
	result := make([]netip.Addr, 0, len(values))
	for _, value := range values {
		if !value.IsValid() {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}

		normalized := strings.ToLower(value)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result
}

func mergeServerMatchConfig(base ServerConfig, match ServerMatchConfig) ServerConfig {
	result := base
	override := match.OverrideConfig()

	baseValue := reflect.ValueOf(&result).Elem()
	overrideValue := reflect.ValueOf(override)
	baseType := baseValue.Type()

	for i := 0; i < baseValue.NumField(); i++ {
		field := baseType.Field(i)
		tag := field.Tag.Get("toml")
		if tag == "" || !match.IsDefined(tag) {
			continue
		}

		baseValue.Field(i).Set(overrideValue.FieldByName(field.Name))
	}

	return result
}
