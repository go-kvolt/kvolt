package context

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeJoinRoot_AndUnderRoot(t *testing.T) {
	if _, err := safeJoinRoot("  ", "a"); err == nil {
		t.Fatal("empty root")
	}
	oldAbs := absPath
	absPath = func(string) (string, error) { return "", errors.New("abs fail") }
	if _, err := safeJoinRoot(".", "a"); err == nil {
		t.Fatal("abs fail")
	}
	absPath = oldAbs

	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("x"), 0o644)
	p, err := safeJoinRoot(dir, "ok.txt")
	if err != nil || p == "" {
		t.Fatal(err, p)
	}
	if _, err := safeJoinRoot(dir, ".."+string(filepath.Separator)+"outside"); err == nil {
		t.Fatal("escape")
	}
	// missing path stays lexical
	p2, err := safeJoinRoot(dir, "missing.txt")
	if err != nil || p2 == "" {
		t.Fatal(err, p2)
	}

	oldRel := pathRel
	pathRel = func(string, string) (string, error) { return "", errors.New("rel fail") }
	if underRoot(dir, filepath.Join(dir, "ok.txt")) {
		t.Fatal("rel fail should be false")
	}
	pathRel = oldRel

	oldEval := evalSymlinks
	evalSymlinks = func(string) (string, error) { return "", errors.New("eval fail") }
	// existing file with eval fail → error
	if _, err := safeJoinRoot(dir, "ok.txt"); err == nil {
		t.Fatal("eval fail on existing")
	}
	evalSymlinks = oldEval
}
