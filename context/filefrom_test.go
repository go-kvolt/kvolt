package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeJoinRoot_BlocksEscape(t *testing.T) {
	root := t.TempDir()
	if _, err := safeJoinRoot(root, "../outside.txt"); err == nil {
		t.Fatal("expected escape error")
	}
	got, err := safeJoinRoot(root, "ok.txt")
	if err != nil {
		t.Fatal(err)
	}
	absRoot, _ := filepath.Abs(root)
	if r, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = r
	}
	want := filepath.Join(absRoot, "ok.txt")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSafeJoinRoot_BlocksSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlink not permitted:", err)
	}
	if _, err := safeJoinRoot(root, filepath.Join("escape", "secret.txt")); err == nil {
		t.Fatal("expected symlink escape to be blocked")
	}
}
