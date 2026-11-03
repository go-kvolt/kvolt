package context

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func BenchmarkBindJSON(b *testing.B) {
	body := `{"party":"acme","amount_paise":1000}`
	var dest struct {
		Party       string `json:"party"`
		AmountPaise int64  `json:"amount_paise"`
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		c := New(w, r)
		if err := c.BindJSON(&dest); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONEncode(b *testing.B) {
	payload := map[string]any{"ok": true, "n": 42}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		c := New(w, r)
		if err := c.JSON(200, payload); err != nil {
			b.Fatal(err)
		}
	}
}
