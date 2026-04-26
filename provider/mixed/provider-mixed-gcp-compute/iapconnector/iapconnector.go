package iapconnector

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/blacknon/lssh/providerapi"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	iapWebsocketURL        = "wss://tunnel.cloudproxy.app/v4/connect"
	iapOrigin              = "bot:iap-tunneler"
	iapSubprotocol         = "relay.tunnel.cloudproxy.app"
	iapDefaultInterface    = "nic0"
	iapDefaultTokenScope   = "https://www.googleapis.com/auth/cloud-platform"
	iapTagConnectSuccess   = 0x0001
	iapTagReconnectSuccess = 0x0002
	iapTagData             = 0x0004
	iapTagAck              = 0x0007
	iapMaxDataFrameSize    = 16384
)

type Config struct {
	Project         string
	Zone            string
	InstanceName    string
	Interface       string
	TargetPort      string
	CredentialsFile string
	Scopes          []string
}

func ConfigFromPlan(plan providerapi.ConnectorPlan) (Config, error) {
	cfg := Config{
		Project:         detailString(plan.Details, "project"),
		Zone:            detailString(plan.Details, "zone"),
		InstanceName:    detailString(plan.Details, "instance_name"),
		Interface:       firstNonEmpty(detailString(plan.Details, "interface"), iapDefaultInterface),
		TargetPort:      detailString(plan.Details, "target_port"),
		CredentialsFile: detailString(plan.Details, "credentials_file"),
		Scopes:          detailStringSlice(plan.Details["scopes"]),
	}
	if cfg.Project == "" || cfg.Zone == "" || cfg.InstanceName == "" || cfg.TargetPort == "" {
		return Config{}, fmt.Errorf("gcp iap plan is missing project, zone, instance_name, or target_port")
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{iapDefaultTokenScope}
	}
	return cfg, nil
}

func DialTarget(ctx context.Context, cfg Config) (net.Conn, error) {
	rawURL, err := iapConnectURL(cfg)
	if err != nil {
		return nil, err
	}

	tokenSource, err := googleTokenSource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("load gcp credentials for iap tunnel: %w", err)
	}

	token, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("retrieve gcp access token for iap tunnel: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+token.AccessToken)
	headers.Set("Origin", iapOrigin)

	ws, err := dialWebSocket(ctx, rawURL, headers, []string{iapSubprotocol})
	if err != nil {
		return nil, fmt.Errorf("open gcp iap websocket: %w", err)
	}

	conn := newIAPConn(ws)
	if err := conn.waitForConnect(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func iapConnectURL(cfg Config) (string, error) {
	u, err := url.Parse(iapWebsocketURL)
	if err != nil {
		return "", err
	}

	query := u.Query()
	query.Set("project", cfg.Project)
	query.Set("zone", cfg.Zone)
	query.Set("instance", cfg.InstanceName)
	query.Set("interface", firstNonEmpty(cfg.Interface, iapDefaultInterface))
	query.Set("port", cfg.TargetPort)
	query.Set("newWebsocket", "true")
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func googleTokenSource(ctx context.Context, cfg Config) (oauth2.TokenSource, error) {
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{iapDefaultTokenScope}
	}

	if credentialsFile := strings.TrimSpace(cfg.CredentialsFile); credentialsFile != "" {
		data, err := os.ReadFile(credentialsFile)
		if err != nil {
			return nil, err
		}
		creds, err := google.CredentialsFromJSON(ctx, data, scopes...)
		if err != nil {
			return nil, err
		}
		return creds.TokenSource, nil
	}

	return google.DefaultTokenSource(ctx, scopes...)
}

type iapConn struct {
	ws *wsConn

	readMu    sync.Mutex
	readCond  *sync.Cond
	readBuf   []byte
	readErr   error
	closed    bool
	closeOnce sync.Once

	connectCh chan error
	writeMu   sync.Mutex
	ackMu     sync.Mutex
	ackedRead uint64
}

func newIAPConn(ws *wsConn) *iapConn {
	conn := &iapConn{
		ws:        ws,
		connectCh: make(chan error, 1),
	}
	conn.readCond = sync.NewCond(&conn.readMu)
	go conn.readLoop()
	return conn
}

func (c *iapConn) waitForConnect(ctx context.Context) error {
	select {
	case err := <-c.connectCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *iapConn) Read(p []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	for len(c.readBuf) == 0 && c.readErr == nil {
		c.readCond.Wait()
	}
	if len(c.readBuf) == 0 && c.readErr != nil {
		return 0, c.readErr
	}

	n := copy(p, c.readBuf)
	c.readBuf = c.readBuf[n:]
	return n, nil
}

func (c *iapConn) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		chunkSize := len(p)
		if chunkSize > iapMaxDataFrameSize {
			chunkSize = iapMaxDataFrameSize
		}
		chunk := p[:chunkSize]
		frame := createIAPDataFrame(chunk)

		c.writeMu.Lock()
		err := c.ws.WriteBinary(frame)
		c.writeMu.Unlock()
		if err != nil {
			return total, err
		}

		total += len(chunk)
		p = p[chunkSize:]
	}
	return total, nil
}

func (c *iapConn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.readMu.Lock()
		c.closed = true
		if c.readErr == nil {
			c.readErr = io.EOF
		}
		c.readCond.Broadcast()
		c.readMu.Unlock()
		err = c.ws.Close()
	})
	return err
}

func (c *iapConn) LocalAddr() net.Addr  { return dummyAddr("gcp-iap-local") }
func (c *iapConn) RemoteAddr() net.Addr { return dummyAddr("gcp-iap-remote") }

func (c *iapConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *iapConn) SetReadDeadline(t time.Time) error  { return c.ws.SetReadDeadline(t) }
func (c *iapConn) SetWriteDeadline(t time.Time) error { return c.ws.SetWriteDeadline(t) }

func (c *iapConn) readLoop() {
	defer c.readCond.Broadcast()

	for {
		msg, err := c.ws.ReadMessage()
		if err != nil {
			c.signalReadErr(err)
			c.signalConnect(err)
			return
		}

		if len(msg) < 2 {
			continue
		}

		tag := binary.BigEndian.Uint16(msg[:2])
		payload := msg[2:]

		switch tag {
		case iapTagConnectSuccess:
			c.signalConnect(nil)
		case iapTagReconnectSuccess:
			c.signalConnect(nil)
		case iapTagData:
			data, err := extractIAPData(payload)
			if err != nil {
				c.signalReadErr(err)
				c.signalConnect(err)
				return
			}
			c.pushReadData(data)
			if err := c.sendAck(uint64(len(data))); err != nil {
				c.signalReadErr(err)
				return
			}
		case iapTagAck:
			continue
		default:
			continue
		}
	}
}

func (c *iapConn) signalConnect(err error) {
	select {
	case c.connectCh <- err:
	default:
	}
}

func (c *iapConn) pushReadData(data []byte) {
	c.readMu.Lock()
	c.readBuf = append(c.readBuf, data...)
	c.readCond.Broadcast()
	c.readMu.Unlock()
}

func (c *iapConn) signalReadErr(err error) {
	c.readMu.Lock()
	if c.readErr == nil {
		c.readErr = err
	}
	c.readCond.Broadcast()
	c.readMu.Unlock()
}

func (c *iapConn) sendAck(delta uint64) error {
	c.ackMu.Lock()
	defer c.ackMu.Unlock()

	c.ackedRead += delta
	frame := createIAPAckFrame(c.ackedRead)

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.ws.WriteBinary(frame)
}

func createIAPDataFrame(data []byte) []byte {
	frame := make([]byte, 2+4+len(data))
	binary.BigEndian.PutUint16(frame[0:2], iapTagData)
	binary.BigEndian.PutUint32(frame[2:6], uint32(len(data)))
	copy(frame[6:], data)
	return frame
}

func createIAPAckFrame(total uint64) []byte {
	frame := make([]byte, 2+8)
	binary.BigEndian.PutUint16(frame[0:2], iapTagAck)
	binary.BigEndian.PutUint64(frame[2:10], total)
	return frame
}

func extractIAPData(payload []byte) ([]byte, error) {
	if len(payload) < 4 {
		return nil, fmt.Errorf("gcp iap data frame is too short")
	}
	size := binary.BigEndian.Uint32(payload[:4])
	if int(size) > len(payload[4:]) {
		return nil, fmt.Errorf("gcp iap data frame is truncated")
	}
	return append([]byte(nil), payload[4:4+size]...), nil
}

type wsConn struct {
	conn net.Conn
	r    *bufio.Reader
	mu   sync.Mutex
}

func dialWebSocket(ctx context.Context, rawURL string, header http.Header, subprotocols []string) (*wsConn, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	host := parsed.Host
	if !strings.Contains(host, ":") {
		switch parsed.Scheme {
		case "wss":
			host += ":443"
		default:
			host += ":80"
		}
	}

	dialer := &net.Dialer{}
	var conn net.Conn
	switch parsed.Scheme {
	case "wss":
		conn, err = tls.DialWithDialer(dialer, "tcp", host, &tls.Config{
			ServerName: parsed.Hostname(),
		})
	case "ws":
		conn, err = dialer.DialContext(ctx, "tcp", host)
	default:
		return nil, fmt.Errorf("unsupported websocket scheme %q", parsed.Scheme)
	}
	if err != nil {
		return nil, err
	}

	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		_ = conn.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	req.Header = make(http.Header)
	for k, values := range header {
		for _, value := range values {
			req.Header.Add(k, value)
		}
	}
	req.Header.Set("Host", parsed.Host)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", key)
	if len(subprotocols) > 0 {
		req.Header.Set("Sec-WebSocket-Protocol", strings.Join(subprotocols, ", "))
	}

	if err := req.Write(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected websocket status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	expectedAccept := websocketAcceptKey(key)
	if !strings.EqualFold(resp.Header.Get("Sec-WebSocket-Accept"), expectedAccept) {
		_ = resp.Body.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("invalid websocket accept header")
	}

	if len(subprotocols) > 0 {
		serverProtocol := strings.TrimSpace(resp.Header.Get("Sec-WebSocket-Protocol"))
		if serverProtocol != "" && serverProtocol != subprotocols[0] {
			_ = resp.Body.Close()
			_ = conn.Close()
			return nil, fmt.Errorf("unexpected websocket subprotocol %q", serverProtocol)
		}
	}

	return &wsConn{conn: conn, r: reader}, nil
}

func websocketAcceptKey(key string) string {
	h := sha1.New()
	_, _ = io.WriteString(h, key+"258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (c *wsConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

func (c *wsConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return io.EOF
	}
	return c.conn.SetReadDeadline(t)
}

func (c *wsConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return io.EOF
	}
	return c.conn.SetWriteDeadline(t)
}

func (c *wsConn) WriteBinary(payload []byte) error {
	return c.writeFrame(0x2, payload)
}

func (c *wsConn) writeControl(opcode byte, payload []byte) error {
	return c.writeFrame(opcode, payload)
}

func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return io.EOF
	}

	header := []byte{0x80 | opcode}
	maskBit := byte(0x80)
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, maskBit|byte(length))
	case length <= 65535:
		header = append(header, maskBit|126, byte(length>>8), byte(length))
	default:
		header = append(header, maskBit|127,
			byte(uint64(length)>>56), byte(uint64(length)>>48), byte(uint64(length)>>40), byte(uint64(length)>>32),
			byte(uint64(length)>>24), byte(uint64(length)>>16), byte(uint64(length)>>8), byte(uint64(length)))
	}

	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	frame := append(header, mask...)
	masked := append([]byte(nil), payload...)
	for i := range masked {
		masked[i] ^= mask[i%4]
	}
	frame = append(frame, masked...)
	_, err := c.conn.Write(frame)
	return err
}

func (c *wsConn) ReadMessage() ([]byte, error) {
	for {
		first, err := c.r.ReadByte()
		if err != nil {
			return nil, err
		}
		second, err := c.r.ReadByte()
		if err != nil {
			return nil, err
		}

		fin := first&0x80 != 0
		opcode := first & 0x0f
		masked := second&0x80 != 0
		length := int(second & 0x7f)
		switch length {
		case 126:
			var size uint16
			if err := binary.Read(c.r, binary.BigEndian, &size); err != nil {
				return nil, err
			}
			length = int(size)
		case 127:
			var size uint64
			if err := binary.Read(c.r, binary.BigEndian, &size); err != nil {
				return nil, err
			}
			length = int(size)
		}

		var mask []byte
		if masked {
			mask = make([]byte, 4)
			if _, err := io.ReadFull(c.r, mask); err != nil {
				return nil, err
			}
		}

		payload := make([]byte, length)
		if _, err := io.ReadFull(c.r, payload); err != nil {
			return nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}

		if !fin {
			return nil, fmt.Errorf("fragmented websocket frames are not supported")
		}

		switch opcode {
		case 0x1, 0x2:
			return payload, nil
		case 0x8:
			return nil, io.EOF
		case 0x9:
			if err := c.writeControl(0xA, payload); err != nil {
				return nil, err
			}
		case 0xA:
			continue
		default:
			continue
		}
	}
}

func detailString(details map[string]interface{}, key string) string {
	if details == nil {
		return ""
	}
	if value, ok := details[key]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return ""
}

func detailStringSlice(raw interface{}) []string {
	values, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type dummyAddr string

func (a dummyAddr) Network() string { return "gcp-iap" }
func (a dummyAddr) String() string  { return string(a) }
