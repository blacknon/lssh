package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildBidirectionalPlansCopiesMissingFilesBothWays(t *testing.T) {
	t.Parallel()

	leftRoot := t.TempDir()
	rightRoot := t.TempDir()
	now := time.Unix(1_000, 0)

	mustWriteFile(t, filepath.Join(leftRoot, "left.txt"), "left", now)
	mustWriteFile(t, filepath.Join(rightRoot, "right.txt"), "right", now)

	leftFS, err := NewLocalFS()
	if err != nil {
		t.Fatalf("NewLocalFS returned error: %v", err)
	}
	rightFS, err := NewLocalFS()
	if err != nil {
		t.Fatalf("NewLocalFS returned error: %v", err)
	}

	leftToRight, rightToLeft, err := BuildBidirectionalPlans(leftFS, rightFS, leftRoot, rightRoot)
	if err != nil {
		t.Fatalf("BuildBidirectionalPlans returned error: %v", err)
	}

	if err := ApplyPlan(context.Background(), leftFS, rightFS, leftToRight, ApplyOptions{}); err != nil {
		t.Fatalf("ApplyPlan leftToRight returned error: %v", err)
	}
	if err := ApplyPlan(context.Background(), rightFS, leftFS, rightToLeft, ApplyOptions{}); err != nil {
		t.Fatalf("ApplyPlan rightToLeft returned error: %v", err)
	}

	assertFileContent(t, filepath.Join(leftRoot, "right.txt"), "right")
	assertFileContent(t, filepath.Join(rightRoot, "left.txt"), "left")
}

func TestBuildBidirectionalPlansPrefersNewerFile(t *testing.T) {
	t.Parallel()

	leftRoot := t.TempDir()
	rightRoot := t.TempDir()
	older := time.Unix(1_000, 0)
	newer := older.Add(2 * time.Second)

	mustWriteFile(t, filepath.Join(leftRoot, "shared.txt"), "old-left", older)
	mustWriteFile(t, filepath.Join(rightRoot, "shared.txt"), "new-right", newer)

	leftFS, err := NewLocalFS()
	if err != nil {
		t.Fatalf("NewLocalFS returned error: %v", err)
	}
	rightFS, err := NewLocalFS()
	if err != nil {
		t.Fatalf("NewLocalFS returned error: %v", err)
	}

	leftToRight, rightToLeft, err := BuildBidirectionalPlans(leftFS, rightFS, leftRoot, rightRoot)
	if err != nil {
		t.Fatalf("BuildBidirectionalPlans returned error: %v", err)
	}

	if len(leftToRight.Desired) != 0 {
		t.Fatalf("expected no leftToRight changes, got %#v", leftToRight.Desired)
	}

	if err := ApplyPlan(context.Background(), rightFS, leftFS, rightToLeft, ApplyOptions{}); err != nil {
		t.Fatalf("ApplyPlan rightToLeft returned error: %v", err)
	}

	assertFileContent(t, filepath.Join(leftRoot, "shared.txt"), "new-right")
	assertFileContent(t, filepath.Join(rightRoot, "shared.txt"), "new-right")
}

func TestBuildBidirectionalPlansUsesChecksumWhenTimestampsMatch(t *testing.T) {
	t.Parallel()

	leftRoot := t.TempDir()
	rightRoot := t.TempDir()
	sameTime := time.Unix(1_000, 0)

	mustWriteFile(t, filepath.Join(leftRoot, "shared.txt"), "left-content", sameTime)
	mustWriteFile(t, filepath.Join(rightRoot, "shared.txt"), "right-data!!", sameTime)

	leftFS, err := NewLocalFS()
	if err != nil {
		t.Fatalf("NewLocalFS returned error: %v", err)
	}
	rightFS, err := NewLocalFS()
	if err != nil {
		t.Fatalf("NewLocalFS returned error: %v", err)
	}

	leftToRight, rightToLeft, err := BuildBidirectionalPlans(leftFS, rightFS, leftRoot, rightRoot)
	if err != nil {
		t.Fatalf("BuildBidirectionalPlans returned error: %v", err)
	}

	if len(leftToRight.Desired) == 0 && len(rightToLeft.Desired) == 0 {
		t.Fatal("expected checksum difference to produce a sync plan")
	}
	if _, ok := leftToRight.Desired[filepath.Join(rightRoot, "shared.txt")]; !ok {
		t.Fatalf("expected checksum difference to schedule shared.txt copy, got leftToRight=%#v", leftToRight.Desired)
	}
}

func mustWriteFile(t *testing.T, path string, body string, modTime time.Time) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%q) returned error: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("WriteFile(%q) returned error: %v", path, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes(%q) returned error: %v", path, err)
	}
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) returned error: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("file %q = %q, want %q", path, string(got), want)
	}
}
