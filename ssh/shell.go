package ssh

import (
	"fmt"
)

func (r *Run) shell() (err error) {
	// server config
	server := r.ServerList[0]
	config := r.Conf.Server[server]

	// header
	proxies, err := getProxyRoute(server, r.Conf)
	for _, p := range proxies {
		fmt.Println(p.Name)
	}

	// Craete sshlib.Connect (Connect Proxy loop)
	connect, err := r.createSshConnect(server)
	if err != nil {
		return
	}

	// Create session
	session, err := connect.CreateSession()
	if err != nil {
		return
	}

	// ssh-agent
	if config.SSHAgentUse {
		connect.Agent = r.agent
		connect.ForwardSshAgent(session)
	}

	// Port Forwarding
	if r.PortForwardLocal != "" && r.PortForwardRemote != "" {
		err := connect.TCPForward(r.PortForwardLocal, r.PortForwardRemote)
		if err != nil {
			fmt.Println(err)
		}
	}

	// TODO(blacknon): local rc file add
	// if config.LocalRcUse {
	// } else {
	// 	// Connect shell
	// 	connect.Shell(session)
	// }

	// Connect shell
	connect.Shell(session)

	return
}

// runLocalRcShell connect to remote shell using local bashrc
// func shellLocalRC(session *ssh.Session, localrcPath string, decoder string) (err error) {

// 	// command
// 	cmd := fmt.Sprintf("bash --rcfile <(echo %s|((base64 --help | grep -q coreutils) && base64 -d <(cat) || base64 -D <(cat) ))", c.LocalRcData)

// 	// decode command
// 	if len(c.LocalRcDecodeCmd) > 0 {
// 		cmd = fmt.Sprintf("bash --rcfile <(echo %s | %s)", c.LocalRcData, c.LocalRcDecodeCmd)
// 	}

// 	err = session.Start(cmd)

// 	return session, err
// }
