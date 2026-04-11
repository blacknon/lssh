package lsdiff

import (
	"strings"
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestResolveTargetsExplicitModeRequiresTwoTargets(t *testing.T) {
	_, err := ResolveTargets(conf.Config{}, []string{"@web01:/etc/hosts"})
	if err == nil || !strings.Contains(err.Error(), "at least two targets") {
		t.Fatalf("ResolveTargets() error = %v", err)
	}
}

func TestResolveTargetsExplicitModeSkipsSelector(t *testing.T) {
	targets, err := ResolveTargets(conf.Config{}, []string{"@web01:/etc/hosts", "@web02:/etc/hosts"})
	if err != nil {
		t.Fatalf("ResolveTargets() error = %v", err)
	}
	if len(targets) != 2 || targets[0].Host != "web01" || targets[1].Host != "web02" {
		t.Fatalf("targets = %#v", targets)
	}
}

func TestResolveTargetsRejectsMixedCommonAndExplicitArgs(t *testing.T) {
	_, err := ResolveTargets(conf.Config{}, []string{"/etc/hosts", "@web02:/etc/hosts"})
	if err == nil || !strings.Contains(err.Error(), "single common remote path") {
		t.Fatalf("ResolveTargets() error = %v", err)
	}
}
