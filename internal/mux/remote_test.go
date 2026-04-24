package mux

import (
	"runtime"
	"strings"
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
	ssmconnector "github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/ssmconnector"
)

func TestBuildAWSNativeShellSessionConfigAddsLocalRC(t *testing.T) {
	sessionCfg := buildAWSNativeShellSessionConfig(
		conf.ServerConfig{
			LocalRcUse: "yes",
		},
		ssmconnector.ShellConfig{
			BaseConfig: ssmconnector.BaseConfig{
				InstanceID: "i-1234567890",
				Region:     "ap-northeast-1",
			},
		},
	)

	if sessionCfg.StartupMarker != "__LSSH_LOCALRC_READY__" {
		t.Fatalf("StartupMarker = %q, want %q", sessionCfg.StartupMarker, "__LSSH_LOCALRC_READY__")
	}
	if sessionCfg.ExpandNewlineToCRLF {
		t.Fatal("ExpandNewlineToCRLF = true, want false to match lssh native shell input handling")
	}
	if !strings.Contains(sessionCfg.StartupCommand, "exec bash -lc") {
		t.Fatalf("StartupCommand = %q, want interactive local_rc startup command", sessionCfg.StartupCommand)
	}
}

func TestBuildAWSNativeShellSessionConfigWithoutLocalRC(t *testing.T) {
	sessionCfg := buildAWSNativeShellSessionConfig(
		conf.ServerConfig{},
		ssmconnector.ShellConfig{
			BaseConfig: ssmconnector.BaseConfig{
				InstanceID: "i-1234567890",
				Region:     "ap-northeast-1",
			},
		},
	)

	if sessionCfg.StartupCommand != "" {
		t.Fatalf("StartupCommand = %q, want empty without local_rc", sessionCfg.StartupCommand)
	}
	if sessionCfg.StartupMarker != "" {
		t.Fatalf("StartupMarker = %q, want empty without local_rc", sessionCfg.StartupMarker)
	}
	if sessionCfg.ExpandNewlineToCRLF {
		t.Fatal("ExpandNewlineToCRLF = true, want false without local_rc")
	}
	if sessionCfg.InitialCols != 0 || sessionCfg.InitialRows != 0 {
		t.Fatalf("InitialCols/InitialRows = %d/%d, want zero before mux runtime sets pane size", sessionCfg.InitialCols, sessionCfg.InitialRows)
	}
}

func TestBuildLocalRcCommandExportsTERM(t *testing.T) {
	t.Setenv("TERM", "screen-256color")

	cmd := buildLocalRcCommand([]string{}, "", false, "")
	if !strings.Contains(cmd, "export TERM=screen-256color;") {
		t.Fatalf("buildLocalRcCommand() = %q, want TERM export", cmd)
	}
}

func TestShouldUseAWSNativeShellInMux(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native mux shell fallback is disabled on Windows")
	}

	if !shouldUseAWSNativeShellInMux(ssmconnector.ShellConfig{}) {
		t.Fatal("shouldUseAWSNativeShellInMux(start) = false, want true")
	}
	if !shouldUseAWSNativeShellInMux(ssmconnector.ShellConfig{Runtime: "plugin"}) {
		t.Fatal("shouldUseAWSNativeShellInMux(plugin start) = false, want true")
	}
	if shouldUseAWSNativeShellInMux(ssmconnector.ShellConfig{
		Runtime:       "plugin",
		SessionAction: "attach",
	}) {
		t.Fatal("shouldUseAWSNativeShellInMux(plugin attach) = true, want false")
	}
	if !shouldUseAWSNativeShellInMux(ssmconnector.ShellConfig{
		Runtime:       "native",
		SessionAction: "attach",
	}) {
		t.Fatal("shouldUseAWSNativeShellInMux(native attach) = false, want true")
	}
}

func TestResizeDeduperShouldSend(t *testing.T) {
	var d resizeDeduper

	if !d.ShouldSend(128, 27) {
		t.Fatal("first resize was dropped, want sent")
	}
	if d.ShouldSend(128, 27) {
		t.Fatal("duplicate resize was sent, want dropped")
	}
	if !d.ShouldSend(130, 32) {
		t.Fatal("changed resize was dropped, want sent")
	}
}

func TestDedupeResizeFunc(t *testing.T) {
	var calls [][2]int

	resize := dedupeResizeFunc(80, 24, func(cols, rows int) error {
		calls = append(calls, [2]int{cols, rows})
		return nil
	})

	if err := resize(80, 24); err != nil {
		t.Fatalf("resize(80,24) error = %v", err)
	}
	if err := resize(80, 24); err != nil {
		t.Fatalf("resize(80,24) duplicate error = %v", err)
	}
	if err := resize(100, 30); err != nil {
		t.Fatalf("resize(100,30) error = %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("resize call count = %d, want 1 changed-size call only", len(calls))
	}
	if calls[0] != ([2]int{100, 30}) {
		t.Fatalf("resize calls[0] = %v, want [100 30]", calls[0])
	}
}
