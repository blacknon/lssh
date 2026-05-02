package monitor

import "testing"

func TestGetDiskIOBytesStoresHistoryPerDevice(t *testing.T) {
	t.Parallel()

	node := NewNode("test")
	node.DiskIOs["/dev/sda"] = []*DiskIO{
		{Device: "/dev/sda", ReadBytes: 100, WriteBytes: 200},
		{Device: "/dev/sda", ReadBytes: 150, WriteBytes: 260},
	}
	node.DiskIOs["/dev/sdb"] = []*DiskIO{
		{Device: "/dev/sdb", ReadBytes: 10, WriteBytes: 20},
		{Device: "/dev/sdb", ReadBytes: 40, WriteBytes: 80},
	}
	node.DiskReadIOBytes["/dev/sda"] = []int64{0}
	node.DiskWriteIOBytes["/dev/sda"] = []int64{0}
	node.DiskReadIOBytes["/dev/sdb"] = []int64{0}
	node.DiskWriteIOBytes["/dev/sdb"] = []int64{0}

	node.getDiskIOBytes("/dev/sda")
	node.getDiskIOBytes("/dev/sdb")

	if got := node.DiskReadIOBytes["/dev/sda"][1]; got != 50 {
		t.Fatalf("sda read delta = %d, want 50", got)
	}
	if got := node.DiskWriteIOBytes["/dev/sda"][1]; got != 60 {
		t.Fatalf("sda write delta = %d, want 60", got)
	}
	if got := node.DiskReadIOBytes["/dev/sdb"][1]; got != 30 {
		t.Fatalf("sdb read delta = %d, want 30", got)
	}
	if got := node.DiskWriteIOBytes["/dev/sdb"][1]; got != 60 {
		t.Fatalf("sdb write delta = %d, want 60", got)
	}
}

func TestGetNetworkIOStoresHistoryPerDeviceAndTrimsAllSeries(t *testing.T) {
	t.Parallel()

	node := NewNode("test")
	node.NetworkIOsLimit = 1
	node.NetworkIOs["eth0"] = []*NetworkIO{
		{Device: "eth0", RXBytes: 100, RXPackets: 10, TXBytes: 200, TXPackets: 20},
		{Device: "eth0", RXBytes: 130, RXPackets: 13, TXBytes: 260, TXPackets: 27},
	}
	node.NetworkIOs["eth1"] = []*NetworkIO{
		{Device: "eth1", RXBytes: 1000, RXPackets: 100, TXBytes: 2000, TXPackets: 200},
		{Device: "eth1", RXBytes: 1100, RXPackets: 110, TXBytes: 2050, TXPackets: 220},
	}
	node.NetworkRXBytes["eth0"] = []uint64{1}
	node.NetworkRXPackets["eth0"] = []uint64{1}
	node.NetworkTXBytes["eth0"] = []uint64{1}
	node.NetworkTXPackets["eth0"] = []uint64{1}
	node.NetworkRXBytes["eth1"] = []uint64{2}
	node.NetworkRXPackets["eth1"] = []uint64{2}
	node.NetworkTXBytes["eth1"] = []uint64{2}
	node.NetworkTXPackets["eth1"] = []uint64{2}

	node.getNetworkIO("eth0")
	node.getNetworkIO("eth1")

	if len(node.NetworkTXPackets["eth0"]) != 1 {
		t.Fatalf("eth0 tx packet history len = %d, want 1", len(node.NetworkTXPackets["eth0"]))
	}
	if len(node.NetworkTXPackets["eth1"]) != 1 {
		t.Fatalf("eth1 tx packet history len = %d, want 1", len(node.NetworkTXPackets["eth1"]))
	}
	if got := node.NetworkTXPackets["eth0"][0]; got != 7 {
		t.Fatalf("eth0 tx packet delta = %d, want 7", got)
	}
	if got := node.NetworkTXPackets["eth1"][0]; got != 20 {
		t.Fatalf("eth1 tx packet delta = %d, want 20", got)
	}
	if got := node.NetworkRXBytes["eth0"][0]; got != 30 {
		t.Fatalf("eth0 rx byte delta = %d, want 30", got)
	}
	if got := node.NetworkRXBytes["eth1"][0]; got != 100 {
		t.Fatalf("eth1 rx byte delta = %d, want 100", got)
	}
}
