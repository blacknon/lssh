//go:build !windows && !linux

package mux

import (
	"os"

	"golang.org/x/sys/unix"
)

func configureNativeSessionPTY(tty *os.File) error {
	if tty == nil {
		return nil
	}

	fd := int(tty.Fd())
	termios, err := unix.IoctlGetTermios(fd, ioctlReadTermios())
	if err != nil {
		return err
	}

	// Match the direct terminal path used by lssh native SSM shells:
	// disable canonical processing so each key reaches the remote session
	// immediately instead of being line-buffered as "pwd\r\n".
	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0

	return unix.IoctlSetTermios(fd, ioctlWriteTermios(), termios)
}

func ioctlReadTermios() uint {
	return unix.TIOCGETA
}

func ioctlWriteTermios() uint {
	return unix.TIOCSETA
}
