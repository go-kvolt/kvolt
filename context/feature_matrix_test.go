package context

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/router"
)

func TestBindURI_AndForm(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/users/9", nil)
	c := New(w, r)
	c.Params = router.Params{{Key: "id", Value: "9"}}
	var uri struct {
		ID int `uri:"id"`
	}
	if err := c.BindURI(&uri); err != nil || uri.ID != 9 {
		t.Fatalf("BindURI: %+v err=%v", uri, err)
	}

	r2 := httptest.NewRequest("POST", "/", strings.NewReader("name=bob&age=3"))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c2 := New(httptest.NewRecorder(), r2)
	var form struct {
		Name string `form:"name"`
		Age  int    `form:"age"`
	}
	if err := c2.BindForm(&form); err != nil {
		t.Fatal(err)
	}
	if form.Name != "bob" || form.Age != 3 {
		t.Fatalf("form=%+v", form)
	}
}

func TestRedirect_RejectsOpenRedirect(t *testing.T) {
	c := New(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if err := c.Redirect(302, "https://evil.example/"); err == nil {
		t.Fatal("absolute URL should be rejected")
	}
	if err := c.Redirect(302, "//evil.example/"); err == nil {
		t.Fatal("protocol-relative should be rejected")
	}
	w := httptest.NewRecorder()
	c2 := New(w, httptest.NewRequest("GET", "/", nil))
	if err := c2.Redirect(302, "/safe"); err != nil {
		t.Fatal(err)
	}
	if loc := w.Header().Get("Location"); loc != "/safe" {
		t.Fatalf("Location=%q", loc)
	}
}

func TestJSON_BindAndRespond(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"n":7}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c := New(w, r)
	var in struct {
		N int `json:"n"`
	}
	if err := c.BindJSON(&in); err != nil || in.N != 7 {
		t.Fatalf("bind=%+v err=%v", in, err)
	}
	if err := c.JSON(201, map[string]int{"n": in.N}); err != nil {
		t.Fatal(err)
	}
	if w.Code != 201 || !strings.Contains(w.Body.String(), `"n":7`) {
		t.Fatalf("resp code=%d body=%s", w.Code, w.Body.String())
	}
}
