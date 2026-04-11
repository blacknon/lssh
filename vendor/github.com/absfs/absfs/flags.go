package absfs

import (
	"fmt"
	"os"
	"strings"
)

const (
	O_ACCESS = 0x3 // masks the access mode (O_RDONLY, O_WRONLY, or O_RDWR)

	// Exactly one of O_RDONLY, O_WRONLY, or O_RDWR must be specified.
	O_RDONLY = os.O_RDONLY // open the file read-only.
	O_WRONLY = os.O_WRONLY // open the file write-only.
	O_RDWR   = os.O_RDWR   // open the file read-write.

	// The remaining values may be or'ed in to control behavior.
	O_APPEND = os.O_APPEND // append data to the file when writing.
	O_CREATE = os.O_CREATE // create a new file if none exists.
	O_EXCL   = os.O_EXCL   // used with O_CREATE, file must not exist.
	O_SYNC   = os.O_SYNC   // open for synchronous I/O.
	O_TRUNC  = os.O_TRUNC  // if possible, truncate file when opened.
)

// Flags - represents access and permission flags for use with file opening
// functions.
type Flags int

// String - returns the list of values set in a `Flag` separated by "|".
func (f Flags) String() string {
	var out []string
	// flags := int(f)
	switch int(f) & O_ACCESS {
	case O_RDONLY:
		out = append(out, "O_RDONLY")
	case O_RDWR:
		out = append(out, "O_RDWR")
	case O_WRONLY:
		out = append(out, "O_WRONLY")
	}

	names := []string{"O_APPEND", "O_CREATE", "O_EXCL", "O_SYNC", "O_TRUNC"}
	for i, flag := range []Flags{Flags(O_APPEND), Flags(O_CREATE), Flags(O_EXCL), Flags(O_SYNC), Flags(O_TRUNC)} {
		if (flag & f) != 0 {
			out = append(out, names[i])
		}
	}
	return strings.Join(out, "|")
}

// ParseFlags - parses a string of flags separated by "|" and returns a `Flags`
// value and an error.
//
// Following is the list of recognized flags, "O_RDONLY", "O_RDWR", "O_WRONLY",
// "O_APPEND", "O_CREATE", "O_EXCL", "O_SYNC", "O_TRUNC". The access mode
// values ("O_RDONLY", "O_RDWR", "O_WRONLY") are mutually exclusive, only one
// may be specified.  If no access mode is specified "O_RDONLY" is the default.
// All other flags may appear more than once, but subsequent occurrences have no
// effect.
func ParseFlags(input string) (Flags, error) {
	var acc string
	var out Flags

	for _, v := range strings.Split(input, "|") {
		switch v {
		case "O_RDONLY":
			if len(acc) != 0 {
				return 0, fmt.Errorf("error parsing flags, multiple access modes %q, %q", acc, v)
			}
			acc = v
			continue
		case "O_RDWR":
			if len(acc) != 0 {
				return 0, fmt.Errorf("error parsing flags, multiple access modes %q, %q", acc, v)
			}
			acc = v
			continue
		case "O_WRONLY":
			if len(acc) != 0 {
				return 0, fmt.Errorf("error parsing flags, multiple access modes %q, %q", acc, v)
			}
			acc = v
			continue
		case "O_APPEND":
			out |= Flags(O_APPEND)
			continue
		case "O_CREATE":
			out |= Flags(O_CREATE)
			continue
		case "O_EXCL":
			out |= Flags(O_EXCL)
			continue
		case "O_SYNC":
			out |= Flags(O_SYNC)
			continue
		case "O_TRUNC":
			out |= Flags(O_TRUNC)
			continue
		default:
			return 0, fmt.Errorf("error parsing flags, unrecognized flag %q", v)
		}
	}

	switch acc {
	case "O_RDONLY":
	case "O_RDWR":
		out |= Flags(O_RDWR)
	case "O_WRONLY":
		out |= Flags(O_WRONLY)
	}
	return out, nil
}
