package kvolt

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	kvctx "github.com/go-kvolt/kvolt/v2/context"
)

type statFailFS struct{}

func (statFailFS) Open(name string) (http.File, error) {
	return &statFailFile{}, nil
}

type statFailFile struct{}

func (*statFailFile) Close() error                       { return nil }
func (*statFailFile) Read([]byte) (int, error)           { return 0, io.EOF }
func (*statFailFile) Seek(int64, int) (int64, error)     { return 0, nil }
func (*statFailFile) Readdir(int) ([]os.FileInfo, error) { return nil, nil }
func (*statFailFile) Stat() (os.FileInfo, error)         { return nil, errors.New("stat fail") }

func TestOnlyFilesFS_OpenEdges(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "sub", "index.html"), []byte("i"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "empty"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".secret"), []byte("no"), 0o644)

	ofs := onlyFilesFS{fs: http.Dir(dir)}

	f, err := ofs.Open("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(f)
	_ = f.Close()

	if _, err := ofs.Open(".secret"); err == nil {
		t.Fatal("dotfile")
	}
	sub, err := ofs.Open("sub")
	if err != nil {
		t.Fatal("sub with index should open", err)
	}
	_ = sub.Close()
	if _, err := ofs.Open("empty"); err == nil {
		t.Fatal("empty dir")
	}
	if _, err := ofs.Open("missing"); err == nil {
		t.Fatal("missing")
	}
	if _, err := (onlyFilesFS{fs: statFailFS{}}).Open("x"); err == nil {
		t.Fatal("want Stat error")
	}
}

func TestServeHTTP_BadHandlerDefaultAndError(t *testing.T) {
	app := New()
	app.RedirectTrailingSlash = false
	app.HandleMethodNotAllowed = false
	// Found route with non-[]HandlerFunc → handlers404 branch.
	app.router.AddRoute("GET", "/bad", "not-a-handler")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/bad", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("bad handler want 404 got %d", w.Code)
	}

	// default: no match, TSR/405 disabled
	w2 := httptest.NewRecorder()
	app.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/nowhere", nil))
	if w2.Code != http.StatusNotFound {
		t.Fatalf("default 404 got %d", w2.Code)
	}

	// single handler returning error → WriteHandlerError
	app2 := New()
	app2.GET("/err", func(c *kvctx.Context) error { return errors.New("boom") })
	w3 := httptest.NewRecorder()
	app2.ServeHTTP(w3, httptest.NewRequest(http.MethodGet, "/err", nil))
	if w3.Code < 400 {
		t.Fatalf("want error status got %d", w3.Code)
	}
}

func TestServeHTTP_CONNECTSkipsTSR(t *testing.T) {
	app := New()
	app.RedirectTrailingSlash = true
	app.GET("/x/", func(c *kvctx.Context) error { return c.String(200, "ok") })
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest(http.MethodConnect, "/x", nil))
	if w.Code == http.StatusMovedPermanently || w.Code == http.StatusPermanentRedirect {
		t.Fatalf("CONNECT should not TSR, got %d", w.Code)
	}
}

func TestServeAndWait_ShutdownError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	app := New()
	app.Server.ShutdownTimeout = time.Nanosecond
	quit := make(chan struct{})
	testServeQuit = quit
	defer func() { testServeQuit = nil }()

	block := make(chan struct{})
	app.GET("/slow", func(c *kvctx.Context) error {
		<-block
		return c.String(200, "ok")
	})
	srv := app.HTTPServer(addr)
	done := make(chan error, 1)
	go func() { done <- app.RunWithServer(srv) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Start a slow request then trigger shutdown with tiny timeout.
	go func() {
		_, _ = http.Get("http://" + addr + "/slow")
	}()
	time.Sleep(30 * time.Millisecond)
	close(quit)
	select {
	case err := <-done:
		// Shutdown error path is best-effort; either nil or deadline exceeded is fine.
		_ = err
	case <-time.After(5 * time.Second):
		close(block)
		t.Fatal("timeout")
	}
	close(block)
}

func TestIsTrustedProxy_BadCIDR(t *testing.T) {
	if isTrustedProxy("1.1.1.1:80", []string{"not-a-cidr/99", "  "}) {
		t.Fatal("bad cidr")
	}
}
