package ssh

import (
	"os"

	"golang.org/x/crypto/ssh"
)

// Connect struct add ssh.Session
type shConnect struct {
	Connect
	Session ssh.Session
}

type shell struct {
	Signal   chan os.Signal
	Connects []*shConnect
	PROMPT   string
	OPROMPT  string
	Count    int
}

func (s *shell) CreateConn() {
	// conn := new(shConnect)
}
