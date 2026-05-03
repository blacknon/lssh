package lsftp

import (
	"reflect"
	"strings"
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestSelectSFTPServers(t *testing.T) {
	cfg := conf.Config{
		Server: map[string]conf.ServerConfig{
			"web01": {},
			"db01":  {},
		},
	}

	t.Run("explicit hosts", func(t *testing.T) {
		got, err := selectSFTPServers([]string{"web01"}, []string{"web01", "db01"}, []string{"web01", "db01"}, cfg, nil)
		if err != nil {
			t.Fatalf("selectSFTPServers() error = %v", err)
		}
		if !reflect.DeepEqual(got, []string{"web01"}) {
			t.Fatalf("selectSFTPServers() = %#v", got)
		}
	})

	t.Run("reject missing host", func(t *testing.T) {
		_, err := selectSFTPServers([]string{"missing"}, []string{"web01"}, []string{"web01"}, cfg, nil)
		if err == nil || !strings.Contains(err.Error(), "Input Server not found") {
			t.Fatalf("selectSFTPServers() error = %v", err)
		}
	})

	t.Run("prompt selection", func(t *testing.T) {
		got, err := selectSFTPServers(nil, []string{"web01"}, []string{"web01"}, cfg, func(_ string, _ []string, _ conf.Config, _ bool) ([]string, error) {
			return []string{"web01"}, nil
		})
		if err != nil {
			t.Fatalf("selectSFTPServers() error = %v", err)
		}
		if !reflect.DeepEqual(got, []string{"web01"}) {
			t.Fatalf("selectSFTPServers() = %#v", got)
		}
	})
}
