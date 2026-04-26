package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/creack/pty"
)

func main() {
	addr := ":23"
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		go func() {
			defer conn.Close()
			if err := handleConn(conn); err != nil {
				log.Printf("session error from %s: %v", conn.RemoteAddr(), err)
			}
		}()
	}
}

func handleConn(conn net.Conn) error {
	hostAlias := getenvDefault("TELNET_HOST_ALIAS", "telnet-host")
	username := getenvDefault("TELNET_USER", "demo")
	password := getenvDefault("TELNET_PASSWORD", "demo-password")

	reader := bufio.NewReader(conn)

	gotUser, err := promptNonEmptyLine(conn, reader, fmt.Sprintf("%s\r\nlogin: ", hostAlias))
	if err != nil {
		return err
	}

	if _, err := io.WriteString(conn, "password: "); err != nil {
		return err
	}
	gotPassword, err := readLine(reader)
	if err != nil {
		return err
	}

	if gotUser != username || gotPassword != password {
		_, _ = io.WriteString(conn, "\r\nLogin incorrect\r\n")
		return nil
	}

	return startShell(conn, username, hostAlias)
}

func promptNonEmptyLine(conn net.Conn, reader *bufio.Reader, prompt string) (string, error) {
	for {
		if _, err := io.WriteString(conn, prompt); err != nil {
			return "", err
		}
		line, err := readLine(reader)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(line) == "" {
			prompt = "login: "
			continue
		}
		return line, nil
	}
}

func startShell(conn net.Conn, username, hostAlias string) error {
	targetUser, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("lookup user %q: %w", username, err)
	}

	uid, err := strconv.ParseUint(targetUser.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("parse uid: %w", err)
	}
	gid, err := strconv.ParseUint(targetUser.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("parse gid: %w", err)
	}

	cmd := exec.Command("/bin/bash", "-i")
	cmd.Dir = targetUser.HomeDir
	cmd.Env = []string{
		"HOME=" + targetUser.HomeDir,
		"USER=" + username,
		"LOGNAME=" + username,
		"SHELL=/bin/bash",
		"TERM=xterm",
		"HOSTNAME=" + hostAlias,
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
		Setsid:  true,
		Setctty: true,
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("start shell: %w", err)
	}
	defer ptmx.Close()

	copyDone := make(chan error, 2)
	cmdDone := make(chan error, 1)

	go func() {
		_, copyErr := io.Copy(ptmx, conn)
		copyDone <- copyErr
	}()
	go func() {
		_, copyErr := io.Copy(conn, ptmx)
		copyDone <- copyErr
	}()
	go func() {
		cmdDone <- cmd.Wait()
	}()

	for i := 0; i < 3; i++ {
		select {
		case err = <-cmdDone:
			_ = ptmx.Close()
			return normalizeShellExit(err)
		case err = <-copyDone:
			if err == io.EOF || isClosedNetworkError(err) {
				_ = ptmx.Close()
				return nil
			}
			if err != nil {
				_ = ptmx.Close()
				return err
			}
		}
	}

	return nil
}

func readLine(reader *bufio.Reader) (string, error) {
	var builder strings.Builder
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		if b == '\r' || b == '\n' {
			if b == '\r' && reader.Buffered() > 0 {
				if next, err := reader.Peek(1); err == nil && len(next) == 1 && next[0] == '\n' {
					_, _ = reader.ReadByte()
				}
			}
			return builder.String(), nil
		}
		builder.WriteByte(b)
	}
}

func getenvDefault(key, def string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return def
}

func isClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "closed network connection") || strings.Contains(text, "broken pipe")
}

func normalizeShellExit(err error) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return err
	}
	if exitErr.ExitCode() == 0 {
		return nil
	}
	return err
}
