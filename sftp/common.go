// Copyright (c) 2022 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

import (
	"io/fs"
	"os"
	"strconv"
)

//
func DupPermutationsRecursive0(n, k int) [][]int {
	if k == 0 {
		pattern := []int{}
		return [][]int{pattern}
	}

	ans := [][]int{}
	for num := 0; num < n; num++ {
		childPatterns := DupPermutationsRecursive0(n, k-1)
		for _, childPattern := range childPatterns {
			pattern := append([]int{num}, childPattern...)
			ans = append(ans, pattern)
		}
	}

	return ans
}

// A function that returns the value of fs.Filemode from the permissions and umask passed in an array.
// ex)
//   defaultPerm ... [0,7,7,7]
//   umask ... [0,2,2]
func GeneratePermWithUmask(defaultPerm, umask []string) fs.FileMode {
	// set 1st char
	setPermStr := defaultPerm[0]

	// set 2nd char
	perm1char, _ := strconv.Atoi(defaultPerm[1])
	umask1char, _ := strconv.Atoi(umask[0])
	setPermStr = setPermStr + strconv.Itoa(perm1char-umask1char)

	// set 3rd char
	perm2char, _ := strconv.Atoi(defaultPerm[2])
	umask2char, _ := strconv.Atoi(umask[1])
	setPermStr = setPermStr + strconv.Itoa(perm2char-umask2char)

	// set 4th char
	perm3char, _ := strconv.Atoi(defaultPerm[3])
	umask3char, _ := strconv.Atoi(umask[2])
	setPermStr = setPermStr + strconv.Itoa(perm3char-umask3char)

	perm, _ := strconv.ParseUint(setPermStr, 8, 32)

	return os.FileMode(perm)
}
