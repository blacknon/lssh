//go:build solaris
// +build solaris

package gateway

import (
	"net"
	"os/exec"
)

func readNetstat() ([]byte, error) {
	routeCmd := exec.Command("netstat", "-rn")
	return routeCmd.CombinedOutput()
}

func discoverGatewaysOSSpecific() (ips []net.IP, err error) {
	bytes, err := readNetstat()
	if err != nil {
		return nil, err
	}

	return parseUnixGatewayIPs(bytes)
}

func discoverGatewayInterfaceOSSpecific() (ip net.IP, err error) {
	bytes, err := readNetstat()
	if err != nil {
		return nil, err
	}

	return parseUnixInterfaceIP(bytes)
}
