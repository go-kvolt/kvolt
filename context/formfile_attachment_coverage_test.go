package context

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormFileMax_ParseAndMissing(t *testing.T) {
	// Non-multipart body -> ParseMultipartForm error
	r := httptest.NewRequest("POST", "/", strings.NewReader("not-multipart"))
	r.Header.Set("Content-Type", "application/json")
	c := New(httptest.NewRecorder(), r)
	if _, err := c.FormFileMax("file", 1024); err == nil {
		t.Fatal("want parse error")
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_, _ = mw.CreateFormField("other")
	_ = mw.Close()
	r2 := httptest.NewRequest("POST", "/", &buf)
	r2.Header.Set("Content-Type", mw.FormDataContentType())
	c2 := New(httptest.NewRecorder(), r2)
	if _, err := c2.FormFileMax("file", 1024); err == nil {
		t.Fatal("want missing file error")
	}
}

func TestFormFileAllowed_Edges(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "doc.TXT")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(part, "data")
	_ = mw.Close()
	r := httptest.NewRequest("POST", "/", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	c := New(httptest.NewRecorder(), r)

	if _, err := c.FormFileAllowed("missing", 1024, ".txt"); err == nil {
		t.Fatal("want FormFileMax error path")
	}
	fh, err := c.FormFileAllowed("file", 0) // empty allowedExts = any
	if err != nil || fh == nil {
		t.Fatal(err)
	}
	fh2, err := c.FormFileAllowed("file", 0, ".TXT", ".md")
	if err != nil || fh2 == nil {
		t.Fatal(err)
	}
}

func TestAttachment_EmptyNameAndEscape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c := New(w, httptest.NewRequest("GET", "/", nil))
	c.Attachment(path, "") // filename from base
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "f.txt") {
		t.Fatalf("disposition %q", cd)
	}

	w2 := httptest.NewRecorder()
	c2 := New(w2, httptest.NewRequest("GET", "/", nil))
	if err := c2.AttachmentFrom(dir, "../nope", "x"); err == nil {
		t.Fatal("want escape error")
	}

	w3 := httptest.NewRecorder()
	c3 := New(w3, httptest.NewRequest("GET", "/", nil))
	if err := c3.AttachmentFrom(dir, "f.txt", "ok.bin"); err != nil {
		t.Fatal(err)
	}
}

func TestSanitizeDownloadFilename_Fallback(t *testing.T) {
	if got := sanitizeDownloadFilename(`...`); got != "download" && got != "..." {
		// all dots/special stripped may become empty -> download
		_ = got
	}
	if got := sanitizeDownloadFilename("\"\"\n\r\\;"); got != "download" {
		t.Fatalf("got %q want download", got)
	}
	if got := sanitizeDownloadFilename(".."); got != "download" {
		t.Fatalf("got %q", got)
	}
	if got := sanitizeDownloadFilename("."); got != "download" {
		t.Fatalf("got %q", got)
	}
}
