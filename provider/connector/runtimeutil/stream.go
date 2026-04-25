package runtimeutil

import (
	"io"
	"net"
	"os"
	"sync"
)

func StreamInteractiveSession(stdin io.Writer, stdout, stderr io.Reader, wait func() error) error {
	var wg sync.WaitGroup
	if stdout != nil {
		wg.Add(1)
		go copyStream(&wg, os.Stdout, stdout)
	}
	if stderr != nil {
		wg.Add(1)
		go copyStream(&wg, os.Stderr, stderr)
	}
	if stdin != nil {
		go func() {
			_, _ = io.Copy(stdin, os.Stdin)
		}()
	}
	err := wait()
	wg.Wait()
	return err
}

func CopyStream(dst io.Writer, src io.Reader) {
	_, _ = io.Copy(dst, src)
}

func copyStream(wg *sync.WaitGroup, dst io.Writer, src io.Reader) {
	defer wg.Done()
	CopyStream(dst, src)
}

func BridgeConn(conn net.Conn) error {
	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := io.Copy(conn, os.Stdin)
		if tcpConn, ok := conn.(interface{ CloseWrite() error }); ok {
			_ = tcpConn.CloseWrite()
		}
		errCh <- err
	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(os.Stdout, conn)
		errCh <- err
	}()

	wg.Wait()
	_ = conn.Close()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}
