package router

import "testing"

func TestStaticSiblingPreferredOverParam(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/user/new", "new")
	r.AddRoute("GET", "/user/:id", "id")

	h, ps, ok := r.Find("GET", "/user/new")
	if !ok || h != "new" {
		t.Fatalf("want static new, got ok=%v h=%v id=%q", ok, h, ps.Get("id"))
	}
	h2, ps2, ok2 := r.Find("GET", "/user/42")
	if !ok2 || h2 != "id" || ps2.Get("id") != "42" {
		t.Fatalf("want param, got ok=%v h=%v id=%q", ok2, h2, ps2.Get("id"))
	}
}

func TestParamThenStatic_StaticWins(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/user/:id", "id")
	r.AddRoute("GET", "/user/new", "new")

	h, _, ok := r.Find("GET", "/user/new")
	if !ok || h != "new" {
		t.Fatalf("want static new, got ok=%v h=%v", ok, h)
	}
	h2, ps2, ok2 := r.Find("GET", "/user/9")
	if !ok2 || h2 != "id" || ps2.Get("id") != "9" {
		t.Fatalf("want param, got ok=%v h=%v id=%q", ok2, h2, ps2.Get("id"))
	}
}

func TestAuthLoginStaticOverProviderParam(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/auth/login", "login")
	r.AddRoute("GET", "/auth/:provider", "provider")

	h, _, ok := r.Find("GET", "/auth/login")
	if !ok || h != "login" {
		t.Fatalf("want login, got ok=%v h=%v", ok, h)
	}
	h2, ps, ok2 := r.Find("GET", "/auth/github")
	if !ok2 || h2 != "provider" || ps.Get("provider") != "github" {
		t.Fatalf("want provider, got ok=%v h=%v p=%q", ok2, h2, ps.Get("provider"))
	}
}

func TestConflictingParamNamesPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on conflicting param names")
		}
	}()
	r := New()
	r.AddRoute("GET", "/u/:id", "a")
	r.AddRoute("GET", "/u/:name", "b")
}

func TestCatchAllAndStaticSibling(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/assets/logo.png", "logo")
	r.AddRoute("GET", "/assets/*filepath", "rest")

	h, _, ok := r.Find("GET", "/assets/logo.png")
	if !ok || h != "logo" {
		t.Fatalf("static: ok=%v h=%v", ok, h)
	}
	h2, ps, ok2 := r.Find("GET", "/assets/css/app.css")
	if !ok2 || h2 != "rest" || ps.Get("filepath") == "" {
		t.Fatalf("catch-all: ok=%v h=%v fp=%q", ok2, h2, ps.Get("filepath"))
	}
}
