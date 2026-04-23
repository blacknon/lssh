package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"

	"github.com/blacknon/lssh/internal/providerapi"
	ssmconnector "github.com/blacknon/lssh/provider/mixed/provider-mixed-aws-ec2/ssmconnector"
)

func runNativeSSMLocalPortForward(plan providerapi.ConnectorPlan, cfg ssmconnector.PortForwardLocalConfig) error {
	listenAddress := net.JoinHostPort(cfg.ListenHost, cfg.ListenPort)
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return err
	}
	notifyParentReady()

	server := &connectorTCPForwardServer{
		listener: listener,
		dialContext: func(ctx context.Context) (net.Conn, error) {
			return dialProviderManagedTransport(plan, "aws-ssm")
		},
		active: map[net.Conn]struct{}{},
	}

	return server.serveUntilInterrupt()
}

type connectorTCPForwardServer struct {
	listener    net.Listener
	dialContext func(ctx context.Context) (net.Conn, error)

	wg     sync.WaitGroup
	mu     sync.Mutex
	active map[net.Conn]struct{}
}

func (s *connectorTCPForwardServer) serveUntilInterrupt() error {
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

func (s *connectorTCPForwardServer) serve() error {
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

			remote, err := s.dialContext(context.Background())
			if err != nil {
				return
			}
			defer remote.Close()

			_ = relayConnectorTCP(client, remote)
		}(conn)
	}
}

func (s *connectorTCPForwardServer) shutdown() {
	_ = s.listener.Close()

	s.mu.Lock()
	for conn := range s.active {
		_ = conn.Close()
	}
	s.mu.Unlock()

	s.wg.Wait()
}

func (s *connectorTCPForwardServer) trackConn(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[conn] = struct{}{}
}

func (s *connectorTCPForwardServer) untrackConn(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, conn)
}

func relayConnectorTCP(client net.Conn, remote net.Conn) error {
	errCh := make(chan error, 2)

	go func() {
		_, err := io.Copy(remote, client)
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
		return fmt.Errorf("relay connector tcp stream: %w", err)
	}
	return nil
}
