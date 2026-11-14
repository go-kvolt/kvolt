package context

import (
	"net/http/httptest"
	"testing"
)

type Pagination struct {
	Page  int `query:"page"`
	Limit int `query:"limit"`
}

type UserQuery struct {
	Pagination
	Search string   `query:"search"`
	Active *bool    `query:"active"`
	Age    *int     `query:"age"`
	Score  *float64 `query:"score"`
}

func TestBindQuery_EmbeddedAndPointers(t *testing.T) {
	req := httptest.NewRequest("GET", "/users?page=5&limit=25&search=golang&active=true&age=30&score=98.5", nil)
	c := New(httptest.NewRecorder(), req)

	var q UserQuery
	if err := c.BindQuery(&q); err != nil {
		t.Fatalf("unexpected BindQuery error: %v", err)
	}

	if q.Page != 5 || q.Limit != 25 {
		t.Fatalf("expected embedded Pagination Page=5, Limit=25; got Page=%d, Limit=%d", q.Page, q.Limit)
	}
	if q.Search != "golang" {
		t.Fatalf("expected search=golang, got %s", q.Search)
	}
	if q.Active == nil || !*q.Active {
		t.Fatalf("expected active=true, got %+v", q.Active)
	}
	if q.Age == nil || *q.Age != 30 {
		t.Fatalf("expected age=30, got %+v", q.Age)
	}
	if q.Score == nil || *q.Score != 98.5 {
		t.Fatalf("expected score=98.5, got %+v", q.Score)
	}
}
