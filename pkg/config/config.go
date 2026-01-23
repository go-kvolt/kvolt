package config

import (
	"os"
	"reflect"
	"strconv"
)

// Load reads environment variables into a struct.
// Supported tag: env:"MY_ENV_VAR"
func Load(target interface{}) error {
	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return nil // or error
	}

	val = val.Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("env")
		if tag == "" {
			continue
		}

		envVal := os.Getenv(tag)
		if envVal == "" {
			// Check for default
			def := field.Tag.Get("default")
			if def != "" {
				envVal = def
			} else {
				continue
			}
		}

		// Set value based on type
		fieldVal := val.Field(i)
		switch fieldVal.Kind() {
		case reflect.String:
			fieldVal.SetString(envVal)
		case reflect.Int:
			if v, err := strconv.Atoi(envVal); err == nil {
				fieldVal.SetInt(int64(v))
			}
		case reflect.Bool:
			if v, err := strconv.ParseBool(envVal); err == nil {
				fieldVal.SetBool(v)
			}
		}
	}
	return nil
}
