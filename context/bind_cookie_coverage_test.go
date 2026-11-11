package context

import (
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/router"
)

func TestBindQueryFormURI_Edges(t *testing.T) {
	cNil := &Context{}
	if err := cNil.BindQuery(&struct{}{}); err == nil {
		t.Fatal("nil request BindQuery")
	}
	if err := cNil.BindForm(&struct{}{}); err == nil {
		t.Fatal("nil request BindForm")
	}

	type Q struct {
		Name   string   `query:"name"`
		Age    int      `form:"age"`
		Ok     bool     `query:"ok"`
		Skip   string   `query:"-"`
		hidden string   // unexported
		Score  float64  `query:"score"`
		Count  uint     `query:"count"`
		Tags   []string `query:"tags"`
	}
	r := httptest.NewRequest("GET", "/?name=a&age=2&ok=true&score=1.5&count=3&tags=%5B%22x%22%5D", nil)
	c := New(httptest.NewRecorder(), r)
	var q Q
	q.hidden = "secret"
	if q.hidden != "secret" {
		t.Fatal("hidden")
	}
	if err := c.BindQuery(&q); err != nil {
		t.Fatal(err)
	}
	if q.Name != "a" || !q.Ok || q.Score != 1.5 || q.Count != 3 {
		t.Fatalf("%+v", q)
	}
	if err := c.BindQuery(q); err == nil {
		t.Fatal("want non-pointer error")
	}
	var n int
	if err := c.BindQuery(&n); err == nil {
		t.Fatal("want non-struct error")
	}

	r2 := httptest.NewRequest("POST", "/", strings.NewReader("age=9&name=b"))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c2 := New(httptest.NewRecorder(), r2)
	var f Q
	if err := c2.BindForm(&f); err != nil {
		t.Fatal(err)
	}
	if f.Age != 9 {
		t.Fatalf("age=%d", f.Age)
	}

	var buf strings.Builder
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("age", "1")
	_ = mw.Close()
	r3 := httptest.NewRequest("POST", "/", strings.NewReader(buf.String()))
	r3.Header.Set("Content-Type", mw.FormDataContentType())
	c3 := New(httptest.NewRecorder(), r3)
	var f3 Q
	if err := c3.BindForm(&f3); err != nil {
		t.Fatal(err)
	}

	c4 := New(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	c4.Params = router.Params{{Key: "id", Value: "7"}}
	var uri struct {
		ID int `uri:"id"`
	}
	if err := c4.BindURI(&uri); err != nil || uri.ID != 7 {
		t.Fatalf("%v %+v", err, uri)
	}

	// setField parse errors
	r5 := httptest.NewRequest("GET", "/?ok=notbool&age=xx&count=-1&score=bad&tags={", nil)
	c5 := New(httptest.NewRecorder(), r5)
	var bad Q
	_ = c5.BindQuery(&bad) // first failing field returns
}

func TestCookieOptions_Edges(t *testing.T) {
	w := httptest.NewRecorder()
	c := New(w, httptest.NewRequest("GET", "/", nil))
	c.SetCookieWith("a", "b", CookieOptions{}) // default path
	c.SetCookieWith("c", "d", CookieOptions{Path: "/p", MaxAge: 10, Secure: true, HTTPOnly: true, SameSite: http.SameSiteStrictMode})
	c.ClearCookieWith("a", CookieOptions{})
	c.ClearCookieWith("c", CookieOptions{Path: "/p", Secure: true})
	c.ClearCookie("x", "", "")
}
