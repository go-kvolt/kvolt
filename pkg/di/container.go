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
	if c == nil {
		return
	}
	if service == nil {
		panic("di: cannot provide nil service")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.services == nil {
		c.services = make(map[reflect.Type]reflect.Value)
	}

	val := reflect.ValueOf(service)
	typ := reflect.TypeOf(service)

	c.services[typ] = val
}

// Invoke looks up a service by type or implemented interface.
// Usage: var s *MyService; container.Invoke(&s)
func (c *Container) Invoke(target interface{}) bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Ptr || val.IsNil() || !val.Elem().CanSet() {
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
	if c == nil {
		return fmt.Errorf("di: container is nil")
	}
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

	var results []reflect.Value
	var callErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("di: constructor panicked: %v", r)
			}
		}()
		results = fn.Call(args)
	}()
	if callErr != nil {
		return callErr
	}
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
	if c.services == nil {
		c.services = make(map[reflect.Type]reflect.Value)
	}
	c.services[ft.Out(0)] = instance
	if ft.Out(0) != instance.Type() {
		c.services[instance.Type()] = instance
	}
	c.mu.Unlock()
	return nil
}

// MustProvideFunc is ProvideFunc that panics on error.
func (c *Container) MustProvideFunc(constructor interface{}) {
	if c == nil {
		return
	}
	if err := c.ProvideFunc(constructor); err != nil {
		panic(err)
	}
}
