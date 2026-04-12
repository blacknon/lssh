package smbfs

import (
	"encoding/binary"
	"time"
)

// SMB2 Protocol constants
const (
	// SMB2 protocol signature
	SMB2ProtocolID = "\xFESMB"

	// SMB2 header size
	SMB2HeaderSize = 64

	// Maximum sizes
	MaxTransactSize = 8 * 1024 * 1024  // 8MB
	MaxReadSize     = 8 * 1024 * 1024  // 8MB
	MaxWriteSize    = 8 * 1024 * 1024  // 8MB
	MaxBufferSize   = 64 * 1024        // 64KB default buffer
)

// SMB2 Dialects
type SMBDialect uint16

const (
	SMB2_0_2 SMBDialect = 0x0202 // Windows Vista/Server 2008
	SMB2_1   SMBDialect = 0x0210 // Windows 7/Server 2008 R2
	SMB3_0   SMBDialect = 0x0300 // Windows 8/Server 2012
	SMB3_0_2 SMBDialect = 0x0302 // Windows 8.1/Server 2012 R2
	SMB3_1_1 SMBDialect = 0x0311 // Windows 10/Server 2016+
)

// String returns the dialect name
func (d SMBDialect) String() string {
	switch d {
	case SMB2_0_2:
		return "SMB 2.0.2"
	case SMB2_1:
		return "SMB 2.1"
	case SMB3_0:
		return "SMB 3.0"
	case SMB3_0_2:
		return "SMB 3.0.2"
	case SMB3_1_1:
		return "SMB 3.1.1"
	default:
		return "Unknown"
	}
}

// SupportedDialects lists dialects we support (highest to lowest preference)
var SupportedDialects = []SMBDialect{
	SMB3_1_1,
	SMB3_0_2,
	SMB3_0,
	SMB2_1,
	SMB2_0_2,
}

// NT Status codes
type NTStatus uint32

const (
	STATUS_SUCCESS                  NTStatus = 0x00000000
	STATUS_PENDING                  NTStatus = 0x00000103
	STATUS_BUFFER_OVERFLOW          NTStatus = 0x80000005
	STATUS_NO_MORE_FILES            NTStatus = 0x80000006
	STATUS_INVALID_PARAMETER        NTStatus = 0xC000000D
	STATUS_NO_SUCH_FILE             NTStatus = 0xC000000F
	STATUS_END_OF_FILE              NTStatus = 0xC0000011
	STATUS_MORE_PROCESSING_REQUIRED NTStatus = 0xC0000016
	STATUS_ACCESS_DENIED            NTStatus = 0xC0000022
	STATUS_OBJECT_NAME_INVALID      NTStatus = 0xC0000033
	STATUS_OBJECT_NAME_NOT_FOUND    NTStatus = 0xC0000034
	STATUS_OBJECT_NAME_COLLISION    NTStatus = 0xC0000035
	STATUS_OBJECT_PATH_NOT_FOUND    NTStatus = 0xC000003A
	STATUS_SHARING_VIOLATION        NTStatus = 0xC0000043
	STATUS_DELETE_PENDING           NTStatus = 0xC0000056
	STATUS_PRIVILEGE_NOT_HELD       NTStatus = 0xC0000061
	STATUS_LOGON_FAILURE            NTStatus = 0xC000006D
	STATUS_ACCOUNT_RESTRICTION      NTStatus = 0xC000006E
	STATUS_PASSWORD_EXPIRED         NTStatus = 0xC0000071
	STATUS_INSUFFICIENT_RESOURCES   NTStatus = 0xC000009A
	STATUS_FILE_IS_A_DIRECTORY      NTStatus = 0xC00000BA
	STATUS_BAD_NETWORK_NAME         NTStatus = 0xC00000CC
	STATUS_NOT_SAME_DEVICE          NTStatus = 0xC00000D4
	STATUS_FILE_RENAMED             NTStatus = 0xC00000D5
	STATUS_NOT_A_DIRECTORY          NTStatus = 0xC0000103
	STATUS_FILE_CLOSED              NTStatus = 0xC0000128
	STATUS_CANCELLED                NTStatus = 0xC0000120
	STATUS_NETWORK_NAME_DELETED     NTStatus = 0xC00000C9
	STATUS_USER_SESSION_DELETED     NTStatus = 0xC0000203
	STATUS_NOT_FOUND                NTStatus = 0xC0000225
	STATUS_INVALID_DEVICE_REQUEST   NTStatus = 0xC0000010
	STATUS_DIRECTORY_NOT_EMPTY      NTStatus = 0xC0000101
	STATUS_NOT_SUPPORTED            NTStatus = 0xC00000BB
)

// IsSuccess returns true if status indicates success
func (s NTStatus) IsSuccess() bool {
	return s == STATUS_SUCCESS
}

// IsError returns true if status indicates an error (high bit set)
func (s NTStatus) IsError() bool {
	return s&0xC0000000 == 0xC0000000
}

// String returns the status name
func (s NTStatus) String() string {
	switch s {
	case STATUS_SUCCESS:
		return "STATUS_SUCCESS"
	case STATUS_PENDING:
		return "STATUS_PENDING"
	case STATUS_BUFFER_OVERFLOW:
		return "STATUS_BUFFER_OVERFLOW"
	case STATUS_NO_MORE_FILES:
		return "STATUS_NO_MORE_FILES"
	case STATUS_INVALID_PARAMETER:
		return "STATUS_INVALID_PARAMETER"
	case STATUS_NO_SUCH_FILE:
		return "STATUS_NO_SUCH_FILE"
	case STATUS_END_OF_FILE:
		return "STATUS_END_OF_FILE"
	case STATUS_MORE_PROCESSING_REQUIRED:
		return "STATUS_MORE_PROCESSING_REQUIRED"
	case STATUS_ACCESS_DENIED:
		return "STATUS_ACCESS_DENIED"
	case STATUS_OBJECT_NAME_INVALID:
		return "STATUS_OBJECT_NAME_INVALID"
	case STATUS_OBJECT_NAME_NOT_FOUND:
		return "STATUS_OBJECT_NAME_NOT_FOUND"
	case STATUS_OBJECT_NAME_COLLISION:
		return "STATUS_OBJECT_NAME_COLLISION"
	case STATUS_OBJECT_PATH_NOT_FOUND:
		return "STATUS_OBJECT_PATH_NOT_FOUND"
	case STATUS_SHARING_VIOLATION:
		return "STATUS_SHARING_VIOLATION"
	case STATUS_LOGON_FAILURE:
		return "STATUS_LOGON_FAILURE"
	case STATUS_FILE_IS_A_DIRECTORY:
		return "STATUS_FILE_IS_A_DIRECTORY"
	case STATUS_BAD_NETWORK_NAME:
		return "STATUS_BAD_NETWORK_NAME"
	case STATUS_NOT_A_DIRECTORY:
		return "STATUS_NOT_A_DIRECTORY"
	case STATUS_FILE_CLOSED:
		return "STATUS_FILE_CLOSED"
	case STATUS_CANCELLED:
		return "STATUS_CANCELLED"
	case STATUS_NOT_FOUND:
		return "STATUS_NOT_FOUND"
	case STATUS_DIRECTORY_NOT_EMPTY:
		return "STATUS_DIRECTORY_NOT_EMPTY"
	case STATUS_NOT_SUPPORTED:
		return "STATUS_NOT_SUPPORTED"
	default:
		return "STATUS_UNKNOWN"
	}
}

// SMB2 Header flags
const (
	SMB2_FLAGS_SERVER_TO_REDIR   uint32 = 0x00000001 // Response flag
	SMB2_FLAGS_ASYNC_COMMAND     uint32 = 0x00000002
	SMB2_FLAGS_RELATED_OPERATIONS uint32 = 0x00000004 // Compound request
	SMB2_FLAGS_SIGNED            uint32 = 0x00000008 // Message is signed
	SMB2_FLAGS_PRIORITY_MASK     uint32 = 0x00000070
	SMB2_FLAGS_DFS_OPERATIONS    uint32 = 0x10000000
	SMB2_FLAGS_REPLAY_OPERATION  uint32 = 0x20000000
)

// SMB2Header represents the fixed 64-byte header for all SMB2/3 messages
type SMB2Header struct {
	ProtocolID     [4]byte  // 0xFE 'S' 'M' 'B'
	StructureSize  uint16   // Always 64
	CreditCharge   uint16   // Number of credits consumed
	Status         NTStatus // NT status code (response) or channel sequence (request)
	Command        uint16   // SMB2 command code
	CreditRequest  uint16   // Credits requested (request) or granted (response)
	Flags          uint32   // Flags
	NextCommand    uint32   // Offset to next command in compound
	MessageID      uint64   // Unique message identifier
	Reserved       uint32   // Reserved (or AsyncID high bits)
	TreeID         uint32   // Tree identifier
	SessionID      uint64   // Session identifier
	Signature      [16]byte // Message signature (if signed)
}

// IsResponse returns true if this is a response message
func (h *SMB2Header) IsResponse() bool {
	return h.Flags&SMB2_FLAGS_SERVER_TO_REDIR != 0
}

// IsSigned returns true if the message is signed
func (h *SMB2Header) IsSigned() bool {
	return h.Flags&SMB2_FLAGS_SIGNED != 0
}

// Marshal encodes the header to bytes
func (h *SMB2Header) Marshal() []byte {
	buf := make([]byte, SMB2HeaderSize)
	copy(buf[0:4], SMB2ProtocolID)
	binary.LittleEndian.PutUint16(buf[4:6], h.StructureSize)
	binary.LittleEndian.PutUint16(buf[6:8], h.CreditCharge)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(h.Status))
	binary.LittleEndian.PutUint16(buf[12:14], h.Command)
	binary.LittleEndian.PutUint16(buf[14:16], h.CreditRequest)
	binary.LittleEndian.PutUint32(buf[16:20], h.Flags)
	binary.LittleEndian.PutUint32(buf[20:24], h.NextCommand)
	binary.LittleEndian.PutUint64(buf[24:32], h.MessageID)
	binary.LittleEndian.PutUint32(buf[32:36], h.Reserved)
	binary.LittleEndian.PutUint32(buf[36:40], h.TreeID)
	binary.LittleEndian.PutUint64(buf[40:48], h.SessionID)
	copy(buf[48:64], h.Signature[:])
	return buf
}

// UnmarshalSMB2Header decodes an SMB2 header from bytes
func UnmarshalSMB2Header(data []byte) (*SMB2Header, error) {
	if len(data) < SMB2HeaderSize {
		return nil, ErrInvalidMessage
	}

	h := &SMB2Header{
		StructureSize: binary.LittleEndian.Uint16(data[4:6]),
		CreditCharge:  binary.LittleEndian.Uint16(data[6:8]),
		Status:        NTStatus(binary.LittleEndian.Uint32(data[8:12])),
		Command:       binary.LittleEndian.Uint16(data[12:14]),
		CreditRequest: binary.LittleEndian.Uint16(data[14:16]),
		Flags:         binary.LittleEndian.Uint32(data[16:20]),
		NextCommand:   binary.LittleEndian.Uint32(data[20:24]),
		MessageID:     binary.LittleEndian.Uint64(data[24:32]),
		Reserved:      binary.LittleEndian.Uint32(data[32:36]),
		TreeID:        binary.LittleEndian.Uint32(data[36:40]),
		SessionID:     binary.LittleEndian.Uint64(data[40:48]),
	}
	copy(h.ProtocolID[:], data[0:4])
	copy(h.Signature[:], data[48:64])
	return h, nil
}

// SMB2Message wraps a header and payload
type SMB2Message struct {
	Header  *SMB2Header
	Payload []byte

	// Raw message bytes (for preauth hash computation)
	RawBytes []byte

	// Signing information (set when message should be signed)
	SigningKey []byte     // Key to use for signing
	Dialect    SMBDialect // Dialect for signing algorithm selection
}

// FileID is a 128-bit SMB2 file identifier
type FileID struct {
	Persistent uint64
	Volatile   uint64
}

// Marshal encodes the FileID to bytes
func (f FileID) Marshal() []byte {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint64(buf[0:8], f.Persistent)
	binary.LittleEndian.PutUint64(buf[8:16], f.Volatile)
	return buf
}

// UnmarshalFileID decodes a FileID from bytes
func UnmarshalFileID(data []byte) FileID {
	return FileID{
		Persistent: binary.LittleEndian.Uint64(data[0:8]),
		Volatile:   binary.LittleEndian.Uint64(data[8:16]),
	}
}

// IsZero returns true if the FileID is zero/invalid
func (f FileID) IsZero() bool {
	return f.Persistent == 0 && f.Volatile == 0
}

// Windows FILETIME helpers
// FILETIME is 100-nanosecond intervals since January 1, 1601 UTC

const (
	// Offset between Unix epoch (1970) and Windows epoch (1601) in 100-ns intervals
	windowsEpochOffset = 116444736000000000
)

// TimeToFiletime converts a Go time.Time to Windows FILETIME
func TimeToFiletime(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}
	// Convert to 100-nanosecond intervals since Unix epoch
	nsec := t.UnixNano()
	// Convert to 100-ns intervals and add Windows epoch offset
	return uint64(nsec/100) + windowsEpochOffset
}

// FiletimeToTime converts a Windows FILETIME to Go time.Time
func FiletimeToTime(ft uint64) time.Time {
	if ft == 0 {
		return time.Time{}
	}
	// Subtract Windows epoch offset and convert from 100-ns intervals
	nsec := int64(ft-windowsEpochOffset) * 100
	return time.Unix(0, nsec)
}

// SMB2 File Attributes are defined in attributes.go

// SMB2 Access Mask (desired access rights)
const (
	FILE_READ_DATA         uint32 = 0x00000001
	FILE_WRITE_DATA        uint32 = 0x00000002
	FILE_APPEND_DATA       uint32 = 0x00000004
	FILE_READ_EA           uint32 = 0x00000008
	FILE_WRITE_EA          uint32 = 0x00000010
	FILE_EXECUTE           uint32 = 0x00000020
	FILE_DELETE_CHILD      uint32 = 0x00000040
	FILE_READ_ATTRIBUTES   uint32 = 0x00000080
	FILE_WRITE_ATTRIBUTES  uint32 = 0x00000100
	DELETE                 uint32 = 0x00010000
	READ_CONTROL           uint32 = 0x00020000
	WRITE_DAC              uint32 = 0x00040000
	WRITE_OWNER            uint32 = 0x00080000
	SYNCHRONIZE            uint32 = 0x00100000
	ACCESS_SYSTEM_SECURITY uint32 = 0x01000000
	MAXIMUM_ALLOWED        uint32 = 0x02000000
	GENERIC_ALL            uint32 = 0x10000000
	GENERIC_EXECUTE        uint32 = 0x20000000
	GENERIC_WRITE          uint32 = 0x40000000
	GENERIC_READ           uint32 = 0x80000000
)

// SMB2 Share Access
const (
	FILE_SHARE_READ   uint32 = 0x00000001
	FILE_SHARE_WRITE  uint32 = 0x00000002
	FILE_SHARE_DELETE uint32 = 0x00000004
)

// SMB2 Create Disposition
const (
	FILE_SUPERSEDE    uint32 = 0x00000000 // If exists, replace; if not, create
	FILE_OPEN         uint32 = 0x00000001 // Open existing file
	FILE_CREATE       uint32 = 0x00000002 // Create new file; fail if exists
	FILE_OPEN_IF      uint32 = 0x00000003 // Open if exists; create if not
	FILE_OVERWRITE    uint32 = 0x00000004 // Open and overwrite; fail if not exists
	FILE_OVERWRITE_IF uint32 = 0x00000005 // Open and overwrite; create if not
)

// SMB2 Create Options
const (
	FILE_DIRECTORY_FILE            uint32 = 0x00000001
	FILE_WRITE_THROUGH             uint32 = 0x00000002
	FILE_SEQUENTIAL_ONLY           uint32 = 0x00000004
	FILE_NO_INTERMEDIATE_BUFFERING uint32 = 0x00000008
	FILE_SYNCHRONOUS_IO_ALERT      uint32 = 0x00000010
	FILE_SYNCHRONOUS_IO_NONALERT   uint32 = 0x00000020
	FILE_NON_DIRECTORY_FILE        uint32 = 0x00000040
	FILE_COMPLETE_IF_OPLOCKED      uint32 = 0x00000100
	FILE_NO_EA_KNOWLEDGE           uint32 = 0x00000200
	FILE_RANDOM_ACCESS             uint32 = 0x00000800
	FILE_DELETE_ON_CLOSE           uint32 = 0x00001000
	FILE_OPEN_BY_FILE_ID           uint32 = 0x00002000
	FILE_OPEN_FOR_BACKUP_INTENT    uint32 = 0x00004000
	FILE_NO_COMPRESSION            uint32 = 0x00008000
	FILE_OPEN_REPARSE_POINT        uint32 = 0x00200000
	FILE_OPEN_NO_RECALL            uint32 = 0x00400000
)

// SMB2 Create Action (returned in CREATE response)
const (
	FILE_SUPERSEDED  uint32 = 0x00000000
	FILE_OPENED      uint32 = 0x00000001
	FILE_CREATED     uint32 = 0x00000002
	FILE_OVERWRITTEN uint32 = 0x00000003
)

// SMB2 Security Mode
const (
	SMB2_NEGOTIATE_SIGNING_ENABLED  uint16 = 0x0001
	SMB2_NEGOTIATE_SIGNING_REQUIRED uint16 = 0x0002
)

// SMB2 Capabilities
const (
	SMB2_GLOBAL_CAP_DFS                uint32 = 0x00000001
	SMB2_GLOBAL_CAP_LEASING            uint32 = 0x00000002
	SMB2_GLOBAL_CAP_LARGE_MTU          uint32 = 0x00000004
	SMB2_GLOBAL_CAP_MULTI_CHANNEL      uint32 = 0x00000008
	SMB2_GLOBAL_CAP_PERSISTENT_HANDLES uint32 = 0x00000010
	SMB2_GLOBAL_CAP_DIRECTORY_LEASING  uint32 = 0x00000020
	SMB2_GLOBAL_CAP_ENCRYPTION         uint32 = 0x00000040
)

// SMB2 Share Type
const (
	SMB2_SHARE_TYPE_DISK  uint8 = 0x01
	SMB2_SHARE_TYPE_PIPE  uint8 = 0x02
	SMB2_SHARE_TYPE_PRINT uint8 = 0x03
)

// SMB2 Share Flags
const (
	SMB2_SHAREFLAG_MANUAL_CACHING              uint32 = 0x00000000
	SMB2_SHAREFLAG_AUTO_CACHING                uint32 = 0x00000010
	SMB2_SHAREFLAG_VDO_CACHING                 uint32 = 0x00000020
	SMB2_SHAREFLAG_NO_CACHING                  uint32 = 0x00000030
	SMB2_SHAREFLAG_DFS                         uint32 = 0x00000001
	SMB2_SHAREFLAG_DFS_ROOT                    uint32 = 0x00000002
	SMB2_SHAREFLAG_RESTRICT_EXCLUSIVE_OPENS    uint32 = 0x00000100
	SMB2_SHAREFLAG_FORCE_SHARED_DELETE         uint32 = 0x00000200
	SMB2_SHAREFLAG_ALLOW_NAMESPACE_CACHING     uint32 = 0x00000400
	SMB2_SHAREFLAG_ACCESS_BASED_DIRECTORY_ENUM uint32 = 0x00000800
	SMB2_SHAREFLAG_FORCE_LEVELII_OPLOCK        uint32 = 0x00001000
	SMB2_SHAREFLAG_ENABLE_HASH_V1              uint32 = 0x00002000
	SMB2_SHAREFLAG_ENABLE_HASH_V2              uint32 = 0x00004000
	SMB2_SHAREFLAG_ENCRYPT_DATA                uint32 = 0x00008000
)

// SMB2 Share Capabilities
const (
	SMB2_SHARE_CAP_DFS                     uint32 = 0x00000008
	SMB2_SHARE_CAP_CONTINUOUS_AVAILABILITY uint32 = 0x00000010
	SMB2_SHARE_CAP_SCALEOUT                uint32 = 0x00000020
	SMB2_SHARE_CAP_CLUSTER                 uint32 = 0x00000040
	SMB2_SHARE_CAP_ASYMMETRIC              uint32 = 0x00000080
)

// SMB2 Info Type (for QUERY_INFO and SET_INFO)
const (
	SMB2_0_INFO_FILE       uint8 = 0x01
	SMB2_0_INFO_FILESYSTEM uint8 = 0x02
	SMB2_0_INFO_SECURITY   uint8 = 0x03
	SMB2_0_INFO_QUOTA      uint8 = 0x04
)

// File Information Classes
const (
	FileDirectoryInformation         uint8 = 1
	FileFullDirectoryInformation     uint8 = 2
	FileBothDirectoryInformation     uint8 = 3
	FileBasicInformation             uint8 = 4
	FileStandardInformation          uint8 = 5
	FileInternalInformation          uint8 = 6
	FileEaInformation                uint8 = 7
	FileAccessInformation            uint8 = 8
	FileNameInformation              uint8 = 9
	FileRenameInformation            uint8 = 10
	FileLinkInformation              uint8 = 11
	FileNamesInformation             uint8 = 12
	FileDispositionInformation       uint8 = 13
	FilePositionInformation          uint8 = 14
	FileFullEaInformation            uint8 = 15
	FileModeInformation              uint8 = 16
	FileAlignmentInformation         uint8 = 17
	FileAllInformation               uint8 = 18
	FileAllocationInformation        uint8 = 19
	FileEndOfFileInformation         uint8 = 20
	FileAlternateNameInformation     uint8 = 21
	FileStreamInformation            uint8 = 22
	FilePipeInformation              uint8 = 23
	FilePipeLocalInformation         uint8 = 24
	FilePipeRemoteInformation        uint8 = 25
	FileMailslotQueryInformation     uint8 = 26
	FileMailslotSetInformation       uint8 = 27
	FileCompressionInformation       uint8 = 28
	FileObjectIdInformation          uint8 = 29
	FileMoveClusterInformation       uint8 = 31
	FileQuotaInformation             uint8 = 32
	FileReparsePointInformation      uint8 = 33
	FileNetworkOpenInformation       uint8 = 34
	FileAttributeTagInformation      uint8 = 35
	FileIdBothDirectoryInformation   uint8 = 37
	FileIdFullDirectoryInformation   uint8 = 38
	FileValidDataLengthInformation   uint8 = 39
	FileShortNameInformation         uint8 = 40
	FileIdGlobalTxDirectoryInformation uint8 = 50
)

// Filesystem Information Classes
const (
	FileFsVolumeInformation    uint8 = 1
	FileFsLabelInformation     uint8 = 2
	FileFsSizeInformation      uint8 = 3
	FileFsDeviceInformation    uint8 = 4
	FileFsAttributeInformation uint8 = 5
	FileFsControlInformation   uint8 = 6
	FileFsFullSizeInformation  uint8 = 7
	FileFsObjectIdInformation  uint8 = 8
	FileFsSectorSizeInformation uint8 = 11
)
