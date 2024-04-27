package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/capillariesio/capillaries-deploy/pkg/l"
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
elapsed:%0.3f
-----------------------
`, er.Cmd, er.Stdout, er.Stderr, errString, er.Elapsed)
}

func CmdChainExecToString(title string, logContent string, err error, isVerbose bool) string {
	if err != nil {
		title = fmt.Sprintf("%s: %s%s%s", title, l.LogColorRed, err, l.LogColorReset)
	} else {
		title = fmt.Sprintf("%s: %sOK%s", title, l.LogColorGreen, l.LogColorReset)
	}

	if isVerbose {
		return fmt.Sprintf(
			`
=========================================
%s
=========================================
%s
=========================================
`, title, logContent)
	}

	return title
}

func ExecLocal(cmdPath string, params []string, envVars map[string]string, dir string, timeoutSeconds int) ExecResult {
	cmdCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*int(time.Second)))
	defer cancel()

	p := exec.CommandContext(cmdCtx, cmdPath, params...)

	if dir != "" {
		p.Dir = dir
	}

	for k, v := range envVars {
		if strings.Contains(v, " ") {
			p.Env = append(p.Env, fmt.Sprintf("%s='%s'", k, v))
		} else {
			p.Env = append(p.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Inherit $HOME so we can use ~
	if _, ok := envVars["HOME"]; !ok {
		p.Env = append(p.Env, fmt.Sprintf("HOME=%s", os.Getenv("HOME")))
	}

	// Do not use pipes, work with raw data, otherwise stdout/stderr
	// will not be easily available in the timeout scenario
	var stdout, stderr bytes.Buffer
	p.Stdout = &stdout
	p.Stderr = &stderr

	// Run
	runStartTime := time.Now()
	err := p.Run()
	elapsed := time.Since(runStartTime).Seconds()

	rawInput := fmt.Sprintf("%s %s", cmdPath, strings.Join(params, " "))
	rawOutput := stdout.String()
	rawErrors := stderr.String()
	if err != nil {
		// Cmd not found, nonzero exit status etc
		return ExecResult{rawInput, rawOutput, rawErrors, elapsed, err}
	} else if cmdCtx.Err() == context.DeadlineExceeded {
		// Timeout occurred, err.Error() is probably: 'signal: killed'
		return ExecResult{rawInput, rawOutput, rawErrors, elapsed, fmt.Errorf("cmd execution timeout exceeded")}
	}

	return ExecResult{rawInput, rawOutput, rawErrors, elapsed, nil}
}
