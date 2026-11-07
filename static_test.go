package kvolt

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatic_BlocksDotfilesAndListings(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=1"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "nested.txt"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := New()
	app.Static("/assets", root)

	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "/assets/ok.txt", nil))
	if w.Code != 200 || w.Body.String() != "hi" {
		t.Fatalf("ok.txt: code=%d body=%q", w.Code, w.Body.String())
	}

	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, httptest.NewRequest("GET", "/assets/.env", nil))
	if w2.Code != 404 {
		t.Fatalf(".env: want 404 got %d body=%q", w2.Code, w2.Body.String())
	}

	w3 := httptest.NewRecorder()
	app.ServeHTTP(w3, httptest.NewRequest("GET", "/assets/subdir/", nil))
	if w3.Code != 404 {
		t.Fatalf("subdir listing: want 404 got %d body=%q", w3.Code, w3.Body.String())
	}
	if strings.Contains(w3.Body.String(), "nested.txt") {
		t.Fatal("directory listing leaked nested.txt")
	}
}
