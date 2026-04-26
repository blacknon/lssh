package ssh

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	conf "github.com/blacknon/lssh/internal/config"
)

func TestConnectorHTTPDynamicForwardSpecRejectsInvalidPort(t *testing.T) {
	_, err := connectorHTTPDynamicForwardSpec(conf.ServerConfig{
		HTTPDynamicPortForward: "localhost",
	})
	if err == nil {
		t.Fatal("connectorHTTPDynamicForwardSpec() error = nil, want non-nil")
	}
}

func TestConnectorHTTPProxyServerReusesTransportConnections(t *testing.T) {
	backendListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen(backend) error = %v", err)
	}
	defer backendListener.Close()

	backendServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "ok")
		}),
	}
	defer backendServer.Close()
	go func() {
		_ = backendServer.Serve(backendListener)
	}()

	var dialCount int32
	dialContext := func(ctx context.Context, network, address string) (net.Conn, error) {
		if !strings.HasPrefix(address, "example.test:") {
			t.Fatalf("dial address = %q, want example.test host", address)
		}
		atomic.AddInt32(&dialCount, 1)
		var d net.Dialer
		return d.DialContext(ctx, network, backendListener.Addr().String())
	}

	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen(proxy) error = %v", err)
	}
	defer proxyListener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proxyServer := newConnectorHTTPProxyServer(proxyListener, dialContext)
	go func() {
		_ = proxyServer.serveUntilStop(ctx)
	}()

	proxyURL, err := url.Parse("http://" + proxyListener.Addr().String())
	if err != nil {
		t.Fatalf("url.Parse(proxy) error = %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
	defer client.CloseIdleConnections()

	for i := 0; i < 2; i++ {
		resp, err := client.Get("http://example.test/")
		if err != nil {
			t.Fatalf("client.Get() #%d error = %v", i+1, err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("ReadAll() #%d error = %v", i+1, err)
		}
		if string(body) != "ok" {
			t.Fatalf("body #%d = %q, want ok", i+1, string(body))
		}
	}

	if got := atomic.LoadInt32(&dialCount); got != 1 {
		t.Fatalf("dial count = %d, want 1", got)
	}
}
