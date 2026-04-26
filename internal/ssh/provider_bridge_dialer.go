package ssh

import (
	"context"
	"io"
	"net"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/internal/connectorruntime"
	"golang.org/x/net/proxy"
)

func (r *Run) providerBridgeDialer(server string) connectorruntime.Dialer {
	return connectorruntime.DialerFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		return r.dialServerViaProxyChain(ctx, server, network, address)
	})
}

func (r *Run) dialServerViaProxyChain(ctx context.Context, server, network, address string) (net.Conn, error) {
	if network == "" {
		network = "tcp"
	}

	proxyRoute, err := getProxyRoute(server, r.Conf)
	if err != nil {
		return nil, err
	}

	var dialer sshlib.ProxyDialer = proxy.Direct
	closers := []io.Closer{}
	for _, p := range proxyRoute {
		config := r.Conf

		switch p.Type {
		case "http", "https", "socks", "socks5":
			c := config.Proxy[p.Name]
			pxy := &sshlib.Proxy{
				Type:      p.Type,
				Forwarder: dialer,
				Addr:      c.Addr,
				Port:      c.Port,
				User:      c.User,
				Password:  c.Pass,
			}
			dialer, err = pxy.CreateProxyDialer()
		case "command":
			pxy := &sshlib.Proxy{
				Type:    p.Type,
				Command: p.Name,
			}
			dialer, err = pxy.CreateProxyDialer()
		default:
			c := config.Server[p.Name]
			pxy := &sshlib.Connect{
				ProxyDialer: dialer,
			}
			if err = pxy.CreateClient(c.Addr, c.Port, c.User, r.serverAuthMethodMap[p.Name]); err != nil {
				return nil, err
			}
			closers = append(closers, pxy)
			dialer = pxy.Client
		}
		if err != nil {
			_ = multiCloser{closers: closers}.Close()
			return nil, err
		}
	}

	conn, err := dialWithProxyDialer(ctx, dialer, network, address)
	if err != nil {
		_ = multiCloser{closers: closers}.Close()
		return nil, err
	}
	if len(closers) == 0 {
		return conn, nil
	}

	return &connectorManagedSSHNetConn{
		Conn:          conn,
		closeResource: multiCloser{closers: closers},
	}, nil
}

func dialWithProxyDialer(ctx context.Context, dialer sshlib.ProxyDialer, network, address string) (net.Conn, error) {
	if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
		return contextDialer.DialContext(ctx, network, address)
	}
	if connectorDialer, ok := dialer.(interface {
		DialContext(ctx context.Context, network, address string) (net.Conn, error)
	}); ok {
		return connectorDialer.DialContext(ctx, network, address)
	}
	return dialer.Dial(network, address)
}
