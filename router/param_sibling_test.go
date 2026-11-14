package router

import "testing"

func TestParamSiblingRoutes_BothMatch(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/orders/:orderId/assets", "assets")
	r.AddRoute("GET", "/orders/:orderId/take", "take")

	h1, ps1, ok1 := r.Find("GET", "/orders/ORD-1/assets")
	if !ok1 || h1 != "assets" || ps1.Get("orderId") != "ORD-1" {
		t.Fatalf("assets: ok=%v h=%v orderId=%q", ok1, h1, ps1.Get("orderId"))
	}

	h2, ps2, ok2 := r.Find("GET", "/orders/ORD-2/take")
	if !ok2 || h2 != "take" || ps2.Get("orderId") != "ORD-2" {
		t.Fatalf("take: ok=%v h=%v orderId=%q", ok2, h2, ps2.Get("orderId"))
	}
}

func TestParamSiblingRoutes_RegistrationOrder(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/orders/:orderId/take", "take")
	r.AddRoute("GET", "/orders/:orderId/assets", "assets")

	h1, _, ok1 := r.Find("GET", "/orders/O1/take")
	if !ok1 || h1 != "take" {
		t.Fatalf("take first registered last: ok=%v h=%v", ok1, h1)
	}
	h2, _, ok2 := r.Find("GET", "/orders/O2/assets")
	if !ok2 || h2 != "assets" {
		t.Fatalf("assets: ok=%v h=%v", ok2, h2)
	}
}

func TestParamSiblingRoutes_KaspxStylePaths(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/api/executive/orders/:orderId/assets", "list-assets")
	r.AddRoute("GET", "/api/executive/files/:orderId/:assetId", "file")
	r.AddRoute("POST", "/api/designer/orders/:orderId/decision", "decision")
	r.AddRoute("POST", "/api/designer/preview-assets/:orderId", "preview")

	h1, ps1, ok1 := r.Find("GET", "/api/executive/orders/O-9/assets")
	if !ok1 || h1 != "list-assets" || ps1.Get("orderId") != "O-9" {
		t.Fatalf("exec assets: ok=%v h=%v id=%q", ok1, h1, ps1.Get("orderId"))
	}

	h2, ps2, ok2 := r.Find("GET", "/api/executive/files/O-9/ast-42")
	if !ok2 || h2 != "file" || ps2.Get("orderId") != "O-9" || ps2.Get("assetId") != "ast-42" {
		t.Fatalf("exec file: ok=%v h=%v orderId=%q assetId=%q", ok2, h2, ps2.Get("orderId"), ps2.Get("assetId"))
	}

	h3, ps3, ok3 := r.Find("POST", "/api/designer/orders/O-8/decision")
	if !ok3 || h3 != "decision" || ps3.Get("orderId") != "O-8" {
		t.Fatalf("designer decision: ok=%v h=%v id=%q", ok3, h3, ps3.Get("orderId"))
	}

	h4, ps4, ok4 := r.Find("POST", "/api/designer/preview-assets/O-7")
	if !ok4 || h4 != "preview" || ps4.Get("orderId") != "O-7" {
		t.Fatalf("designer preview: ok=%v h=%v id=%q", ok4, h4, ps4.Get("orderId"))
	}
}

func TestParamRoute_AuthProviderCallback(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/auth/:provider", "provider")
	r.AddRoute("GET", "/auth/:provider/callback", "callback")

	h1, ps1, ok1 := r.Find("GET", "/auth/twitter")
	if !ok1 || h1 != "provider" || ps1.Get("provider") != "twitter" {
		t.Fatalf("provider: ok=%v h=%v p=%q", ok1, h1, ps1.Get("provider"))
	}

	h2, ps2, ok2 := r.Find("GET", "/auth/twitter/callback")
	if !ok2 || h2 != "callback" || ps2.Get("provider") != "twitter" {
		t.Fatalf("callback: ok=%v h=%v p=%q", ok2, h2, ps2.Get("provider"))
	}
}

func TestEdgeSplit_PreservesPattern(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/testing-long-prefix", "long")
	r.AddRoute("GET", "/test", "short")

	res1 := r.FindEx("GET", "/testing-long-prefix", nil)
	if !res1.Found || res1.Pattern != "/testing-long-prefix" {
		t.Fatalf("res1 want Pattern /testing-long-prefix got %q", res1.Pattern)
	}

	res2 := r.FindEx("GET", "/test", nil)
	if !res2.Found || res2.Pattern != "/test" {
		t.Fatalf("res2 want Pattern /test got %q", res2.Pattern)
	}
}
