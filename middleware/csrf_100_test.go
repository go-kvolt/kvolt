package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestCSRF_DefaultWrapper(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		CSRF(),
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c.Next()
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w.Header().Get("Set-Cookie") == "" {
		t.Fatal("want csrf cookie")
	}
}

func TestNewCSRFToken_RandPanic(t *testing.T) {
	old := csrfRandRead
	csrfRandRead = func(b []byte) (int, error) { return 0, errors.New("no entropy") }
	defer func() { csrfRandRead = old }()
	defer func() {
		if recover() == nil {
			t.Fatal("want newCSRFToken panic")
		}
	}()
	_ = newCSRFToken([]byte("secret"))
}

func TestCSRFSecretBytes_RandPanic(t *testing.T) {
	old := csrfRandRead
	csrfRandRead = func(b []byte) (int, error) { return 0, errors.New("no entropy") }
	defer func() { csrfRandRead = old }()

	csrfProcessSecretOnce = sync.Once{}
	csrfProcessSecret = nil
	defer func() {
		if recover() == nil {
			t.Fatal("want csrfSecretBytes panic")
		}
	}()
	_ = csrfSecretBytes(CSRFConfig{}) // empty secret → process rand
}
