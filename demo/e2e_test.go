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

	t.Run("client bastion command is configured", func(t *testing.T) {
		assertClientCommandContains(t, demoDir,
			`grep -q '^ForceCommand /usr/local/bin/demo-lssh-bastion.sh$' ~/.demo-sshd/sshd_config && grep -q '^ssh-ed25519 ' ~/.ssh/authorized_keys && /usr/local/bin/demo-lssh-bastion.sh --list`,
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

	t.Run("control master local forward works", func(t *testing.T) {
		pidFile := "/tmp/lssh-demo-local-forward.pid"
		t.Cleanup(func() {
			stopClientForward(t, demoDir, pidFile)
		})

		startClientForward(t, demoDir, pidFile,
			"lssh --host OverSshProxyCM -N -L 10081:localhost:22",
			"127.0.0.1:10081",
		)

		assertClientCommandContains(t, demoDir,
			`banner=$(nc -w 5 127.0.0.1 10081 | head -c 8); printf '%s' "$banner"`,
			"SSH-2.0-",
		)
	})

	t.Run("control master dynamic forward works", func(t *testing.T) {
		pidFile := "/tmp/lssh-demo-dynamic-forward.pid"
		t.Cleanup(func() {
			stopClientForward(t, demoDir, pidFile)
		})

		startClientForward(t, demoDir, pidFile,
			"lssh --host OverSshProxyCM -N -D 10080",
			"127.0.0.1:10080",
		)

		assertClientCommandContains(t, demoDir,
			`banner=$(nc -X 5 -x 127.0.0.1:10080 -w 5 172.31.1.41 22 | head -c 8); printf '%s' "$banner"`,
			"SSH-2.0-",
		)
	})

	t.Run("control master remote forward works", func(t *testing.T) {
		pidFile := "/tmp/lssh-demo-remote-forward.pid"
		t.Cleanup(func() {
			stopClientForward(t, demoDir, pidFile)
		})

		startClientForward(t, demoDir, pidFile,
			"lssh --host OverSshProxyCM -N -R 10082:172.31.0.10:22",
			"",
		)

		waitForComposeExecContains(t, demoDir, "over_proxy_ssh",
			`banner=$(bash -lc 'exec 3<>/dev/tcp/127.0.0.1/10082; head -c 8 <&3' 2>/dev/null || true); printf '%s' "$banner"`,
			"SSH-2.0-",
		)
	})

	t.Run("local rc is available on remote shell", func(t *testing.T) {
		assertClientCommandContains(t, demoDir,
			`lssh --host LocalRcKeyAuth 'type lvim >/dev/null && type ltmux >/dev/null && echo local_rc_ok'`,
			"local_rc_ok",
		)
	})

	t.Run("generated vim wrapper is loaded", func(t *testing.T) {
		assertClientCommandContains(t, demoDir,
			`lssh --host LocalRcKeyAuth 'declare -f lvim | grep -F "vim -u <(printf" && echo lvim_wrapper_ok'`,
			"lvim_wrapper_ok",
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

	deadline := time.Now().Add(150 * time.Second)
	check := `
failed=0
for item in \
  "client_sshd 127.0.0.1 2222" \
  "password_ssh 172.31.0.21 22" \
  "key_ssh 172.31.0.22 22" \
  "ssh_proxy 172.31.0.31 22" \
  "http_proxy 172.31.0.32 8888" \
  "socks_proxy 172.31.0.33 1080"
do
  set -- $item
  name="$1"
  host="$2"
  port="$3"
  if nc -z -w 2 "$host" "$port" >/dev/null 2>&1; then
    echo "ok $name $host:$port"
  else
    echo "ng $name $host:$port"
    failed=1
  fi
done
exit $failed
`
	var lastOutput string

	for time.Now().Before(deadline) {
		output, err := runClientCommand(demoDir, check)
		if err == nil {
			return
		}
		lastOutput = output
		time.Sleep(2 * time.Second)
	}

	t.Fatalf("demo services did not become ready in time\nlast readiness output:\n%s\n\ncompose ps:\n%s\n\nproxy logs:\n%s",
		lastOutput,
		mustRunComposeCommand(t, demoDir, "ps", "-a"),
		mustRunComposeCommand(t, demoDir, "logs", "--tail=200", "http_proxy", "deep_http_proxy", "socks_proxy", "deep_socks_proxy"),
	)
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
	cmd := exec.Command("docker", "compose", "exec", "-T", "--user", "demo", "client", "bash", "-lc", command)
	cmd.Dir = demoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w", err)
	}
	return string(output), nil
}

func mustRunComposeCommand(t *testing.T, demoDir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Dir = demoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("command failed: docker compose %s\n%v\n%s", strings.Join(args, " "), err, string(output))
	}

	return string(output)
}

func startClientForward(t *testing.T, demoDir, pidFile, forwardCommand, waitAddr string) {
	t.Helper()

	stopClientForward(t, demoDir, pidFile)

	startCmd := fmt.Sprintf(
		`rm -f %[1]s; nohup %[2]s >/tmp/$(basename %[1]s).log 2>&1 & echo $! > %[1]s`,
		pidFile,
		forwardCommand,
	)

	if output, err := runClientCommand(demoDir, startCmd); err != nil {
		t.Fatalf("failed to start forward: %s\nerror: %v\noutput:\n%s", forwardCommand, err, output)
	}

	if waitAddr == "" {
		time.Sleep(2 * time.Second)
		return
	}

	deadline := time.Now().Add(10 * time.Second)
	checkCmd := fmt.Sprintf("nc -z -w 2 %s %s", strings.Split(waitAddr, ":")[0], strings.Split(waitAddr, ":")[1])
	for time.Now().Before(deadline) {
		if _, err := runClientCommand(demoDir, checkCmd); err == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}

	logOutput, _ := runClientCommand(demoDir, fmt.Sprintf("cat /tmp/$(basename %s).log || true", pidFile))
	t.Fatalf("forward did not become ready: %s\nlog:\n%s", forwardCommand, logOutput)
}

func stopClientForward(t *testing.T, demoDir, pidFile string) {
	t.Helper()

	_, _ = runClientCommand(demoDir,
		fmt.Sprintf(`if [ -f %[1]s ]; then kill $(cat %[1]s) >/dev/null 2>&1 || true; rm -f %[1]s; fi`, pidFile),
	)
}

func waitForComposeExecContains(t *testing.T, demoDir, service, command, want string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	var lastOutput string

	for time.Now().Before(deadline) {
		output, err := runComposeServiceCommand(demoDir, service, command)
		lastOutput = output
		if err == nil && strings.Contains(output, want) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}

	t.Fatalf("output missing %q for service %s command %s\nlast output:\n%s", want, service, command, lastOutput)
}

func runComposeServiceCommand(demoDir, service, command string) (string, error) {
	cmd := exec.Command("docker", "compose", "exec", "-T", service, "bash", "-lc", command)
	cmd.Dir = demoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w", err)
	}
	return string(output), nil
}
