package sh

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/capillariesio/capillaries-deploy/pkg/exec"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

//go:embed scripts/*
var embeddedScriptsFs embed.FS

func ExecEmbeddedScriptLocally(lb *l.LogBuilder, embeddedScriptPath string, params []string, envVars map[string]string, isVerbose bool, timeoutSeconds int) error {
	cmdBytes, err := embeddedScriptsFs.ReadFile(embeddedScriptPath)
	if err != nil {
		return err
	}

	f, err := os.CreateTemp("", strings.ReplaceAll(embeddedScriptPath, "/", "_")+"_")
	if err != nil {
		return err
	}
	fullTempPath := f.Name()
	defer os.Remove(fullTempPath)

	_, err = f.Write(cmdBytes)
	if err != nil {
		f.Close()
		return err
	}

	err = f.Chmod(0644)
	if err != nil {
		f.Close()
		return err
	}

	f.Close()

	er := exec.ExecLocal(fullTempPath, params, envVars, filepath.Dir(fullTempPath), timeoutSeconds)
	lb.Add(er.ToString())
	if er.Error != nil {
		return er.Error
	}
	return nil
}

func ExecEmbeddedScriptsOnInstance(sshConfig *exec.SshConfigDef, ipAddress string, embeddedScriptPaths []string, envVars map[string]string, isVerbose bool) (l.LogMsg, error) {
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

func execEmbeddedScriptOnInstance(sshConfig *exec.SshConfigDef, lb *l.LogBuilder, ipAddress string, embeddedScriptPath string, params []string, envVars map[string]string, isVerbose bool) error {
	cmdBytes, err := embeddedScriptsFs.ReadFile(embeddedScriptPath)
	if err != nil {
		return err
	}
	er := exec.ExecSsh(sshConfig, ipAddress, string(cmdBytes), envVars)
	lb.Add(er.ToString())
	if er.Error != nil {
		return fmt.Errorf("cannot execute script %s on %s: %s", embeddedScriptPath, ipAddress, er.Error.Error())
	}
	return nil
}
