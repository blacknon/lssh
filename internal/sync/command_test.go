package sync

import "testing"

func TestParseCommandArgs(t *testing.T) {
	t.Parallel()

	got, err := ParseCommandArgs([]string{"sync", "--delete", "-p", "-P", "4", "local:/src", "remote:/dst"})
	if err != nil {
		t.Fatalf("ParseCommandArgs returned error: %v", err)
	}

	if !got.Delete || !got.Permission || got.ParallelNum != 4 {
		t.Fatalf("unexpected parsed flags: %#v", got)
	}
	if len(got.Sources) != 1 || got.Sources[0] != "local:/src" || got.Destination != "remote:/dst" {
		t.Fatalf("unexpected parsed paths: %#v", got)
	}
}

func TestParsePathSpec(t *testing.T) {
	t.Parallel()

	spec, err := ParsePathSpec("remote:@web01,web02:/srv/app")
	if err != nil {
		t.Fatalf("ParsePathSpec returned error: %v", err)
	}
	if !spec.IsRemote || len(spec.Hosts) != 2 || spec.Path != "/srv/app" {
		t.Fatalf("unexpected remote spec: %#v", spec)
	}

	spec, err = ParsePathSpec("local:/tmp/file")
	if err != nil {
		t.Fatalf("ParsePathSpec returned error: %v", err)
	}
	if spec.IsRemote || spec.Path != "/tmp/file" {
		t.Fatalf("unexpected local spec: %#v", spec)
	}
}
