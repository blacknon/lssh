package sshlib

import (
	"errors"
	"net"

	"github.com/pkg/sftp"
)

func (c *Connect) newSFTPClient() (*sftp.Client, error) {
	if c.isControlClient() {
		return c.controlClient.newSFTPClient()
	}
	if c.Client == nil {
		return nil, errors.New("ssh client is nil")
	}
	return sftp.NewClient(c.Client)
}

func (c *controlClient) newSFTPClient() (*sftp.Client, error) {
	resp, err := c.request(controlRequest{
		Type:    controlRequestSubsystem,
		Command: "sftp",
	})
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial("unix", resp.StreamPath)
	if err != nil {
		return nil, err
	}

	client, err := sftp.NewClientPipe(conn, conn)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return client, nil
}
