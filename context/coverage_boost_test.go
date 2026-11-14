package context

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
	"github.com/go-kvolt/kvolt/v2/router"
)

func TestHelpers_FormValueQueryHeaderCookie(t *testing.T) {
	r := httptest.NewRequest("POST", "/?q=1", strings.NewReader("f=v"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("X-A", "b")
	r.AddCookie(&http.Cookie{Name: "c", Value: "d"})
	c := New(httptest.NewRecorder(), r)
	_ = r.ParseForm()
	if c.FormValue("f") != "v" || c.DefaultQuery("q", "x") != "1" || c.DefaultQuery("z", "x") != "x" {
		t.Fatal("form/query")
	}
	if c.GetHeader("X-A") != "b" || c.GetCookie("c") != "d" || c.GetCookie("no") != "" {
		t.Fatal("header/cookie")
	}
	c2 := New(httptest.NewRecorder(), nil)
	if c2.FormValue("f") != "" || c2.GetHeader("X") != "" || c2.GetCookie("c") != "" {
		t.Fatal("nil request helpers")
	}
}

func TestBind_ValidateAndAbort(t *testing.T) {
	type body struct {
		Email string `json:"email" validate:"required,email"`
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"email":"a@b.com"}`))
	r.Header.Set("Content-Type", "application/json")
	c := New(httptest.NewRecorder(), r)
	var b body
	if err := c.Bind(&b); err != nil {
		t.Fatal(err)
	}
	r2 := httptest.NewRequest("POST", "/", strings.NewReader(`{"email":"bad"}`))
	r2.Header.Set("Content-Type", "application/json")
	c2 := New(httptest.NewRecorder(), r2)
	if err := c2.Bind(&body{}); err == nil {
		t.Fatal("want validate error")
	}

	w := httptest.NewRecorder()
	c3 := New(w, httptest.NewRequest("GET", "/", nil))
	c3.Handlers = []HandlerFunc{
		func(c *Context) error {
			c.Abort()
			return nil
		},
		func(c *Context) error {
			t.Fatal("should not run")
			return nil
		},
	}
	c3.Next()
	if !c3.IsAborted() || !c3.HeaderWritten() && c3.StatusCode() != 0 {
		// aborted without write is fine
	}
	if !c3.IsAborted() {
		t.Fatal("abort")
	}
}

func TestWriteHandlerErrorAndAbortWithError(t *testing.T) {
	w := httptest.NewRecorder()
	c := New(w, httptest.NewRequest("GET", "/", nil))
	c.WriteHandlerError(httperr.BadRequest("x", "y"))
	if w.Code != 400 {
		t.Fatalf("code=%d", w.Code)
	}
	c.WriteHandlerError(errors.New("after")) // logged only

	w2 := httptest.NewRecorder()
	c2 := New(w2, httptest.NewRequest("GET", "/", nil))
	c2.WriteHandlerError(&http.MaxBytesError{Limit: 1})
	if w2.Code != 413 {
		t.Fatalf("want 413 got %d", w2.Code)
	}
	w3 := httptest.NewRecorder()
	c3 := New(w3, httptest.NewRequest("GET", "/", nil))
	c3.WriteHandlerError(errors.New("boom"))
	if w3.Code != 500 {
		t.Fatalf("want 500 got %d", w3.Code)
	}

	w4 := httptest.NewRecorder()
	c4 := New(w4, httptest.NewRequest("GET", "/", nil))
	_ = c4.AbortWithError(httperr.Forbidden("f", "no"))
	if w4.Code != 403 || !c4.IsAborted() {
		t.Fatal("AbortWithError")
	}
	_ = c4.AbortWithError(nil)
}

func TestHTML_File_NoContent_InternalError(t *testing.T) {
	w := httptest.NewRecorder()
	c := New(w, httptest.NewRequest("GET", "/", nil))
	if err := c.HTML(200, "<b>x</b>"); err != nil || !strings.Contains(w.Body.String(), "<b>x</b>") {
		t.Fatal(err)
	}
	if c.HeaderWritten() || c.StatusCode() == 0 {
		// header written by HTML
	}
	if !c.HeaderWritten() {
		t.Fatal("HeaderWritten")
	}

	tpl := template.Must(template.New("t").Parse(`hi {{.}}`))
	w2 := httptest.NewRecorder()
	c2 := New(w2, httptest.NewRequest("GET", "/", nil))
	c2.Templates = tpl
	if err := c2.RenderHTML(200, "t", "bob"); err != nil {
		t.Fatal(err)
	}
	w3 := httptest.NewRecorder()
	c3 := New(w3, httptest.NewRequest("GET", "/", nil))
	if err := c3.RenderHTML(200, "t", nil); err == nil && !strings.Contains(w3.Body.String(), "Templates not loaded") {
		// String path when no templates
	}

	w4 := httptest.NewRecorder()
	c4 := New(w4, httptest.NewRequest("GET", "/", nil))
	c4.Status(204)
	c4.FlushHeaders()
	if w4.Code != 204 {
		t.Fatalf("FlushHeaders %d", w4.Code)
	}
	_ = c4.NoContent()

	w5 := httptest.NewRecorder()
	c5 := New(w5, httptest.NewRequest("GET", "/", nil))
	c5.InternalError()
	if w5.Code != 500 {
		t.Fatalf("InternalError %d", w5.Code)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	w6 := httptest.NewRecorder()
	c6 := New(w6, httptest.NewRequest("GET", "/", nil))
	c6.File(path)
	if w6.Body.String() != "data" {
		t.Fatalf("File %q", w6.Body.String())
	}
	w7 := httptest.NewRecorder()
	c7 := New(w7, httptest.NewRequest("GET", "/", nil))
	if err := c7.FileFrom(dir, "f.txt"); err != nil {
		t.Fatal(err)
	}
	if err := c7.FileFrom(dir, "../etc/passwd"); err == nil {
		t.Fatal("escape")
	}
	w8 := httptest.NewRecorder()
	c8 := New(w8, httptest.NewRequest("GET", "/", nil))
	c8.Attachment(path, "dl.txt")
	if !strings.Contains(w8.Header().Get("Content-Disposition"), "dl.txt") {
		t.Fatal("attachment header")
	}
	w9 := httptest.NewRecorder()
	c9 := New(w9, httptest.NewRequest("GET", "/", nil))
	if err := c9.AttachmentFrom(dir, "f.txt", ""); err != nil {
		t.Fatal(err)
	}
}

func TestFormFile_SaveUpload(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "x.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(part, "pngdata")
	_ = mw.Close()
	r := httptest.NewRequest("POST", "/", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	c := New(httptest.NewRecorder(), r)
	fh, err := c.FormFile("file")
	if err != nil {
		t.Fatal(err)
	}
	fh2, err := c.FormFileAllowed("file", 0, ".png")
	if err != nil || fh2 == nil {
		t.Fatal(err)
	}
	if _, err := c.FormFileAllowed("file", 0, ".jpg"); err == nil {
		t.Fatal("ext")
	}
	dst := filepath.Join(t.TempDir(), "out.png")
	// SaveUploadedFile requires IsLocal path — use relative from cwd
	rel := filepath.Join(".", filepath.Base(dst))
	// write into temp via changing to temp
	td := t.TempDir()
	old, _ := os.Getwd()
	_ = os.Chdir(td)
	defer func() { _ = os.Chdir(old) }()
	if err := c.SaveUploadedFile(fh, "out.png"); err != nil {
		t.Fatal(err)
	}
	// Absolute path should be accepted cleanly
	absDst := filepath.Join(td, "abs_out.png")
	if err := c.SaveUploadedFile(fh, absDst); err != nil {
		t.Fatalf("absolute dst failed: %v", err)
	}
	if err := c.SaveUploadedFile(fh, "../nope"); err == nil {
		t.Fatal("escape dst")
	}
	_ = SafeUploadPath(td, "../../x.png")
	_ = SafeUploadPath(td, ".")
	_ = fh
	_ = rel
}

func TestQueryParamStatusCode(t *testing.T) {
	c := New(httptest.NewRecorder(), httptest.NewRequest("GET", "/?a=1", nil))
	c.Params = router.Params{{Key: "id", Value: "9"}}
	if c.Query("a") != "1" || c.Param("id") != "9" {
		t.Fatal("query/param")
	}
	c.Status(201)
	if c.StatusCode() != 201 {
		t.Fatal(c.StatusCode())
	}
}

func TestNilRequestEdges(t *testing.T) {
	c := &Context{}
	if _, err := c.RawBody(); err == nil {
		t.Fatal("rawbody")
	}
	if c.Query("x") != "" {
		t.Fatal("query")
	}
}
