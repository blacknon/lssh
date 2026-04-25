package opensshlib

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/blacknon/lssh/providerapi"
	"github.com/pkg/sftp"
)

func TestOpenSSHSFTPHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_OPENSSH_SFTP_HELPER") != "1" {
		return
	}
	if root := os.Getenv("OPENSSH_SFTP_HELPER_ROOT"); root != "" {
		if err := os.Chdir(root); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}

	rwc := stdioReadWriteCloser{Reader: os.Stdin, WriteCloser: os.Stdout}
	server, err := sftp.NewServer(rwc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := server.Serve(); err != nil && err != io.EOF {
		fmt.Fprintln(os.Stderr, err)
		_ = server.Close()
		os.Exit(2)
	}
	_ = server.Close()
	os.Exit(0)
}

func TestStartSFTPTransportCommand(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	ctx := context.Background()
	transport, err := startSFTPTransportCommand(ctx, os.Args[0], []string{"-test.run=TestOpenSSHSFTPHelperProcess", "--"}, []string{
		"GO_WANT_OPENSSH_SFTP_HELPER=1",
		"OPENSSH_SFTP_HELPER_ROOT=" + root,
	})
	if err != nil {
		t.Fatalf("startSFTPTransportCommand() error = %v", err)
	}
	defer func() {
		if cerr := transport.Close(); cerr != nil {
			t.Fatalf("Close() error = %v", cerr)
		}
	}()

	entries, err := transport.Client.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "hello.txt" {
		t.Fatalf("ReadDir() = %#v", entries)
	}

	writer, err := transport.Client.Create("written.txt")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := writer.Write([]byte("world")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "written.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "world" {
		t.Fatalf("written.txt = %q, want %q", string(data), "world")
	}
}

func TestStartSFTPTransportPlan(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	transport, err := StartSFTPTransportPlan(context.Background(), providerapi.ConnectorPlan{
		Kind:    "command",
		Program: os.Args[0],
		Args:    []string{"-test.run=TestOpenSSHSFTPHelperProcess", "--"},
		Env: map[string]string{
			"GO_WANT_OPENSSH_SFTP_HELPER": "1",
			"OPENSSH_SFTP_HELPER_ROOT":    root,
		},
	})
	if err != nil {
		t.Fatalf("StartSFTPTransportPlan() error = %v", err)
	}
	defer func() {
		if cerr := transport.Close(); cerr != nil {
			t.Fatalf("Close() error = %v", cerr)
		}
	}()

	entries, err := transport.Client.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "hello.txt" {
		t.Fatalf("ReadDir() = %#v", entries)
	}
}

type stdioReadWriteCloser struct {
	io.Reader
	io.WriteCloser
}
