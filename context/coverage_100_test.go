package context

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) { return 0, errors.New("read boom") }
func (errReadCloser) Close() error             { return nil }

type failWriter struct {
	http.ResponseWriter
}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write boom") }

func TestCoverage100_ContextGaps(t *testing.T) {
	t.Run("bindForm_parseForm_error", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", nil)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Body = errReadCloser{}
		c := New(httptest.NewRecorder(), r)
		if err := c.BindForm(&struct {
			A string `form:"a"`
		}{}); err == nil {
			t.Fatal("want ParseForm error")
		}
	})

	t.Run("setField_cannot_set", func(t *testing.T) {
		type unexp struct{ a string }
		v := reflect.ValueOf(unexp{a: "x"}).Field(0) // not addressable
		if err := setField(v, "y"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("clientip_xri_hostport", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "10.0.0.1:9"
		r.Header.Set("X-Real-IP", "203.0.113.9:443")
		c := New(httptest.NewRecorder(), r)
		c.TrustedProxies = []string{"10.0.0.1"}
		if got := c.ClientIP(); got != "203.0.113.9" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("statusCode_headerWritten_zero_status", func(t *testing.T) {
		c := New(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		c.headerWritten = true
		c.status = 0
		if c.StatusCode() != http.StatusOK {
			t.Fatalf("got %d", c.StatusCode())
		}
	})

	t.Run("writeHeader_uses_existing_status", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := New(w, httptest.NewRequest("GET", "/", nil))
		c.status = 201
		c.writeHeader(0)
		if w.Code != 201 {
			t.Fatalf("got %d", w.Code)
		}
	})

	t.Run("rawBody_read_error", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", nil)
		r.Body = errReadCloser{}
		c := New(httptest.NewRecorder(), r)
		if _, err := c.RawBody(); err == nil {
			t.Fatal("want read error")
		}
	})

	t.Run("saveUploadedFile_open_and_create_errors", func(t *testing.T) {
		c := New(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil))
		fh := &multipart.FileHeader{Filename: "x.png"}
		if err := c.SaveUploadedFile(fh, "out.png"); err == nil {
			t.Fatal("want Open error")
		}

		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		part, _ := mw.CreateFormFile("f", "y.png")
		_, _ = io.WriteString(part, "data")
		_ = mw.Close()
		r := httptest.NewRequest("POST", "/", &buf)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		c2 := New(httptest.NewRecorder(), r)
		good, err := c2.FormFile("f")
		if err != nil {
			t.Fatal(err)
		}
		td := t.TempDir()
		old, _ := os.Getwd()
		_ = os.Chdir(td)
		defer func() { _ = os.Chdir(old) }()
		if err := c2.SaveUploadedFile(good, filepath.Join("missing_dir", "out.png")); err == nil {
			t.Fatal("want Create error")
		}
	})

	t.Run("safeJoinRoot_and_underRoot", func(t *testing.T) {
		if _, err := safeJoinRoot("", "a"); err == nil {
			t.Fatal("empty root")
		}
		oldAbs := absPath
		absPath = func(string) (string, error) { return "", errors.New("abs fail") }
		if _, err := safeJoinRoot("x", "a"); err == nil {
			t.Fatal("abs error")
		}
		absPath = oldAbs

		root := t.TempDir()
		// Existing file so Lstat succeeds; force EvalSymlinks error.
		name := "present.txt"
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		oldEval := evalSymlinks
		evalSymlinks = func(path string) (string, error) {
			if filepath.Base(path) == name {
				return "", errors.New("eval fail")
			}
			return filepath.EvalSymlinks(path)
		}
		if _, err := safeJoinRoot(root, name); err == nil {
			t.Fatal("want evalSymlinks error")
		}
		// Resolved path escapes root.
		evalSymlinks = func(path string) (string, error) {
			if filepath.Base(path) == name {
				return filepath.Join(t.TempDir(), "outside.txt"), nil
			}
			return filepath.EvalSymlinks(path)
		}
		if _, err := safeJoinRoot(root, name); err == nil {
			t.Fatal("want escape via resolved path")
		}
		evalSymlinks = oldEval

		oldRel := pathRel
		pathRel = func(string, string) (string, error) { return "", errors.New("rel fail") }
		if underRoot("/a", "/b") {
			t.Fatal("rel error → false")
		}
		pathRel = oldRel
		if runtime.GOOS == "windows" {
			if underRoot(`C:\root`, `D:\other`) {
				t.Fatal("cross-volume should be false")
			}
		}
	})

	t.Run("redirect_and_safe_relative", func(t *testing.T) {
		w := httptest.NewRecorder()
		c := New(w, httptest.NewRequest("GET", "http://ex.com/", nil))
		if err := c.Redirect(200, "/ok"); err != nil { // code out of 300-308 → 302
			t.Fatal(err)
		}
		if w.Code != http.StatusFound {
			t.Fatalf("code=%d", w.Code)
		}
		if err := c.RedirectURL(200, ""); err == nil {
			t.Fatal("empty RedirectURL")
		}
		w2 := httptest.NewRecorder()
		c2 := New(w2, httptest.NewRequest("GET", "http://ex.com/", nil))
		if err := c2.RedirectURL(200, "https://ex.com/x"); err != nil {
			t.Fatal(err)
		}
		if w2.Code != http.StatusFound {
			t.Fatalf("RedirectURL code=%d", w2.Code)
		}

		if isSafeRelativeRedirect("") || isSafeRelativeRedirect("  ") {
			t.Fatal("empty")
		}
		if isSafeRelativeRedirect("/http://evil") || isSafeRelativeRedirect("/path://x") {
			t.Fatal("://")
		}
		if isSafeRelativeRedirect("/javascript:alert(1)") || isSafeRelativeRedirect("/data:text/html") || isSafeRelativeRedirect("/vbscript:x") {
			t.Fatal("scheme-ish")
		}
	})

	t.Run("sameOrigin_parse_error", func(t *testing.T) {
		r := httptest.NewRequest("GET", "http://ex.com/", nil)
		r.Host = "ex.com"
		r.Header.Set("Origin", "://") // url.Parse error on some Go versions; or invalid
		// Force parse failure with malformed URL that Parse rejects
		r.Header.Set("Origin", "http://[::1")
		if sameOrigin(r) {
			t.Fatal("want false on bad origin")
		}
	})

	t.Run("ssevent_write_error", func(t *testing.T) {
		c := New(failWriter{ResponseWriter: httptest.NewRecorder()}, httptest.NewRequest("GET", "/", nil))
		if err := c.SSEvent(SSEvent{Data: "x"}); err == nil {
			t.Fatal("want write error")
		}
	})
}
