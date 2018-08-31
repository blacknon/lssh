package ssh

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/blacknon/lssh/conf"
	"golang.org/x/net/proxy"
)

type direct struct{}

func (direct) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}

type httpProxy struct {
	host     string
	haveAuth bool
	username string
	password string
	forward  proxy.Dialer
}

func (s *httpProxy) Dial(network, addr string) (net.Conn, error) {
	c, err := s.forward.Dial("tcp", s.host)
	if err != nil {
		return nil, err
	}

	reqURL, err := url.Parse("http://" + addr)
	if err != nil {
		c.Close()
		return nil, err
	}
	reqURL.Scheme = ""

	req, err := http.NewRequest("CONNECT", reqURL.String(), nil)
	if err != nil {
		c.Close()
		return nil, err
	}
	req.Close = false
	if s.haveAuth {
		req.SetBasicAuth(s.username, s.password)
	}
	req.Header.Set("User-Agent", "Poweredby Golang")

	err = req.Write(c)
	if err != nil {
		c.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(c), req)
	if err != nil {
		resp.Body.Close()
		c.Close()
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		c.Close()
		err = fmt.Errorf("Connect server using proxy error, StatusCode [%d]", resp.StatusCode)
		return nil, err
	}

	return c, nil
}

func newHTTPProxy(uri *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	s := new(httpProxy)
	s.host = uri.Host
	s.forward = forward
	if uri.User != nil {
		s.haveAuth = true
		s.username = uri.User.Username()
		s.password, _ = uri.User.Password()
	}
	return s, nil
}

func createProxyDialerHttp(proxyConf conf.ProxyConfig) (proxyDialer proxy.Dialer, err error) {
	proxy.RegisterDialerType("http", newHTTPProxy)
	proxy.RegisterDialerType("https", newHTTPProxy)

	directProxy := direct{}

	proxyURL := "http://" + proxyConf.Addr + ":" + proxyConf.Port
	if proxyConf.User != "" && proxyConf.Pass != "" {
		proxyURL = "http://" + proxyConf.User + ":" + proxyConf.Pass + "@" + proxyConf.Addr + ":" + proxyConf.Port
	}

	proxyURI, _ := url.Parse(proxyURL)
	proxyDialer, err = proxy.FromURL(proxyURI, directProxy)

	return
}
