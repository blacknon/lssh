// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
)

const sshTunnelDeviceAny uint32 = 0x7fffffff

const (
	utunFamilyUnspec = 0
	utunFamilyIPv4   = 2
	utunFamilyIPv6   = 30
)

// TunnelDeviceAny requests that the next available tunnel interface be used.
//
// This is equivalent to OpenSSH's "any" tunnel device keyword.
const TunnelDeviceAny = -1

// TunnelMode is the forwarding mode used by OpenSSH tunnel forwarding.
type TunnelMode uint32

const (
	// TunnelModePointToPoint corresponds to `ssh -w` using a tun device.
	TunnelModePointToPoint TunnelMode = 1

	// TunnelModeEthernet corresponds to OpenSSH ethernet tunnel forwarding.
	TunnelModeEthernet TunnelMode = 2
)

type tunnelOpenChannelMessage struct {
	Mode uint32
	Unit uint32
}

type tunnelDevice struct {
	io.ReadWriteCloser
	Name string
	Unit int
}

type utunDevice struct {
	file *os.File
}

// Tunnel is an active SSH tunnel forwarding session.
type Tunnel struct {
	LocalName string
	LocalID   int
	RemoteID  int
	Mode      TunnelMode

	local   io.ReadWriteCloser
	channel ssh.Channel
	remote  io.Closer

	closeOnce sync.Once
	doneOnce  sync.Once
	done      chan error
}

// Close tears down the local device and SSH tunnel channel.
func (t *Tunnel) Close() error {
	var errs []error

	t.closeOnce.Do(func() {
		if t.channel != nil {
			if err := t.channel.Close(); err != nil && !errors.Is(err, io.EOF) {
				errs = append(errs, err)
			}
		}

		if t.remote != nil {
			if err := t.remote.Close(); err != nil && !errors.Is(err, io.EOF) {
				errs = append(errs, err)
			}
		}

		if err := t.local.Close(); err != nil && !errors.Is(err, io.EOF) {
			errs = append(errs, err)
		}
	})

	t.finish(nil)

	return errors.Join(errs...)
}

// Wait blocks until the tunnel stops.
func (t *Tunnel) Wait() error {
	return <-t.done
}

func (t *Tunnel) finish(err error) {
	t.doneOnce.Do(func() {
		t.done <- err
		close(t.done)
	})
}

// Tunnel opens a point-to-point tunnel device and starts forwarding packets.
//
// This is equivalent to `ssh -w localTun:remoteTun`.
// Use TunnelDeviceAny for either side to request "any".
func (c *Connect) Tunnel(localTun, remoteTun int) (*Tunnel, error) {
	return c.TunnelWithMode(localTun, remoteTun, TunnelModePointToPoint)
}

// TunnelWithMode opens a tunnel device and starts forwarding packets.
//
// This mirrors OpenSSH's Tunnel/TunnelDevice directives. The caller is still
// responsible for configuring IP addresses and routes on both ends.
func (c *Connect) TunnelWithMode(localTun, remoteTun int, mode TunnelMode) (*Tunnel, error) {
	if err := mode.validate(); err != nil {
		return nil, err
	}

	if c.isControlClient() {
		return c.openControlTunnel(localTun, remoteTun, mode)
	}

	if c.Client == nil {
		return nil, errors.New("ssh client is nil")
	}

	localDevice, err := openTunnelDevice(localTun, mode)
	if err != nil {
		return nil, fmt.Errorf("open local tunnel device (local=%s, mode=%s): %w", describeTunnelUnit(localTun), mode.String(), err)
	}

	msg := tunnelOpenChannelMessage{
		Mode: uint32(mode),
		Unit: normalizeTunnelUnit(remoteTun),
	}

	channel, reqs, err := c.Client.OpenChannel("tun@openssh.com", ssh.Marshal(&msg))
	if err != nil {
		localDevice.Close()
		return nil, fmt.Errorf("open ssh tunnel channel tun@openssh.com (remote=%s, mode=%s): %w", describeTunnelUnit(remoteTun), mode.String(), err)
	}

	go ssh.DiscardRequests(reqs)

	tunnel := &Tunnel{
		LocalName: localDevice.Name,
		LocalID:   localDevice.Unit,
		RemoteID:  remoteTun,
		Mode:      mode,
		local:     localDevice,
		channel:   channel,
		done:      make(chan error, 1),
	}

	go tunnel.copyPackets(channel, localDevice)
	go tunnel.copyPackets(localDevice, channel)

	return tunnel, nil
}

func (c *Connect) openControlTunnel(localTun, remoteTun int, mode TunnelMode) (*Tunnel, error) {
	localDevice, err := openTunnelDevice(localTun, mode)
	if err != nil {
		return nil, fmt.Errorf("open local tunnel device (local=%s, mode=%s): %w", describeTunnelUnit(localTun), mode.String(), err)
	}

	req := controlRequest{
		Type: controlRequestTunnel,
		Tunnel: controlTunnelOptions{
			Mode: mode,
			Unit: remoteTun,
		},
	}

	resp, err := c.requestControl(req)
	if err != nil {
		_ = localDevice.Close()
		return nil, err
	}

	conn, err := net.Dial("unix", resp.StreamPath)
	if err != nil {
		_ = localDevice.Close()
		return nil, err
	}

	tunnel := &Tunnel{
		LocalName: localDevice.Name,
		LocalID:   localDevice.Unit,
		RemoteID:  remoteTun,
		Mode:      mode,
		local:     localDevice,
		remote:    conn,
		done:      make(chan error, 1),
	}

	go tunnel.copyControlInput(conn, localDevice)
	go tunnel.copyControlOutput(localDevice, conn)

	return tunnel, nil
}

func (t *Tunnel) copyPackets(dst io.Writer, src io.Reader) {
	buf := make([]byte, 64*1024)

	for {
		n, err := src.Read(buf)
		if err != nil {
			if shouldRetryTunnelCopyError(err) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if !errors.Is(err, io.EOF) {
				t.finish(err)
			} else {
				t.finish(nil)
			}
			_ = t.Close()
			return
		}

		if n == 0 {
			continue
		}

		if _, err := dst.Write(buf[:n]); err != nil {
			if shouldRetryTunnelCopyError(err) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if !errors.Is(err, io.EOF) {
				t.finish(err)
			} else {
				t.finish(nil)
			}
			_ = t.Close()
			return
		}
	}
}

func (t *Tunnel) copyControlInput(dst io.Writer, src io.Reader) {
	buf := make([]byte, 64*1024)

	for {
		n, err := src.Read(buf)
		if err != nil {
			if shouldRetryTunnelCopyError(err) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if !errors.Is(err, io.EOF) {
				t.finish(err)
			} else {
				t.finish(nil)
			}
			_ = t.Close()
			return
		}

		if n == 0 {
			continue
		}

		if err := writeStreamFrame(dst, streamFrameStdin, buf[:n]); err != nil {
			if !errors.Is(err, io.EOF) {
				t.finish(err)
			} else {
				t.finish(nil)
			}
			_ = t.Close()
			return
		}
	}
}

func (t *Tunnel) copyControlOutput(dst io.Writer, src io.Reader) {
	for {
		frameType, payload, err := readStreamFrame(src)
		if err != nil {
			if err == io.EOF {
				t.finish(nil)
			} else {
				t.finish(err)
			}
			_ = t.Close()
			return
		}

		switch frameType {
		case streamFrameStdout:
			if len(payload) == 0 {
				continue
			}
			if _, err := dst.Write(payload); err != nil {
				t.finish(err)
				_ = t.Close()
				return
			}
		case streamFrameError:
			if len(payload) > 0 {
				t.finish(errors.New(string(payload)))
			} else {
				t.finish(errors.New("sshlib: control tunnel failed"))
			}
			_ = t.Close()
			return
		case streamFrameExit:
			if len(payload) == 4 && binary.BigEndian.Uint32(payload) == 0 {
				t.finish(nil)
			} else {
				t.finish(errors.New("sshlib: control tunnel closed unexpectedly"))
			}
			_ = t.Close()
			return
		}
	}
}

func shouldRetryTunnelCopyError(err error) bool {
	switch {
	case errors.Is(err, syscall.EIO):
		return true
	case errors.Is(err, syscall.EHOSTDOWN):
		return true
	case errors.Is(err, syscall.EAGAIN):
		return true
	case errors.Is(err, syscall.EWOULDBLOCK):
		return true
	default:
		return false
	}
}

func (m TunnelMode) validate() error {
	switch m {
	case TunnelModePointToPoint, TunnelModeEthernet:
		return nil
	default:
		return fmt.Errorf("unsupported tunnel mode: %d", m)
	}
}

func (m TunnelMode) String() string {
	switch m {
	case TunnelModePointToPoint:
		return "point-to-point"
	case TunnelModeEthernet:
		return "ethernet"
	default:
		return fmt.Sprintf("unknown(%d)", m)
	}
}

func (m TunnelMode) devicePrefix() string {
	switch m {
	case TunnelModeEthernet:
		return "tap"
	default:
		return "tun"
	}
}

func normalizeTunnelUnit(unit int) uint32 {
	if unit < 0 {
		return sshTunnelDeviceAny
	}
	return uint32(unit)
}

func parseTunnelInterfaceUnit(name string) (int, error) {
	i := len(name)
	for i > 0 && name[i-1] >= '0' && name[i-1] <= '9' {
		i--
	}

	if i == len(name) {
		return 0, fmt.Errorf("tunnel interface %q does not end with a numeric unit", name)
	}

	return strconv.Atoi(strings.TrimSpace(name[i:]))
}

func describeTunnelUnit(unit int) string {
	if unit == TunnelDeviceAny {
		return "any"
	}
	return strconv.Itoa(unit)
}

func (d *utunDevice) Close() error {
	return d.file.Close()
}

func (d *utunDevice) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	packet := make([]byte, len(p)+4)
	n, err := d.file.Read(packet)
	if n <= 4 {
		if err != nil {
			return 0, err
		}
		return 0, io.ErrUnexpectedEOF
	}

	copy(p, packet[4:n])
	return n - 4, err
}

func (d *utunDevice) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	packet := make([]byte, len(p)+4)
	binary.BigEndian.PutUint32(packet[:4], utunPacketFamily(p))
	copy(packet[4:], p)

	n, err := d.file.Write(packet)
	if n < 4 {
		return 0, err
	}

	return n - 4, err
}

func utunPacketFamily(packet []byte) uint32 {
	if len(packet) == 0 {
		return utunFamilyUnspec
	}

	version := packet[0] >> 4
	switch version {
	case 4:
		return utunFamilyIPv4
	case 6:
		return utunFamilyIPv6
	default:
		return utunFamilyUnspec
	}
}

func buildLinuxTunnelName(unit int, mode TunnelMode) string {
	if unit < 0 {
		return ""
	}

	return mode.devicePrefix() + strconv.Itoa(unit)
}

func utunControlUnit(unit int) uint32 {
	if unit < 0 {
		return 0
	}

	return uint32(unit + 1)
}

func validateTunnelDevice(mode TunnelMode, name string) error {
	if mode == TunnelModeEthernet && !strings.HasPrefix(name, "tap") {
		return fmt.Errorf("expected TAP device for ethernet tunnel, got %q", name)
	}

	if mode == TunnelModePointToPoint {
		prefixes := []string{"tun", "utun"}
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				return nil
			}
		}
		return fmt.Errorf("expected TUN device for point-to-point tunnel, got %q", name)
	}

	return nil
}

func (t *Tunnel) LocalInterface() *net.Interface {
	iface, err := net.InterfaceByName(t.LocalName)
	if err != nil {
		return nil
	}
	return iface
}
