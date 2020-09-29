// +build windows

package runcmd

import (
	"os"
	"os/exec"
	"syscall"
)

// To keep alive the child process you have to put it in a different process
// group.
// You do that by setting CREATE_NEW_PROCESS_GROUP to true.
func start(cmd *exec.Cmd) (*os.Process, error) {
	keepAliveChild := true
	if keepAliveChild {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{
				CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
			}
		} else {
			cmd.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP
		}
	}
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	process := cmd.Process
	return process, nil
}
