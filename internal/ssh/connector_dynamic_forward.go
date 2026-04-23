package ssh

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	conf "github.com/blacknon/lssh/internal/config"
	"github.com/blacknon/lssh/internal/providerapi"
)

func (r *Run) runConnectorDynamicPortForward(server string, config conf.ServerConfig, connectorName string) error {
	port, err := connectorDynamicForwardSpec(config)
	if err != nil {
		return fmt.Errorf("server %q uses connector %q; %w", server, connectorName, err)
	}
	if r.X11 || r.X11Trusted {
		return fmt.Errorf("server %q uses connector %q; X11 forwarding is not supported with dynamic port forwarding", server, connectorName)
	}
	if connectorName != "aws-ssm" {
		return fmt.Errorf("server %q uses connector %q; dynamic port forwarding (-D) is not supported yet", server, connectorName)
	}

	if _, err := r.prepareConnectorDialTransport(server, "example.com", "80"); err != nil {
		return err
	}

	r.PrintSelectServer()
	r.printDynamicPortForward(port)

	return runConnectorWithPrePost(config, func() error {
		return runConnectorSOCKS5ForwardServer(port, func(ctx context.Context, network, address string) (net.Conn, error) {
			return r.dialConnectorTarget(ctx, server, network, address)
		})
	})
}

func connectorDynamicForwardSpec(config conf.ServerConfig) (string, error) {
	port := strings.TrimSpace(config.DynamicPortForward)
	if port == "" {
		return "", fmt.Errorf("dynamic port forwarding is not configured")
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("dynamic port forwarding port %q is invalid", port)
	}
	return port, nil
}

func (r *Run) prepareConnectorDialTransport(server, targetHost, targetPort string) (providerapi.ConnectorPrepareResult, error) {
	operation := providerapi.ConnectorOperation{
		Name: "tcp_dial_transport",
		Options: map[string]interface{}{
			"target_host": targetHost,
			"target_port": targetPort,
		},
	}

	prepared, err := r.Conf.PrepareConnector(server, operation)
	if err != nil {
		return providerapi.ConnectorPrepareResult{}, err
	}

	connectorName := r.Conf.ServerConnectorName(server)
	planConnector := connectorNameFromPlan(prepared.Plan, connectorName)
	if !prepared.Supported {
		return providerapi.ConnectorPrepareResult{}, fmt.Errorf("server %q connector %q: %v", server, planConnector, prepared.Plan.Details["reason"])
	}

	return prepared, nil
}

func (r *Run) dialConnectorTarget(ctx context.Context, server, network, address string) (net.Conn, error) {
	if network == "" {
		network = "tcp"
	}
	if network != "tcp" {
		return nil, fmt.Errorf("connector dial transport supports only tcp network")
	}

	targetHost, targetPort, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid dial target %q: %w", address, err)
	}

	prepared, err := r.prepareConnectorDialTransport(server, targetHost, targetPort)
	if err != nil {
		return nil, err
	}

	planConnector := connectorNameFromPlan(prepared.Plan, r.Conf.ServerConnectorName(server))
	switch prepared.Plan.Kind {
	case "provider-managed":
		return dialProviderManagedTransport(prepared.Plan, planConnector)
	default:
		return nil, fmt.Errorf("server %q connector %q returned unsupported dial plan kind %q", server, planConnector, prepared.Plan.Kind)
	}
}

func runConnectorSOCKS5ForwardServer(port string, dialContext func(ctx context.Context, network, address string) (net.Conn, error)) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("localhost", port))
	if err != nil {
		return err
	}
	notifyParentReady()

	server := &connectorSOCKS5Server{
		listener:    listener,
		dialContext: dialContext,
		active:      map[net.Conn]struct{}{},
	}

	return server.serveUntilInterrupt()
}

type connectorSOCKS5Server struct {
	listener    net.Listener
	dialContext func(ctx context.Context, network, address string) (net.Conn, error)

	wg     sync.WaitGroup
	mu     sync.Mutex
	active map[net.Conn]struct{}
}

func (s *connectorSOCKS5Server) serveUntilInterrupt() error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.serve()
	}()

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	select {
	case err := <-errCh:
		return err
	case <-interrupts:
		s.shutdown()
		err := <-errCh
		if err == nil || errors.Is(err, net.ErrClosed) || isClosedNetworkError(err) {
			return nil
		}
		return err
	}
}

func (s *connectorSOCKS5Server) serve() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}

		s.trackConn(conn)
		s.wg.Add(1)
		go func(client net.Conn) {
			defer s.wg.Done()
			defer s.untrackConn(client)
			defer client.Close()

			_ = s.handleConn(client)
		}(conn)
	}
}

func (s *connectorSOCKS5Server) shutdown() {
	_ = s.listener.Close()

	s.mu.Lock()
	for conn := range s.active {
		_ = conn.Close()
	}
	s.mu.Unlock()

	s.wg.Wait()
}

func (s *connectorSOCKS5Server) trackConn(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[conn] = struct{}{}
}

func (s *connectorSOCKS5Server) untrackConn(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, conn)
}

func (s *connectorSOCKS5Server) handleConn(client net.Conn) error {
	reader := bufio.NewReader(client)

	if err := socks5NegotiateNoAuth(reader, client); err != nil {
		return err
	}

	targetAddress, err := socks5ReadConnectTarget(reader)
	if err != nil {
		_ = socks5WriteReply(client, 0x01)
		return err
	}

	debugConnectorSOCKS5f("dial start target=%s", targetAddress)
	dialCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	remote, err := s.dialContext(dialCtx, "tcp", targetAddress)
	if err != nil {
		debugConnectorSOCKS5f("dial error target=%s err=%v", targetAddress, err)
		_ = socks5WriteReply(client, 0x04)
		return err
	}
	defer remote.Close()
	debugConnectorSOCKS5f("dial ok target=%s", targetAddress)

	if err := socks5WriteReply(client, 0x00); err != nil {
		debugConnectorSOCKS5f("reply error target=%s err=%v", targetAddress, err)
		return err
	}
	debugConnectorSOCKS5f("reply ok target=%s", targetAddress)

	return relayConnectorSOCKS5(client, remote, reader)
}

func debugConnectorSOCKS5f(format string, args ...interface{}) {
	if os.Getenv("LSSH_DEBUG_AWS_SSM_DYN") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "[lssh socks5] "+format+"\n", args...)
}

func socks5NegotiateNoAuth(reader *bufio.Reader, client net.Conn) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return err
	}
	if header[0] != 0x05 {
		return fmt.Errorf("unsupported socks version %d", header[0])
	}

	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(reader, methods); err != nil {
		return err
	}

	supportsNoAuth := false
	for _, method := range methods {
		if method == 0x00 {
			supportsNoAuth = true
			break
		}
	}
	if !supportsNoAuth {
		_, _ = client.Write([]byte{0x05, 0xff})
		return fmt.Errorf("socks5 client does not support no-auth method")
	}

	_, err := client.Write([]byte{0x05, 0x00})
	return err
}

func socks5ReadConnectTarget(reader *bufio.Reader) (string, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return "", err
	}
	if header[0] != 0x05 {
		return "", fmt.Errorf("unsupported socks request version %d", header[0])
	}
	if header[1] != 0x01 {
		return "", fmt.Errorf("only socks5 CONNECT is supported")
	}

	host, err := socks5ReadAddress(reader, header[3])
	if err != nil {
		return "", err
	}

	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, portBytes); err != nil {
		return "", err
	}
	port := binary.BigEndian.Uint16(portBytes)
	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

func socks5ReadAddress(reader *bufio.Reader, atyp byte) (string, error) {
	switch atyp {
	case 0x01:
		buf := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return "", err
		}
		return net.IP(buf).String(), nil
	case 0x03:
		length, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		buf := make([]byte, int(length))
		if _, err := io.ReadFull(reader, buf); err != nil {
			return "", err
		}
		return string(buf), nil
	case 0x04:
		buf := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return "", err
		}
		return net.IP(buf).String(), nil
	default:
		return "", fmt.Errorf("unsupported socks address type %d", atyp)
	}
}

func socks5WriteReply(client net.Conn, status byte) error {
	_, err := client.Write([]byte{0x05, status, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	return err
}

func relayConnectorSOCKS5(client net.Conn, remote net.Conn, reader *bufio.Reader) error {
	errCh := make(chan error, 2)

	go func() {
		_, err := io.Copy(remote, io.MultiReader(reader, client))
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(client, remote)
		errCh <- err
	}()

	err := <-errCh
	_ = client.Close()
	_ = remote.Close()
	<-errCh
	if err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

func isClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "use of closed network connection")
}
