//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"syscall"
)

const createNewProcessGroup = 0x00000200

func killProcessTreePlatform(cmd *exec.Cmd, hard bool) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	args := []string{"/T"}
	if hard {
		args = append(args, "/F")
	}
	args = append(args, "/PID", strconv.Itoa(cmd.Process.Pid))
	return exec.Command("taskkill", args...).Run()
}

func procAttrsForDev() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}
