package rexec

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"golang.org/x/crypto/ssh"
)

type ExecResult struct {
	Cmd     string
	Stdout  string
	Stderr  string
	Elapsed float64
	Error   error
}

func (er *ExecResult) ToString() string {
	var errString string
	if er.Error != nil {
		errString = er.Error.Error()
	}
	return fmt.Sprintf(`
-----------------------
cmd:
%s
stdout:
%s
stderr:
%s
error:
%s
remote cmd elapsed:%0.3f
-----------------------
`, er.Cmd, er.Stdout, er.Stderr, errString, er.Elapsed)
}

type SshConfigDef struct {
	ExternalIpAddress string `json:"external_ip_address"` // Bastion
	Port              int    `json:"port"`
	User              string `json:"user"`
	PrivateKeyPath    string `json:"private_key_path"`
}

type TunneledSshClient struct {
	ProxySshClient  *ssh.Client
	TunneledTcpConn net.Conn
	TunneledSshConn ssh.Conn
	SshClient       *ssh.Client
}

func (tsc *TunneledSshClient) Close() {
	if tsc.SshClient != nil {
		tsc.SshClient.Close()
	}
	if tsc.TunneledSshConn != nil {
		tsc.TunneledSshConn.Close()
	}
	if tsc.TunneledTcpConn != nil {
		tsc.TunneledTcpConn.Close()
	}
	if tsc.ProxySshClient != nil {
		tsc.ProxySshClient.Close()
	}
}

// Our jumphost implementation
func NewTunneledSshClient(sshConfig *SshConfigDef, ipAddress string) (*TunneledSshClient, error) {
	bastionSshClientConfig, err := NewSshClientConfig(
		sshConfig.User,
		sshConfig.PrivateKeyPath)
	if err != nil {
		return nil, err
	}

	bastionUrl := fmt.Sprintf("%s:%d", sshConfig.ExternalIpAddress, sshConfig.Port)

	tsc := TunneledSshClient{}

	if ipAddress == sshConfig.ExternalIpAddress {
		// Go directly to bastion
		tsc.SshClient, err = ssh.Dial("tcp", bastionUrl, bastionSshClientConfig)
		if err != nil {
			return nil, fmt.Errorf("dial direct to bastion %s failed: %s", bastionUrl, err.Error())
		}
	} else {
		// Dial twice
		tsc.ProxySshClient, err = ssh.Dial("tcp", bastionUrl, bastionSshClientConfig)
		if err != nil {
			return nil, fmt.Errorf("dial to bastion proxy %s failed: %s", bastionUrl, err.Error())
		}

		internalUrl := fmt.Sprintf("%s:%d", ipAddress, sshConfig.Port)

		tsc.TunneledTcpConn, err = tsc.ProxySshClient.Dial("tcp", internalUrl)
		if err != nil {
			return nil, fmt.Errorf("dial to internal URL %s failed: %s", internalUrl, err.Error())
		}

		tunneledSshClientConfig, err := NewSshClientConfig(
			sshConfig.User,
			sshConfig.PrivateKeyPath)
		if err != nil {
			return nil, err
		}
		var chans <-chan ssh.NewChannel
		var reqs <-chan *ssh.Request
		tsc.TunneledSshConn, chans, reqs, err = ssh.NewClientConn(tsc.TunneledTcpConn, internalUrl, tunneledSshClientConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot establish ssh connection via TCP tunnel to internal URL %s: %s", internalUrl, err.Error())
		}

		tsc.SshClient = ssh.NewClient(tsc.TunneledSshConn, chans, reqs)
	}

	return &tsc, nil
}

func ExecSsh(sshConfig *SshConfigDef, ipAddress string, cmd string, envVars map[string]string) ExecResult {
	cmdBuilder := strings.Builder{}
	for k, v := range envVars {
		if strings.Contains(v, " ") {
			cmdBuilder.WriteString(fmt.Sprintf("%s='%s'\n", k, v))
		} else {
			cmdBuilder.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
	}
	cmdBuilder.WriteString(cmd)

	tsc, err := NewTunneledSshClient(sshConfig, ipAddress)
	if err != nil {
		return ExecResult{cmdBuilder.String(), "", "", 0, err}
	}
	defer tsc.Close()

	session, err := tsc.SshClient.NewSession()
	if err != nil {
		return ExecResult{cmdBuilder.String(), "", "", 0, fmt.Errorf("cannot create session for %s: %s", ipAddress, err.Error())}
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// TODO: it would be nice to have an execution timeout

	runStartTime := time.Now()
	err = session.Run(cmdBuilder.String())
	elapsed := time.Since(runStartTime).Seconds()
	if err == nil {
		if len(stderr.String()) > 0 {
			err = fmt.Errorf("%s", stderr.String())
		}
	}

	er := ExecResult{cmd, stdout.String(), stderr.String(), elapsed, err}
	return er
}

func ExecCommandOnInstance(sshConfig *SshConfigDef, ipAddress string, cmd string, isVerbose bool) (l.LogMsg, error) {
	lb := l.NewLogBuilder(fmt.Sprintf("ExecCommandOnInstance: %s - %s", ipAddress, cmd), isVerbose)
	er := ExecSsh(sshConfig, ipAddress, cmd, map[string]string{})
	lb.Add(er.ToString())
	if er.Error != nil {
		return lb.Complete(er.Error)
	}
	return lb.Complete(nil)
}

// Used for file transfer
func ExecSshForClient(sshClient *ssh.Client, cmd string) (string, string, error) {
	session, err := sshClient.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("cannot create session for %s: %s", sshClient.RemoteAddr(), err.Error())
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if err := session.Run(cmd); err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("cannot execute '%s' at %s: %s (stderr: %s)", sshClient.RemoteAddr(), cmd, err.Error(), stderr.String())
	}
	return stdout.String(), stderr.String(), nil
}

// Used on volume attachment
func ExecSshAndReturnLastLine(sshConfig *SshConfigDef, ipAddress string, cmd string) (string, ExecResult) {
	er := ExecSsh(sshConfig, ipAddress, cmd, map[string]string{})
	if er.Error != nil {
		return "", er
	}
	lines := strings.Split(strings.Trim(er.Stdout, "\n "), "\n")
	return strings.TrimSpace(lines[len(lines)-1]), er
}
