package ssh

import (
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestConnectorLocalRCEnabledDefaultConfig(t *testing.T) {
	if !connectorLocalRCEnabled(&Run{}, conf.ServerConfig{LocalRcUse: "yes"}) {
		t.Fatal("connectorLocalRCEnabled() = false, want true when config enables localrc")
	}
}

func TestConnectorLocalRCEnabledRespectsNotLocalRC(t *testing.T) {
	if connectorLocalRCEnabled(&Run{IsNotBashrc: true}, conf.ServerConfig{LocalRcUse: "yes"}) {
		t.Fatal("connectorLocalRCEnabled() = true, want false with --not-localrc")
	}
}

func TestConnectorLocalRCEnabledRespectsLocalRCFlag(t *testing.T) {
	if !connectorLocalRCEnabled(&Run{IsBashrc: true}, conf.ServerConfig{LocalRcUse: "no"}) {
		t.Fatal("connectorLocalRCEnabled() = false, want true with --localrc")
	}
}
