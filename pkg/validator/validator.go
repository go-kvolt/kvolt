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
	emailRegex = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
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
			// Format: min=5
			if strings.HasPrefix(rule, "min=") {
				limit, _ := strconv.Atoi(strings.TrimPrefix(rule, "min="))
				if val.Kind() == reflect.String {
					if len(val.String()) < limit {
						return fmt.Errorf("field %s must be at least %d characters", field.Name, limit)
					}
				}
			}
		}
	}
	return nil
}

func isEmpty(v reflect.Value) bool {
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
