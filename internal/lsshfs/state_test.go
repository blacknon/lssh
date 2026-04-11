package lsshfs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseRemoteSpec(t *testing.T) {
	tests := []struct {
		value    string
		host     string
		path     string
		hasError bool
	}{
		{value: "@web01:/var/www", host: "web01", path: "/var/www"},
		{value: "web01:/var/www", host: "web01", path: "/var/www"},
		{value: "/var/www", path: "/var/www"},
		{value: "@:/var/www", hasError: true},
	}

	for _, tt := range tests {
		host, path, err := parseRemoteSpec(tt.value)
		if tt.hasError {
			if err == nil {
				t.Fatalf("parseRemoteSpec(%q) error = nil", tt.value)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseRemoteSpec(%q) error = %v", tt.value, err)
		}
		if host != tt.host || path != tt.path {
			t.Fatalf("parseRemoteSpec(%q) = (%q,%q), want (%q,%q)", tt.value, host, path, tt.host, tt.path)
		}
	}
}

func TestMountRecordLifecycle(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	record := MountRecord{
		Host:       "web01",
		RemotePath: "/srv/app",
		MountPoint: "/tmp/app",
		Backend:    "fuse",
		PID:        1234,
		ReadWrite:  true,
	}

	if err := writeMountRecord(record); err != nil {
		t.Fatalf("writeMountRecord() error = %v", err)
	}

	got, err := loadMountRecord(record.MountPoint)
	if err != nil {
		t.Fatalf("loadMountRecord() error = %v", err)
	}
	if !reflect.DeepEqual(got, record) {
		t.Fatalf("loadMountRecord() = %#v, want %#v", got, record)
	}

	records, err := listMountRecords()
	if err != nil {
		t.Fatalf("listMountRecords() error = %v", err)
	}
	if len(records) != 1 || !reflect.DeepEqual(records[0], record) {
		t.Fatalf("listMountRecords() = %#v, want %#v", records, []MountRecord{record})
	}

	if err := removeMountRecord(record.MountPoint); err != nil {
		t.Fatalf("removeMountRecord() error = %v", err)
	}

	_, err = loadMountRecord(record.MountPoint)
	if !os.IsNotExist(err) {
		t.Fatalf("loadMountRecord() after remove error = %v, want not exist", err)
	}
}
