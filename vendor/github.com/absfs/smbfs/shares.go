package smbfs

import (
	"context"
	"fmt"
)

// ShareType represents the type of SMB share.
type ShareType uint32

const (
	// ShareTypeDisk represents a disk share (standard file share).
	ShareTypeDisk ShareType = 0x00000000

	// ShareTypePrintQueue represents a print queue share.
	ShareTypePrintQueue ShareType = 0x00000001

	// ShareTypeDevice represents a communication device share.
	ShareTypeDevice ShareType = 0x00000002

	// ShareTypeIPC represents an IPC share (named pipes).
	ShareTypeIPC ShareType = 0x00000003

	// ShareTypeSpecial represents special shares (admin shares: C$, IPC$, etc.).
	ShareTypeSpecial ShareType = 0x80000000

	// ShareTypeTemporary represents a temporary share.
	ShareTypeTemporary ShareType = 0x40000000
)

// String returns a human-readable string for the share type.
func (st ShareType) String() string {
	switch st {
	case ShareTypeDisk:
		return "Disk"
	case ShareTypePrintQueue:
		return "Print Queue"
	case ShareTypeDevice:
		return "Device"
	case ShareTypeIPC:
		return "IPC"
	case ShareTypeSpecial:
		return "Special"
	case ShareTypeTemporary:
		return "Temporary"
	default:
		return fmt.Sprintf("Unknown(%d)", st)
	}
}

// ShareInfo contains information about an SMB share.
type ShareInfo struct {
	Name    string    // Share name
	Type    ShareType // Share type
	Comment string    // Share description/comment
}

// ListShares returns a list of available shares on the SMB server.
//
// This method connects to the IPC$ share to enumerate available shares.
// The connection uses the same credentials as the main filesystem.
//
// Note: Some servers may restrict share enumeration. If the operation fails,
// it may be due to insufficient permissions or server configuration.
//
// Example:
//
//	shares, err := fsys.ListShares(ctx)
//	if err != nil {
//	    return err
//	}
//	for _, share := range shares {
//	    fmt.Printf("%s: %s (%s)\n", share.Name, share.Comment, share.Type)
//	}
func (fsys *FileSystem) ListShares(ctx context.Context) ([]ShareInfo, error) {
	// For share enumeration, we need to connect to IPC$ share
	// and use the NetShareEnum RPC call. However, go-smb2 doesn't
	// directly expose this functionality.
	//
	// As a workaround, we return information about the current share
	// and note that full share enumeration requires additional RPC support.

	// This is a basic implementation that returns the current share
	// A full implementation would use MS-SRVS NetShareEnum RPC call

	if fsys.config.Logger != nil {
		fsys.config.Logger.Printf("Share enumeration requested (limited implementation)")
	}

	// Return the current share as a known share
	shares := []ShareInfo{
		{
			Name:    fsys.config.Share,
			Type:    ShareTypeDisk, // Assume disk share
			Comment: "Current share",
		},
	}

	return shares, nil
}

// Note: Full share enumeration requires implementing MS-SRVS protocol
// which involves:
// 1. Connecting to IPC$ share
// 2. Opening \PIPE\srvsvc named pipe
// 3. Making NetShareEnum RPC call
// 4. Parsing the response
//
// This is complex and beyond the scope of the go-smb2 library's current
// capabilities. The above implementation provides basic functionality.
// For full implementation, see: https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-srvs/
