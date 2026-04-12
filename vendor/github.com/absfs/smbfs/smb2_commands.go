package smbfs

// SMB2 Command opcodes
const (
	SMB2_NEGOTIATE       uint16 = 0x0000
	SMB2_SESSION_SETUP   uint16 = 0x0001
	SMB2_LOGOFF          uint16 = 0x0002
	SMB2_TREE_CONNECT    uint16 = 0x0003
	SMB2_TREE_DISCONNECT uint16 = 0x0004
	SMB2_CREATE          uint16 = 0x0005
	SMB2_CLOSE           uint16 = 0x0006
	SMB2_FLUSH           uint16 = 0x0007
	SMB2_READ            uint16 = 0x0008
	SMB2_WRITE           uint16 = 0x0009
	SMB2_LOCK            uint16 = 0x000A
	SMB2_IOCTL           uint16 = 0x000B
	SMB2_CANCEL          uint16 = 0x000C
	SMB2_ECHO            uint16 = 0x000D
	SMB2_QUERY_DIRECTORY uint16 = 0x000E
	SMB2_CHANGE_NOTIFY   uint16 = 0x000F
	SMB2_QUERY_INFO      uint16 = 0x0010
	SMB2_SET_INFO        uint16 = 0x0011
	SMB2_OPLOCK_BREAK    uint16 = 0x0012
)

// CommandName returns the human-readable name for an SMB2 command
func CommandName(cmd uint16) string {
	switch cmd {
	case SMB2_NEGOTIATE:
		return "NEGOTIATE"
	case SMB2_SESSION_SETUP:
		return "SESSION_SETUP"
	case SMB2_LOGOFF:
		return "LOGOFF"
	case SMB2_TREE_CONNECT:
		return "TREE_CONNECT"
	case SMB2_TREE_DISCONNECT:
		return "TREE_DISCONNECT"
	case SMB2_CREATE:
		return "CREATE"
	case SMB2_CLOSE:
		return "CLOSE"
	case SMB2_FLUSH:
		return "FLUSH"
	case SMB2_READ:
		return "READ"
	case SMB2_WRITE:
		return "WRITE"
	case SMB2_LOCK:
		return "LOCK"
	case SMB2_IOCTL:
		return "IOCTL"
	case SMB2_CANCEL:
		return "CANCEL"
	case SMB2_ECHO:
		return "ECHO"
	case SMB2_QUERY_DIRECTORY:
		return "QUERY_DIRECTORY"
	case SMB2_CHANGE_NOTIFY:
		return "CHANGE_NOTIFY"
	case SMB2_QUERY_INFO:
		return "QUERY_INFO"
	case SMB2_SET_INFO:
		return "SET_INFO"
	case SMB2_OPLOCK_BREAK:
		return "OPLOCK_BREAK"
	default:
		return "UNKNOWN"
	}
}

// IsValidCommand returns true if the command is a valid SMB2 command
func IsValidCommand(cmd uint16) bool {
	return cmd <= SMB2_OPLOCK_BREAK
}
