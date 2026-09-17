package context

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetCookieAndClear(t *testing.T) {
	rec := httptest.NewRecorder()
	c := New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.SetCookie("sid", "abc", 3600, "/", "", true, true)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "sid" || cookies[0].Value != "abc" {
		t.Fatalf("cookies %#v", cookies)
	}
	if !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Fatal("flags")
	}

	rec2 := httptest.NewRecorder()
	c2 := New(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	c2.ClearCookie("sid", "/", "")
	cleared := rec2.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Fatalf("clear %#v", cleared)
	}
}
