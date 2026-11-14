package di

import (
	"fmt"
	"reflect"
	"sync"
)

// Container is a simple dependency injection container.
type Container struct {
	services map[reflect.Type]reflect.Value
	mu       sync.RWMutex
}

// NewContainer creates a new DI container.
func NewContainer() *Container {
	return &Container{
		services: make(map[reflect.Type]reflect.Value),
	}
}

// Provide registers a service instance (Singleton).
// Usage: container.Provide(&MyService{})
func (c *Container) Provide(service interface{}) {
	if service == nil {
		panic("di: cannot provide nil service")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	val := reflect.ValueOf(service)
	typ := reflect.TypeOf(service)

	c.services[typ] = val
}

// Invoke looks up a service by type or implemented interface.
// Usage: var s *MyService; container.Invoke(&s)
func (c *Container) Invoke(target interface{}) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return false
	}

	elemType := val.Type().Elem()

	if service, ok := c.services[elemType]; ok {
		val.Elem().Set(service)
		return true
	}

	if elemType.Kind() == reflect.Interface {
		for sType, service := range c.services {
			if sType.Implements(elemType) {
				val.Elem().Set(service)
				return true
			}
		}
	}

	return false
}

// ProvideFunc registers a service by invoking a constructor.
// The constructor must be a function that returns T or (T, error).
// Each parameter type is resolved from the container (exact type or implemented interface).
//
//	c.Provide(&Dep{})
//	c.ProvideFunc(func(d *Dep) *Service { return &Service{Dep: d} })
func (c *Container) ProvideFunc(constructor interface{}) error {
	fn := reflect.ValueOf(constructor)
	if fn.Kind() != reflect.Func {
		return fmt.Errorf("di: ProvideFunc requires a function")
	}
	ft := fn.Type()
	numOut := ft.NumOut()
	if numOut < 1 || numOut > 2 {
		return fmt.Errorf("di: constructor must return T or (T, error)")
	}
	if numOut == 2 && !ft.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return fmt.Errorf("di: second return value must be error")
	}

	args := make([]reflect.Value, ft.NumIn())
	c.mu.RLock()
	for i := 0; i < ft.NumIn(); i++ {
		inType := ft.In(i)
		svc, ok := c.services[inType]
		if !ok && inType.Kind() == reflect.Interface {
			for sType, sVal := range c.services {
				if sType.Implements(inType) {
					svc = sVal
					ok = true
					break
				}
			}
		}
		if !ok {
			c.mu.RUnlock()
			return fmt.Errorf("di: missing dependency %s", inType.String())
		}
		args[i] = svc
	}
	c.mu.RUnlock()

	results := fn.Call(args)
	if numOut == 2 {
		if errVal := results[1]; !errVal.IsNil() {
			return errVal.Interface().(error)
		}
	}

	instance := results[0]
	if (instance.Kind() == reflect.Ptr || instance.Kind() == reflect.Interface) && instance.IsNil() {
		return fmt.Errorf("di: constructor returned nil instance")
	}
	c.mu.Lock()
	c.services[instance.Type()] = instance
	c.mu.Unlock()
	return nil
}

// MustProvideFunc is ProvideFunc that panics on error.
func (c *Container) MustProvideFunc(constructor interface{}) {
	if err := c.ProvideFunc(constructor); err != nil {
		panic(err)
	}
}
