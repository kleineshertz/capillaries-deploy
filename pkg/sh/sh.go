package sh

import (
	"embed"
	"fmt"

	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/rexec"
)

//go:embed scripts/*
var embeddedScriptsFs embed.FS

func ExecEmbeddedScriptsOnInstance(sshConfig *rexec.SshConfigDef, ipAddress string, embeddedScriptPaths []string, envVars map[string]string, isVerbose bool) (l.LogMsg, error) {
	lb := l.NewLogBuilder(fmt.Sprintf("ExecEmbeddedScriptsOnInstance: %s on %s", embeddedScriptPaths, ipAddress), isVerbose)

	if len(embeddedScriptPaths) == 0 {
		lb.Add(fmt.Sprintf("no commands to execute on %s", ipAddress))
		return lb.Complete(nil)
	}
	for _, embeddedScriptPath := range embeddedScriptPaths {
		if err := execEmbeddedScriptOnInstance(sshConfig, lb, ipAddress, embeddedScriptPath, []string{}, envVars, isVerbose); err != nil {
			lb.Complete(err)
		}
	}
	return lb.Complete(nil)
}

func execEmbeddedScriptOnInstance(sshConfig *rexec.SshConfigDef, lb *l.LogBuilder, ipAddress string, embeddedScriptPath string, params []string, envVars map[string]string, isVerbose bool) error {
	cmdBytes, err := embeddedScriptsFs.ReadFile(embeddedScriptPath)
	if err != nil {
		return err
	}
	er := rexec.ExecSsh(sshConfig, ipAddress, string(cmdBytes), envVars)
	lb.Add(er.ToString())
	if er.Error != nil {
		return fmt.Errorf("cannot execute script %s on %s: %s", embeddedScriptPath, ipAddress, er.Error.Error())
	}
	return nil
}
