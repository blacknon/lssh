package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
)

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
