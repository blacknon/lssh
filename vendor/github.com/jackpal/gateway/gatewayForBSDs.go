//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package gateway

import (
	"net"
	"os/exec"
	"syscall"

	"golang.org/x/net/route"
)

func readNetstat() ([]byte, error) {
	routeCmd := exec.Command("netstat", "-rn")
	return routeCmd.CombinedOutput()
}

func discoverGatewaysOSSpecific() (ips []net.IP, err error) {
	rib, err := route.FetchRIB(syscall.AF_INET, syscall.NET_RT_DUMP, 0)
	if err != nil {
		return nil, err
	}

	msgs, err := route.ParseRIB(syscall.NET_RT_DUMP, rib)
	if err != nil {
		return nil, err
	}

	var result []net.IP
	for _, m := range msgs {
		switch m := m.(type) {
		case *route.RouteMessage:
			var ip net.IP
			switch sa := m.Addrs[syscall.RTAX_GATEWAY].(type) {
			case *route.Inet4Addr:
				ip = net.IPv4(sa.IP[0], sa.IP[1], sa.IP[2], sa.IP[3])
				result = append(result, ip)
			case *route.Inet6Addr:
				ip = make(net.IP, net.IPv6len)
				copy(ip, sa.IP[:])
				result = append(result, ip)
			}
		}
	}
	if len(result) == 0 {
		return nil, &ErrNoGateway{}
	}
	return result, nil
}

func discoverGatewayInterfaceOSSpecific() (ip net.IP, err error) {
	bytes, err := readNetstat()
	if err != nil {
		return nil, err
	}

	return parseUnixInterfaceIP(bytes)
}
