package validator

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// Validator validates structs based on tags.
type Validator struct {
	// Cache for compiled structs could go here
}

var (
	emailRegex = regexp.MustCompile(`(?i)^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,63}$`)
)

// Validate validates a struct.
func Validate(s interface{}) error {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil // strict: error?
	}

	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("validate")
		if tag == "" {
			continue
		}

		rules := strings.Split(tag, ",")
		for _, rule := range rules {
			val := v.Field(i)

			// Check Required
			if rule == "required" {
				if isEmpty(val) {
					return fmt.Errorf("field %s is required", field.Name)
				}
			}

			// Check Email
			if rule == "email" && val.Kind() == reflect.String {
				if !emailRegex.MatchString(val.String()) {
					return fmt.Errorf("field %s must be a valid email", field.Name)
				}
			}

			// Check Min/Max (simplified)
			// Format: min=5, max=10
			if strings.HasPrefix(rule, "min=") {
				limit, err := strconv.Atoi(strings.TrimPrefix(rule, "min="))
				if err == nil {
					if val.Kind() == reflect.String {
						if len(val.String()) < limit {
							return fmt.Errorf("field %s must be at least %d characters", field.Name, limit)
						}
					} else if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
						if val.Len() < limit {
							return fmt.Errorf("field %s must contain at least %d items", field.Name, limit)
						}
					} else if val.Kind() >= reflect.Int && val.Kind() <= reflect.Int64 {
						if val.Int() < int64(limit) {
							return fmt.Errorf("field %s must be at least %d", field.Name, limit)
						}
					} else if val.Kind() >= reflect.Uint && val.Kind() <= reflect.Uint64 {
						if val.Uint() < uint64(limit) {
							return fmt.Errorf("field %s must be at least %d", field.Name, limit)
						}
					}
				}
			}

			if strings.HasPrefix(rule, "max=") {
				limit, err := strconv.Atoi(strings.TrimPrefix(rule, "max="))
				if err == nil {
					if val.Kind() == reflect.String {
						if len(val.String()) > limit {
							return fmt.Errorf("field %s must be at most %d characters", field.Name, limit)
						}
					} else if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
						if val.Len() > limit {
							return fmt.Errorf("field %s must contain at most %d items", field.Name, limit)
						}
					} else if val.Kind() >= reflect.Int && val.Kind() <= reflect.Int64 {
						if val.Int() > int64(limit) {
							return fmt.Errorf("field %s must be at most %d", field.Name, limit)
						}
					} else if val.Kind() >= reflect.Uint && val.Kind() <= reflect.Uint64 {
						if val.Uint() > uint64(limit) {
							return fmt.Errorf("field %s must be at most %d", field.Name, limit)
						}
					}
				}
			}
		}
	}
	return nil
}

func isEmpty(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	default:
		return false // zero value check for primitives?
	}
}
