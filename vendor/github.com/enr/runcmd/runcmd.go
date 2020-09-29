package runcmd

import (
	"os/exec"
	"syscall"
)

func getExitStatus(err error) int {
	if err == nil {
		return 0
	}
	exitStatus := 1
	if msg, ok := err.(*exec.ExitError); ok {
		exitStatus = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}
	return exitStatus

	// if ee, ok := err.(*exec.ExitError); ok && err != nil {
	// 	status := ee.ProcessState.Sys().(syscall.WaitStatus)
	// 	if status.Exited() {
	// 		// A non-zero return code isn't considered an error here.
	// 		result.Code = status.ExitStatus()
	// 		err = nil
	// 	}
	// 	logger.Infof("run result: %v", ee)
	// }
}
