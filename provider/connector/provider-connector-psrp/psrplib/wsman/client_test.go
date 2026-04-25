package wsman

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestEndpointURL(t *testing.T) {
	endpoint := Endpoint{Scheme: "https", Host: "host.example", Port: 5986}
	if got := endpoint.URL(); got != "https://host.example:5986/wsman" {
		t.Fatalf("URL() = %q, want %q", got, "https://host.example:5986/wsman")
	}
}

func TestBuildEnvelope(t *testing.T) {
	raw, err := BuildEnvelope(Header{
		Action:      "http://schemas.xmlsoap.org/ws/2004/09/transfer/Create",
		To:          "http://server:5985/wsman",
		ResourceURI: "http://schemas.microsoft.com/wbem/wsman/1/windows/shell/cmd",
		MessageID:   "uuid:test",
		ReplyTo:     "http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous",
		Selectors: []Selector{
			{Name: "ShellId", Value: "uuid:shell-id"},
		},
	}, "<r:CommandLine/>")
	if err != nil {
		t.Fatalf("BuildEnvelope() error = %v", err)
	}

	xml := string(raw)
	for _, fragment := range []string{
		`<a:Action>http://schemas.xmlsoap.org/ws/2004/09/transfer/Create</a:Action>`,
		`<w:ResourceURI>http://schemas.microsoft.com/wbem/wsman/1/windows/shell/cmd</w:ResourceURI>`,
		`<w:MaxEnvelopeSize`,
		`>512000</w:MaxEnvelopeSize>`,
		`<w:Locale xml:lang="en-US"></w:Locale>`,
		`<p:DataLocale xml:lang="en-US"></p:DataLocale>`,
		`<w:OperationTimeout>PT60.000S</w:OperationTimeout>`,
		`<p:SessionId`,
		`<w:Selector Name="ShellId">uuid:shell-id</w:Selector>`,
		`<r:CommandLine/>`,
	} {
		if !strings.Contains(xml, fragment) {
			t.Fatalf("envelope missing fragment %q in %s", fragment, xml)
		}
	}
}

func TestTransportInsecureTLS(t *testing.T) {
	client := Client{InsecureTLS: true}
	transport, ok := client.transport().(*http.Transport)
	if !ok {
		t.Fatalf("transport() did not return *http.Transport")
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig = nil, want non-nil")
	}
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify = false, want true")
	}
}

func TestTransportSecureTLSDefault(t *testing.T) {
	client := Client{}
	transport, ok := client.transport().(*http.Transport)
	if !ok {
		t.Fatalf("transport() did not return *http.Transport")
	}
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify = true, want false")
	}
}

func TestBuildEnvelopeWithOptionSet(t *testing.T) {
	raw, err := BuildEnvelope(Header{
		Action:      "http://schemas.xmlsoap.org/ws/2004/09/transfer/Create",
		To:          "http://server:5985/wsman",
		ResourceURI: "http://schemas.microsoft.com/powershell/Microsoft.PowerShell",
		MessageID:   "uuid:test",
		ReplyTo:     "http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous",
		SessionID:   "uuid:session-id",
		OperationID: "uuid:operation-id",
		SequenceID:  7,
		TrackOperation: true,
		Options: []Option{{
			Name:       "ProtocolVersion",
			Value:      "2.2",
			MustComply: true,
		}},
	}, "<r:Shell/>")
	if err != nil {
		t.Fatalf("BuildEnvelope() error = %v", err)
	}

	xml := string(raw)
	for _, fragment := range []string{
		`<w:Option Name="ProtocolVersion" MustComply="true">2.2</w:Option>`,
		`<w:OperationTimeout>PT60.000S</w:OperationTimeout>`,
		`<p:SessionId>uuid:session-id</p:SessionId>`,
		`<p:OperationID>uuid:operation-id</p:OperationID>`,
		`<p:SequenceId>7</p:SequenceId>`,
		`<r:Shell/>`,
	} {
		if !strings.Contains(xml, fragment) {
			t.Fatalf("envelope missing fragment %q in %s", fragment, xml)
		}
	}
}

func TestBuildEnvelopeUsesProvidedLocaleAndTimeout(t *testing.T) {
	raw, err := BuildEnvelope(Header{
		Action:      "http://schemas.xmlsoap.org/ws/2004/09/transfer/Create",
		To:          "http://server:5985/wsman",
		ResourceURI: "http://schemas.microsoft.com/powershell/Microsoft.PowerShell",
		MessageID:   "uuid:test",
		ReplyTo:     "http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous",
		Locale:      "ja-JP",
		DataLocale:  "ja-JP",
		Timeout:     90 * time.Second,
		MaxEnvelope: 153600,
	}, "<r:Shell/>")
	if err != nil {
		t.Fatalf("BuildEnvelope() error = %v", err)
	}

	xml := string(raw)
	for _, fragment := range []string{
		`<w:MaxEnvelopeSize`,
		`>153600</w:MaxEnvelopeSize>`,
		`<w:Locale xml:lang="ja-JP"></w:Locale>`,
		`<p:DataLocale xml:lang="ja-JP"></p:DataLocale>`,
		`<w:OperationTimeout>PT90.000S</w:OperationTimeout>`,
	} {
		if !strings.Contains(xml, fragment) {
			t.Fatalf("envelope missing fragment %q in %s", fragment, xml)
		}
	}
}
