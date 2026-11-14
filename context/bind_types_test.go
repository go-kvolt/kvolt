package context

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/router"
)

func TestBindForm_AllFieldKinds(t *testing.T) {
	type body struct {
		Name   string         `form:"name"`
		Ok     bool           `form:"ok"`
		Age    int            `form:"age"`
		Count  uint           `form:"count"`
		Score  float64        `form:"score"`
		Tags   []string       `form:"tags"`
		Meta   map[string]int `form:"meta"`
		Nested struct {
			X int `json:"x"`
		} `form:"nested"`
		Skip string `form:"-"`
	}
	form := "name=Ada&ok=true&age=30&count=2&score=1.5&tags=%5B%22a%22%2C%22b%22%5D&meta=%7B%22n%22%3A1%7D&nested=%7B%22x%22%3A9%7D"
	r := httptest.NewRequest("POST", "/", strings.NewReader(form))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := New(httptest.NewRecorder(), r)
	var b body
	if err := c.BindForm(&b); err != nil {
		t.Fatal(err)
	}
	if b.Name != "Ada" || !b.Ok || b.Age != 30 || b.Count != 2 || b.Score != 1.5 {
		t.Fatalf("%+v", b)
	}
	if len(b.Tags) != 2 || b.Meta["n"] != 1 || b.Nested.X != 9 {
		t.Fatalf("%+v", b)
	}

	cNil := New(httptest.NewRecorder(), nil)
	if err := cNil.BindForm(&b); err == nil {
		t.Fatal("nil request")
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("name", "M")
	_ = mw.Close()
	rMP := httptest.NewRequest("POST", "/", &buf)
	rMP.Header.Set("Content-Type", mw.FormDataContentType())
	var b2 body
	if err := New(httptest.NewRecorder(), rMP).BindForm(&b2); err != nil {
		t.Fatal(err)
	}
	if b2.Name != "M" {
		t.Fatalf("%+v", b2)
	}
}

func TestBindForm_ParseErrorsAndBadTypes(t *testing.T) {
	type bad struct {
		N int `form:"n"`
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader("n=x"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := New(httptest.NewRecorder(), r).BindForm(&bad{}); err == nil {
		t.Fatal("want parse int error")
	}

	type u struct {
		N uint `form:"n"`
	}
	r2 := httptest.NewRequest("POST", "/", strings.NewReader("n=-1"))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := New(httptest.NewRecorder(), r2).BindForm(&u{}); err == nil {
		t.Fatal("want uint error")
	}

	type f struct {
		N float32 `form:"n"`
	}
	r3 := httptest.NewRequest("POST", "/", strings.NewReader("n=bad"))
	r3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := New(httptest.NewRecorder(), r3).BindForm(&f{}); err == nil {
		t.Fatal("want float error")
	}

	type bo struct {
		Ok bool `form:"ok"`
	}
	r4 := httptest.NewRequest("POST", "/", strings.NewReader("ok=nope"))
	r4.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := New(httptest.NewRecorder(), r4).BindForm(&bo{}); err == nil {
		t.Fatal("want bool error")
	}

	type sl struct {
		Tags []string `form:"tags"`
	}
	r5 := httptest.NewRequest("POST", "/", strings.NewReader("tags={"))
	r5.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := New(httptest.NewRecorder(), r5).BindForm(&sl{}); err == nil {
		t.Fatal("want json error")
	}

	c := New(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	c.Params = router.Params{{Key: "id", Value: "1"}}
	type uri struct {
		ID int `uri:"id"`
	}
	var u2 uri
	if err := c.BindURI(&u2); err != nil || u2.ID != 1 {
		t.Fatal(err, u2)
	}
}

func TestBindQuery_CommaSeparatedSlices(t *testing.T) {
	type Filter struct {
		Statuses []string `query:"status"`
		UserIDs  []int    `query:"ids"`
	}
	r := httptest.NewRequest("GET", "/invoices?status=paid,pending&ids=10,20,30", nil)
	c := New(httptest.NewRecorder(), r)
	var f Filter
	if err := c.BindQuery(&f); err != nil {
		t.Fatalf("BindQuery: %v", err)
	}
	if len(f.Statuses) != 2 || f.Statuses[0] != "paid" || f.Statuses[1] != "pending" {
		t.Errorf("Statuses mismatch: %+v", f.Statuses)
	}
	if len(f.UserIDs) != 3 || f.UserIDs[0] != 10 || f.UserIDs[1] != 20 || f.UserIDs[2] != 30 {
		t.Errorf("UserIDs mismatch: %+v", f.UserIDs)
	}
}

func TestBindQuery_EmptySlice(t *testing.T) {
	type Filter struct {
		Statuses []string  `query:"status"`
		UserIDs  []int     `query:"ids"`
		Rates    []float64 `query:"rates"`
		Flags    []bool    `query:"flags"`
	}
	r := httptest.NewRequest("GET", "/test?status=&ids=&rates=&flags=", nil)
	c := New(httptest.NewRecorder(), r)
	var f Filter
	if err := c.BindQuery(&f); err != nil {
		t.Fatalf("BindQuery empty slice: %v", err)
	}
	if len(f.Statuses) != 0 || len(f.UserIDs) != 0 || len(f.Rates) != 0 || len(f.Flags) != 0 {
		t.Fatalf("expected empty slices, got %+v", f)
	}
}
