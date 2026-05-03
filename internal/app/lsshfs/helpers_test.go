package lsshfs

import (
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestSelectMountHost(t *testing.T) {
	cfg := conf.Config{
		Server: map[string]conf.ServerConfig{
			"web01": {},
			"db01":  {},
		},
	}
	names := []string{"web01", "db01"}

	host, appendFlag, err := selectMountHost([]string{"web01"}, names, cfg, "")
	if err != nil {
		t.Fatalf("selectMountHost() error = %v", err)
	}
	if host != "web01" || appendFlag {
		t.Fatalf("selectMountHost() = host:%q append:%t", host, appendFlag)
	}

	host, appendFlag, err = selectMountHost(nil, names, cfg, "db01")
	if err != nil {
		t.Fatalf("selectMountHost() error = %v", err)
	}
	if host != "db01" || appendFlag {
		t.Fatalf("selectMountHost() = host:%q append:%t", host, appendFlag)
	}
}
