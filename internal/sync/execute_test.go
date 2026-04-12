package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestApplyPlanUsesChecksumWhenSizeAndTimeMatch(t *testing.T) {
	t.Parallel()

	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	sameTime := time.Unix(1_000, 0)

	srcFile := filepath.Join(srcRoot, "file.txt")
	dstFile := filepath.Join(dstRoot, "file.txt")

	mustWriteFile(t, srcFile, "abc", sameTime)
	mustWriteFile(t, dstFile, "xyz", sameTime)

	srcFS, err := NewLocalFS()
	if err != nil {
		t.Fatalf("NewLocalFS returned error: %v", err)
	}
	dstFS, err := NewLocalFS()
	if err != nil {
		t.Fatalf("NewLocalFS returned error: %v", err)
	}

	plan, err := BuildPlan(srcFS, dstFS, []string{srcFile}, dstFile)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if err := ApplyPlan(context.Background(), srcFS, dstFS, plan, ApplyOptions{}); err != nil {
		t.Fatalf("ApplyPlan returned error: %v", err)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(got) != "abc" {
		t.Fatalf("destination content = %q, want %q", string(got), "abc")
	}
}

func TestApplyPlanDryRunDoesNotModifyDestination(t *testing.T) {
	t.Parallel()

	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	sameTime := time.Unix(1_000, 0)

	srcFile := filepath.Join(srcRoot, "file.txt")
	dstFile := filepath.Join(dstRoot, "file.txt")

	mustWriteFile(t, srcFile, "abc", sameTime)
	mustWriteFile(t, dstFile, "xyz", sameTime)

	srcFS, err := NewLocalFS()
	if err != nil {
		t.Fatalf("NewLocalFS returned error: %v", err)
	}
	dstFS, err := NewLocalFS()
	if err != nil {
		t.Fatalf("NewLocalFS returned error: %v", err)
	}

	plan, err := BuildPlan(srcFS, dstFS, []string{srcFile}, dstFile)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if err := ApplyPlan(context.Background(), srcFS, dstFS, plan, ApplyOptions{DryRun: true}); err != nil {
		t.Fatalf("ApplyPlan returned error: %v", err)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(got) != "xyz" {
		t.Fatalf("destination content = %q, want %q", string(got), "xyz")
	}
}
