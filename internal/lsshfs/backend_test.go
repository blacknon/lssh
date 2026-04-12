package lsshfs

import (
	"reflect"
	"runtime"
	"testing"
)

func TestBackendForGOOS(t *testing.T) {
	tests := []struct {
		goos string
		want Backend
		ok   bool
	}{
		{goos: "linux", want: BackendFUSE, ok: true},
		{goos: "darwin", want: BackendNFS, ok: true},
		{goos: "windows", want: BackendWinFsp, ok: true},
		{goos: "plan9", ok: false},
	}

	for _, tt := range tests {
		got, err := backendForGOOS(tt.goos)
		if tt.ok && err != nil {
			t.Fatalf("backendForGOOS(%q) error = %v", tt.goos, err)
		}
		if !tt.ok && err == nil {
			t.Fatalf("backendForGOOS(%q) error = nil", tt.goos)
		}
		if tt.ok && got != tt.want {
			t.Fatalf("backendForGOOS(%q) = %q, want %q", tt.goos, got, tt.want)
		}
	}
}

func TestNormalizeMountPoint(t *testing.T) {
	tests := []struct {
		name       string
		goos       string
		mountpoint string
		want       string
		wantErr    bool
	}{
		{name: "linux path", goos: "linux", mountpoint: ".", wantErr: false},
		{name: "windows drive", goos: "windows", mountpoint: "z:", want: "Z:", wantErr: false},
		{name: "windows invalid", goos: "windows", mountpoint: `C:\mount`, wantErr: true},
	}

	for _, tt := range tests {
		got, err := normalizeMountPoint(tt.goos, tt.mountpoint)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("%s: normalizeMountPoint() error = nil", tt.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: normalizeMountPoint() error = %v", tt.name, err)
		}
		if tt.want != "" && got != tt.want {
			t.Fatalf("%s: normalizeMountPoint() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestMountCommand(t *testing.T) {
	got, err := mountCommand("darwin", "/mnt/test", 2049, "share", nil)
	if err != nil {
		t.Fatalf("mountCommand(darwin) error = %v", err)
	}

	want := CommandSpec{
		Name: "mount_nfs",
		Args: []string{"-o", "port=2049,mountport=2049,tcp,nfsvers=3", "127.0.0.1:/", "/mnt/test"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mountCommand(darwin) = %#v, want %#v", got, want)
	}
}

func TestUnmountCommands(t *testing.T) {
	tests := []struct {
		goos string
		want []CommandSpec
	}{
		{
			goos: "linux",
			want: []CommandSpec{
				{Name: "fusermount3", Args: []string{"-u", "/mnt/test"}},
				{Name: "fusermount", Args: []string{"-u", "/mnt/test"}},
				{Name: "umount", Args: []string{"/mnt/test"}},
			},
		},
		{
			goos: "darwin",
			want: []CommandSpec{
				{Name: "umount", Args: []string{"/mnt/test"}},
			},
		},
		{
			goos: "windows",
			want: []CommandSpec{
				{Name: "sshfs.exe", Args: []string{"unmount", "Z:"}},
				{Name: "net", Args: []string{"use", "Z:", "/delete", "/y"}},
			},
		},
	}

	for _, tt := range tests {
		target := "/mnt/test"
		if tt.goos == "windows" {
			target = "Z:"
		}

		got, err := unmountCommands(tt.goos, target)
		if err != nil {
			t.Fatalf("unmountCommands(%q) error = %v", tt.goos, err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("unmountCommands(%q) = %#v, want %#v", tt.goos, got, tt.want)
		}
	}
}

func TestRuntimeBackendMatchesCurrentOS(t *testing.T) {
	got, err := runtimeBackend()
	if err != nil {
		t.Fatalf("runtimeBackend() error = %v", err)
	}

	want, err := backendForGOOS(runtime.GOOS)
	if err != nil {
		t.Fatalf("backendForGOOS(%q) error = %v", runtime.GOOS, err)
	}

	if got != want {
		t.Fatalf("runtimeBackend() = %q, want %q", got, want)
	}
}

func TestIsMountActiveUnsupportedOS(t *testing.T) {
	active, err := isMountActive("plan9", `/mnt/test`)
	if err != nil {
		t.Fatalf("isMountActive() error = %v", err)
	}
	if active {
		t.Fatalf("isMountActive() = true, want false")
	}
}

func TestIsMountActiveWindowsChecksDrivePath(t *testing.T) {
	drive := t.TempDir()

	active, err := isMountActive("windows", drive)
	if err != nil {
		t.Fatalf("isMountActive() error = %v", err)
	}
	if !active {
		t.Fatalf("isMountActive() = false, want true")
	}
}
