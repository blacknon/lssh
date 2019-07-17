package ssh

import "fmt"

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

	// Connect shell
	connect.Shell(session)

	return
}
