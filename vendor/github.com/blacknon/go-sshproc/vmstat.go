// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshproc

import (
	"io"
	"strconv"
	"strings"

	proc "github.com/c9s/goprocinfo/linux"
)

func (p *ConnectWithProc) ReadVMStat(path string) (*proc.VMStat, error) {
	file, err := p.sftp.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	content := string(b)
	lines := strings.Split(content, "\n")
	vmstat := proc.VMStat{}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := fields[0]
		value, _ := strconv.ParseUint(fields[1], 10, 64)
		switch name {
		case "nr_free_pages":
			vmstat.NrFreePages = value
		case "nr_alloc_batch":
			vmstat.NrAllocBatch = value
		case "nr_inactive_anon":
			vmstat.NrInactiveAnon = value
		case "nr_active_anon":
			vmstat.NrActiveAnon = value
		case "nr_inactive_file":
			vmstat.NrInactiveFile = value
		case "nr_active_file":
			vmstat.NrActiveFile = value
		case "nr_unevictable":
			vmstat.NrUnevictable = value
		case "nr_mlock":
			vmstat.NrMlock = value
		case "nr_anon_pages":
			vmstat.NrAnonPages = value
		case "nr_mapped":
			vmstat.NrMapped = value
		case "nr_file_pages":
			vmstat.NrFilePages = value
		case "nr_dirty":
			vmstat.NrDirty = value
		case "nr_writeback":
			vmstat.NrWriteback = value
		case "nr_slab_reclaimable":
			vmstat.NrSlabReclaimable = value
		case "nr_slab_unreclaimable":
			vmstat.NrSlabUnreclaimable = value
		case "nr_page_table_pages":
			vmstat.NrPageTablePages = value
		case "nr_kernel_stack":
			vmstat.NrKernelStack = value
		case "nr_unstable":
			vmstat.NrUnstable = value
		case "nr_bounce":
			vmstat.NrBounce = value
		case "nr_vmscan_write":
			vmstat.NrVmscanWrite = value
		case "nr_vmscan_immediate_reclaim":
			vmstat.NrVmscanImmediateReclaim = value
		case "nr_writeback_temp":
			vmstat.NrWritebackTemp = value
		case "nr_isolated_anon":
			vmstat.NrIsolatedAnon = value
		case "nr_isolated_file":
			vmstat.NrIsolatedFile = value
		case "nr_shmem":
			vmstat.NrShmem = value
		case "nr_dirtied":
			vmstat.NrDirtied = value
		case "nr_written":
			vmstat.NrWritten = value
		case "numa_hit":
			vmstat.NumaHit = value
		case "numa_miss":
			vmstat.NumaMiss = value
		case "numa_foreign":
			vmstat.NumaForeign = value
		case "numa_interleave":
			vmstat.NumaInterleave = value
		case "numa_local":
			vmstat.NumaLocal = value
		case "numa_other":
			vmstat.NumaOther = value
		case "workingset_refault":
			vmstat.WorkingsetRefault = value
		case "workingset_activate":
			vmstat.WorkingsetActivate = value
		case "workingset_nodereclaim":
			vmstat.WorkingsetNodereclaim = value
		case "nr_anon_transparent_hugepages":
			vmstat.NrAnonTransparentHugepages = value
		case "nr_free_cma":
			vmstat.NrFreeCma = value
		case "nr_dirty_threshold":
			vmstat.NrDirtyThreshold = value
		case "nr_dirty_background_threshold":
			vmstat.NrDirtyBackgroundThreshold = value
		case "pgpgin":
			vmstat.PagePagein = value
		case "pgpgout":
			vmstat.PagePageout = value
		case "pswpin":
			vmstat.PageSwapin = value
		case "pswpout":
			vmstat.PageSwapout = value
		case "pgalloc_dma":
			vmstat.PageAllocDMA = value
		case "pgalloc_dma32":
			vmstat.PageAllocDMA32 = value
		case "pgalloc_normal":
			vmstat.PageAllocNormal = value
		case "pgalloc_movable":
			vmstat.PageAllocMovable = value
		case "pgfree":
			vmstat.PageFree = value
		case "pgactivate":
			vmstat.PageActivate = value
		case "pgdeactivate":
			vmstat.PageDeactivate = value
		case "pgfault":
			vmstat.PageFault = value
		case "pgmajfault":
			vmstat.PageMajorFault = value
		case "pgrefill_dma":
			vmstat.PageRefillDMA = value
		case "pgrefill_dma32":
			vmstat.PageRefillDMA32 = value
		case "pgrefill_normal":
			vmstat.PageRefillMormal = value
		case "pgrefill_movable":
			vmstat.PageRefillMovable = value
		case "pgsteal_kswapd_dma":
			vmstat.PageStealKswapdDMA = value
		case "pgsteal_kswapd_dma32":
			vmstat.PageStealKswapdDMA32 = value
		case "pgsteal_kswapd_normal":
			vmstat.PageStealKswapdNormal = value
		case "pgsteal_kswapd_movable":
			vmstat.PageStealKswapdMovable = value
		case "pgsteal_direct_dma":
			vmstat.PageStealDirectDMA = value
		case "pgsteal_direct_dma32":
			vmstat.PageStealDirectDMA32 = value
		case "pgsteal_direct_normal":
			vmstat.PageStealDirectNormal = value
		case "pgsteal_direct_movable":
			vmstat.PageStealDirectMovable = value
		case "pgscan_kswapd_dma":
			vmstat.PageScanKswapdDMA = value
		case "pgscan_kswapd_dma32":
			vmstat.PageScanKswapdDMA32 = value
		case "pgscan_kswapd_normal":
			vmstat.PageScanKswapdNormal = value
		case "pgscan_kswapd_movable":
			vmstat.PageScanKswapdMovable = value
		case "pgscan_direct_dma":
			vmstat.PageScanDirectDMA = value
		case "pgscan_direct_dma32":
			vmstat.PageScanDirectDMA32 = value
		case "pgscan_direct_normal":
			vmstat.PageScanDirectNormal = value
		case "pgscan_direct_movable":
			vmstat.PageScanDirectMovable = value
		case "pgscan_direct_throttle":
			vmstat.PageScanDirectThrottle = value
		case "zone_reclaim_failed":
			vmstat.ZoneReclaimFailed = value
		case "pginodesteal":
			vmstat.PageInodeSteal = value
		case "slabs_scanned":
			vmstat.SlabsScanned = value
		case "kswapd_inodesteal":
			vmstat.KswapdInodesteal = value
		case "kswapd_low_wmark_hit_quickly":
			vmstat.KswapdLowWatermarkHitQuickly = value
		case "kswapd_high_wmark_hit_quickly":
			vmstat.KswapdHighWatermarkHitQuickly = value
		case "pageoutrun":
			vmstat.PageoutRun = value
		case "allocstall":
			vmstat.AllocStall = value
		case "pgrotated":
			vmstat.PageRotated = value
		case "drop_pagecache":
			vmstat.DropPagecache = value
		case "drop_slab":
			vmstat.DropSlab = value
		case "numa_pte_updates":
			vmstat.NumaPteUpdates = value
		case "numa_huge_pte_updates":
			vmstat.NumaHugePteUpdates = value
		case "numa_hint_faults":
			vmstat.NumaHintFaults = value
		case "numa_hint_faults_local":
			vmstat.NumaHintFaults_local = value
		case "numa_pages_migrated":
			vmstat.NumaPagesMigrated = value
		case "pgmigrate_success":
			vmstat.PageMigrateSuccess = value
		case "pgmigrate_fail":
			vmstat.PageMigrateFail = value
		case "compact_migrate_scanned":
			vmstat.CompactMigrateScanned = value
		case "compact_free_scanned":
			vmstat.CompactFreeScanned = value
		case "compact_isolated":
			vmstat.CompactIsolated = value
		case "compact_stall":
			vmstat.CompactStall = value
		case "compact_fail":
			vmstat.CompactFail = value
		case "compact_success":
			vmstat.CompactSuccess = value
		case "htlb_buddy_alloc_success":
			vmstat.HtlbBuddyAllocSuccess = value
		case "htlb_buddy_alloc_fail":
			vmstat.HtlbBuddyAllocFail = value
		case "unevictable_pgs_culled":
			vmstat.UnevictablePagesCulled = value
		case "unevictable_pgs_scanned":
			vmstat.UnevictablePagesScanned = value
		case "unevictable_pgs_rescued":
			vmstat.UnevictablePagesRescued = value
		case "unevictable_pgs_mlocked":
			vmstat.UnevictablePagesMlocked = value
		case "unevictable_pgs_munlocked":
			vmstat.UnevictablePagesMunlocked = value
		case "unevictable_pgs_cleared":
			vmstat.UnevictablePagesCleared = value
		case "unevictable_pgs_stranded":
			vmstat.UnevictablePagesStranded = value
		case "thp_fault_alloc":
			vmstat.THPFaultAlloc = value
		case "thp_fault_fallback":
			vmstat.THPFaultFallback = value
		case "thp_collapse_alloc":
			vmstat.THPCollapseAlloc = value
		case "thp_collapse_alloc_failed":
			vmstat.THPCollapseAllocFailed = value
		case "thp_split":
			vmstat.THPSplit = value
		case "thp_zero_page_alloc":
			vmstat.THPZeroPageAlloc = value
		case "thp_zero_page_alloc_failed":
			vmstat.THPZeroPageAllocFailed = value
		}
	}
	return &vmstat, nil
}
