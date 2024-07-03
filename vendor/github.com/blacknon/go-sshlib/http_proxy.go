// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sshlib

import (
	"io"
	"net"
	"net/http"
	"time"
)

// httpTransfer copies data between src and dst
func httpTransfer(dst io.WriteCloser, src io.ReadCloser) {
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}

// handleHTTPSProxy handles CONNECT method for HTTPS requests
func handleHTTPSProxy(dial func(network, addr string) (net.Conn, error), w http.ResponseWriter, r *http.Request) {
	destConn, err := dial("tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Write 200 OK response to the client
	w.WriteHeader(http.StatusOK)

	// Get underlying connection from ResponseWriter
	clientConn, buf, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		destConn.Close()
		return
	}

	// Make sure to set read/write deadlines for both connections
	clientConn.SetDeadline(time.Time{})
	destConn.SetDeadline(time.Time{})

	go httpTransfer(destConn, clientConn)
	go httpTransfer(clientConn, destConn)

	// Ensure any buffered data from the client is written to the destination
	if buf.Reader.Buffered() > 0 {
		io.Copy(destConn, buf)
	}
}

// handleHTTPProxy handles HTTP requests
func handleHTTPProxy(dial func(network, addr string) (net.Conn, error), w http.ResponseWriter, r *http.Request) {
	r.RequestURI = ""
	r.URL.Scheme = "http"
	if r.URL.Host == "" {
		r.URL.Host = r.Host
	}

	transport := &http.Transport{
		Dial: dial,
	}

	resp, err := transport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	for key, value := range resp.Header {
		for _, v := range value {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
