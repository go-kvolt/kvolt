package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var stopTimeout = 5 * time.Second

// devBinaryPath returns a stable temp path for the dev build of entryPoint.
func devBinaryPath(entryPoint string) (string, error) {
	abs, err := filepathAbs(entryPoint)
	if err != nil {
		return "", err
	}
	wd, err := osGetwd()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(wd + "\x00" + abs))
	base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	name := fmt.Sprintf("kvolt-dev-%s-%x", base, sum[:4])
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(os.TempDir(), name), nil
}

// stopApp terminates the running dev server and its child processes.
func stopApp() {
	if cmdProcess == nil || cmdProcess.Process == nil {
		cmdProcess = nil
		return
	}
	_ = killProcessTree(cmdProcess)

	done := make(chan struct{})
	go func() {
		_ = cmdProcess.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(stopTimeout):
		_ = killProcessTree(cmdProcess, true)
		<-done
	}
	cmdProcess = nil
}

func killProcessTree(cmd *exec.Cmd, force ...bool) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return killProcessTreePlatform(cmd, len(force) > 0 && force[0])
}
