package apputil

import (
	"reflect"
	"strings"
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestSelectOperationHosts(t *testing.T) {
	cfg := conf.Config{
		Server: map[string]conf.ServerConfig{
			"web01": {},
			"db01":  {},
		},
	}

	t.Run("explicit hosts", func(t *testing.T) {
		got, err := SelectOperationHosts([]string{"web01"}, []string{"web01", "db01"}, []string{"web01", "db01"}, cfg, "sftp_transport", "no servers", "unsupported", "prompt>>", true, nil)
		if err != nil {
			t.Fatalf("SelectOperationHosts() error = %v", err)
		}
		if !reflect.DeepEqual(got, []string{"web01"}) {
			t.Fatalf("SelectOperationHosts() = %#v", got)
		}
	})

	t.Run("reject missing host", func(t *testing.T) {
		_, err := SelectOperationHosts([]string{"missing"}, []string{"web01"}, []string{"web01"}, cfg, "sftp_transport", "no servers", "unsupported", "prompt>>", true, nil)
		if err == nil || !strings.Contains(err.Error(), "Input Server not found") {
			t.Fatalf("SelectOperationHosts() error = %v", err)
		}
	})

	t.Run("prompt selection", func(t *testing.T) {
		got, err := SelectOperationHosts(nil, []string{"web01"}, []string{"web01"}, cfg, "sftp_transport", "no servers", "unsupported", "prompt>>", true, func(_ string, _ []string, _ conf.Config, _ bool) ([]string, error) {
			return []string{"web01"}, nil
		})
		if err != nil {
			t.Fatalf("SelectOperationHosts() error = %v", err)
		}
		if !reflect.DeepEqual(got, []string{"web01"}) {
			t.Fatalf("SelectOperationHosts() = %#v", got)
		}
	})
}

func TestValidateTransferCopyTypes(t *testing.T) {
	t.Run("mixed", func(t *testing.T) {
		if err := ValidateTransferCopyTypes(true, true, false, 0); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("valid", func(t *testing.T) {
		if err := ValidateTransferCopyTypes(true, false, false, 0); err != nil {
			t.Fatalf("ValidateTransferCopyTypes() error = %v", err)
		}
	})
}
