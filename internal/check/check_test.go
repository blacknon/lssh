package check

import "testing"

func TestExistServerRequiresAllHostsToExist(t *testing.T) {
	t.Parallel()

	if !ExistServer([]string{"web1", "web2"}, []string{"web1", "web2", "db1"}) {
		t.Fatalf("expected all existing hosts to pass")
	}

	if ExistServer([]string{"web1", "missing"}, []string{"web1", "web2", "db1"}) {
		t.Fatalf("expected missing host to fail validation")
	}
}
