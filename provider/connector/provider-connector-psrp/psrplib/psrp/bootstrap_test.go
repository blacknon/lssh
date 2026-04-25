package psrp

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestBuildSessionCapabilityData(t *testing.T) {
	raw := string(BuildSessionCapabilityData("", "", ""))
	for _, want := range []string{
		`<Version N="protocolversion">2.3</Version>`,
		`<Version N="PSVersion">2.0</Version>`,
		`<Version N="SerializationVersion">1.1.0.1</Version>`,
		`<Nil N="TimeZone" />`,
		`<B N="MustComply">true</B>`,
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("session capability missing %q in %s", want, raw)
		}
	}
}

func TestBuildOpenRunspacePoolCreationXML(t *testing.T) {
	raw, err := BuildOpenRunspacePoolCreationXML(
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		DefaultRunspacePoolInit(),
	)
	if err != nil {
		t.Fatalf("BuildOpenRunspacePoolCreationXML() error = %v", err)
	}
	if !strings.Contains(string(raw), "<creationXml") {
		t.Fatalf("creation xml missing root element: %s", raw)
	}
}

func TestBuildInitRunspacePoolDataIncludesFullHostDefaultData(t *testing.T) {
	raw := string(BuildInitRunspacePoolData(DefaultRunspacePoolInit()))
	hostIndex := strings.Index(raw, `<Obj N="HostInfo" RefId="4">`)
	appArgsIndex := strings.Index(raw, `<Obj N="ApplicationArguments" RefId="3">`)
	if hostIndex == -1 {
		hostIndex = strings.Index(raw, `<Obj N="HostInfo" RefId="6">`)
	}
	if hostIndex == -1 || appArgsIndex == -1 {
		t.Fatalf("host/app args elements missing in %s", raw)
	}
	if appArgsIndex > hostIndex {
		t.Fatalf("ApplicationArguments should appear before HostInfo in %s", raw)
	}

	for _, want := range []string{
		`<Obj N="ApplicationArguments" RefId="3">`,
		`<T>System.Management.Automation.PSPrimitiveDictionary</T>`,
		`<S N="Key">PSVersionTable</S>`,
		`<S N="Key">PSVersion</S><Version N="Value">2.0</Version>`,
		`<S N="Key">PSEdition</S><S N="Value">Desktop</S>`,
		`<S N="Key">PSRemotingProtocolVersion</S><Version N="Value">2.3</Version>`,
		`<ToString>Unknown</ToString><I32>2</I32>`,
		`<I32 N="Key">0</I32>`,
		`<I32 N="Key">1</I32>`,
		`<I32 N="Key">2</I32>`,
		`<I32 N="Key">3</I32>`,
		`<I32 N="Key">4</I32>`,
		`<I32 N="Key">5</I32>`,
		`<I32 N="Key">6</I32>`,
		`<I32 N="Key">7</I32>`,
		`<I32 N="Key">8</I32>`,
		`<I32 N="Key">9</I32>`,
		`<S N="T">System.ConsoleColor</S>`,
		`<S N="T">System.Management.Automation.Host.Coordinates</S>`,
		`<S N="T">System.Management.Automation.Host.Size</S>`,
		`<S N="T">System.String</S>`,
		`<I32 N="y">17</I32>`,
		`<I32 N="height">6000</I32>`,
		`<I32 N="height">50</I32>`,
		`<I32 N="width">239</I32><I32 N="height">91</I32>`,
		`Administrator: Windows PowerShell`,
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("init runspace pool data missing %q in %s", want, raw)
		}
	}
}
