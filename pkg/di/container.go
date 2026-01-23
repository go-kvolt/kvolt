package di

import (
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
	c.mu.Lock()
	defer c.mu.Unlock()

	val := reflect.ValueOf(service)
	typ := reflect.TypeOf(service)

	c.services[typ] = val
}

// Invoke looks up a service by type.
// Usage: var s *MyService; container.Invoke(&s)
func (c *Container) Invoke(target interface{}) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return false
	}

	// We need the type of the element the pointer points to
	elemType := val.Type().Elem()

	if service, ok := c.services[elemType]; ok {
		val.Elem().Set(service)
		return true
	}

	return false
}

// TODO: implementing Constructor injection (func NewService(dep Dep)) requires more complex reflection.
// For KVolt v1, we stick to simple Instance Registration for speed.
