package ssmsession

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/blacknon/lssh/internal/connectorruntime"
	"github.com/blacknon/lssh/internal/providerbuiltin"
	"github.com/google/uuid"
	"github.com/kballard/go-shellquote"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	messageSchemaVersion = "1.0"
	serviceName          = "ssmmessages"
	clientVersion        = "lssh-native"
	sessionTypeStandard  = "Standard_Stream"
	sessionTypePort      = "Port"

	messageTypeInputStream  = "input_stream_data"
	messageTypeOutputStream = "output_stream_data"
	messageTypeAcknowledge  = "acknowledge"
	messageTypeChannelClose = "channel_closed"

	payloadTypeOutput            uint32 = 1
	payloadTypeSize              uint32 = 3
	payloadTypeHandshakeRequest  uint32 = 5
	payloadTypeHandshakeResponse uint32 = 6
	payloadTypeHandshakeComplete uint32 = 7
	payloadTypeFlag              uint32 = 10
	payloadTypeStdErr            uint32 = 11

	streamExitMarkerPrefix = "__LSSH_EXIT__:"
	startupMarkerWaitLimit = 3 * time.Second
	startupCommandDelay    = 200 * time.Millisecond
)

type Config struct {
	InstanceID             string
	Region                 string
	Platform               string
	Profile                string
	SharedConfigFiles      []string
	SharedCredentialsFiles []string
	DocumentName           string
	StartupCommand         string
	StartupMarker          string
	ExpandNewlineToCRLF    bool
	InitialCols            int
	InitialRows            int
	PortForwardHost        string
	PortForwardPort        string
	PortForwardLocalPort   string
}

type Session struct {
	cfg      Config
	clientID string

	mu   sync.Mutex
	conn *wsConn

	sizeMu         sync.Mutex
	pendingResize  connectorruntime.TerminalSize
	hasPendingSize bool

	ready           chan struct{}
	startupTrigger  chan struct{}
	readyOnce       sync.Once
	closeOnce       sync.Once
	readErr         chan error
	sequence        int64
	lastOutputSeq   int64
	startupMu       sync.Mutex
	startupSuppress bool
	startupMarker   string
	startupBuffer   bytes.Buffer
	startupSince    time.Time
	startupOnce     sync.Once
	sessionType     string
}

func (s *Session) markReady() {
	s.readyOnce.Do(func() { close(s.ready) })
}

func (s *Session) isReady() bool {
	select {
	case <-s.ready:
		return true
	default:
		return false
	}
}

type openDataChannelInput struct {
	MessageSchemaVersion string `json:"MessageSchemaVersion"`
	RequestID            string `json:"RequestId"`
	TokenValue           string `json:"TokenValue"`
	ClientID             string `json:"ClientId"`
	ClientVersion        string `json:"ClientVersion"`
}

type requestedClientAction struct {
	ActionType       string          `json:"ActionType"`
	ActionParameters json.RawMessage `json:"ActionParameters"`
}

type handshakeRequestPayload struct {
	AgentVersion           string                  `json:"AgentVersion"`
	RequestedClientActions []requestedClientAction `json:"RequestedClientActions"`
}

type sessionTypeRequest struct {
	SessionType string      `json:"SessionType"`
	Properties  interface{} `json:"Properties"`
}

type processedClientAction struct {
	ActionType   string      `json:"ActionType"`
	ActionStatus int         `json:"ActionStatus"`
	ActionResult interface{} `json:"ActionResult"`
	Error        string      `json:"Error"`
}

type handshakeResponsePayload struct {
	ClientVersion          string                  `json:"ClientVersion"`
	ProcessedClientActions []processedClientAction `json:"ProcessedClientActions"`
	Errors                 []string                `json:"Errors"`
}

type handshakeCompletePayload struct {
	CustomerMessage string `json:"CustomerMessage"`
}

type acknowledgeContent struct {
	MessageType         string `json:"AcknowledgedMessageType"`
	MessageID           string `json:"AcknowledgedMessageId"`
	SequenceNumber      int64  `json:"AcknowledgedMessageSequenceNumber"`
	IsSequentialMessage bool   `json:"IsSequentialMessage"`
}

type clientMessage struct {
	MessageType    string
	SchemaVersion  uint32
	CreatedDate    uint64
	SequenceNumber int64
	Flags          uint64
	MessageID      uuid.UUID
	PayloadType    uint32
	Payload        []byte
}

func New(cfg Config) *Session {
	return &Session{
		cfg:            cfg,
		clientID:       uuid.NewString(),
		ready:          make(chan struct{}),
		startupTrigger: make(chan struct{}),
		readErr:        make(chan error, 1),
		lastOutputSeq:  -1,
		startupMarker:  strings.TrimSpace(cfg.StartupMarker),
	}
}

func StartShell(ctx context.Context, cfg Config, stdin io.Reader, stdout, stderr io.Writer) error {
	session := New(cfg)
	defer session.Close()
	return session.Run(ctx, connectorruntime.StreamConfig{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
}

func (s *Session) Run(ctx context.Context, stream connectorruntime.StreamConfig) error {
	ssmCfg, err := loadAWSConfig(ctx, s.cfg)
	if err != nil {
		return err
	}

	startInput := &ssm.StartSessionInput{
		Target: aws.String(s.cfg.InstanceID),
	}
	if strings.TrimSpace(s.cfg.DocumentName) != "" {
		startInput.DocumentName = aws.String(s.cfg.DocumentName)
	}

	client := ssm.NewFromConfig(ssmCfg)
	startOut, err := client.StartSession(ctx, startInput)
	if err != nil {
		return fmt.Errorf("start ssm session: %w", err)
	}
	credentials, err := ssmCfg.Credentials.Retrieve(ctx)
	if err != nil {
		return fmt.Errorf("retrieve aws credentials for ssm websocket: %w", err)
	}

	if err := s.openWebSocket(ctx, aws.ToString(startOut.StreamUrl), aws.ToString(startOut.TokenValue), credentials); err != nil {
		return err
	}

	go s.readLoop(stream.Stdout, stream.Stderr)

	select {
	case <-s.ready:
	case err := <-s.readErr:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}

	_ = s.sendCurrentTerminalSize()
	if s.cfg.InitialCols > 0 && s.cfg.InitialRows > 0 {
		_ = s.sendSize(s.cfg.InitialCols, s.cfg.InitialRows)
	}
	_ = s.flushPendingResize()
	if strings.TrimSpace(s.cfg.StartupCommand) != "" {
		select {
		case <-s.startupTrigger:
		case <-time.After(startupMarkerWaitLimit):
		case <-ctx.Done():
			return ctx.Err()
		}
		select {
		case <-time.After(startupCommandDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
		if err := s.sendStartupCommand(s.cfg.StartupCommand); err != nil {
			return err
		}
	}

	if stream.Stdin != nil {
		go s.copyInput(stream.Stdin)
	}

	select {
	case err := <-s.readErr:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Session) Resize(ctx context.Context, size connectorruntime.TerminalSize) error {
	if !s.isReady() {
		s.storePendingResize(size)
		return nil
	}
	return s.sendSize(size.Cols, size.Rows)
}

func (s *Session) storePendingResize(size connectorruntime.TerminalSize) {
	if size.Cols <= 0 || size.Rows <= 0 {
		return
	}
	s.sizeMu.Lock()
	defer s.sizeMu.Unlock()
	s.pendingResize = size
	s.hasPendingSize = true
}

func (s *Session) flushPendingResize() error {
	s.sizeMu.Lock()
	if !s.hasPendingSize {
		s.sizeMu.Unlock()
		return nil
	}
	size := s.pendingResize
	s.hasPendingSize = false
	s.pendingResize = connectorruntime.TerminalSize{}
	s.sizeMu.Unlock()

	return s.sendSize(size.Cols, size.Rows)
}

func (s *Session) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.conn != nil {
			closeErr = s.conn.Close()
			s.conn = nil
		}
	})
	return closeErr
}

func (s *Session) openWebSocket(ctx context.Context, streamURL, token string, credentials aws.Credentials) error {
	header, err := signWebSocketHeaders(ctx, streamURL, s.cfg.Region, credentials)
	if err != nil {
		return err
	}

	conn, err := dialWebSocket(ctx, streamURL, header)
	if err != nil {
		return fmt.Errorf("open ssm websocket: %w", err)
	}

	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()

	openMessage := openDataChannelInput{
		MessageSchemaVersion: messageSchemaVersion,
		RequestID:            uuid.NewString(),
		TokenValue:           token,
		ClientID:             s.clientID,
		ClientVersion:        clientVersion,
	}
	body, err := json.Marshal(openMessage)
	if err != nil {
		return fmt.Errorf("marshal open data channel request: %w", err)
	}

	return s.sendText(body)
}

func (s *Session) copyInput(stdin io.Reader) {
	buf := make([]byte, 1024)
	for {
		n, err := stdin.Read(buf)
		if n > 0 {
			payload := normalizeInteractiveInputPayload(buf[:n], s.cfg.ExpandNewlineToCRLF)
			if sendErr := s.sendInputPayload(payloadTypeOutput, payload); sendErr != nil {
				select {
				case s.readErr <- sendErr:
				default:
				}
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				select {
				case s.readErr <- err:
				default:
				}
			}
			return
		}
	}
}

func normalizeInteractiveInputPayload(payload []byte, expandCRLF bool) []byte {
	if len(payload) == 0 {
		return nil
	}

	var normalized []byte
	for i, b := range payload {
		if expandCRLF {
			switch b {
			case '\r':
				if i+1 < len(payload) && payload[i+1] == '\n' {
					normalized = append(normalized, '\r')
				} else {
					normalized = append(normalized, '\r', '\n')
				}
				continue
			case '\n':
				if i > 0 && payload[i-1] == '\r' {
					normalized = append(normalized, '\n')
					continue
				}
				if i == 0 || payload[i-1] != '\r' {
					normalized = append(normalized, '\r', '\n')
					continue
				}
			}
		}
		if b == '\r' {
			if i+1 < len(payload) && payload[i+1] == '\n' {
				normalized = append(normalized, '\r')
				continue
			}
			normalized = append(normalized, '\r')
			continue
		}
		if b == '\n' && (i == 0 || payload[i-1] != '\r') {
			normalized = append(normalized, '\r')
			continue
		}
		if b == '\n' && i > 0 && payload[i-1] == '\r' {
			continue
		}
		normalized = append(normalized, b)
	}

	return normalized
}

func (s *Session) readLoop(stdout, stderr io.Writer) {
	for {
		raw, err := s.receive()
		if err != nil {
			if isNetworkClose(err) {
				s.readErr <- nil
			} else {
				s.readErr <- err
			}
			return
		}

		msg, err := decodeClientMessage(raw)
		if err != nil {
			s.readErr <- err
			return
		}

		switch msg.MessageType {
		case messageTypeOutputStream:
			if err := s.handleOutputMessage(msg, stdout, stderr); err != nil {
				s.readErr <- err
				return
			}
		case messageTypeAcknowledge:
			continue
		case messageTypeChannelClose:
			s.readErr <- nil
			return
		default:
			continue
		}
	}
}

func (s *Session) handleOutputMessage(msg clientMessage, stdout, stderr io.Writer) error {
	if err := s.sendAcknowledge(msg); err != nil {
		return err
	}
	if msg.SequenceNumber <= s.lastOutputSeq {
		return nil
	}
	s.lastOutputSeq = msg.SequenceNumber

	switch msg.PayloadType {
	case payloadTypeOutput:
		s.markStartupTrigger()
		s.markReady()
		payload := s.filterStartupOutput(msg.Payload)
		if len(payload) == 0 {
			return nil
		}
		if stdout != nil {
			_, _ = stdout.Write(payload)
		}
	case payloadTypeStdErr:
		s.markStartupTrigger()
		s.markReady()
		if stderr != nil {
			_, _ = stderr.Write(msg.Payload)
		}
	case payloadTypeFlag:
		return s.handleFlagMessage(msg.Payload)
	case payloadTypeHandshakeRequest:
		return s.handleHandshakeRequest(msg.Payload)
	case payloadTypeHandshakeComplete:
		return s.handleHandshakeComplete(msg.Payload, stdout)
	default:
		return nil
	}

	return nil
}

func (s *Session) handleHandshakeRequest(payload []byte) error {
	var request handshakeRequestPayload
	if err := json.Unmarshal(payload, &request); err != nil {
		return fmt.Errorf("decode handshake request: %w", err)
	}

	response := handshakeResponsePayload{
		ClientVersion:          clientVersion,
		ProcessedClientActions: make([]processedClientAction, 0, len(request.RequestedClientActions)),
	}

	for _, action := range request.RequestedClientActions {
		processed := processedClientAction{ActionType: action.ActionType}
		switch action.ActionType {
		case "SessionType":
			var sessionReq sessionTypeRequest
			if err := json.Unmarshal(action.ActionParameters, &sessionReq); err != nil {
				processed.ActionStatus = 2
				processed.Error = err.Error()
				response.Errors = append(response.Errors, err.Error())
			} else if sessionReq.SessionType != sessionTypeStandard && sessionReq.SessionType != sessionTypePort {
				processed.ActionStatus = 3
				processed.Error = fmt.Sprintf("unsupported session type %q", sessionReq.SessionType)
				response.Errors = append(response.Errors, processed.Error)
			} else {
				s.sessionType = sessionReq.SessionType
				processed.ActionStatus = 1
			}
		default:
			processed.ActionStatus = 3
			processed.Error = fmt.Sprintf("unsupported action %q", action.ActionType)
			response.Errors = append(response.Errors, processed.Error)
		}
		response.ProcessedClientActions = append(response.ProcessedClientActions, processed)
	}

	body, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("marshal handshake response: %w", err)
	}
	if err := s.sendInputPayload(payloadTypeHandshakeResponse, body); err != nil {
		return err
	}
	s.markReady()
	return nil
}

func (s *Session) handleHandshakeComplete(payload []byte, stdout io.Writer) error {
	var complete handshakeCompletePayload
	if err := json.Unmarshal(payload, &complete); err == nil {
		if complete.CustomerMessage != "" && stdout != nil && s.sessionType != sessionTypePort {
			_, _ = io.WriteString(stdout, complete.CustomerMessage+"\n")
		}
	}
	s.markReady()
	return nil
}

func (s *Session) sendAcknowledge(msg clientMessage) error {
	body, err := json.Marshal(acknowledgeContent{
		MessageType:         msg.MessageType,
		MessageID:           msg.MessageID.String(),
		SequenceNumber:      msg.SequenceNumber,
		IsSequentialMessage: true,
	})
	if err != nil {
		return fmt.Errorf("marshal acknowledge: %w", err)
	}

	ack := clientMessage{
		MessageType:   messageTypeAcknowledge,
		SchemaVersion: 1,
		CreatedDate:   uint64(time.Now().UnixMilli()),
		MessageID:     uuid.New(),
		PayloadType:   0,
		Payload:       body,
	}
	return s.sendBinary(encodeClientMessage(ack))
}

func (s *Session) sendInputPayload(payloadType uint32, payload []byte) error {
	s.sequence++
	msg := clientMessage{
		MessageType:    messageTypeInputStream,
		SchemaVersion:  1,
		CreatedDate:    uint64(time.Now().UnixMilli()),
		SequenceNumber: s.sequence - 1,
		MessageID:      uuid.New(),
		PayloadType:    payloadType,
		Payload:        payload,
	}
	return s.sendBinary(encodeClientMessage(msg))
}

func (s *Session) sendFlag(flag uint32) error {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, flag)
	return s.sendInputPayload(payloadTypeFlag, buf)
}

func (s *Session) sendStartupCommand(command string) error {
	const chunkSize = 512

	if s.startupMarker != "" {
		s.startStartupSuppress()
	}

	payload := []byte(command)
	for len(payload) > 0 {
		size := chunkSize
		if len(payload) < size {
			size = len(payload)
		}
		if err := s.sendInputPayload(payloadTypeOutput, payload[:size]); err != nil {
			return err
		}
		payload = payload[size:]
	}

	return s.sendInputPayload(payloadTypeOutput, []byte{'\r'})
}

func (s *Session) filterStartupOutput(payload []byte) []byte {
	s.startupMu.Lock()
	defer s.startupMu.Unlock()

	if !s.startupSuppress || s.startupMarker == "" {
		return payload
	}

	s.startupBuffer.Write(payload)
	if !s.startupSince.IsZero() && time.Since(s.startupSince) > startupMarkerWaitLimit {
		visible := append([]byte(nil), s.startupBuffer.Bytes()...)
		s.startupBuffer.Reset()
		s.startupSuppress = false
		s.startupSince = time.Time{}
		return visible
	}

	buffer := s.startupBuffer.String()
	index := strings.Index(buffer, s.startupMarker)
	if index < 0 {
		return nil
	}

	visible := buffer[index+len(s.startupMarker):]
	visible = strings.TrimLeft(visible, "\r\n")
	s.startupBuffer.Reset()
	s.startupSuppress = false
	s.startupSince = time.Time{}
	if visible == "" {
		return nil
	}

	return []byte(visible)
}

func (s *Session) startStartupSuppress() {
	s.startupMu.Lock()
	s.startupSuppress = true
	s.startupBuffer.Reset()
	s.startupSince = time.Now()
	s.startupMu.Unlock()

	go func() {
		timer := time.NewTimer(startupMarkerWaitLimit)
		defer timer.Stop()
		<-timer.C

		s.startupMu.Lock()
		defer s.startupMu.Unlock()
		if !s.startupSuppress {
			return
		}
		s.startupSuppress = false
		s.startupBuffer.Reset()
		s.startupSince = time.Time{}
	}()
}

func (s *Session) markStartupTrigger() {
	s.startupOnce.Do(func() {
		close(s.startupTrigger)
	})
}

func (s *Session) sendSize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	body, err := json.Marshal(map[string]uint32{
		"cols": uint32(cols),
		"rows": uint32(rows),
	})
	if err != nil {
		return fmt.Errorf("marshal size payload: %w", err)
	}
	return s.sendInputPayload(payloadTypeSize, body)
}

func (s *Session) sendCurrentTerminalSize() error {
	fd := 1
	if !terminal.IsTerminal(fd) {
		return nil
	}
	cols, rows, err := terminal.GetSize(fd)
	if err != nil {
		return nil
	}
	return s.sendSize(cols, rows)
}

func (s *Session) sendText(payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return fmt.Errorf("ssm websocket is closed")
	}
	return s.conn.WriteText(payload)
}

func (s *Session) sendBinary(payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return fmt.Errorf("ssm websocket is closed")
	}
	return s.conn.WriteBinary(payload)
}

func (s *Session) receive() ([]byte, error) {
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		return nil, io.EOF
	}
	return conn.ReadMessage()
}

func encodeClientMessage(msg clientMessage) []byte {
	const payloadOffset = 120
	const headerLength = 116
	payloadDigest := sha256.Sum256(msg.Payload)

	buf := make([]byte, payloadOffset+len(msg.Payload))
	binary.BigEndian.PutUint32(buf[0:4], headerLength)
	copy(buf[4:36], fixedMessageType(msg.MessageType))
	binary.BigEndian.PutUint32(buf[36:40], msg.SchemaVersion)
	binary.BigEndian.PutUint64(buf[40:48], msg.CreatedDate)
	binary.BigEndian.PutUint64(buf[48:56], uint64(msg.SequenceNumber))
	binary.BigEndian.PutUint64(buf[56:64], msg.Flags)
	copy(buf[64:80], encodeUUID(msg.MessageID))
	copy(buf[80:112], payloadDigest[:])
	binary.BigEndian.PutUint32(buf[112:116], msg.PayloadType)
	binary.BigEndian.PutUint32(buf[116:120], uint32(len(msg.Payload)))
	copy(buf[payloadOffset:], msg.Payload)
	return buf
}

func decodeClientMessage(raw []byte) (clientMessage, error) {
	const minimumMessageLength = 120
	if len(raw) < minimumMessageLength {
		return clientMessage{}, fmt.Errorf("invalid ssm client message length %d", len(raw))
	}

	header := binary.BigEndian.Uint32(raw[0:4])
	if int(header) > len(raw) || header < 116 {
		return clientMessage{}, fmt.Errorf("invalid ssm header length %d", header)
	}
	payloadLen := binary.BigEndian.Uint32(raw[116:120])
	payloadOffset := int(header) + 4
	if payloadOffset+int(payloadLen) > len(raw) {
		return clientMessage{}, fmt.Errorf("invalid ssm payload length %d", payloadLen)
	}

	messageID, err := uuid.FromBytes(raw[64:80])
	if err != nil {
		return clientMessage{}, fmt.Errorf("decode ssm message id: %w", err)
	}
	messageID = decodeUUID(messageID[:])

	return clientMessage{
		MessageType:    trimMessageType(raw[4:36]),
		SchemaVersion:  binary.BigEndian.Uint32(raw[36:40]),
		CreatedDate:    binary.BigEndian.Uint64(raw[40:48]),
		SequenceNumber: int64(binary.BigEndian.Uint64(raw[48:56])),
		Flags:          binary.BigEndian.Uint64(raw[56:64]),
		MessageID:      messageID,
		PayloadType:    binary.BigEndian.Uint32(raw[112:116]),
		Payload:        append([]byte(nil), raw[payloadOffset:payloadOffset+int(payloadLen)]...),
	}, nil
}

func fixedMessageType(value string) []byte {
	buf := make([]byte, 32)
	for i := range buf {
		buf[i] = ' '
	}
	copy(buf, []byte(value))
	return buf
}

func trimMessageType(value []byte) string {
	return strings.TrimRight(strings.TrimRight(string(value), "\x00"), " ")
}

func encodeUUID(id uuid.UUID) []byte {
	raw := id[:]
	result := make([]byte, 16)
	copy(result[0:8], raw[8:16])
	copy(result[8:16], raw[0:8])
	return result
}

func decodeUUID(raw []byte) uuid.UUID {
	decoded := make([]byte, 16)
	copy(decoded[0:8], raw[8:16])
	copy(decoded[8:16], raw[0:8])
	id, _ := uuid.FromBytes(decoded)
	return id
}

func signWebSocketHeaders(ctx context.Context, rawURL, region string, credentials aws.Credentials) (http.Header, error) {
	if isPresignedURL(rawURL) {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	signer := v4.NewSigner()
	emptyHash := sha256.Sum256(nil)
	payloadHash := hex.EncodeToString(emptyHash[:])
	if err := signer.SignHTTP(ctx, credentials, req, payloadHash, serviceName, region, time.Now()); err != nil {
		return nil, fmt.Errorf("sign ssm websocket request: %w", err)
	}
	return req.Header, nil
}

func isPresignedURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	query := parsed.Query()
	keys := []string{
		"X-Amz-Algorithm",
		"X-Amz-Credential",
		"X-Amz-Date",
		"X-Amz-Expires",
		"X-Amz-SignedHeaders",
		"X-Amz-Signature",
	}
	for _, key := range keys {
		if query.Get(key) != "" {
			return true
		}
	}
	return false
}

func loadAWSConfig(ctx context.Context, cfg Config) (aws.Config, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.Profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(cfg.Profile))
	}
	if files := providerbuiltin.ExpandPaths(cfg.SharedConfigFiles); len(files) > 0 {
		opts = append(opts, awsconfig.WithSharedConfigFiles(files))
	}
	if files := providerbuiltin.ExpandPaths(cfg.SharedCredentialsFiles); len(files) > 0 {
		opts = append(opts, awsconfig.WithSharedCredentialsFiles(files))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
}

func isNetworkClose(err error) bool {
	if err == nil || err == io.EOF {
		return true
	}
	return strings.Contains(err.Error(), "use of closed network connection") || strings.Contains(err.Error(), "EOF")
}

func RunStreamCommand(ctx context.Context, cfg Config, commandLine string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if strings.EqualFold(cfg.Platform, "windows") {
		return 0, fmt.Errorf("aws ssm native stream exec does not support windows yet")
	}
	if strings.TrimSpace(commandLine) == "" {
		return 0, fmt.Errorf("aws ssm native stream exec requires a command")
	}

	exitWriter := newStreamExitStatusWriter(stderr)
	session := New(cfg)
	session.cfg.StartupCommand = buildStreamCommandLine(commandLine)
	defer session.Close()

	err := session.Run(ctx, connectorruntime.StreamConfig{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: exitWriter,
	})
	if flushErr := exitWriter.Flush(); flushErr != nil && err == nil {
		err = flushErr
	}

	if code, ok := exitWriter.ExitCode(); ok {
		return code, err
	}
	if err != nil {
		return 1, err
	}
	return 0, nil
}

func buildStreamCommandLine(commandLine string) string {
	// Use non-login shell semantics so native SSM exec behaves closer to SSH command mode.
	wrapped := fmt.Sprintf("status=0; sh -c %s; status=$?; printf '%s%%s\\n' \"$status\" >&2; exit \"$status\"",
		shellSingleQuote(commandLine), streamExitMarkerPrefix)
	return wrapped
}

func BuildStreamCommandLine(commandLine string) string {
	return buildStreamCommandLine(commandLine)
}

type streamExitStatusWriter struct {
	dst      io.Writer
	buf      bytes.Buffer
	exitCode int
	hasCode  bool
}

func newStreamExitStatusWriter(dst io.Writer) *streamExitStatusWriter {
	return &streamExitStatusWriter{dst: dst}
}

func (w *streamExitStatusWriter) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			if err := w.flushLine(true); err != nil {
				return 0, err
			}
			continue
		}
		w.buf.WriteByte(b)
	}
	return len(p), nil
}

func (w *streamExitStatusWriter) Flush() error {
	return w.flushLine(false)
}

func (w *streamExitStatusWriter) ExitCode() (int, bool) {
	return w.exitCode, w.hasCode
}

func (w *streamExitStatusWriter) flushLine(hadNewline bool) error {
	if w.buf.Len() == 0 {
		return nil
	}

	line := w.buf.String()
	w.buf.Reset()
	if code, ok := parseStreamExitCode(line); ok {
		w.exitCode = code
		w.hasCode = true
		return nil
	}

	if w.dst == nil {
		return nil
	}

	if hadNewline {
		line += "\n"
	}
	_, err := io.WriteString(w.dst, line)
	return err
}

func parseStreamExitCode(line string) (int, bool) {
	if !strings.HasPrefix(line, streamExitMarkerPrefix) {
		return 0, false
	}
	value := strings.TrimSpace(strings.TrimPrefix(line, streamExitMarkerPrefix))
	if value == "" {
		return 0, false
	}
	var code int
	if _, err := fmt.Sscanf(value, "%d", &code); err != nil {
		return 0, false
	}
	return code, true
}

func shellSingleQuote(value string) string {
	return shellquote.Join(value)
}

type wsConn struct {
	conn net.Conn
	r    *bufio.Reader
	mu   sync.Mutex
}

func dialWebSocket(ctx context.Context, rawURL string, header http.Header) (*wsConn, error) {
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

	var conn net.Conn
	dialer := &net.Dialer{}
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
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected websocket status %s", resp.Status)
	}

	return &wsConn{conn: conn, r: reader}, nil
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

func (c *wsConn) WriteText(payload []byte) error {
	return c.writeFrame(0x1, payload)
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
