// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

type IPv6 struct {
	Interface string
	IPAddress net.IP
	Prefix    string
}

func (p *ConnectWithProc) ReadIfInet6(path string) (data []IPv6, err error) {
	// Read /proc/net/fib_trie
	file, err := p.sftp.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 6 {
			ipv6Hex := fields[0]
			device := fields[5]
			prefixLength := fields[2] // プレフィックス長は3番目のフィールド

			ipv6Addr := parseIPv6ToNetIP(ipv6Hex)

			ipv6 := IPv6{
				Interface: device,
				IPAddress: ipv6Addr,
				Prefix:    prefixLength,
			}

			data = append(data, ipv6)
		}
	}

	err = scanner.Err()
	return
}

func parseIPv6ToNetIP(hex string) net.IP {
	var ipParts []byte
	for i := 0; i < len(hex); i += 2 {
		byteVal := hex[i : i+2]
		val, _ := parseHexByte(byteVal)
		ipParts = append(ipParts, val)
	}
	return net.IP(ipParts)
}

func parseHexByte(hexByte string) (byte, error) {
	var b byte
	_, err := fmt.Sscanf(hexByte, "%02x", &b)
	return b, err
}
