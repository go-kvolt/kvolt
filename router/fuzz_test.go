package router

import "testing"

func FuzzRouterFind(f *testing.F) {
	r := New()
	r.AddRoute("GET", "/", func(c any) error { return nil })
	r.AddRoute("GET", "/ping", func(c any) error { return nil })
	r.AddRoute("GET", "/users/:id", func(c any) error { return nil })
	r.AddRoute("GET", "/a/:p1/:p2/:p3/:p4/:p5", func(c any) error { return nil })
	f.Add("/")
	f.Add("/ping")
	f.Add("/users/42")
	f.Add("/a/1/2/3/4/5")
	f.Add("/nope")
	f.Fuzz(func(t *testing.T, path string) {
		if path == "" || path[0] != '/' {
			return
		}
		_, _, _ = r.Find("GET", path)
	})
}
