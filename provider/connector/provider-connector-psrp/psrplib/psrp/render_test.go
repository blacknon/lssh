package psrp

import "testing"

func TestRenderSerializedText(t *testing.T) {
	got := RenderSerializedText([]byte(`<Obj RefId="0"><MS><S N="Cmd">Get-Location</S></MS></Obj>`))
	if got != "Get-Location" {
		t.Fatalf("RenderSerializedText() = %q, want %q", got, "Get-Location")
	}
}
