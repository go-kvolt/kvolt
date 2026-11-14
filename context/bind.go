package context

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// BindQuery binds URL query parameters into obj using `form` or `query` tags (fallback: field name).
func (c *Context) BindQuery(obj interface{}) error {
	if c.Request == nil || c.Request.URL == nil {
		return fmt.Errorf("nil request")
	}
	return bindMap(obj, c.Request.URL.Query(), "query", "form")
}

// BindForm binds application/x-www-form-urlencoded or multipart form fields.
func (c *Context) BindForm(obj interface{}) error {
	if c.Request == nil {
		return fmt.Errorf("nil request")
	}
	ct := c.Request.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			return err
		}
	} else if err := c.Request.ParseForm(); err != nil {
		return err
	}
	return bindMap(obj, c.Request.Form, "form")
}

// BindURI binds path params into obj using `uri` or `param` tags.
func (c *Context) BindURI(obj interface{}) error {
	values := make(url.Values)
	for _, p := range c.Params {
		values.Set(p.Key, p.Value)
	}
	return bindMap(obj, values, "uri", "param")
}

func bindMap(obj interface{}, values url.Values, tags ...string) error {
	return bindFromGetter(obj, func(key string) string {
		return values.Get(key)
	}, tags...)
}

func bindFromGetter(obj interface{}, get func(string) string, tags ...string) error {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("bind target must be a non-nil pointer")
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("bind target must be a struct pointer")
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name := fieldBindName(f, tags...)
		if name == "" || name == "-" {
			continue
		}
		raw := get(name)
		if raw == "" {
			continue
		}
		if err := setField(v.Field(i), raw); err != nil {
			return err
		}
	}
	return nil
}

func fieldBindName(f reflect.StructField, tags ...string) string {
	for _, tag := range tags {
		if v := f.Tag.Get(tag); v != "" {
			return strings.Split(v, ",")[0]
		}
	}
	return f.Name
}

func setField(fv reflect.Value, raw string) error {
	if !fv.CanSet() {
		return nil
	}
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, fv.Type().Bits())
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, fv.Type().Bits())
		if err != nil {
			return err
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		bitSize := 64
		if fv.Kind() == reflect.Float32 {
			bitSize = 32
		}
		n, err := strconv.ParseFloat(raw, bitSize)
		if err != nil {
			return err
		}
		fv.SetFloat(n)
	case reflect.Struct, reflect.Slice, reflect.Map:
		trimmed := strings.TrimSpace(raw)
		if fv.Kind() == reflect.Slice && !strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "{") {
			parts := strings.Split(raw, ",")
			elemKind := fv.Type().Elem().Kind()
			if elemKind == reflect.String {
				fv.Set(reflect.ValueOf(parts))
				return nil
			} else if elemKind >= reflect.Int && elemKind <= reflect.Int64 {
				slice := reflect.MakeSlice(fv.Type(), len(parts), len(parts))
				for idx, p := range parts {
					n, parseErr := strconv.ParseInt(strings.TrimSpace(p), 10, fv.Type().Elem().Bits())
					if parseErr != nil {
						return fmt.Errorf("bind %s: %w", fv.Type().String(), parseErr)
					}
					slice.Index(idx).SetInt(n)
				}
				fv.Set(slice)
				return nil
			}
		}
		ptr := reflect.New(fv.Type())
		if err := json.Unmarshal([]byte(raw), ptr.Interface()); err != nil {
			return fmt.Errorf("bind %s: %w", fv.Type().String(), err)
		}
		fv.Set(ptr.Elem())
	}
	return nil
}

// FormValue is a short-hand for Request.FormValue.
func (c *Context) FormValue(key string) string {
	if c.Request == nil {
		return ""
	}
	return c.Request.FormValue(key)
}

// DefaultQuery returns query key or def when empty.
func (c *Context) DefaultQuery(key, def string) string {
	if v := c.Query(key); v != "" {
		return v
	}
	return def
}

// GetHeader returns a request header.
func (c *Context) GetHeader(key string) string {
	if c.Request == nil {
		return ""
	}
	return c.Request.Header.Get(key)
}

// GetCookie returns a cookie value or empty string.
func (c *Context) GetCookie(name string) string {
	if c.Request == nil {
		return ""
	}
	ck, err := c.Request.Cookie(name)
	if err != nil {
		return ""
	}
	return ck.Value
}
