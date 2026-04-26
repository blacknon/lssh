//go:build windows
// +build windows

package gateway

import (
	"net"
	"os/exec"
	"syscall"
)

func discoverGatewaysOSSpecific() (ips []net.IP, err error) {
	routeCmd := exec.Command("route", "print", "0.0.0.0")
	routeCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := routeCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return parseWindowsGatewayIPs(output)
}

func discoverGatewayInterfaceOSSpecific() (ip net.IP, err error) {
	routeCmd := exec.Command("route", "print", "0.0.0.0")
	routeCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := routeCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	ips, err := parseWindowsInterfaceIP(output)
	if err != nil {
		return nil, err
	}
	return ips[0], nil
}
