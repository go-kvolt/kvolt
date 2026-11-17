package kvolt

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/middleware"
)

type bindUser struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"omitempty,email"`
	Age   int    `json:"age"`
}

// A handler that returns the error from Bind must answer 4xx for bad input, not 500,
// and bad input must not be logged as a server fault.
func TestBind_BadInputIsClientError(t *testing.T) {
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(prev)

	app := New()
	app.POST("/users", func(c *context.Context) error {
		var u bindUser
		if err := c.Bind(&u); err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, u)
	})

	cases := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"malformed json", `{bad json`, http.StatusBadRequest, "invalid_request"},
		{"empty body", ``, http.StatusBadRequest, "invalid_request"},
		{"truncated json", `{"name":"a"`, http.StatusBadRequest, "invalid_request"},
		{"wrong type", `{"name":"a","age":"x"}`, http.StatusBadRequest, "invalid_request"},
		{"missing required", `{"name":""}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"invalid email", `{"name":"a","email":"nope"}`, http.StatusUnprocessableEntity, "validation_failed"},
		{"valid", `{"name":"a","email":"a@b.co"}`, http.StatusCreated, ""},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(tc.body)))
		if rec.Code != tc.status {
			t.Errorf("%s: status = %d, want %d (%s)", tc.name, rec.Code, tc.status, rec.Body.String())
			continue
		}
		if tc.code == "" {
			continue
		}
		var resp struct {
			Error   string              `json:"error"`
			Message string              `json:"message"`
			Details []map[string]string `json:"details"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Errorf("%s: response is not JSON: %v", tc.name, err)
			continue
		}
		if resp.Error != tc.code || resp.Message == "" {
			t.Errorf("%s: body = %s, want error %q", tc.name, rec.Body.String(), tc.code)
		}
		// Go type names and parser internals stay out of the response.
		for _, leak := range []string{"Go struct", "invalid character", "unmarshal"} {
			if strings.Contains(rec.Body.String(), leak) {
				t.Errorf("%s: response leaks %q: %s", tc.name, leak, rec.Body.String())
			}
		}
		switch tc.name {
		case "wrong type":
			if !strings.Contains(resp.Message, `"age"`) {
				t.Errorf("message should name the field: %q", resp.Message)
			}
		case "missing required":
			if len(resp.Details) != 1 || resp.Details[0]["field"] != "Name" || resp.Details[0]["rule"] != "required" {
				t.Errorf("details = %v, want Name/required", resp.Details)
			}
		case "invalid email":
			if len(resp.Details) != 1 || resp.Details[0]["field"] != "Email" || resp.Details[0]["rule"] != "email" {
				t.Errorf("details = %v, want Email/email", resp.Details)
			}
		}
	}
	if logs.Len() != 0 {
		t.Errorf("bad input was logged as a handler error:\n%s", logs.String())
	}
}

// Query, form and path binding failures are client errors too.
func TestBindQueryFormURI_BadInputIsClientError(t *testing.T) {
	app := New()
	app.GET("/search", func(c *context.Context) error {
		var q struct {
			Page int `query:"page"`
		}
		if err := c.BindQuery(&q); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, q)
	})
	app.GET("/items/:id", func(c *context.Context) error {
		var p struct {
			ID int `uri:"id"`
		}
		if err := c.BindURI(&p); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, p)
	})
	app.POST("/form", func(c *context.Context) error {
		var f struct {
			Count int `form:"count"`
		}
		if err := c.BindForm(&f); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, f)
	})

	get := func(target string) int {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec.Code
	}
	if got := get("/search?page=abc"); got != http.StatusBadRequest {
		t.Errorf("bad query: status = %d, want 400", got)
	}
	if got := get("/search?page=2"); got != http.StatusOK {
		t.Errorf("good query: status = %d, want 200", got)
	}
	if got := get("/items/abc"); got != http.StatusBadRequest {
		t.Errorf("bad path param: status = %d, want 400", got)
	}
	if got := get("/items/7"); got != http.StatusOK {
		t.Errorf("good path param: status = %d, want 200", got)
	}

	post := func(body string) int {
		r := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, r)
		return rec.Code
	}
	if got := post("count=many"); got != http.StatusBadRequest {
		t.Errorf("bad form value: status = %d, want 400", got)
	}
	if got := post("count=%zz"); got != http.StatusBadRequest {
		t.Errorf("malformed form encoding: status = %d, want 400", got)
	}
	if got := post("count=3"); got != http.StatusOK {
		t.Errorf("good form: status = %d, want 200", got)
	}
}

// An oversized body is still 413, not 400.
func TestBind_OversizedBodyIs413(t *testing.T) {
	app := New()
	app.Use(middleware.MaxBodySize(16))
	app.POST("/users", func(c *context.Context) error {
		var u bindUser
		return c.Bind(&u)
	})
	rec := httptest.NewRecorder()
	body := `{"name":"` + strings.Repeat("a", 200) + `"}`
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body)))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 (%s)", rec.Code, rec.Body.String())
	}
}

// The original error stays reachable and its text is unchanged, so existing handler
// code that inspects bind errors keeps working. Misuse stays a plain (500) error.
func TestBindError_KeepsOriginalError(t *testing.T) {
	c := context.New(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"age":"x"}`)))
	var u bindUser
	err := c.BindJSON(&u)

	var be *context.BindError
	if !errors.As(err, &be) || be.Validation {
		t.Fatalf("want a non-validation *BindError, got %T %v", err, err)
	}
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) || typeErr.Field != "age" {
		t.Fatalf("original *json.UnmarshalTypeError not reachable: %v", err)
	}
	if err.Error() != typeErr.Error() {
		t.Fatalf("error text changed: %q vs %q", err.Error(), typeErr.Error())
	}

	// Binding into something that is not a struct pointer is a programming error.
	q := context.New(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/?a=1", nil))
	var notStruct int
	if err := q.BindQuery(&notStruct); err == nil || errors.As(err, &be) {
		t.Fatalf("misuse must be a plain error, got %T %v", err, err)
	}

	var nilErr *context.BindError
	if nilErr.Error() == "" || nilErr.Unwrap() != nil {
		t.Fatalf("nil *BindError must be safe to use")
	}
}
