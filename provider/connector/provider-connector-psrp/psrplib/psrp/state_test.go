package psrp

import "testing"

func TestParseRunspacePoolStateData(t *testing.T) {
	raw := []byte(`<Obj RefId="0"><MS><Obj N="RunspaceState" RefId="1"><ToString>Opened</ToString><I32>2</I32></Obj></MS></Obj>`)

	info, err := ParseRunspacePoolStateData(raw)
	if err != nil {
		t.Fatalf("ParseRunspacePoolStateData() error = %v", err)
	}
	if info.State != RunspacePoolStateOpened {
		t.Fatalf("State = %v, want %v", info.State, RunspacePoolStateOpened)
	}
	if info.StateName != "Opened" {
		t.Fatalf("StateName = %q, want Opened", info.StateName)
	}
}

func TestRunspacePoolStateTerminal(t *testing.T) {
	if !RunspacePoolStateOpened.Terminal() {
		t.Fatal("Opened should be terminal for bootstrap wait")
	}
	if RunspacePoolStateOpening.Terminal() {
		t.Fatal("Opening should not be terminal for bootstrap wait")
	}
}
