package validator

import (
	"testing"
)

type AuditInfo struct {
	CreatedBy string `validate:"required"`
}

type Article struct {
	AuditInfo
	Title *string `validate:"required,min=5"`
	Email *string `validate:"email"`
	Views *int    `validate:"min=0,max=1000"`
}

func TestValidator_PointersAndEmbedded(t *testing.T) {
	title := "Go 1.24 Release"
	email := "dev@kvolt.dev"
	views := 500

	art := Article{
		AuditInfo: AuditInfo{CreatedBy: "admin"},
		Title:     &title,
		Email:     &email,
		Views:     &views,
	}

	if err := Validate(&art); err != nil {
		t.Fatalf("expected valid struct, got err: %v", err)
	}

	// 1. Missing embedded field
	artNoAudit := art
	artNoAudit.CreatedBy = ""
	if err := Validate(&artNoAudit); err == nil {
		t.Fatal("expected error for empty embedded CreatedBy")
	}

	// 2. Pointer pointing to empty string should fail required
	emptyTitle := ""
	artEmptyTitle := art
	artEmptyTitle.Title = &emptyTitle
	if err := Validate(&artEmptyTitle); err == nil {
		t.Fatal("expected error for pointer to empty title")
	}

	// 3. Pointer min length violation
	shortTitle := "abc"
	artShort := art
	artShort.Title = &shortTitle
	if err := Validate(&artShort); err == nil {
		t.Fatal("expected min error for title < 5")
	}

	// 4. Pointer invalid email
	badEmail := "not-an-email"
	artBadEmail := art
	artBadEmail.Email = &badEmail
	if err := Validate(&artBadEmail); err == nil {
		t.Fatal("expected email validation error")
	}

	// 5. Pointer max violation
	highViews := 5000
	artHighViews := art
	artHighViews.Views = &highViews
	if err := Validate(&artHighViews); err == nil {
		t.Fatal("expected max validation error for views")
	}
}
