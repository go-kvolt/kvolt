package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestToExportName(t *testing.T) {
	if toExportName("") != "" || toExportName("foo") != "Foo" {
		t.Fatal(toExportName("foo"))
	}
}

func TestWriteFileAndGenerate(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()

	_ = os.MkdirAll("out", 0o755)
	writeFile(filepath.Join("out", "a.txt"), "hi")
	b, err := os.ReadFile(filepath.Join("out", "a.txt"))
	if err != nil || string(b) != "hi" {
		t.Fatal(err, string(b))
	}

	generateHandler("hello")
	generateMiddleware("authcheck")
	if _, err := os.Stat(filepath.Join("internal", "handler", "hello.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join("internal", "middleware", "authcheck.go")); err != nil {
		t.Fatal(err)
	}
}

func TestShouldRestartOnEvent(t *testing.T) {
	if !shouldRestartOnEvent(fsnotify.Event{Name: "foo.go", Op: fsnotify.Write}) {
		t.Fatal("go write")
	}
	if shouldRestartOnEvent(fsnotify.Event{Name: "foo.txt", Op: fsnotify.Write}) {
		t.Fatal("txt")
	}
	if !shouldRestartOnEvent(fsnotify.Event{Name: "a.yaml", Op: fsnotify.Create}) {
		t.Fatal("yaml")
	}
}

func TestDevBinaryPath(t *testing.T) {
	p, err := devBinaryPath(t.TempDir())
	if err != nil || p == "" {
		t.Fatalf("%q %v", p, err)
	}
}

func TestCreateProject(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	createProject("demo")
	if _, err := os.Stat(filepath.Join("demo", "cmd", "api", "main.go")); err != nil {
		t.Fatal(err)
	}
}

func TestProcAttrsForDev(t *testing.T) {
	_ = procAttrsForDev()
}
