package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestMainCLIHelpVersionKey(t *testing.T) {
	old := os.Args
	defer func() { os.Args = old }()

	os.Args = []string{"kvolt"}
	main()

	os.Args = []string{"kvolt", "-h"}
	main()

	os.Args = []string{"kvolt", "--help"}
	main()

	os.Args = []string{"kvolt", "-v"}
	main()

	os.Args = []string{"kvolt", "--version"}
	main()

	os.Args = []string{"kvolt", "version"}
	main()

	os.Args = []string{"kvolt", "unknown-cmd"}
	main()

	os.Args = []string{"kvolt", "key"}
	main()
}

func TestRunNewGenerateDockerBuild(t *testing.T) {
	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	old := os.Args
	defer func() { os.Args = old }()

	os.Args = []string{"kvolt", "new"}
	runNew()

	os.Args = []string{"kvolt", "new", "proj"}
	runNew()
	if _, err := os.Stat(filepath.Join("proj", "cmd", "api", "main.go")); err != nil {
		t.Fatal(err)
	}

	os.Args = []string{"kvolt", "generate"}
	runGenerate()

	os.Args = []string{"kvolt", "generate", "handler", "ping"}
	runGenerate()

	os.Args = []string{"kvolt", "generate", "middleware", "mw"}
	runGenerate()

	os.Args = []string{"kvolt", "generate", "other", "x"}
	runGenerate()

	runDocker()
	if _, err := os.Stat("Dockerfile"); err != nil {
		t.Fatal(err)
	}

	os.Args = []string{"kvolt", "build"}
	runBuild() // missing entry — should return without exit

	// write minimal main and build
	_ = os.WriteFile("main.go", []byte("package main\nfunc main() {}\n"), 0o644)
	os.Args = []string{"kvolt", "build", "-o", "bin/app", "-e", "main.go"}
	runBuild()
}

func TestRunFmtAndTest(t *testing.T) {
	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()
	_ = os.WriteFile("go.mod", []byte("module tmp\n\ngo 1.22\n"), 0o644)
	_ = os.WriteFile("x.go", []byte("package tmp\n"), 0o644)

	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"kvolt", "fmt"}
	runFmt()

	os.Args = []string{"kvolt", "test"}
	// may exit 1 if no tests — wrap carefully by not calling if risky
	// create a trivial test
	_ = os.WriteFile("x_test.go", []byte("package tmp\nimport \"testing\"\nfunc TestX(t *testing.T) {}\n"), 0o644)
	runTest()
	os.Args = []string{"kvolt", "test", "-cover"}
	runTest()
}

func TestWatcherLoopAndRestart(t *testing.T) {
	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	_ = os.WriteFile("go.mod", []byte("module tmpdev\n\ngo 1.22\n"), 0o644)
	_ = os.MkdirAll("cmd/api", 0o755)
	_ = os.WriteFile("cmd/api/main.go", []byte("package main\nfunc main() { select {} }\n"), 0o644)

	restartApp("cmd/api/main.go")
	time.Sleep(200 * time.Millisecond)
	stopApp()

	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	_ = w.Add(dir)
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		runWatcherLoop(w.Events, w.Errors, timer)
	}()
	_ = os.WriteFile(filepath.Join(dir, "x.go"), []byte("package tmpdev\n"), 0o644)
	time.Sleep(100 * time.Millisecond)
	_ = w.Close()
	select {
	case <-loopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher loop did not exit after Close")
	}
}

func TestPrintHelpDirect(t *testing.T) {
	printHelp()
}

func TestRunRunAndRunDevUntil(t *testing.T) {
	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	_ = os.WriteFile("go.mod", []byte("module tmpdev2\n\ngo 1.22\n"), 0o644)
	_ = os.MkdirAll("cmd/api", 0o755)
	_ = os.WriteFile("cmd/api/main.go", []byte("package main\nfunc main() { select {} }\n"), 0o644)

	stop := make(chan struct{})
	runDevStop = stop
	defer func() { runDevStop = nil }()

	done := make(chan struct{})
	go func() {
		defer close(done)
		old := os.Args
		os.Args = []string{"kvolt", "run", "-e", "cmd/api/main.go"}
		runRun()
		os.Args = old
	}()
	time.Sleep(400 * time.Millisecond)
	close(stop)
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("runRun hang")
	}
}

func TestWriteFileError(t *testing.T) {
	writeFile(filepath.Join(t.TempDir(), "no", "such", "dir", "f.txt"), "x")
}
