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
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type matchContext struct {
	LocalIPs    []netip.Addr
	Gateways    []netip.Addr
	Username    string
	Hostname    string
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
}

type namedMatch struct {
	name   string
	config ServerMatchConfig
}

var detectMatchContext = buildMatchContext

func decodeConfigFile(path string, c *Config) error {
	md, err := toml.DecodeFile(path, c)
	if err != nil {
		return err
	}

	applyMatchMetadata(c, md)
	return nil
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

		serverConf.Match[branchName] = matchConf
		c.Server[serverName] = serverConf
	}
}

func (c *Config) ResolveConditionalMatches() error {
	reqs, err := c.validateMatchConfigs()
	if err != nil {
		return err
	}
	if !reqs.needLocalIP && !reqs.needGateway && !reqs.needUsername && !reqs.needHostname {
		return nil
	}

	ctx := detectMatchContext(reqs)

	for serverName, serverConf := range c.Server {
		if len(serverConf.Match) == 0 {
			continue
		}

		matches := sortedMatches(serverConf.Match)
		for _, branch := range matches {
			ok := branchMatches(serverName, branch.name, branch.config, ctx)
			if !ok {
				continue
			}

			merged := serverConfigReduct(serverConf, branch.config.OverrideConfig())
			merged.Match = serverConf.Match
			c.Server[serverName] = merged
			break
		}
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
	when := branch.When

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

	return true
}

func matchStringList(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if value == candidate {
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

	return ctx
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
