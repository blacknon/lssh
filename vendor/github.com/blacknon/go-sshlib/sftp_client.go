package sshlib

import (
	"net"

	"github.com/pkg/sftp"
)

func (c *Connect) OpenSFTP() (*sftp.Client, error) {
	return c.newSFTPClient()
}

func (c *Connect) newSFTPClient() (*sftp.Client, error) {
	if c.isControlClient() {
		return c.controlClient.newSFTPClient()
	}
	if err := c.ensureActiveConnection(); err != nil {
		return nil, err
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
