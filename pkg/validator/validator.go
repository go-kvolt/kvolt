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
	if s == nil {
		return nil
	}
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil // strict: error?
	}

	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Recursively validate embedded anonymous struct fields
		if field.Anonymous {
			anonVal := v.Field(i)
			for anonVal.Kind() == reflect.Ptr || anonVal.Kind() == reflect.Interface {
				if anonVal.IsNil() {
					break
				}
				anonVal = anonVal.Elem()
			}
			if anonVal.IsValid() && anonVal.Kind() == reflect.Struct && anonVal.CanInterface() {
				if err := Validate(anonVal.Interface()); err != nil {
					return err
				}
			}
		}

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

			// Dereference pointer for value-based rules (email, min, max)
			checkVal := val
			for checkVal.Kind() == reflect.Ptr || checkVal.Kind() == reflect.Interface {
				if checkVal.IsNil() {
					break
				}
				checkVal = checkVal.Elem()
			}
			if !checkVal.IsValid() || (checkVal.Kind() == reflect.Ptr && checkVal.IsNil()) {
				continue
			}

			// Check Email
			if rule == "email" && checkVal.Kind() == reflect.String {
				if !emailRegex.MatchString(checkVal.String()) {
					return fmt.Errorf("field %s must be a valid email", field.Name)
				}
			}

			// Check Min/Max (simplified)
			// Format: min=5, max=10
			if strings.HasPrefix(rule, "min=") {
				limit, err := strconv.Atoi(strings.TrimPrefix(rule, "min="))
				if err == nil {
					if checkVal.Kind() == reflect.String {
						if len(checkVal.String()) < limit {
							return fmt.Errorf("field %s must be at least %d characters", field.Name, limit)
						}
					} else if checkVal.Kind() == reflect.Slice || checkVal.Kind() == reflect.Array {
						if checkVal.Len() < limit {
							return fmt.Errorf("field %s must contain at least %d items", field.Name, limit)
						}
					} else if checkVal.Kind() >= reflect.Int && checkVal.Kind() <= reflect.Int64 {
						if checkVal.Int() < int64(limit) {
							return fmt.Errorf("field %s must be at least %d", field.Name, limit)
						}
					} else if checkVal.Kind() >= reflect.Uint && checkVal.Kind() <= reflect.Uint64 {
						if checkVal.Uint() < uint64(limit) {
							return fmt.Errorf("field %s must be at least %d", field.Name, limit)
						}
					}
				}
			}

			if strings.HasPrefix(rule, "max=") {
				limit, err := strconv.Atoi(strings.TrimPrefix(rule, "max="))
				if err == nil {
					if checkVal.Kind() == reflect.String {
						if len(checkVal.String()) > limit {
							return fmt.Errorf("field %s must be at most %d characters", field.Name, limit)
						}
					} else if checkVal.Kind() == reflect.Slice || checkVal.Kind() == reflect.Array {
						if checkVal.Len() > limit {
							return fmt.Errorf("field %s must contain at most %d items", field.Name, limit)
						}
					} else if checkVal.Kind() >= reflect.Int && checkVal.Kind() <= reflect.Int64 {
						if checkVal.Int() > int64(limit) {
							return fmt.Errorf("field %s must be at most %d", field.Name, limit)
						}
					} else if checkVal.Kind() >= reflect.Uint && checkVal.Kind() <= reflect.Uint64 {
						if checkVal.Uint() > uint64(limit) {
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
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	default:
		return false // zero value check for primitives?
	}
}
