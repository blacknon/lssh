// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sshproc "github.com/blacknon/go-sshproc"
	conf "github.com/blacknon/lssh/internal/config"
	sshrun "github.com/blacknon/lssh/internal/ssh"
	"github.com/c9s/goprocinfo/linux"
)

var fstype = map[string]bool{
	"ext2":     true,
	"ext3":     true,
	"ext4":     true,
	"btrfs":    true,
	"xfs":      true,
	"vfat":     true,
	"ntfs":     true,
	"exfat":    true,
	"reiserfs": true,
	"jfs":      true,
	"zfs":      true,
	"udev":     true,
	"tmpfs":    true,
}

// CPUUsage is monitoring cpu struct. cpu is CPUStatAll only
type CPUUsage struct {
	linux.CPUStat
	Detail    []linux.CPUStat
	Timestamp time.Time
}

type CPUUsageTop struct {
	Low    float64
	Normal float64
	Kernel float64
	Guest  float64
	Total  float64
}

// MemoryUsage is monitoring memory struct
type MemoryUsage struct {
	*linux.MemInfo
	timestamp time.Time
}

type DiskUsage struct {
	MountPoint   string
	FSType       string
	Device       string
	All          uint64
	Used         uint64
	Free         uint64
	ReadIOBytes  []int64
	WriteIOBytes []int64
}

type DiskIO struct {
	Device     string
	ReadIOs    uint64
	ReadBytes  int64
	WriteIOs   uint64
	WriteBytes int64
}

type NetworkUsage struct {
	Device      string
	IPv4Address string
	IPv6Address string
	RXBytes     []uint64
	TXBytes     []uint64
	RXPackets   []uint64
	TXPackets   []uint64
}

type NetworkIO struct {
	Device    string
	RXPackets uint64
	RXBytes   uint64
	TXPackets uint64
	TXBytes   uint64
	sync.RWMutex
}

type nodeSnapshot struct {
	CPUCore       int
	MemInfo       *linux.MemInfo
	KernelVersion string
	Uptime        *linux.Uptime
	TaskCount     uint64
	LoadAvg       *linux.LoadAvg
	DiskUsages    []*DiskUsage
	NetworkUsages []*NetworkUsage
	IPv4          []sshproc.IPv4
	IPv6          []sshproc.IPv6
	PressureCPU   *sshproc.PressureStat
	PressureMem   *sshproc.PressureStat
	PressureIO    *sshproc.PressureStat
	FileNr        *sshproc.FileNr
	TCPStates     sshproc.SocketStateCount
	UDPStates     sshproc.SocketStateCount
}

// Node is monitoring node struct
type Node struct {
	ServerName string

	con *sshproc.ConnectWithProc

	// Path
	PathProcStat      string
	PathProcCpuinfo   string
	PathProcMeminfo   string
	PathProcUptime    string
	PathProcLoadavg   string
	PathProcMounts    string
	PathProcDiskStats string

	// CPU Usage
	cpuUsage      []CPUUsage
	cpuUsageLimit int

	// DiskIO
	DiskIOs          map[string][]*DiskIO
	DiskIOsLimit     int
	DiskReadIOBytes  map[string][]int64
	DiskWriteIOBytes map[string][]int64

	// NetworkIO
	NetworkIOs       map[string][]*NetworkIO
	NetworkIOsLimit  int
	NetworkRXBytes   map[string][]uint64
	NetworkRXPackets map[string][]uint64
	NetworkTXBytes   map[string][]uint64
	NetworkTXPackets map[string][]uint64

	// Summary history
	PressureHistoryLimit  int
	PressureCPUHistory    []float64
	PressureCPU60History  []float64
	PressureCPU300History []float64
	PressureMemHistory    []float64
	PressureMem60History  []float64
	PressureIOHistory     []float64
	PressureIO60History   []float64

	FileNrHistoryLimit int
	FileNrAllocHistory []uint64

	SocketHistoryLimit int
	TCPEstHistory      []uint64
	TCPTWHistory       []uint64
	TCPListenHistory   []uint64

	// Top
	NodeTop *NodeTop

	snapshot nodeSnapshot

	graphScale             GraphScaleConfig
	networkGraphMaxBytes   map[string]uint64
	diskReadGraphMaxBytes  map[string]uint64
	diskWriteGraphMaxBytes map[string]uint64

	sync.RWMutex
}

// NewNode is create new Node struct.
// with set default values
func NewNode(name string) *Node {
	//
	procConnect := &sshproc.ConnectWithProc{Connect: nil}

	node := &Node{
		ServerName: name,

		con: procConnect,

		// set default path
		PathProcStat:      "/proc/stat",
		PathProcCpuinfo:   "/proc/cpuinfo",
		PathProcMeminfo:   "/proc/meminfo",
		PathProcUptime:    "/proc/uptime",
		PathProcLoadavg:   "/proc/loadavg",
		PathProcMounts:    "/proc/mounts",
		PathProcDiskStats: "/proc/diskstats",

		// CPU Usage
		cpuUsage:      []CPUUsage{},
		cpuUsageLimit: 480,

		// DiskIO
		DiskIOs:          map[string][]*DiskIO{},
		DiskIOsLimit:     480,
		DiskReadIOBytes:  map[string][]int64{},
		DiskWriteIOBytes: map[string][]int64{},

		// NetworkIO
		NetworkIOs:       map[string][]*NetworkIO{},
		NetworkIOsLimit:  480,
		NetworkRXBytes:   map[string][]uint64{},
		NetworkRXPackets: map[string][]uint64{},
		NetworkTXBytes:   map[string][]uint64{},
		NetworkTXPackets: map[string][]uint64{},

		PressureHistoryLimit: 60,
		FileNrHistoryLimit:   60,
		SocketHistoryLimit:   60,

		graphScale:             newGraphScaleConfig(conf.MonitorGraphConfig{}),
		networkGraphMaxBytes:   map[string]uint64{},
		diskReadGraphMaxBytes:  map[string]uint64{},
		diskWriteGraphMaxBytes: map[string]uint64{},
	}

	// Create Top
	_ = node.CreateNodeTop()

	return node
}

func (n *Node) CheckClientAlive() bool {
	if n.con == nil {
		return false
	}

	if n.con.Connect == nil {
		return false
	}

	return n.con.CheckSftpClient()
}

func (n *Node) Connect(r *sshrun.Run) (err error) {
	// Connect establishes a fresh SSH/SFTP session for this node.
	// The monitor owns the retry policy and may call this repeatedly after
	// disconnects. Callers should treat the previous transport as replaceable.
	// Create *sshlib.Connect
	con, err := r.CreateSshConnectDirect(n.ServerName)
	if err != nil {
		log.Printf("CreateSshConnect %s Error: %s", n.ServerName, err)
		n.con.Connect = nil
		return
	}

	// Create Session and run KeepAlive
	con.SendKeepAliveInterval = 10

	procCon := &sshproc.ConnectWithProc{Connect: con}
	err = procCon.CreateSftpClient()
	if err != nil {
		log.Printf("CreateSftpClient %s Error: %s", n.ServerName, err)
		n.con.Connect = nil
		return
	}

	n.con = procCon

	return
}

// GetCPUCore is get cpu core num
func (n *Node) GetCPUCore() (cn int, err error) {
	if !n.CheckClientAlive() {
		return
	}

	n.RLock()
	cn = n.snapshot.CPUCore
	n.RUnlock()
	return
}

func (n *Node) GetCPUUsage() (usage float64, err error) {
	if !n.CheckClientAlive() {
		return
	}

	n.RLock()
	cpuUsage := append([]CPUUsage(nil), n.cpuUsage...)
	n.RUnlock()

	usage = 0.0
	if len(cpuUsage) >= 2 {
		lUsage := cpuUsage[len(cpuUsage)-1]
		pUsage := cpuUsage[len(cpuUsage)-2]

		// Get total usage
		lUsageTotal := sumFloat64(
			float64(lUsage.User),
			float64(lUsage.Nice),
			float64(lUsage.System),
			float64(lUsage.Idle),
			float64(lUsage.IOWait),
			float64(lUsage.IRQ),
			float64(lUsage.SoftIRQ),
			float64(lUsage.Steal),
			float64(lUsage.Guest),
			float64(lUsage.GuestNice),
		)

		pUsageTotal := sumFloat64(
			float64(pUsage.User),
			float64(pUsage.Nice),
			float64(pUsage.System),
			float64(pUsage.Idle),
			float64(pUsage.IOWait),
			float64(pUsage.IRQ),
			float64(pUsage.SoftIRQ),
			float64(pUsage.Steal),
			float64(pUsage.Guest),
			float64(pUsage.GuestNice),
		)

		// Get idle
		lIdle := float64(lUsage.Idle)
		pIdle := float64(pUsage.Idle)

		// Get diff total
		totalDiff := lUsageTotal - pUsageTotal
		idleDiff := lIdle - pIdle

		usage = (totalDiff - idleDiff) / totalDiff * 100
	}

	return
}

func cpuStatTotal(stat linux.CPUStat) float64 {
	return sumFloat64(
		float64(stat.User),
		float64(stat.Nice),
		float64(stat.System),
		float64(stat.Idle),
		float64(stat.IOWait),
		float64(stat.IRQ),
		float64(stat.SoftIRQ),
		float64(stat.Steal),
		float64(stat.Guest),
		float64(stat.GuestNice),
	)
}

func (n *Node) GetCPUUsageWithSparkline() (usage float64, sparkline string, err error) {
	if !n.CheckClientAlive() {
		return
	}

	n.RLock()
	cpuUsage := append([]CPUUsage(nil), n.cpuUsage...)
	n.RUnlock()

	usages := []float64{}
	sparklineNums := 11

	for i := 1; i < sparklineNums; i++ {
		l := i
		p := i + 1

		if len(cpuUsage) < p+1 {
			usages = append(usages, 0.0)
			continue
		}

		lUsage := cpuUsage[len(cpuUsage)-l]
		pUsage := cpuUsage[len(cpuUsage)-p]

		// Get total usage
		lUsageTotal := sumFloat64(
			float64(lUsage.User),
			float64(lUsage.Nice),
			float64(lUsage.System),
			float64(lUsage.Idle),
			float64(lUsage.IOWait),
			float64(lUsage.IRQ),
			float64(lUsage.SoftIRQ),
			float64(lUsage.Steal),
			float64(lUsage.Guest),
			float64(lUsage.GuestNice),
		)

		pUsageTotal := sumFloat64(
			float64(pUsage.User),
			float64(pUsage.Nice),
			float64(pUsage.System),
			float64(pUsage.Idle),
			float64(pUsage.IOWait),
			float64(pUsage.IRQ),
			float64(pUsage.SoftIRQ),
			float64(pUsage.Steal),
			float64(pUsage.Guest),
			float64(pUsage.GuestNice),
		)

		// Get idle
		lIdle := float64(lUsage.Idle)
		pIdle := float64(pUsage.Idle)

		// Get diff total
		totalDiff := lUsageTotal - pUsageTotal
		idleDiff := lIdle - pIdle

		usages = append(usages, (totalDiff-idleDiff)/totalDiff*100)
	}

	usage = usages[0]
	sparkline = ""
	if len(usages) > 2 {
		graph := Graph{
			Data: usages,
			Max:  100,
			Min:  0,
		}

		sparkline = strings.Join(graph.Sparkline(), "")
	}

	return
}

func (n *Node) GetCPUUsageWithBrailleLine() (usage float64, brailleLine string, err error) {
	if !n.CheckClientAlive() {
		return
	}

	n.RLock()
	cpuUsage := append([]CPUUsage(nil), n.cpuUsage...)
	n.RUnlock()

	usages := []float64{}
	for i := 1; i < 22; i++ {
		l := i
		p := i + 1

		if len(cpuUsage) < p+1 {
			usages = append(usages, 0.0)
			continue
		}

		lUsage := cpuUsage[len(cpuUsage)-l]
		pUsage := cpuUsage[len(cpuUsage)-p]

		// Get total usage
		lUsageTotal := sumFloat64(
			float64(lUsage.User),
			float64(lUsage.Nice),
			float64(lUsage.System),
			float64(lUsage.Idle),
			float64(lUsage.IOWait),
			float64(lUsage.IRQ),
			float64(lUsage.SoftIRQ),
			float64(lUsage.Steal),
			float64(lUsage.Guest),
			float64(lUsage.GuestNice),
		)

		pUsageTotal := sumFloat64(
			float64(pUsage.User),
			float64(pUsage.Nice),
			float64(pUsage.System),
			float64(pUsage.Idle),
			float64(pUsage.IOWait),
			float64(pUsage.IRQ),
			float64(pUsage.SoftIRQ),
			float64(pUsage.Steal),
			float64(pUsage.Guest),
			float64(pUsage.GuestNice),
		)

		// Get idle
		lIdle := float64(lUsage.Idle)
		pIdle := float64(pUsage.Idle)

		// Get diff total
		totalDiff := lUsageTotal - pUsageTotal
		idleDiff := lIdle - pIdle

		usages = append(usages, (totalDiff-idleDiff)/totalDiff*100)
	}

	usage = usages[0]
	brailleLine = ""
	if len(usages) > 0 {
		graph := Graph{
			Data: usages,
			Max:  100,
			Min:  0,
		}

		brailleLine = strings.Join(graph.BrailleLine(), "")
	}

	return
}

func (n *Node) GetCPUCoreUsage() (usages []CPUUsageTop, err error) {
	if !n.CheckClientAlive() {
		return
	}

	n.RLock()
	cpuUsage := append([]CPUUsage(nil), n.cpuUsage...)
	n.RUnlock()

	usages = []CPUUsageTop{}
	if len(cpuUsage) >= 2 {
		lUsage := cpuUsage[len(cpuUsage)-1]
		pUsage := cpuUsage[len(cpuUsage)-2]

		for i := 0; i < len(lUsage.Detail); i++ {
			l := lUsage.Detail[i]
			p := pUsage.Detail[i]

			// Get total usage
			lUsageTotal := sumFloat64(
				float64(l.User),
				float64(l.Nice),
				float64(l.System),
				float64(l.Idle),
				float64(l.IOWait),
				float64(l.IRQ),
				float64(l.SoftIRQ),
				float64(l.Steal),
				float64(l.Guest),
				float64(l.GuestNice),
			)
			pUsageTotal := sumFloat64(
				float64(p.User),
				float64(p.Nice),
				float64(p.System),
				float64(p.Idle),
				float64(p.IOWait),
				float64(p.IRQ),
				float64(p.SoftIRQ),
				float64(p.Steal),
				float64(p.Guest),
				float64(p.GuestNice),
			)

			// Get idle
			lIdle := float64(l.Idle)
			pIdle := float64(p.Idle)

			// Get diff total
			totalDiff := lUsageTotal - pUsageTotal
			idleDiff := lIdle - pIdle

			usage := CPUUsageTop{
				Low:    float64(l.Nice-p.Nice) / totalDiff,
				Normal: float64(l.User-p.User) / totalDiff,
				Kernel: float64(l.System-p.System) / totalDiff,
				Guest:  float64(l.Guest-p.Guest) / totalDiff,
				Total:  float64((totalDiff - idleDiff) / totalDiff),
			}

			usages = append(usages, usage)
		}
	}

	return
}

// GetMemoryUsage is get memory usage. return size is byte.
func (n *Node) GetMemoryUsage() (memUsed, memTotal, swapUsed, swapTotal uint64, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	meminfo, err := n.GetMemInfo()
	if err != nil || meminfo == nil {
		return
	}

	// memory
	memUsed = (meminfo.MemTotal - meminfo.MemFree - meminfo.Buffers - meminfo.Cached) * 1024
	memTotal = (meminfo.MemTotal) * 1024

	// swap
	swapUsed = (meminfo.SwapTotal - meminfo.SwapFree) * 1024
	swapTotal = (meminfo.SwapTotal) * 1024

	return
}

func (n *Node) GetMemInfo() (memInfo *linux.MemInfo, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	defer n.RUnlock()
	if n.snapshot.MemInfo == nil {
		return nil, fmt.Errorf("meminfo cache is not found")
	}
	mem := *n.snapshot.MemInfo
	memInfo = &mem
	return
}

func (n *Node) GetKernelVersion() (version string, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	version = n.snapshot.KernelVersion
	n.RUnlock()
	if version == "" {
		err = fmt.Errorf("kernel version cache is not found")
	}
	return
}

func (n *Node) GetUptime() (uptime *linux.Uptime, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	defer n.RUnlock()
	if n.snapshot.Uptime == nil {
		return nil, fmt.Errorf("uptime cache is not found")
	}
	up := *n.snapshot.Uptime
	uptime = &up
	return
}

func (n *Node) GetTaskCounts() (tasks uint64, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	tasks = n.snapshot.TaskCount
	n.RUnlock()
	return
}

func (n *Node) GetLoadAvg() (loadavg *linux.LoadAvg, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	defer n.RUnlock()
	if n.snapshot.LoadAvg == nil {
		return nil, fmt.Errorf("loadavg cache is not found")
	}
	avg := *n.snapshot.LoadAvg
	loadavg = &avg
	return
}

func (n *Node) GetDiskUsage() (diskUsages []*DiskUsage, err error) {
	if n.con.Connect == nil {
		return
	}

	if !n.CheckClientAlive() {
		return
	}

	n.RLock()
	diskUsages = cloneDiskUsages(n.snapshot.DiskUsages)
	n.RUnlock()
	return
}

func (n *Node) GetNetworkUsage() (networkUsages []*NetworkUsage, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	networkUsages = cloneNetworkUsages(n.snapshot.NetworkUsages)
	n.RUnlock()
	return
}

func (n *Node) GetIPv4() (ipv4 []sshproc.IPv4, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	ipv4 = append([]sshproc.IPv4(nil), n.snapshot.IPv4...)
	n.RUnlock()
	return
}

func (n *Node) GetIPV6() (ipv6 []sshproc.IPv6, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	ipv6 = append([]sshproc.IPv6(nil), n.snapshot.IPv6...)
	n.RUnlock()
	return
}

func (n *Node) GetPressureCPU() (pressure *sshproc.PressureStat, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	defer n.RUnlock()
	if n.snapshot.PressureCPU == nil {
		return nil, fmt.Errorf("cpu pressure cache is not found")
	}
	value := *n.snapshot.PressureCPU
	pressure = &value
	return
}

func (n *Node) GetPressureMem() (pressure *sshproc.PressureStat, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	defer n.RUnlock()
	if n.snapshot.PressureMem == nil {
		return nil, fmt.Errorf("memory pressure cache is not found")
	}
	value := *n.snapshot.PressureMem
	pressure = &value
	return
}

func (n *Node) GetPressureIO() (pressure *sshproc.PressureStat, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	defer n.RUnlock()
	if n.snapshot.PressureIO == nil {
		return nil, fmt.Errorf("io pressure cache is not found")
	}
	value := *n.snapshot.PressureIO
	pressure = &value
	return
}

func (n *Node) GetFileNr() (fileNr *sshproc.FileNr, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	defer n.RUnlock()
	if n.snapshot.FileNr == nil {
		return nil, fmt.Errorf("file-nr cache is not found")
	}
	value := *n.snapshot.FileNr
	fileNr = &value
	return
}

func (n *Node) GetTCPStates() (states sshproc.SocketStateCount, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	states = cloneSocketStateCount(n.snapshot.TCPStates)
	n.RUnlock()
	if len(states) == 0 {
		err = fmt.Errorf("tcp state cache is not found")
	}
	return
}

func (n *Node) GetUDPStates() (states sshproc.SocketStateCount, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	states = cloneSocketStateCount(n.snapshot.UDPStates)
	n.RUnlock()
	if len(states) == 0 {
		err = fmt.Errorf("udp state cache is not found")
	}
	return
}

func (n *Node) GetPressureHistory() (cpu, mem, io []float64, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	cpu = append([]float64(nil), n.PressureCPUHistory...)
	mem = append([]float64(nil), n.PressureMemHistory...)
	io = append([]float64(nil), n.PressureIOHistory...)
	n.RUnlock()
	if len(cpu) == 0 && len(mem) == 0 && len(io) == 0 {
		err = fmt.Errorf("pressure history is not found")
	}
	return
}

func (n *Node) GetCPUPressureHistory() (some10, some60, some300 []float64, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	some10 = append([]float64(nil), n.PressureCPUHistory...)
	some60 = append([]float64(nil), n.PressureCPU60History...)
	some300 = append([]float64(nil), n.PressureCPU300History...)
	n.RUnlock()
	if len(some10) == 0 && len(some60) == 0 && len(some300) == 0 {
		err = fmt.Errorf("cpu pressure history is not found")
	}
	return
}

func (n *Node) GetMemPressureHistory() (some10, some60 []float64, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	some10 = append([]float64(nil), n.PressureMemHistory...)
	some60 = append([]float64(nil), n.PressureMem60History...)
	n.RUnlock()
	if len(some10) == 0 && len(some60) == 0 {
		err = fmt.Errorf("memory pressure history is not found")
	}
	return
}

func (n *Node) GetIOPressureHistory() (some10, some60 []float64, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	some10 = append([]float64(nil), n.PressureIOHistory...)
	some60 = append([]float64(nil), n.PressureIO60History...)
	n.RUnlock()
	if len(some10) == 0 && len(some60) == 0 {
		err = fmt.Errorf("io pressure history is not found")
	}
	return
}

func (n *Node) GetFileNrHistory() (alloc []uint64, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	alloc = append([]uint64(nil), n.FileNrAllocHistory...)
	n.RUnlock()
	if len(alloc) == 0 {
		err = fmt.Errorf("file-nr history is not found")
	}
	return
}

func (n *Node) GetTCPStateHistory() (est, tw, listen []uint64, err error) {
	if !n.CheckClientAlive() {
		err = fmt.Errorf("Node is not connected")
		return
	}

	n.RLock()
	est = append([]uint64(nil), n.TCPEstHistory...)
	tw = append([]uint64(nil), n.TCPTWHistory...)
	listen = append([]uint64(nil), n.TCPListenHistory...)
	n.RUnlock()
	if len(est) == 0 && len(tw) == 0 && len(listen) == 0 {
		err = fmt.Errorf("tcp state history is not found")
	}
	return
}

func (n *Node) MonitoringCPUUsage() {
	if !n.CheckClientAlive() {
		// reset cpuUsage
		n.Lock()
		n.cpuUsage = []CPUUsage{}
		n.Unlock()

		return
	}

	timestamp := time.Now()
	stat, err := n.con.ReadStat(n.PathProcStat)
	if err != nil {
		return
	}
	stats := stat.CPUStats

	cpuUsage := CPUUsage{
		stat.CPUStatAll,
		stats,
		timestamp,
	}

	n.Lock()
	n.cpuUsage = append(n.cpuUsage, cpuUsage)
	if len(n.cpuUsage) > n.cpuUsageLimit {
		n.cpuUsage = n.cpuUsage[1:]
	}
	n.Unlock()
}

func (n *Node) MonitoringDiskIO() {
	if !n.CheckClientAlive() {
		n.Lock()
		n.DiskIOs = map[string][]*DiskIO{}
		n.Unlock()
		return
	}

	// Get Disk stats
	stats, err := n.con.ReadDiskStats(n.PathProcDiskStats)
	if err != nil {
		return
	}

	// Get Disk IO
	for _, stat := range stats {
		device := fmt.Sprintf("/dev/%s", stat.Name)

		// Resolve device-mapper names such as dm-0 to /dev/mapper/<name>.
		if strings.HasPrefix(stat.Name, "dm-") {
			sysDeviceName := fmt.Sprintf("/sys/block/%s/dm/name", stat.Name)
			mapperDeviceName, err := n.con.ReadData(sysDeviceName)
			if err != nil {
				continue
			}

			// overwrite device
			device = filepath.Join("/dev/mapper", strings.TrimSpace(mapperDeviceName))
		}

		// Get Disk IO
		diskIO := DiskIO{
			Device:     device,
			ReadIOs:    stat.ReadIOs,
			ReadBytes:  stat.GetReadBytes(),
			WriteIOs:   stat.WriteIOs,
			WriteBytes: stat.GetWriteBytes(),
		}

		n.Lock()
		n.DiskIOs[device] = append(n.DiskIOs[device], &diskIO)
		if len(n.DiskIOs[device]) > n.DiskIOsLimit {
			n.DiskIOs[device] = n.DiskIOs[device][1:]
		}
		n.Unlock()
	}
}

func (n *Node) MonitoringNetworkIO() (err error) {
	if !n.CheckClientAlive() {
		n.Lock()
		n.NetworkIOs = map[string][]*NetworkIO{}
		n.Unlock()
		return
	}

	// Get Network stats
	stats, err := n.con.ReadNetworkStat("/proc/net/dev")
	if err != nil {
		return
	}

	// Get Network IO
	for _, stat := range stats {
		networkIO := NetworkIO{
			Device:    stat.Iface,
			RXPackets: stat.RxPackets,
			RXBytes:   stat.RxBytes,
			TXPackets: stat.TxPackets,
			TXBytes:   stat.TxBytes,
		}

		n.Lock()
		n.NetworkIOs[stat.Iface] = append(n.NetworkIOs[stat.Iface], &networkIO)
		if len(n.NetworkIOs[stat.Iface]) > n.NetworkIOsLimit {
			n.NetworkIOs[stat.Iface] = n.NetworkIOs[stat.Iface][1:]
		}
		n.Unlock()
	}
	return
}

func (n *Node) StartMonitoring() {
	// StartMonitoring is a long-lived reader loop. It does not reconnect by
	// itself; it observes the current transport and waits for Monitor to
	// refresh n.con via Connect().
	ticker := time.NewTicker(monitorSampleInterval)
	defer ticker.Stop()
	tickCount := 0

	for range ticker.C {
		if !n.CheckClientAlive() {
			n.Lock()
			n.cpuUsage = []CPUUsage{}
			n.DiskIOs = map[string][]*DiskIO{}
			n.NetworkIOs = map[string][]*NetworkIO{}
			n.DiskReadIOBytes = map[string][]int64{}
			n.DiskWriteIOBytes = map[string][]int64{}
			n.NetworkRXBytes = map[string][]uint64{}
			n.NetworkRXPackets = map[string][]uint64{}
			n.NetworkTXBytes = map[string][]uint64{}
			n.NetworkTXPackets = map[string][]uint64{}
			n.snapshot.MemInfo = nil
			n.snapshot.Uptime = nil
			n.snapshot.LoadAvg = nil
			n.snapshot.DiskUsages = nil
			n.snapshot.NetworkUsages = nil
			n.snapshot.PressureCPU = nil
			n.snapshot.PressureMem = nil
			n.snapshot.PressureIO = nil
			n.snapshot.FileNr = nil
			n.snapshot.TCPStates = nil
			n.snapshot.UDPStates = nil
			n.PressureCPUHistory = nil
			n.PressureCPU60History = nil
			n.PressureCPU300History = nil
			n.PressureMemHistory = nil
			n.PressureMem60History = nil
			n.PressureIOHistory = nil
			n.PressureIO60History = nil
			n.FileNrAllocHistory = nil
			n.TCPEstHistory = nil
			n.TCPTWHistory = nil
			n.TCPListenHistory = nil
			n.Unlock()
			continue
		}

		n.MonitoringCPUUsage()
		n.MonitoringDiskIO()
		n.MonitoringNetworkIO()
		n.refreshFastSnapshot()

		if tickCount%5 == 0 {
			n.refreshMediumSnapshot()
		}
		if tickCount%15 == 0 {
			n.refreshSlowSnapshot()
		}
		tickCount++
	}
}

func (n *Node) refreshFastSnapshot() {
	meminfo, err := n.con.ReadMemInfo(n.PathProcMeminfo)
	if err == nil {
		n.Lock()
		n.snapshot.MemInfo = meminfo
		n.Unlock()
	}

	uptime, err := n.con.ReadUptime(n.PathProcUptime)
	if err == nil {
		n.Lock()
		n.snapshot.Uptime = uptime
		n.Unlock()
	}

	loadavg, err := n.con.ReadLoadAvg(n.PathProcLoadavg)
	if err == nil {
		n.Lock()
		n.snapshot.LoadAvg = loadavg
		n.Unlock()
	}

	if diskUsages, err := n.buildDiskUsageSnapshot(); err == nil {
		n.Lock()
		n.snapshot.DiskUsages = diskUsages
		n.Unlock()
	}

	if networkUsages, err := n.buildNetworkUsageSnapshot(); err == nil {
		n.Lock()
		n.snapshot.NetworkUsages = networkUsages
		n.Unlock()
	}

	if pressureCPU, err := n.con.ReadPressure("/proc/pressure/cpu"); err == nil {
		n.Lock()
		n.snapshot.PressureCPU = pressureCPU
		if pressureCPU != nil && pressureCPU.Some != nil {
			n.PressureCPUHistory = appendFloat64History(n.PressureCPUHistory, pressureCPU.Some.Avg10, n.PressureHistoryLimit)
			n.PressureCPU60History = appendFloat64History(n.PressureCPU60History, pressureCPU.Some.Avg60, n.PressureHistoryLimit)
			n.PressureCPU300History = appendFloat64History(n.PressureCPU300History, pressureCPU.Some.Avg300, n.PressureHistoryLimit)
		}
		n.Unlock()
	}

	if pressureMem, err := n.con.ReadPressure("/proc/pressure/memory"); err == nil {
		n.Lock()
		n.snapshot.PressureMem = pressureMem
		if pressureMem != nil && pressureMem.Some != nil {
			n.PressureMemHistory = appendFloat64History(n.PressureMemHistory, pressureMem.Some.Avg10, n.PressureHistoryLimit)
			n.PressureMem60History = appendFloat64History(n.PressureMem60History, pressureMem.Some.Avg60, n.PressureHistoryLimit)
		}
		n.Unlock()
	}

	if pressureIO, err := n.con.ReadPressure("/proc/pressure/io"); err == nil {
		n.Lock()
		n.snapshot.PressureIO = pressureIO
		if pressureIO != nil && pressureIO.Some != nil {
			n.PressureIOHistory = appendFloat64History(n.PressureIOHistory, pressureIO.Some.Avg10, n.PressureHistoryLimit)
			n.PressureIO60History = appendFloat64History(n.PressureIO60History, pressureIO.Some.Avg60, n.PressureHistoryLimit)
		}
		n.Unlock()
	}
}

func (n *Node) refreshMediumSnapshot() {
	processList, err := n.con.ListInPID("/proc")
	if err == nil {
		n.Lock()
		n.snapshot.TaskCount = uint64(len(processList))
		n.Unlock()
	}

	if fileNr, err := n.con.ReadFileNr("/proc/sys/fs/file-nr"); err == nil {
		n.Lock()
		n.snapshot.FileNr = fileNr
		if fileNr != nil {
			n.FileNrAllocHistory = appendUint64History(n.FileNrAllocHistory, fileNr.Allocated, n.FileNrHistoryLimit)
		}
		n.Unlock()
	}

	if tcpStates, err := n.readTCPStateSnapshot(); err == nil {
		n.Lock()
		n.snapshot.TCPStates = tcpStates
		n.TCPEstHistory = appendUint64History(n.TCPEstHistory, tcpStates["ESTABLISHED"], n.SocketHistoryLimit)
		n.TCPTWHistory = appendUint64History(n.TCPTWHistory, tcpStates["TIME_WAIT"], n.SocketHistoryLimit)
		n.TCPListenHistory = appendUint64History(n.TCPListenHistory, tcpStates["LISTEN"], n.SocketHistoryLimit)
		n.Unlock()
	}

	if udpStates, err := n.readUDPStateSnapshot(); err == nil {
		n.Lock()
		n.snapshot.UDPStates = udpStates
		n.Unlock()
	}

}

func (n *Node) refreshSlowSnapshot() {
	cpuinfo, err := n.con.ReadCPUInfo(n.PathProcCpuinfo)
	if err == nil {
		n.Lock()
		n.snapshot.CPUCore = cpuinfo.NumCPU()
		n.Unlock()
	}

	data, err := n.con.ReadData("/proc/version")
	if err == nil {
		versionSlice := strings.Split(data, " ")
		if len(versionSlice) >= 3 {
			n.Lock()
			n.snapshot.KernelVersion = strings.Join(versionSlice[:3], " ")
			n.Unlock()
		}
	}

	ipv4, err := n.con.ReadFibTrie("/proc/net/fib_trie", "/proc/net/route")
	if err == nil {
		n.Lock()
		n.snapshot.IPv4 = append([]sshproc.IPv4(nil), ipv4...)
		n.Unlock()
	}

	ipv6, err := n.con.ReadIfInet6("/proc/net/if_inet6")
	if err == nil {
		n.Lock()
		n.snapshot.IPv6 = append([]sshproc.IPv6(nil), ipv6...)
		n.Unlock()
	}
}

func (n *Node) buildDiskUsageSnapshot() ([]*DiskUsage, error) {
	mounts, err := n.con.ReadMounts(n.PathProcMounts)
	if err != nil {
		return nil, err
	}

	result := []*DiskUsage{}
	for _, m := range mounts.Mounts {
		disk, err := n.con.ReadDisk(m.MountPoint)
		if err != nil {
			continue
		}
		if !fstype[m.FSType] {
			continue
		}

		n.getDiskIOBytes(m.Device)

		n.RLock()
		readIO := append([]int64(nil), n.DiskReadIOBytes[m.Device]...)
		writeIO := append([]int64(nil), n.DiskWriteIOBytes[m.Device]...)
		n.RUnlock()

		result = append(result, &DiskUsage{
			MountPoint:   m.MountPoint,
			FSType:       m.FSType,
			Device:       m.Device,
			All:          disk.All,
			Used:         disk.Used,
			Free:         disk.Free,
			ReadIOBytes:  readIO,
			WriteIOBytes: writeIO,
		})
	}

	return result, nil
}

func (n *Node) readTCPStateSnapshot() (sshproc.SocketStateCount, error) {
	states := sshproc.SocketStateCount{}

	tcp4, err := n.con.ReadNetTCPSockets("/proc/net/tcp", n.con.NetIPv4Decoder)
	if err == nil {
		for state, count := range sshproc.CountTCPSocketStates(tcp4) {
			states[state] += count
		}
	}

	tcp6, err := n.con.ReadNetTCPSockets("/proc/net/tcp6", n.con.NetIPv6Decoder)
	if err == nil {
		for state, count := range sshproc.CountTCPSocketStates(tcp6) {
			states[state] += count
		}
	}

	if len(states) == 0 {
		return nil, fmt.Errorf("tcp state cache is not found")
	}
	return states, nil
}

func (n *Node) readUDPStateSnapshot() (sshproc.SocketStateCount, error) {
	states := sshproc.SocketStateCount{}

	udp4, err := n.con.ReadNetUDPSockets("/proc/net/udp", n.con.NetIPv4Decoder)
	if err == nil {
		for state, count := range sshproc.CountUDPSocketStates(udp4) {
			states[state] += count
		}
	}

	udp6, err := n.con.ReadNetUDPSockets("/proc/net/udp6", n.con.NetIPv6Decoder)
	if err == nil {
		for state, count := range sshproc.CountUDPSocketStates(udp6) {
			states[state] += count
		}
	}

	if len(states) == 0 {
		return nil, fmt.Errorf("udp state cache is not found")
	}
	return states, nil
}

func (n *Node) buildNetworkUsageSnapshot() ([]*NetworkUsage, error) {
	networkIOs := make(map[string][]*NetworkIO)
	n.RLock()
	for device, networkIO := range n.NetworkIOs {
		networkIOs[device] = append([]*NetworkIO(nil), networkIO...)
	}
	ipv4 := append([]sshproc.IPv4(nil), n.snapshot.IPv4...)
	ipv6 := append([]sshproc.IPv6(nil), n.snapshot.IPv6...)
	n.RUnlock()
	if len(networkIOs) == 0 {
		return nil, fmt.Errorf("NetworkIOs is not found")
	}

	result := []*NetworkUsage{}
	for device, networkIO := range networkIOs {
		if len(networkIO) <= 1 {
			continue
		}

		n.getNetworkIO(device)

		n.RLock()
		usage := &NetworkUsage{
			Device:    device,
			RXBytes:   append([]uint64(nil), n.NetworkRXBytes[device]...),
			TXBytes:   append([]uint64(nil), n.NetworkTXBytes[device]...),
			RXPackets: append([]uint64(nil), n.NetworkRXPackets[device]...),
			TXPackets: append([]uint64(nil), n.NetworkTXPackets[device]...),
		}
		n.RUnlock()

		for _, ip := range ipv4 {
			if ip.Interface == usage.Device {
				ipAddress := ip.IPAddress
				netMask, _ := ip.Netmask.Size()
				usage.IPv4Address = fmt.Sprintf("%s/%d", ipAddress, netMask)
				break
			}
		}

		for _, ip := range ipv6 {
			if ip.Interface == usage.Device {
				ipAddress := ip.IPAddress
				prefix := ip.Prefix
				usage.IPv6Address = fmt.Sprintf("%s/%s", ipAddress.String(), prefix)
				break
			}
		}

		result = append(result, usage)
	}

	return result, nil
}

func cloneDiskUsages(src []*DiskUsage) []*DiskUsage {
	if len(src) == 0 {
		return nil
	}
	result := make([]*DiskUsage, 0, len(src))
	for _, disk := range src {
		if disk == nil {
			continue
		}
		cp := *disk
		cp.ReadIOBytes = append([]int64(nil), disk.ReadIOBytes...)
		cp.WriteIOBytes = append([]int64(nil), disk.WriteIOBytes...)
		result = append(result, &cp)
	}
	return result
}

func cloneNetworkUsages(src []*NetworkUsage) []*NetworkUsage {
	if len(src) == 0 {
		return nil
	}
	result := make([]*NetworkUsage, 0, len(src))
	for _, usage := range src {
		if usage == nil {
			continue
		}
		cp := *usage
		cp.RXBytes = append([]uint64(nil), usage.RXBytes...)
		cp.TXBytes = append([]uint64(nil), usage.TXBytes...)
		cp.RXPackets = append([]uint64(nil), usage.RXPackets...)
		cp.TXPackets = append([]uint64(nil), usage.TXPackets...)
		result = append(result, &cp)
	}
	return result
}

func (n *Node) getDiskIOBytes(device string) {
	n.Lock()
	defer n.Unlock()

	if len(device) == 0 {
		return
	}

	diskIO := n.DiskIOs[device]
	if len(diskIO) > 1 {
		var readIOBytes int64
		var writeIOBytes int64

		preReadIOBytes := diskIO[len(diskIO)-2].ReadBytes
		preWriteIOBytes := diskIO[len(diskIO)-2].WriteBytes

		readIOBytes = diskIO[len(diskIO)-1].ReadBytes - preReadIOBytes
		writeIOBytes = diskIO[len(diskIO)-1].WriteBytes - preWriteIOBytes

		if _, ok := n.DiskReadIOBytes[device]; !ok {
			n.DiskReadIOBytes[device] = append(n.DiskReadIOBytes[device], 0)
		} else {
			n.DiskReadIOBytes[device] = append(n.DiskReadIOBytes[device], readIOBytes)
		}

		if _, ok := n.DiskWriteIOBytes[device]; !ok {
			n.DiskWriteIOBytes[device] = append(n.DiskWriteIOBytes[device], 0)
		} else {
			n.DiskWriteIOBytes[device] = append(n.DiskWriteIOBytes[device], writeIOBytes)
		}
	}

	if len(n.DiskReadIOBytes[device]) > n.DiskIOsLimit {
		n.DiskReadIOBytes[device] = n.DiskReadIOBytes[device][1:]
	}

	if len(n.DiskWriteIOBytes[device]) > n.DiskIOsLimit {
		n.DiskWriteIOBytes[device] = n.DiskWriteIOBytes[device][1:]
	}

	return
}

func (n *Node) getNetworkIO(device string) {
	n.Lock()
	defer n.Unlock()

	if len(device) == 0 {
		return
	}

	networkIO := n.NetworkIOs[device]
	if len(networkIO) > 1 {
		var rxBytes uint64
		var txBytes uint64
		var rxPackets uint64
		var txPackets uint64

		preRXBytes := networkIO[len(networkIO)-2].RXBytes
		preRXPackets := networkIO[len(networkIO)-2].RXPackets
		preTXBytes := networkIO[len(networkIO)-2].TXBytes
		preTXPackets := networkIO[len(networkIO)-2].TXPackets

		rxBytes = networkIO[len(networkIO)-1].RXBytes - preRXBytes
		rxPackets = networkIO[len(networkIO)-1].RXPackets - preRXPackets
		txBytes = networkIO[len(networkIO)-1].TXBytes - preTXBytes
		txPackets = networkIO[len(networkIO)-1].TXPackets - preTXPackets

		if _, ok := n.NetworkRXBytes[device]; !ok {
			n.NetworkRXBytes[device] = append(n.NetworkRXBytes[device], 0)
		} else {
			n.NetworkRXBytes[device] = append(n.NetworkRXBytes[device], uint64(rxBytes))
		}

		if _, ok := n.NetworkRXPackets[device]; !ok {
			n.NetworkRXPackets[device] = append(n.NetworkRXPackets[device], 0)
		} else {
			n.NetworkRXPackets[device] = append(n.NetworkRXPackets[device], uint64(rxPackets))
		}

		if _, ok := n.NetworkTXBytes[device]; !ok {
			n.NetworkTXBytes[device] = append(n.NetworkTXBytes[device], 0)
		} else {
			n.NetworkTXBytes[device] = append(n.NetworkTXBytes[device], uint64(txBytes))
		}

		if _, ok := n.NetworkTXPackets[device]; !ok {
			n.NetworkTXPackets[device] = append(n.NetworkTXPackets[device], 0)
		} else {
			n.NetworkTXPackets[device] = append(n.NetworkTXPackets[device], uint64(txPackets))
		}

		if len(n.NetworkRXBytes[device]) > n.NetworkIOsLimit {
			n.NetworkRXBytes[device] = n.NetworkRXBytes[device][1:]
		}

		if len(n.NetworkTXBytes[device]) > n.NetworkIOsLimit {
			n.NetworkTXBytes[device] = n.NetworkTXBytes[device][1:]
		}

		if len(n.NetworkRXPackets[device]) > n.NetworkIOsLimit {
			n.NetworkRXPackets[device] = n.NetworkRXPackets[device][1:]
		}

		if len(n.NetworkTXPackets[device]) > n.NetworkIOsLimit {
			n.NetworkTXPackets[device] = n.NetworkTXPackets[device][1:]
		}

	}

	return
}
