package ssh

import (
	"fmt"
	"os"
	"os/user"
	"regexp"
	"strings"
	"time"

	"github.com/blacknon/go-sshlib"
	"github.com/blacknon/lssh/common"
	"golang.org/x/crypto/ssh"
)

// run shell
func (r *Run) shell() (err error) {
	// server config
	server := r.ServerList[0]
	config := r.Conf.Server[server]

	// header
	r.printSelectServer()
	r.printPortForward(r.PortForwardLocal, r.PortForwardRemote)
	r.printProxy(server)
	if config.LocalRcUse == "yes" {
		fmt.Fprintf(os.Stderr, "Information   :This connect use local bashrc.\n")
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

	// run pre local command
	if config.PreCmd != "" {
		runCmdLocal(config.PreCmd)
	}

	// defer run post local command
	if config.PostCmd != "" {
		defer runCmdLocal(config.PostCmd)
	}

	// if terminal log enable
	logConf := r.Conf.Log
	if logConf.Enable {
		logPath := r.getLogPath(server)
		connect.SetLog(logPath, logConf.Timestamp)
	}

	// TODO(blacknon): local rc file add
	if config.LocalRcUse == "yes" {
		err = localrcShell(connect, session, config.LocalRcPath, config.LocalRcDecodeCmd)
	} else {
		// Connect shell
		err = connect.Shell(session)
	}

	return
}

// getLogPath return log file path.
func (r *Run) getLogPath(server string) (logPath string) {
	// check regex
	// if ~/.ssh/config, in ":"
	reg := regexp.MustCompile(`:`)

	if reg.MatchString(server) {
		slice := strings.SplitN(server, ":", 2)
		server = slice[1]
	}

	dir := r.getLogDirPath(server)
	file := time.Now().Format("20060102_150405") + "_" + server + ".log"
	logPath = dir + "/" + file

	return
}

// getLogDirPath return log directory path
func (r *Run) getLogDirPath(server string) (dir string) {
	u, _ := user.Current()
	logConf := r.Conf.Log

	dir = logConf.Dir
	dir = strings.Replace(dir, "~", u.HomeDir, 1)
	dir = strings.Replace(dir, "<Date>", time.Now().Format("20060102"), 1)
	dir = strings.Replace(dir, "<Hostname>", server, 1)

	return
}

// runLocalRcShell connect to remote shell using local bashrc
func localrcShell(connect *sshlib.Connect, session *ssh.Session, localrcPath []string, decoder string) (err error) {
	// set default bashrc
	if len(localrcPath) == 0 {
		localrcPath = []string{"~/.bashrc"}
	}

	// get bashrc base64 data
	rcData, err := common.GetFilesBase64(localrcPath)
	if err != nil {
		return
	}

	// command
	cmd := fmt.Sprintf("bash --rcfile <(echo %s|((base64 --help | grep -q coreutils) && base64 -d <(cat) || base64 -D <(cat) ))", rcData)

	// decode command
	if decoder != "" {
		cmd = fmt.Sprintf("bash --rcfile <(echo %s | %s)", rcData, decoder)
	}

	connect.CmdShell(session, cmd)

	return
}
