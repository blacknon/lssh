package absfs

import (
	"fmt"
	"os"
	"strings"
)

// ParseFileMode - parses a unix style file mode string and returns an
// `os.FileMode` and an error.
func ParseFileMode(input string) (os.FileMode, error) {
	var mode os.FileMode

	if len(input) < 10 {
		return 0, fmt.Errorf("unable to parse file mode string too short length == %d", len(input))
	}

	switch string(input[:1]) {
	case "-":
	case "d":
		mode = os.ModeDir // d: is a directory
	case "a":
		mode = os.ModeAppend // a: append-only
	case "l":
		mode = os.ModeExclusive // l: exclusive use
	case "T":
		mode = os.ModeTemporary // T: temporary file; Plan 9 only
	case "L":
		mode = os.ModeSymlink // L: symbolic link
	case "D":
		mode = os.ModeDevice // D: device file
	case "p":
		mode = os.ModeNamedPipe // p: named pipe (FIFO)
	case "S":
		mode = os.ModeSocket // S: Unix domain socket
	case "u":
		mode = os.ModeSetuid // u: setuid
	case "g":
		mode = os.ModeSetgid // g: setgid
	case "c":
		mode = os.ModeCharDevice // c: Unix character device, when ModeDevice is set
	case "t":
		mode = os.ModeSticky // t: sticky
	default:
		return 0, fmt.Errorf("unable to parse file mode string unrecognized character %q", input[:1])
	}

	permissions := [][2]int{
		// user permissions
		{'r', OS_USER_R},
		{'w', OS_USER_W},
		{'x', OS_USER_X},

		// group permissions
		{'r', OS_GROUP_R},
		{'w', OS_GROUP_W},
		{'x', OS_GROUP_X},

		// others permissions
		{'r', OS_OTH_R},
		{'w', OS_OTH_W},
		{'x', OS_OTH_X},
	}

	for i, c := range strings.ToLower(input[1:]) {
		if c == '-' {
			continue
		}
		if int(c) != permissions[i][0] {
			return 0, fmt.Errorf("unable to parse file mode string unrecognized character %q at %d.", string(input[i+1]), i+1)
		}
		mode |= os.FileMode(permissions[i][1])
	}

	return mode, nil
}

// Permission flags not provided by the standard library.
const (
	OS_READ        = 04
	OS_WRITE       = 02
	OS_EX          = 01
	OS_USER_SHIFT  = 6
	OS_GROUP_SHIFT = 3
	OS_OTH_SHIFT   = 0

	OS_USER_R   = OS_READ << OS_USER_SHIFT
	OS_USER_W   = OS_WRITE << OS_USER_SHIFT
	OS_USER_X   = OS_EX << OS_USER_SHIFT
	OS_USER_RW  = OS_USER_R | OS_USER_W
	OS_USER_RWX = OS_USER_RW | OS_USER_X

	OS_GROUP_R   = OS_READ << OS_GROUP_SHIFT
	OS_GROUP_W   = OS_WRITE << OS_GROUP_SHIFT
	OS_GROUP_X   = OS_EX << OS_GROUP_SHIFT
	OS_GROUP_RW  = OS_GROUP_R | OS_GROUP_W
	OS_GROUP_RWX = OS_GROUP_RW | OS_GROUP_X

	OS_OTH_R   = OS_READ << OS_OTH_SHIFT
	OS_OTH_W   = OS_WRITE << OS_OTH_SHIFT
	OS_OTH_X   = OS_EX << OS_OTH_SHIFT
	OS_OTH_RW  = OS_OTH_R | OS_OTH_W
	OS_OTH_RWX = OS_OTH_RW | OS_OTH_X

	OS_ALL_R   = OS_USER_R | OS_GROUP_R | OS_OTH_R
	OS_ALL_W   = OS_USER_W | OS_GROUP_W | OS_OTH_W
	OS_ALL_X   = OS_USER_X | OS_GROUP_X | OS_OTH_X
	OS_ALL_RW  = OS_ALL_R | OS_ALL_W
	OS_ALL_RWX = OS_ALL_RW | OS_ALL_X
)
