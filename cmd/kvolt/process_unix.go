//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func killProcessTreePlatform(cmd *exec.Cmd, hard bool) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	sig := syscall.SIGINT
	if hard {
		sig = syscall.SIGKILL
	}
	// Negative PID kills the whole process group (Setpgid on start).
	if err := syscall.Kill(-pid, sig); err != nil {
		return cmd.Process.Signal(sig)
	}
	return nil
}

func procAttrsForDev() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
