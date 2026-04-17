//go:build linux

package main

/*
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../../vendor/github.com/bitwarden/sdk-go/internal/cinterface/lib/linux-x64 -Wl,--start-group -lbitwarden_c -lm -Wl,--end-group
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../../vendor/github.com/bitwarden/sdk-go/internal/cinterface/lib/linux-arm64 -Wl,--start-group -lbitwarden_c -lm -Wl,--end-group
*/
import "C"
