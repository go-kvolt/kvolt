package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestMain_TestAndFmtDispatch(t *testing.T) {
	code := withExitCapture(t)
	old := os.Args
	defer func() { os.Args = old }()

	failCmd := func(name string, args ...string) *exec.Cmd {
		if os.PathSeparator == '\\' {
			return exec.Command("cmd", "/c", "exit", "1")
		}
		return exec.Command("false")
	}
	execCommand = failCmd

	os.Args = []string{"kvolt", "test"}
	func() { defer func() { _ = recover() }(); main() }()
	if *code != 1 {
		t.Fatalf("test dispatch exit=%d", *code)
	}
	*code = -1
	os.Args = []string{"kvolt", "fmt"}
	func() { defer func() { _ = recover() }(); main() }()
	if *code != 1 {
		t.Fatalf("fmt dispatch exit=%d", *code)
	}
}

func TestFlagUsagesAndBuildMkdirFail(t *testing.T) {
	old := os.Args
	defer func() { os.Args = old }()

	os.Args = []string{"kvolt", "run", "-h"}
	runRun()
	os.Args = []string{"kvolt", "build", "-h"}
	runBuild()
	os.Args = []string{"kvolt", "test", "-h"}
	runTest()

	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(oldWD) }()
	_ = os.WriteFile("main.go", []byte("package main\nfunc main() {}\n"), 0o644)

	oldMk := mkdirAllFn
	mkdirAllFn = func(path string, perm os.FileMode) error { return errors.New("mkdir fail") }
	defer func() { mkdirAllFn = oldMk }()
	os.Args = []string{"kvolt", "build", "-e", "main.go", "-o", "out/app"}
	runBuild()
}

func TestGenerateMiddlewareMkdirAndWriteFail(t *testing.T) {
	code := withExitCapture(t)
	oldMk, oldW := mkdirAllFn, writeFileFn
	defer func() { mkdirAllFn, writeFileFn = oldMk, oldW }()

	mkdirAllFn = func(path string, perm os.FileMode) error { return errors.New("mkdir") }
	func() { defer func() { _ = recover() }(); generateMiddleware("z") }()
	if *code != 1 {
		t.Fatalf("mw mkdir exit=%d", *code)
	}

	*code = -1
	mkdirAllFn = oldMk
	writeFileFn = func(name string, data []byte, perm os.FileMode) error {
		return errors.New("write")
	}
	func() { defer func() { _ = recover() }(); generateMiddleware("z2") }()
	if *code != 1 {
		t.Fatalf("mw write exit=%d", *code)
	}

	*code = -1
	func() { defer func() { _ = recover() }(); generateHandler("h2") }()
	if *code != 1 {
		t.Fatalf("handler write exit=%d", *code)
	}
}

func TestRunDevUntil_WatcherFatalAndWalkErr(t *testing.T) {
	code := withExitCapture(t)
	oldW := newWatcherFn
	newWatcherFn = func() (*fsnotify.Watcher, error) { return nil, errors.New("no watcher") }
	defer func() { newWatcherFn = oldW }()
	func() { defer func() { _ = recover() }(); runDevUntil("main.go", nil) }()
	if *code != 1 {
		t.Fatalf("watcher fatal exit=%d", *code)
	}

	newWatcherFn = oldW
	oldWalk := filepathWalk
	filepathWalk = func(root string, fn filepath.WalkFunc) error {
		return fn("broken", nil, errors.New("walk err"))
	}
	defer func() { filepathWalk = oldWalk }()

	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(oldWD) }()
	_ = os.WriteFile("go.mod", []byte("module t\n\ngo 1.22\n"), 0o644)
	_ = os.WriteFile("main.go", []byte("package main\nfunc main() { select {} }\n"), 0o644)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		runDevUntil("main.go", stop)
	}()
	time.Sleep(300 * time.Millisecond)
	close(stop)
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("hang")
	}
}

func TestRunDevUntil_DebounceFires(t *testing.T) {
	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(oldWD) }()
	_ = os.WriteFile("go.mod", []byte("module t\n\ngo 1.22\n"), 0o644)
	_ = os.WriteFile("main.go", []byte("package main\nfunc main() { select {} }\n"), 0o644)

	oldDelay := debounceDelay
	debounceDelay = 40 * time.Millisecond
	armInitialDebounce = true
	defer func() {
		debounceDelay = oldDelay
		armInitialDebounce = false
	}()

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		runDevUntil("main.go", stop)
	}()
	// restartApp (build+start) can take a few seconds before the select loop runs.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cmdProcess != nil && cmdProcess.Process != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Let armed debounce timer.C fire inside the select loop (restartApp again).
	time.Sleep(200 * time.Millisecond)
	close(stop)
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("hang")
	}
}

func TestWatcherLoop_ErrorPaths(t *testing.T) {
	events := make(chan fsnotify.Event)
	errs := make(chan error)
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		runWatcherLoop(events, errs, timer)
	}()
	errs <- errors.New("watch err")
	close(errs)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("errs close hang")
	}

	events2 := make(chan fsnotify.Event)
	errs2 := make(chan error)
	timer2 := time.NewTimer(time.Hour)
	timer2.Stop()
	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		runWatcherLoop(events2, errs2, timer2)
	}()
	close(events2)
	select {
	case <-done2:
	case <-time.After(2 * time.Second):
		t.Fatal("events close hang")
	}
}

func TestRestartApp_DevBinaryAndStartFail(t *testing.T) {
	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(oldWD) }()
	_ = os.WriteFile("main.go", []byte("package main\nfunc main() {}\n"), 0o644)

	oldAbs := filepathAbs
	filepathAbs = func(string) (string, error) { return "", errors.New("abs") }
	restartApp("main.go")
	filepathAbs = oldAbs

	oldGetwd := osGetwd
	osGetwd = func() (string, error) { return "", errors.New("wd") }
	restartApp("main.go")
	osGetwd = oldGetwd

	oldExec := execCommand
	defer func() { execCommand = oldExec }()
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "go" {
			// Successful no-op build.
			if os.PathSeparator == '\\' {
				return exec.Command("cmd", "/c", "exit", "0")
			}
			return exec.Command("true")
		}
		return exec.Command(filepath.Join(dir, "definitely-missing-bin-xyz"))
	}
	restartApp("main.go")
}

func TestStopApp_ForceTimeout(t *testing.T) {
	oldTimeout := stopTimeout
	stopTimeout = 5 * time.Millisecond
	defer func() { stopTimeout = oldTimeout }()

	var cmd *exec.Cmd
	if os.PathSeparator == '\\' {
		cmd = exec.Command("cmd", "/c", "ping", "-n", "20", "127.0.0.1")
	} else {
		cmd = exec.Command("sleep", "20")
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	cmdProcess = cmd
	// Soft kill may or may not work quickly; timeout path uses force kill.
	stopApp()
	if cmdProcess != nil {
		t.Fatal("expected cleared")
	}
}
