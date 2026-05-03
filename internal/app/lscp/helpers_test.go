package lscp

import (
	"reflect"
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestSelectSCPServers(t *testing.T) {
	cfg := conf.Config{
		Server: map[string]conf.ServerConfig{
			"web01": {},
			"db01":  {},
		},
	}

	t.Run("explicit hosts", func(t *testing.T) {
		from, to, err := selectSCPServers([]string{"web01"}, []string{"web01", "db01"}, []string{"web01", "db01"}, cfg, false, true)
		if err != nil {
			t.Fatalf("selectSCPServers() error = %v", err)
		}
		if len(from) != 0 || !reflect.DeepEqual(to, []string{"web01"}) {
			t.Fatalf("from=%#v to=%#v", from, to)
		}
	})
}
