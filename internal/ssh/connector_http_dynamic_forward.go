package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	conf "github.com/blacknon/lssh/internal/config"
)

func (r *Run) runConnectorHTTPDynamicPortForward(server string, config conf.ServerConfig, connectorName string) error {
	return runConnectorWithPrePost(config, func() error {
		return r.runConnectorHTTPDynamicPortForwardSession(context.Background(), server, config, connectorName, true)
	})
}

func (r *Run) runConnectorHTTPDynamicPortForwardSession(ctx context.Context, server string, config conf.ServerConfig, connectorName string, printHeader bool) error {
	port, err := connectorHTTPDynamicForwardSpec(config)
	if err != nil {
		return fmt.Errorf("server %q uses connector %q; %w", server, connectorName, err)
	}
	if r.X11 || r.X11Trusted {
		return fmt.Errorf("server %q uses connector %q; X11 forwarding is not supported with http dynamic port forwarding", server, connectorName)
	}
	if _, err := r.prepareConnectorDialTransport(server, "example.com", "80"); err != nil {
		return err
	}

	if printHeader {
		r.PrintSelectServer()
		r.printHTTPDynamicPortForward(port)
	}

	return runConnectorHTTPProxyServer(ctx, port, func(ctx context.Context, network, address string) (net.Conn, error) {
		return r.dialConnectorTarget(ctx, server, network, address)
	})
}

func connectorHTTPDynamicForwardSpec(config conf.ServerConfig) (string, error) {
	port := strings.TrimSpace(config.HTTPDynamicPortForward)
	if port == "" {
		return "", fmt.Errorf("http dynamic port forwarding is not configured")
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("http dynamic port forwarding port %q is invalid", port)
	}
	return port, nil
}

func runConnectorHTTPProxyServer(ctx context.Context, port string, dialContext func(ctx context.Context, network, address string) (net.Conn, error)) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("localhost", port))
	if err != nil {
		return err
	}
	notifyParentReady()

	server := newConnectorHTTPProxyServer(listener, dialContext)
	return server.serveUntilStop(ctx)
}

type connectorHTTPProxyServer struct {
	listener  net.Listener
	server    *http.Server
	transport *http.Transport

	mu     sync.Mutex
	active map[net.Conn]struct{}
}

func newConnectorHTTPProxyServer(listener net.Listener, dialContext func(ctx context.Context, network, address string) (net.Conn, error)) *connectorHTTPProxyServer {
	s := &connectorHTTPProxyServer{
		listener: listener,
		active:   map[net.Conn]struct{}{},
		transport: &http.Transport{
			DialContext:         dialContext,
			MaxIdleConns:        16,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	s.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				s.handleHTTPSProxy(w, r, dialContext)
				return
			}
			s.handleHTTPProxy(w, r, dialContext)
		}),
		ConnState: func(conn net.Conn, state http.ConnState) {
			s.trackConnState(conn, state)
		},
	}

	return s
}

func (s *connectorHTTPProxyServer) serveUntilStop(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.server.Serve(s.listener)
	}()

	select {
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) || isClosedNetworkError(err) {
			return nil
		}
		return err
	case <-ctx.Done():
		s.shutdown()
		err := <-errCh
		if err == nil || errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) || isClosedNetworkError(err) {
			return nil
		}
		return err
	}
}

func (s *connectorHTTPProxyServer) shutdown() {
	_ = s.server.Close()
	s.transport.CloseIdleConnections()

	s.mu.Lock()
	for conn := range s.active {
		_ = conn.Close()
	}
	s.mu.Unlock()
}

func (s *connectorHTTPProxyServer) trackConnState(conn net.Conn, state http.ConnState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch state {
	case http.StateNew, http.StateActive, http.StateIdle, http.StateHijacked:
		s.active[conn] = struct{}{}
	case http.StateClosed:
		delete(s.active, conn)
	}
}

func (s *connectorHTTPProxyServer) handleHTTPSProxy(w http.ResponseWriter, r *http.Request, dialContext func(ctx context.Context, network, address string) (net.Conn, error)) {
	dialCtx, cancel := context.WithCancel(r.Context())
	defer cancel()

	destConn, err := dialContext(dialCtx, "tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "http hijacking is not supported", http.StatusServiceUnavailable)
		_ = destConn.Close()
		return
	}

	clientConn, buf, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		_ = destConn.Close()
		return
	}

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n")); err != nil {
		_ = clientConn.Close()
		_ = destConn.Close()
		return
	}

	if buf.Reader.Buffered() > 0 {
		_, _ = io.Copy(destConn, buf)
	}

	_ = relayConnectorTCP(clientConn, destConn)
}

func (s *connectorHTTPProxyServer) handleHTTPProxy(w http.ResponseWriter, r *http.Request, dialContext func(ctx context.Context, network, address string) (net.Conn, error)) {
	r.RequestURI = ""
	if r.URL.Scheme == "" {
		r.URL.Scheme = "http"
	}
	if r.URL.Host == "" {
		r.URL.Host = r.Host
	}

	resp, err := s.transport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
