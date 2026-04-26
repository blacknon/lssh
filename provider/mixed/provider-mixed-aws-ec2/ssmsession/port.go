package ssmsession

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const (
	portFlagDisconnectToPort uint32 = 1
	portFlagTerminateSession uint32 = 2
	portFlagConnectError     uint32 = 3
)

type nativePortConn struct {
	net.Conn

	session   *Session
	closeOnce sync.Once
}

func (c *nativePortConn) Close() error {
	err := c.Conn.Close()
	c.closeOnce.Do(func() {
		if c.session != nil {
			_ = c.session.sendFlag(portFlagDisconnectToPort)
			_ = c.session.Close()
		}
	})
	return err
}

func DialTarget(ctx context.Context, cfg Config) (net.Conn, error) {
	session := New(cfg)
	session.sessionType = sessionTypePort
	debugPortSessionf("dial begin instance=%s target=%s:%s document=%s", cfg.InstanceID, cfg.PortForwardHost, cfg.PortForwardPort, cfg.DocumentName)

	ssmCfg, err := loadAWSConfig(ctx, cfg)
	if err != nil {
		debugPortSessionf("loadAWSConfig error err=%v", err)
		return nil, err
	}

	startInput := &ssm.StartSessionInput{
		Target: aws.String(cfg.InstanceID),
	}
	documentName := strings.TrimSpace(cfg.DocumentName)
	if documentName == "" {
		documentName = "AWS-StartPortForwardingSessionToRemoteHost"
	}
	startInput.DocumentName = aws.String(documentName)
	startInput.Parameters = buildPortForwardParameters(cfg)

	client := ssm.NewFromConfig(ssmCfg)
	startOut, err := client.StartSession(ctx, startInput)
	if err != nil {
		debugPortSessionf("StartSession error err=%v", err)
		return nil, fmt.Errorf("start ssm port session: %w", err)
	}
	debugPortSessionf("StartSession ok session_id=%s", aws.ToString(startOut.SessionId))
	credentials, err := ssmCfg.Credentials.Retrieve(ctx)
	if err != nil {
		debugPortSessionf("Credentials error err=%v", err)
		return nil, fmt.Errorf("retrieve aws credentials for ssm websocket: %w", err)
	}

	if err := session.openWebSocket(ctx, aws.ToString(startOut.StreamUrl), aws.ToString(startOut.TokenValue), credentials); err != nil {
		debugPortSessionf("openWebSocket error err=%v", err)
		return nil, err
	}
	debugPortSessionf("openWebSocket ok")

	return startPortSessionRelay(ctx, session), nil
}

func startPortSessionRelay(ctx context.Context, session *Session) net.Conn {
	localConn, sessionConn := net.Pipe()
	go session.readLoop(sessionConn, nil)
	go session.copyPortInput(sessionConn)

	go func() {
		select {
		case <-ctx.Done():
			_ = sessionConn.Close()
			_ = localConn.Close()
			_ = session.Close()
		case <-session.readErr:
			_ = sessionConn.Close()
			_ = localConn.Close()
			_ = session.Close()
		}
	}()

	return &nativePortConn{
		Conn:    localConn,
		session: session,
	}
}

func buildPortForwardParameters(cfg Config) map[string][]string {
	localPort := strings.TrimSpace(cfg.PortForwardLocalPort)
	if localPort == "" {
		localPort = "0"
	}

	return map[string][]string{
		"host":            {cfg.PortForwardHost},
		"portNumber":      {cfg.PortForwardPort},
		"localPortNumber": {localPort},
	}
}

func (s *Session) copyPortInput(localConn net.Conn) {
	buf := make([]byte, 32*1024)
	for {
		n, err := localConn.Read(buf)
		if n > 0 {
			debugPortSessionf("copyPortInput read bytes=%d", n)
			payload := append([]byte(nil), buf[:n]...)
			if sendErr := s.sendInputPayload(payloadTypeOutput, payload); sendErr != nil {
				debugPortSessionf("copyPortInput send error err=%v", sendErr)
				s.signalReadError(sendErr)
				return
			}
		}
		if err != nil {
			if err == io.EOF {
				debugPortSessionf("copyPortInput eof")
				_ = s.sendFlag(portFlagDisconnectToPort)
				return
			}
			debugPortSessionf("copyPortInput read error err=%v", err)
			s.signalReadError(err)
			return
		}
	}
}

func (s *Session) handleFlagMessage(payload []byte) error {
	if len(payload) < 4 {
		return nil
	}

	flag := binary.BigEndian.Uint32(payload[:4])
	switch flag {
	case portFlagConnectError:
		debugPortSessionf("flag connect error")
		return fmt.Errorf("aws ssm port session reported connect-to-port error")
	default:
		debugPortSessionf("flag received value=%d", flag)
		return nil
	}
}

func (s *Session) signalReadError(err error) {
	select {
	case s.readErr <- err:
	default:
	}
}

func debugPortSessionf(format string, args ...interface{}) {
	if os.Getenv("LSSH_DEBUG_AWS_SSM_DYN") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "[lssh ssm-port] "+format+"\n", args...)
}
