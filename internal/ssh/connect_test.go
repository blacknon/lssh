package ssh

import (
	"strings"
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestCreateSshConnectRejectsExternalConnector(t *testing.T) {
	run := &Run{
		Conf: conf.Config{
			Server: map[string]conf.ServerConfig{
				"win01": {ConnectorName: "winrm"},
			},
		},
	}

	_, err := run.CreateSshConnect("win01")
	if err == nil {
		t.Fatal("CreateSshConnect() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `uses connector "winrm"`) {
		t.Fatalf("CreateSshConnect() error = %q, want connector-specific error", err)
	}
}
