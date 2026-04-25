package wsman

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	nsSOAPEnvelope = "http://www.w3.org/2003/05/soap-envelope"
	nsAddressing   = "http://schemas.xmlsoap.org/ws/2004/08/addressing"
	nsTransfer     = "http://schemas.xmlsoap.org/ws/2004/09/transfer"
	nsWSMan        = "http://schemas.dmtf.org/wbem/wsman/1/wsman.xsd"
	nsWinShell     = "http://schemas.microsoft.com/wbem/wsman/1/windows/shell"
	nsWSManMS      = "http://schemas.microsoft.com/wbem/wsman/1/wsman.xsd"
)

type Endpoint struct {
	Scheme string
	Host   string
	Port   int
	Path   string
}

func (e Endpoint) URL() string {
	path := e.Path
	if strings.TrimSpace(path) == "" {
		path = "/wsman"
	}
	return fmt.Sprintf("%s://%s:%d%s", e.Scheme, e.Host, e.Port, path)
}

type Client struct {
	Endpoint       Endpoint
	Username       string
	Password       string
	InsecureTLS    bool
	OperationTimer time.Duration
	HTTPClient     *http.Client
	SessionID      uuid.UUID

	mu         sync.Mutex
	sequenceID uint64
}

type Selector struct {
	Name  string
	Value string
}

type Option struct {
	Name       string
	Value      string
	MustComply bool
}

type Header struct {
	Action      string
	ResourceURI string
	MessageID   string
	ReplyTo     string
	To          string
	Selectors   []Selector
	Options     []Option
	Operation   string
	Locale      string
	DataLocale  string
	MaxEnvelope int
	Timeout     time.Duration
	SessionID   string
	OperationID string
	SequenceID  uint64
	TrackOperation bool
}

func (c *Client) Do(ctx context.Context, header Header, body string) ([]byte, error) {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{
			Timeout:   c.timeout(),
			Transport: c.transport(),
		}
	}
	if header.To == "" {
		header.To = c.Endpoint.URL()
	}
	if header.ReplyTo == "" {
		header.ReplyTo = "http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous"
	}
	c.applySessionHeaders(&header)
	if header.Timeout <= 0 {
		header.Timeout = c.timeout()
	}

	envelope, err := BuildEnvelope(header, body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint.URL(), bytes.NewReader(envelope))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/soap+xml;charset=UTF-8")
	if c.Username != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("wsman http status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return payload, nil
}

func (c *Client) timeout() time.Duration {
	if c.OperationTimer > 0 {
		return c.OperationTimer
	}
	return 60 * time.Second
}

func (c *Client) transport() http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if c.InsecureTLS {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true
	}
	return transport
}

func BuildEnvelope(header Header, body string) ([]byte, error) {
	var envelope soapEnvelope
	envelope.XMLName = xml.Name{Space: nsSOAPEnvelope, Local: "Envelope"}
	envelope.NsS = nsSOAPEnvelope
	envelope.NsA = nsAddressing
	envelope.NsW = nsWSMan
	envelope.NsT = nsTransfer
	envelope.NsR = nsWinShell
	envelope.NsP = nsWSManMS
	envelope.Header = soapHeader{
		Action: soapText{Value: header.Action},
		To:     soapText{Value: header.To},
		ResourceURI: soapText{
			Value: header.ResourceURI,
		},
		ReplyTo: soapReplyTo{
			Address: soapText{Value: header.ReplyTo},
		},
		MessageID: soapText{Value: header.MessageID},
		MaxEnvelopeSize: soapMustUnderstandText{
			MustUnderstand: true,
			Value:          formatMaxEnvelopeSize(header.MaxEnvelope),
		},
		Locale: soapLocale{
			Lang:           defaultLocale(header.Locale),
			MustUnderstand: false,
		},
		DataLocale: soapLocale{
			Lang:           defaultLocale(header.DataLocale),
			MustUnderstand: false,
		},
		OperationTimeout: soapText{
			Value: formatOperationTimeout(header.Timeout),
		},
		SessionID: soapMustUnderstandText{
			MustUnderstand: false,
			Value:          header.SessionID,
		},
	}
	if header.TrackOperation {
		envelope.Header.OperationID = &soapMustUnderstandText{
			MustUnderstand: false,
			Value:          header.OperationID,
		}
		envelope.Header.SequenceID = &soapMustUnderstandText{
			MustUnderstand: false,
			Value:          fmt.Sprintf("%d", header.SequenceID),
		}
	}
	if len(header.Selectors) > 0 {
		envelope.Header.SelectorSet = &soapSelectorSet{Selectors: make([]soapSelector, 0, len(header.Selectors))}
		for _, selector := range header.Selectors {
			envelope.Header.SelectorSet.Selectors = append(envelope.Header.SelectorSet.Selectors, soapSelector{
				Name:  selector.Name,
				Value: selector.Value,
			})
		}
	}
	if len(header.Options) > 0 {
		envelope.Header.OptionSet = &soapOptionSet{Options: make([]soapOption, 0, len(header.Options))}
		for _, option := range header.Options {
			envelope.Header.OptionSet.Options = append(envelope.Header.OptionSet.Options, soapOption{
				Name:       option.Name,
				MustComply: option.MustComply,
				Value:      option.Value,
			})
		}
	}
	envelope.Body = soapBody{InnerXML: body}

	data, err := xml.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), data...), nil
}

type soapEnvelope struct {
	XMLName xml.Name   `xml:"s:Envelope"`
	NsS     string     `xml:"xmlns:s,attr"`
	NsA     string     `xml:"xmlns:a,attr"`
	NsT     string     `xml:"xmlns:t,attr"`
	NsW     string     `xml:"xmlns:w,attr"`
	NsR     string     `xml:"xmlns:r,attr"`
	NsP     string     `xml:"xmlns:p,attr"`
	Header  soapHeader `xml:"s:Header"`
	Body    soapBody   `xml:"s:Body"`
}

type soapHeader struct {
	Action           soapText             `xml:"a:Action"`
	To               soapText             `xml:"a:To"`
	ResourceURI      soapText             `xml:"w:ResourceURI"`
	ReplyTo          soapReplyTo          `xml:"a:ReplyTo"`
	MaxEnvelopeSize  soapMustUnderstandText `xml:"w:MaxEnvelopeSize"`
	MessageID        soapText             `xml:"a:MessageID"`
	Locale           soapLocale           `xml:"w:Locale"`
	DataLocale       soapLocale           `xml:"p:DataLocale"`
	SelectorSet      *soapSelectorSet     `xml:"w:SelectorSet,omitempty"`
	OptionSet        *soapOptionSet       `xml:"w:OptionSet,omitempty"`
	OperationTimeout soapText             `xml:"w:OperationTimeout"`
	SessionID        soapMustUnderstandText `xml:"p:SessionId"`
	OperationID      *soapMustUnderstandText `xml:"p:OperationID,omitempty"`
	SequenceID       *soapMustUnderstandText `xml:"p:SequenceId,omitempty"`
}

type soapReplyTo struct {
	Address soapText `xml:"a:Address"`
}

type soapText struct {
	Value string `xml:",chardata"`
}

type soapMustUnderstandText struct {
	MustUnderstand bool   `xml:"s:mustUnderstand,attr,omitempty"`
	Value          string `xml:",chardata"`
}

type soapLocale struct {
	Lang           string `xml:"xml:lang,attr"`
	MustUnderstand bool   `xml:"s:mustUnderstand,attr,omitempty"`
}

type soapSelectorSet struct {
	Selectors []soapSelector `xml:"w:Selector"`
}

type soapSelector struct {
	Name  string `xml:"Name,attr"`
	Value string `xml:",chardata"`
}

type soapOptionSet struct {
	Options []soapOption `xml:"w:Option"`
}

type soapOption struct {
	Name       string `xml:"Name,attr"`
	MustComply bool   `xml:"MustComply,attr,omitempty"`
	Value      string `xml:",chardata"`
}

type soapBody struct {
	InnerXML string `xml:",innerxml"`
}

func defaultLocale(locale string) string {
	if strings.TrimSpace(locale) == "" {
		return "en-US"
	}
	return locale
}

func formatMaxEnvelopeSize(size int) string {
	if size <= 0 {
		size = 512000
	}
	return fmt.Sprintf("%d", size)
}

func formatOperationTimeout(timeout time.Duration) string {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	seconds := timeout.Seconds()
	return fmt.Sprintf("PT%.3fS", seconds)
}

func (c *Client) applySessionHeaders(header *Header) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.SessionID == uuid.Nil {
		c.SessionID = uuid.New()
	}
	c.sequenceID++

	if header.SessionID == "" {
		header.SessionID = "uuid:" + c.SessionID.String()
	}
	if header.TrackOperation {
		if header.OperationID == "" {
			header.OperationID = "uuid:" + uuid.NewString()
		}
		if header.SequenceID == 0 {
			header.SequenceID = c.sequenceID
		}
	}
}
