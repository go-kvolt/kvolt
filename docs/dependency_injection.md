# Dependency Injection

KVolt includes a lightweight DI container in `pkg/di` for singleton registration, lookup, and constructor injection.

## Instance registration

```go
package main

import (
    "fmt"

    "github.com/go-kvolt/kvolt/v2/pkg/di"
)

type PostgresDB struct{}

func (p *PostgresDB) Connect() {
    fmt.Println("Connected to Postgres")
}

func main() {
    container := di.NewContainer()
    container.Provide(&PostgresDB{})

    var db *PostgresDB
    if container.Invoke(&db) {
        db.Connect()
    }
}
```

## Constructor injection

`ProvideFunc` calls a constructor and resolves each parameter type from the container (exact type match, usually pointers).

```go
type Dep struct{}
type Service struct{ Dep *Dep }

container.Provide(&Dep{})
err := container.ProvideFunc(func(d *Dep) *Service {
    return &Service{Dep: d}
})
// or: container.MustProvideFunc(...)

var svc *Service
container.Invoke(&svc)
```

Constructors may return `(T, error)`. Missing dependencies return an error from `ProvideFunc`.

## API

| Method | Description |
| :--- | :--- |
| `NewContainer()` | Create a container |
| `Provide(service)` | Register a singleton instance |
| `ProvideFunc(ctor)` | Build and register via constructor |
| `MustProvideFunc(ctor)` | Like `ProvideFunc`, panics on error |
| `Invoke(target)` | Fill a pointer; returns false if missing |
