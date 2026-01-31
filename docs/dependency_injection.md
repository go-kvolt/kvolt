# Dependency Injection 💉

KVolt includes a lightweight dependency injection container in `pkg/di`. It supports singleton service registration and injection.

## Usage

```go
package main

import (
    "fmt"
    "github.com/go-kvolt/kvolt/pkg/di"
)

type Database interface {
    Connect()
}

type PostgresDB struct{}

func (p *PostgresDB) Connect() {
    fmt.Println("Connected to Postgres")
}

func main() {
    // 1. Create Container
    container := di.NewContainer()

    // 2. Register Service (Singleton)
    container.Provide(&PostgresDB{})

    // 3. Resolve Service
    var db *PostgresDB
    if container.Invoke(&db) {
        db.Connect() // Prints: Connected to Postgres
    }
}
```

## API

-   `NewContainer()`: Creates a new container.
-   `Provide(service interface{})`: Registers a service instance. The type is inferred from the passed value.
-   `Invoke(target interface{}) bool`: Populates the target pointer with the registered service. Returns `true` if found.
