package pshell

import "testing"

func TestResolveShellDiffTargetsCommonPathUsesActiveHosts(t *testing.T) {
	t.Parallel()

	connects := []*sConnect{
		{Name: "web01"},
		{Name: "pve:sv-pve01:vm-gitlab"},
	}

	targets, err := resolveShellDiffTargets(connects, []string{"/etc/hosts"})
	if err != nil {
		t.Fatalf("resolveShellDiffTargets returned error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("resolveShellDiffTargets len = %d, want 2", len(targets))
	}
	if targets[0].Host != "pve:sv-pve01:vm-gitlab" || targets[0].RemotePath != "/etc/hosts" {
		t.Fatalf("unexpected first target: %#v", targets[0])
	}
	if targets[1].Host != "web01" || targets[1].RemotePath != "/etc/hosts" {
		t.Fatalf("unexpected second target: %#v", targets[1])
	}
}

func TestResolveShellDiffTargetsExplicitTargetsWithColonHost(t *testing.T) {
	t.Parallel()

	connects := []*sConnect{
		{Name: "pve:sv-pve01:vm-gitlab"},
		{Name: "pve:sv-pve02:vm-gitlab-runner2"},
	}

	targets, err := resolveShellDiffTargets(connects, []string{
		"@pve:sv-pve01:vm-gitlab:/etc/hosts",
		"@pve:sv-pve02:vm-gitlab-runner2:/tmp/hosts",
	})
	if err != nil {
		t.Fatalf("resolveShellDiffTargets returned error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("resolveShellDiffTargets len = %d, want 2", len(targets))
	}
	if targets[0].Host != "pve:sv-pve01:vm-gitlab" || targets[0].RemotePath != "/etc/hosts" {
		t.Fatalf("unexpected first target: %#v", targets[0])
	}
	if targets[1].Host != "pve:sv-pve02:vm-gitlab-runner2" || targets[1].RemotePath != "/tmp/hosts" {
		t.Fatalf("unexpected second target: %#v", targets[1])
	}
}
