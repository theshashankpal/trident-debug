package debug

import (
	"errors"
	"os/exec"
	"syscall"
)

const (
	ExitCodeSuccess = 0
	ExitCodeFailure = 1
)

func GetExitCodeFromError(err error) int {
	if err == nil {
		return ExitCodeSuccess
	} else {

		// Defaults to 1 in case we can't determine a process exit code
		code := ExitCodeFailure

		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			ws := exitError.Sys().(syscall.WaitStatus)
			code = ws.ExitStatus()
		}

		return code
	}
}
