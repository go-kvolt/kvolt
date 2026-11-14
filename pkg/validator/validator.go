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

		tag := field.Tag.Get("validate")

		// Recursively validate nested struct fields and slices of structs (unless unexported or tagged with "-")
		if field.PkgPath == "" && tag != "-" {
			nestedVal := v.Field(i)
			for nestedVal.Kind() == reflect.Ptr || nestedVal.Kind() == reflect.Interface {
				if nestedVal.IsNil() {
					break
				}
				nestedVal = nestedVal.Elem()
			}
			if nestedVal.IsValid() {
				if nestedVal.Kind() == reflect.Struct && nestedVal.CanInterface() && nestedVal.Type().PkgPath() != "time" {
					if err := Validate(nestedVal.Interface()); err != nil {
						return err
					}
				} else if nestedVal.Kind() == reflect.Slice || nestedVal.Kind() == reflect.Array {
					for j := 0; j < nestedVal.Len(); j++ {
						elem := nestedVal.Index(j)
						for elem.Kind() == reflect.Ptr || elem.Kind() == reflect.Interface {
							if elem.IsNil() {
								break
							}
							elem = elem.Elem()
						}
						if elem.IsValid() && elem.Kind() == reflect.Struct && elem.CanInterface() && elem.Type().PkgPath() != "time" {
							if err := Validate(elem.Interface()); err != nil {
								return err
							}
						}
					}
				} else if nestedVal.Kind() == reflect.Map {
					iter := nestedVal.MapRange()
					for iter.Next() {
						elem := iter.Value()
						for elem.Kind() == reflect.Ptr || elem.Kind() == reflect.Interface {
							if elem.IsNil() {
								break
							}
							elem = elem.Elem()
						}
						if elem.IsValid() && elem.Kind() == reflect.Struct && elem.CanInterface() && elem.Type().PkgPath() != "time" {
							if err := Validate(elem.Interface()); err != nil {
								return err
							}
						}
					}
				}
			}
		}

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
				limitStr := strings.TrimPrefix(rule, "min=")
				limit, err := strconv.ParseFloat(limitStr, 64)
				if err == nil {
					if checkVal.Kind() == reflect.String {
						if float64(len(checkVal.String())) < limit {
							return fmt.Errorf("field %s must be at least %s characters", field.Name, limitStr)
						}
					} else if checkVal.Kind() == reflect.Slice || checkVal.Kind() == reflect.Array || checkVal.Kind() == reflect.Map {
						if float64(checkVal.Len()) < limit {
							return fmt.Errorf("field %s must contain at least %s items", field.Name, limitStr)
						}
					} else if checkVal.Kind() >= reflect.Int && checkVal.Kind() <= reflect.Int64 {
						if float64(checkVal.Int()) < limit {
							return fmt.Errorf("field %s must be at least %s", field.Name, limitStr)
						}
					} else if checkVal.Kind() >= reflect.Uint && checkVal.Kind() <= reflect.Uint64 {
						if float64(checkVal.Uint()) < limit {
							return fmt.Errorf("field %s must be at least %s", field.Name, limitStr)
						}
					} else if checkVal.Kind() == reflect.Float32 || checkVal.Kind() == reflect.Float64 {
						if checkVal.Float() < limit {
							return fmt.Errorf("field %s must be at least %s", field.Name, limitStr)
						}
					}
				}
			}

			if strings.HasPrefix(rule, "max=") {
				limitStr := strings.TrimPrefix(rule, "max=")
				limit, err := strconv.ParseFloat(limitStr, 64)
				if err == nil {
					if checkVal.Kind() == reflect.String {
						if float64(len(checkVal.String())) > limit {
							return fmt.Errorf("field %s must be at most %s characters", field.Name, limitStr)
						}
					} else if checkVal.Kind() == reflect.Slice || checkVal.Kind() == reflect.Array || checkVal.Kind() == reflect.Map {
						if float64(checkVal.Len()) > limit {
							return fmt.Errorf("field %s must contain at most %s items", field.Name, limitStr)
						}
					} else if checkVal.Kind() >= reflect.Int && checkVal.Kind() <= reflect.Int64 {
						if float64(checkVal.Int()) > limit {
							return fmt.Errorf("field %s must be at most %s", field.Name, limitStr)
						}
					} else if checkVal.Kind() >= reflect.Uint && checkVal.Kind() <= reflect.Uint64 {
						if float64(checkVal.Uint()) > limit {
							return fmt.Errorf("field %s must be at most %s", field.Name, limitStr)
						}
					} else if checkVal.Kind() == reflect.Float32 || checkVal.Kind() == reflect.Float64 {
						if checkVal.Float() > limit {
							return fmt.Errorf("field %s must be at most %s", field.Name, limitStr)
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
