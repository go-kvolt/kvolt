package router

import "testing"

func BenchmarkRouter_Static(b *testing.B) {
	r := New()
	r.AddRoute("GET", "/ping", struct{}{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = r.Find("GET", "/ping")
	}
}

func BenchmarkRouter_Param(b *testing.B) {
	r := New()
	r.AddRoute("GET", "/users/:id", struct{}{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = r.Find("GET", "/users/42")
	}
}

func BenchmarkRouter_FiveParams(b *testing.B) {
	r := New()
	r.AddRoute("GET", "/a/:p1/:p2/:p3/:p4/:p5", struct{}{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = r.Find("GET", "/a/1/2/3/4/5")
	}
}

func BenchmarkRouter_CatchAll(b *testing.B) {
	r := New()
	r.AddRoute("GET", "/files/*filepath", struct{}{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = r.Find("GET", "/files/a/b/c.txt")
	}
}
