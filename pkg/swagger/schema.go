package swagger

import (
	"reflect"
	"strings"
	"time"
)

// SchemaFromSample builds a basic OpenAPI 3 schema object from a Go sample value.
// Supports structs (json tags), maps, slices, and primitive kinds. Exotic types become "object".
func SchemaFromSample(sample interface{}) map[string]interface{} {
	if sample == nil {
		return map[string]interface{}{"type": "object"}
	}
	return schemaFromType(reflect.TypeOf(sample), reflect.ValueOf(sample), map[reflect.Type]bool{})
}

func schemaFromType(t reflect.Type, v reflect.Value, seen map[reflect.Type]bool) map[string]interface{} {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
		if v.IsValid() && !v.IsNil() {
			v = v.Elem()
		} else {
			v = reflect.Value{}
		}
	}

	switch t.Kind() {
	case reflect.Bool:
		return map[string]interface{}{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]interface{}{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]interface{}{"type": "number"}
	case reflect.String:
		return map[string]interface{}{"type": "string"}
	case reflect.Slice, reflect.Array:
		item := map[string]interface{}{"type": "object"}
		if t.Elem() != nil {
			item = schemaFromType(t.Elem(), reflect.Value{}, seen)
		}
		return map[string]interface{}{"type": "array", "items": item}
	case reflect.Map:
		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": schemaFromType(t.Elem(), reflect.Value{}, seen),
		}
	case reflect.Struct:
		if t == reflect.TypeOf(time.Time{}) {
			return map[string]interface{}{"type": "string", "format": "date-time"}
		}
		if seen[t] {
			return map[string]interface{}{"type": "object"}
		}
		seen[t] = true
		defer delete(seen, t)
		props := map[string]interface{}{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" { // unexported
				continue
			}
			name, skip, req := jsonFieldName(f)
			if skip {
				continue
			}
			var fv reflect.Value
			if v.IsValid() && v.Kind() == reflect.Struct {
				fv = v.Field(i)
			}
			props[name] = schemaFromType(f.Type, fv, seen)
			if req {
				required = append(required, name)
			}
		}
		out := map[string]interface{}{"type": "object", "properties": props}
		if len(required) > 0 {
			out["required"] = required
		}
		return out
	default:
		return map[string]interface{}{"type": "object"}
	}
}

func jsonFieldName(f reflect.StructField) (name string, skip bool, required bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", true, false
	}
	name = f.Name
	required = true
	if tag != "" {
		parts := strings.Split(tag, ",")
		if parts[0] != "" {
			name = parts[0]
		}
		for _, p := range parts[1:] {
			if p == "omitempty" {
				required = false
			}
		}
	}
	return name, false, required
}
