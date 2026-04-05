// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

type IPv4 struct {
	Interface string
	IPAddress net.IP
	Netmask   net.IPMask
}

func (p *ConnectWithProc) ReadFibTrie(procFibTriePath string, procRoutePath string) (data []IPv4, err error) {
	// Read /proc/net/fib_trie
	fibTrie, err := p.sftp.Open(procFibTriePath)
	if err != nil {
		return
	}
	defer fibTrie.Close()

	scanner := bufio.NewScanner(fibTrie)
	ft, err := readFibTrieLocal(scanner)

	// Read /proc/net/route
	route, err := p.sftp.Open(procRoutePath)
	if err != nil {
		return
	}
	defer route.Close()

	b, err := io.ReadAll(route)
	if err != nil {
		return
	}
	rt := string(b)

	data, err = generateIPSet(rt, ft)

	return
}

func readFibTrieLocal(scanner *bufio.Scanner) (data string, err error) {
	var ft strings.Builder
	var in bool

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Local:") {
			in = true
		}
		if in {
			ft.WriteString(line + "\n")
		}
		if strings.TrimSpace(line) == "32 host" {
			in = false
		}
	}

	err = scanner.Err()
	data = ft.String()
	return
}

func generateIPSet(rt, ft string) (result []IPv4, err error) {
	// for loop lines rt
	rtLines := strings.Split(rt, "\n")
	for _, rl := range rtLines {
		fields := strings.Fields(rl)

		if len(fields) >= 9 && fields[2] == "00000000" && fields[7] != "FFFFFFFF" {
			// set value
			ifName := fields[0]
			netHex := fields[1] + fields[7]

			netDec, maskDec := parseHexToDec(netHex)

			ftLines := strings.Split(ft, "\n")
			var ipAddr string
			var inRange bool

			for _, line := range ftLines {
				if strings.Contains(line, netDec) {
					inRange = true
				}
				if inRange && strings.Contains(line, "32 host") {
					inRange = false
				}
				if inRange {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						ipAddr = fields[1]
					}
				}
			}

			ipAddress := net.ParseIP(ipAddr)
			ipMask := parseIPMask(maskDec)

			ipSet := IPv4{
				Interface: ifName,
				IPAddress: ipAddress,
				Netmask:   ipMask,
			}

			result = append(result, ipSet)
		}
	}

	return
}

func parseHexToDec(netHex string) (netDec string, maskDec string) {
	netParts := make([]int, 4)
	maskParts := make([]int, 4)

	for i := 0; i < 4; i++ {
		netParts[i] = parseHexToInt(netHex[i*2 : i*2+2])
		maskParts[i] = parseHexToInt(netHex[8+i*2 : 8+i*2+2])
	}

	netDec = fmt.Sprintf("%d.%d.%d.%d", netParts[3], netParts[2], netParts[1], netParts[0])
	maskDec = fmt.Sprintf("%d.%d.%d.%d", maskParts[3], maskParts[2], maskParts[1], maskParts[0])

	return netDec, maskDec
}

func parseHexToInt(hexStr string) int {
	value, _ := strconv.ParseInt(hexStr, 16, 32)
	return int(value)
}

func parseIPMask(maskStr string) net.IPMask {
	// Split the mask string into octets
	octets := strings.Split(maskStr, ".")
	if len(octets) != 4 {
		return nil
	}

	// Convert each octet to an integer
	var mask [4]byte
	for i, octet := range octets {
		var octetInt int
		fmt.Sscanf(octet, "%d", &octetInt)
		mask[i] = byte(octetInt)
	}

	return net.IPv4Mask(mask[0], mask[1], mask[2], mask[3])
}
