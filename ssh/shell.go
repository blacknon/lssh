package ssh

import "fmt"

func (r *Run) shell() {
	// server config
	server := r.ServerList[0]
	config := r.Conf.Server[server]

	// Craete sshlib.Connect (Connect Proxy loop)
	connect, err := r.createSshConnect(server)

	// x11 forwarding
	if config.X11 {
		connect.ForwardX11 = true
	}

	// ssh-agent
	connect.ConnectSshAgent()
	if config.SSHAgentUse {
		connect.Agent = r.Agent
		connect.ForwardSshAgent(session)
	}

	// Port Forwarding
	if r.PortForwardLocal != "" && r.PortForwardRemote != "" {
		err := connect.TCPForward(r.PortForwardLocal, r.PortForwardRemote)
		if err != nil {
			fmt.Println(err)
		}
	}

	// Connect shell
	connect.Shell()
}
