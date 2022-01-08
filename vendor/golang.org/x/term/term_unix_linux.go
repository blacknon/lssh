<<<<<<< HEAD:vendor/golang.org/x/term/term_unix_other.go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build aix || linux || solaris || zos
// +build aix linux solaris zos

=======
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

>>>>>>> origin/master:vendor/golang.org/x/term/term_unix_linux.go
package term

import "golang.org/x/sys/unix"

const ioctlReadTermios = unix.TCGETS
const ioctlWriteTermios = unix.TCSETS
