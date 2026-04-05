package demo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDemoDockerComposeE2E(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("demo e2e runs only on linux")
	}

	if os.Getenv("LSSH_RUN_DEMO_E2E") != "1" {
		t.Skip("set LSSH_RUN_DEMO_E2E=1 to run demo e2e")
	}

	demoDir := mustDemoDir(t)
	ensureDockerComposeAvailable(t, demoDir)
	composeUp(t, demoDir)
	t.Cleanup(func() {
		composeDown(t, demoDir)
	})
	waitForServices(t, demoDir)

	t.Run("client ssh bastion exposes lssh via authorized_keys", func(t *testing.T) {
		assertClientCommandContains(t, demoDir,
			"ssh -p 2222 -i ~/.ssh/demo_lssh_ed25519 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null demo@127.0.0.1 -- --list",
			"OverNestedSocksProxy",
		)
	})

	t.Run("client cannot reach backend host directly", func(t *testing.T) {
		output, err := runClientCommand(demoDir, "nc -z -w 2 172.31.1.41 22")
		if err == nil {
			t.Fatalf("expected direct access to fail, but it succeeded: %s", output)
		}
	})

	t.Run("client cannot reach deep backend host directly", func(t *testing.T) {
		output, err := runClientCommand(demoDir, "nc -z -w 2 172.31.2.51 22")
		if err == nil {
			t.Fatalf("expected deep direct access to fail, but it succeeded: %s", output)
		}
	})

	t.Run("password auth works", func(t *testing.T) {
		assertClientCommandContains(t, demoDir,
			"sshpass -p demo-password ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null demo@172.31.0.21 hostname",
			"password-ssh",
		)
	})

	t.Run("key auth works", func(t *testing.T) {
		assertClientCommandContains(t, demoDir,
			"ssh -i ~/.ssh/demo_lssh_ed25519 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null demo@172.31.0.22 hostname",
			"key-ssh",
		)
	})

	t.Run("ssh proxy works", func(t *testing.T) {
		assertClientCommandContains(t, demoDir, "lssh --host OverSshProxy hostname", "over-proxy-ssh")
	})

	t.Run("http proxy works", func(t *testing.T) {
		assertClientCommandContains(t, demoDir, "lssh --host OverHttpProxy hostname", "over-proxy-ssh")
	})

	t.Run("nested ssh proxy works", func(t *testing.T) {
		assertClientCommandContains(t, demoDir, "lssh --host OverNestedSshProxy hostname", "deep-proxy-ssh")
	})

	t.Run("nested http proxy works behind over ssh proxy", func(t *testing.T) {
		assertClientCommandContains(t, demoDir, "lssh --host OverNestedHttpProxy hostname", "over-deep-http-ssh")
	})

	t.Run("nested socks proxy works behind over ssh proxy", func(t *testing.T) {
		assertClientCommandContains(t, demoDir, "lssh --host OverNestedSocksProxy hostname", "over-deep-http-ssh")
	})

	t.Run("socks proxy works", func(t *testing.T) {
		assertClientCommandContains(t, demoDir, "lssh --host OverSocksProxy hostname", "over-proxy-ssh")
	})

	t.Run("local rc is available on remote shell", func(t *testing.T) {
		assertClientCommandContains(t, demoDir,
			`lssh --host LocalRcKeyAuth 'type lvim >/dev/null && type ltmux >/dev/null && echo local_rc_ok'`,
			"local_rc_ok",
		)
	})

	t.Run("generated vim wrapper is usable", func(t *testing.T) {
		assertClientCommandContains(t, demoDir,
			`lssh --host LocalRcKeyAuth 'vim "+set nomore" "+set statusline?" "+q" | tail -n 1'`,
			"statusline=[demo-localrc]",
		)
	})
}

func mustDemoDir(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err != nil {
		t.Fatalf("docker-compose.yml not found in %s: %v", dir, err)
	}

	return dir
}

func ensureDockerComposeAvailable(t *testing.T, demoDir string) {
	t.Helper()

	cmd := exec.Command("docker", "compose", "version")
	cmd.Dir = demoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docker compose is required: %v\n%s", err, string(output))
	}
}

func composeUp(t *testing.T, demoDir string) {
	t.Helper()

	cmd := exec.Command("docker", "compose", "up", "--build", "-d")
	cmd.Dir = demoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docker compose up failed: %v\n%s", err, string(output))
	}
}

func composeDown(t *testing.T, demoDir string) {
	t.Helper()

	cmd := exec.Command("docker", "compose", "down", "-v")
	cmd.Dir = demoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Logf("docker compose down failed: %v\n%s", err, string(output))
	}
}

func waitForServices(t *testing.T, demoDir string) {
	t.Helper()

	deadline := time.Now().Add(90 * time.Second)
	check := "nc -z 127.0.0.1 2222 && nc -z 172.31.0.21 22 && nc -z 172.31.0.22 22 && nc -z 172.31.0.31 22 && nc -z 172.31.0.32 8888 && nc -z 172.31.0.33 1080 && nc -z 172.31.1.41 22"
	var lastOutput string

	for time.Now().Before(deadline) {
		output, err := runClientCommand(demoDir, check)
		if err == nil {
			return
		}
		lastOutput = output
		time.Sleep(2 * time.Second)
	}

	t.Fatalf("demo services did not become ready in time\n%s", lastOutput)
}

func assertClientCommandContains(t *testing.T, demoDir, command, want string) {
	t.Helper()

	output, err := runClientCommand(demoDir, command)
	if err != nil {
		t.Fatalf("command failed: %s\nerror: %v\noutput:\n%s", command, err, output)
	}

	if !strings.Contains(output, want) {
		t.Fatalf("output missing %q for command %s\noutput:\n%s", want, command, output)
	}
}

func runClientCommand(demoDir, command string) (string, error) {
	cmd := exec.Command("docker", "compose", "exec", "-T", "client", "bash", "-lc", command)
	cmd.Dir = demoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w", err)
	}
	return string(output), nil
}
