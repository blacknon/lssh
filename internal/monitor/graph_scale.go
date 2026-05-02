// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	conf "github.com/blacknon/lssh/internal/config"
)

var (
	nvmePartitionPattern   = regexp.MustCompile(`^(nvme\d+n\d+)p\d+$`)
	mmcblkPartitionPattern = regexp.MustCompile(`^(mmcblk\d+)p\d+$`)
	sdLikePartitionPattern = regexp.MustCompile(`^((?:sd|vd|xvd|hd)[a-z]+)\d+$`)
)

const (
	monitorSampleInterval         = 2 * time.Second
	defaultFDGraphMax             = 16384
	defaultNetworkBytesPerSec     = 125000000
	defaultNetworkPacketSizeBytes = 1500
	defaultDiskReadBytesPerSec    = 200000000
	defaultDiskWriteBytesPerSec   = 200000000
	defaultRotationalDiskBytesSec = 200000000
	defaultSolidStateDiskBytesSec = 550000000
	defaultNVMeDiskBytesPerSec    = 3000000000
	defaultMMCBlockBytesPerSec    = 100000000
)

type GraphScaleConfig struct {
	FDMax uint64

	UseNetworkInterfaceSpeed  bool
	NetworkDefaultBytesPerSec uint64
	NetworkBytesPerSec        map[string]uint64

	DiskDefaultReadBytesPerSec  uint64
	DiskDefaultWriteBytesPerSec uint64
	DiskReadBytesPerSec         map[string]uint64
	DiskWriteBytesPerSec        map[string]uint64
}

func newGraphScaleConfig(cfg conf.MonitorGraphConfig) GraphScaleConfig {
	useNetworkInterfaceSpeed := true
	if cfg.UseNetworkInterfaceSpeed != nil {
		useNetworkInterfaceSpeed = *cfg.UseNetworkInterfaceSpeed
	}

	return GraphScaleConfig{
		FDMax:                    firstNonZeroUint64(cfg.FDMax, defaultFDGraphMax),
		UseNetworkInterfaceSpeed: useNetworkInterfaceSpeed,
		NetworkDefaultBytesPerSec: firstNonZeroUint64(
			cfg.NetworkDefaultBytesPerSec,
			defaultNetworkBytesPerSec,
		),
		NetworkBytesPerSec: normalizeInterfaceRateMap(cfg.NetworkBytesPerSec),
		DiskDefaultReadBytesPerSec: firstNonZeroUint64(
			cfg.DiskDefaultReadBytesPerSec,
			defaultDiskReadBytesPerSec,
		),
		DiskDefaultWriteBytesPerSec: firstNonZeroUint64(
			cfg.DiskDefaultWriteBytesPerSec,
			defaultDiskWriteBytesPerSec,
		),
		DiskReadBytesPerSec:  normalizeBlockDeviceRateMap(cfg.DiskReadBytesPerSec),
		DiskWriteBytesPerSec: normalizeBlockDeviceRateMap(cfg.DiskWriteBytesPerSec),
	}
}

func firstNonZeroUint64(values ...uint64) uint64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func bytesPerSample(bytesPerSec uint64) uint64 {
	if bytesPerSec == 0 {
		return 0
	}

	seconds := uint64(monitorSampleInterval / time.Second)
	if seconds == 0 {
		return bytesPerSec
	}
	return bytesPerSec * seconds
}

func normalizeInterfaceRateMap(src map[string]uint64) map[string]uint64 {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]uint64, len(src))
	for key, value := range src {
		normalized := normalizeInterfaceKey(key)
		if normalized == "" || value == 0 {
			continue
		}
		dst[normalized] = value
	}
	return dst
}

func normalizeBlockDeviceRateMap(src map[string]uint64) map[string]uint64 {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]uint64, len(src))
	for key, value := range src {
		normalized := normalizeBlockDeviceKey(key)
		if normalized == "" || value == 0 {
			continue
		}
		dst[normalized] = value
	}
	return dst
}

func normalizeInterfaceKey(key string) string {
	return strings.TrimSpace(key)
}

func normalizeBlockDeviceKey(device string) string {
	device = strings.TrimSpace(device)
	if device == "" {
		return ""
	}

	device = strings.TrimPrefix(device, "/dev/")
	device = filepath.Base(device)
	device = strings.TrimSpace(device)

	if match := nvmePartitionPattern.FindStringSubmatch(device); len(match) == 2 {
		return match[1]
	}
	if match := mmcblkPartitionPattern.FindStringSubmatch(device); len(match) == 2 {
		return match[1]
	}
	if match := sdLikePartitionPattern.FindStringSubmatch(device); len(match) == 2 {
		return match[1]
	}

	return device
}

func (n *Node) ApplyGraphScaleConfig(cfg GraphScaleConfig) {
	n.Lock()
	defer n.Unlock()

	n.graphScale = cfg
	if n.networkGraphMaxBytes == nil {
		n.networkGraphMaxBytes = map[string]uint64{}
	}
	if n.diskReadGraphMaxBytes == nil {
		n.diskReadGraphMaxBytes = map[string]uint64{}
	}
	if n.diskWriteGraphMaxBytes == nil {
		n.diskWriteGraphMaxBytes = map[string]uint64{}
	}
}

func (n *Node) GetFDGraphMax() uint64 {
	n.RLock()
	defer n.RUnlock()

	return firstNonZeroUint64(n.graphScale.FDMax, defaultFDGraphMax)
}

func (n *Node) GetNetworkGraphMaxBytes(device string) uint64 {
	device = normalizeInterfaceKey(device)
	if device == "" {
		return bytesPerSample(defaultNetworkBytesPerSec)
	}

	n.RLock()
	if configured := n.graphScale.NetworkBytesPerSec[device]; configured > 0 {
		n.RUnlock()
		return bytesPerSample(configured)
	}
	if cached := n.networkGraphMaxBytes[device]; cached > 0 {
		n.RUnlock()
		return cached
	}
	useDeviceSpeed := n.graphScale.UseNetworkInterfaceSpeed
	fallback := firstNonZeroUint64(n.graphScale.NetworkDefaultBytesPerSec, defaultNetworkBytesPerSec)
	n.RUnlock()

	maxBytes := bytesPerSample(fallback)
	if useDeviceSpeed {
		speedPath := fmt.Sprintf("/sys/class/net/%s/speed", device)
		if speedMbps, err := n.con.ReadUint64(speedPath); err == nil && speedMbps > 0 {
			maxBytes = bytesPerSample(speedMbps * 1000 * 1000 / 8)
		}
	}

	n.Lock()
	n.networkGraphMaxBytes[device] = maxBytes
	n.Unlock()

	return maxBytes
}

func (n *Node) GetNetworkGraphMaxPackets(device string) uint64 {
	maxBytes := n.GetNetworkGraphMaxBytes(device)
	if maxBytes == 0 {
		return 1
	}

	maxPackets := maxBytes / defaultNetworkPacketSizeBytes
	if maxPackets == 0 {
		return 1
	}

	return maxPackets
}

func (n *Node) GetDiskReadGraphMaxBytes(device string) uint64 {
	return n.getDiskGraphMaxBytes(device, true)
}

func (n *Node) GetDiskWriteGraphMaxBytes(device string) uint64 {
	return n.getDiskGraphMaxBytes(device, false)
}

func (n *Node) getDiskGraphMaxBytes(device string, isRead bool) uint64 {
	deviceKey := normalizeBlockDeviceKey(device)
	if deviceKey == "" {
		if isRead {
			return bytesPerSample(defaultDiskReadBytesPerSec)
		}
		return bytesPerSample(defaultDiskWriteBytesPerSec)
	}

	n.RLock()
	if isRead {
		if configured := n.graphScale.DiskReadBytesPerSec[deviceKey]; configured > 0 {
			n.RUnlock()
			return bytesPerSample(configured)
		}
		if cached := n.diskReadGraphMaxBytes[deviceKey]; cached > 0 {
			n.RUnlock()
			return cached
		}
	} else {
		if configured := n.graphScale.DiskWriteBytesPerSec[deviceKey]; configured > 0 {
			n.RUnlock()
			return bytesPerSample(configured)
		}
		if cached := n.diskWriteGraphMaxBytes[deviceKey]; cached > 0 {
			n.RUnlock()
			return cached
		}
	}
	readFallback := firstNonZeroUint64(n.graphScale.DiskDefaultReadBytesPerSec, defaultDiskReadBytesPerSec)
	writeFallback := firstNonZeroUint64(n.graphScale.DiskDefaultWriteBytesPerSec, defaultDiskWriteBytesPerSec)
	n.RUnlock()

	readBytesPerSec, writeBytesPerSec := n.inferDiskBytesPerSecond(deviceKey, readFallback, writeFallback)
	readMax := bytesPerSample(readBytesPerSec)
	writeMax := bytesPerSample(writeBytesPerSec)

	n.Lock()
	n.diskReadGraphMaxBytes[deviceKey] = readMax
	n.diskWriteGraphMaxBytes[deviceKey] = writeMax
	n.Unlock()

	if isRead {
		return readMax
	}
	return writeMax
}

func (n *Node) inferDiskBytesPerSecond(deviceKey string, readFallback, writeFallback uint64) (uint64, uint64) {
	if strings.HasPrefix(deviceKey, "nvme") {
		return defaultNVMeDiskBytesPerSec, defaultNVMeDiskBytesPerSec
	}
	if strings.HasPrefix(deviceKey, "mmcblk") {
		return defaultMMCBlockBytesPerSec, defaultMMCBlockBytesPerSec
	}

	rotationalPath := fmt.Sprintf("/sys/block/%s/queue/rotational", deviceKey)
	if rotational, err := n.con.ReadUint64(rotationalPath); err == nil {
		if rotational == 0 {
			return defaultSolidStateDiskBytesSec, defaultSolidStateDiskBytesSec
		}
		return defaultRotationalDiskBytesSec, defaultRotationalDiskBytesSec
	}

	return readFallback, writeFallback
}
