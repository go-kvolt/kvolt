# Configuration ⚙️

KVolt provides a robust configuration management package `pkg/config` built on top of `viper` and `godotenv`. It supports loading configuration from environment variables, `.env` files, and configuration files (JSON, YAML, TOML, etc.).

## Usage

Define your configuration struct using `mapstructure` tags (for config files) and `env` tags (for environment variables).

```go
package main

import (
    "fmt"
    "log"
    "github.com/go-kvolt/kvolt/pkg/config"
)

type Config struct {
    AppName  string `mapstructure:"app_name" env:"APP_NAME" default:"KVolt App"`
    Port     int    `mapstructure:"port"     env:"PORT"     default:"8080"`
    Database struct {
        Host string `mapstructure:"host" env:"DB_HOST"`
        Port int    `mapstructure:"port" env:"DB_PORT"`
    } `mapstructure:"database"`
}

func main() {
    var cfg Config

    // Load configuration
    // 1. Checks .env file
    // 2. Checks Environment Variables
    // 3. Checks config.yaml/json/toml in local or ./config folder
    if err := config.Load(&cfg); err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    fmt.Printf("Starting %s on port %d...\n", cfg.AppName, cfg.Port)
}
```

## Real-World Example: Production Config

Here is how you would typically set up configuration for a production app, utilizing environment variable overrides.

`config.yaml`:
```yaml
app_name: "KVolt App"
port: 8080
database:
  host: "localhost"
  port: 5432
  user: "postgres"
  name: "mydb"
```

`main.go`:
```go
package main

import (
    "fmt"
    "log"
    "github.com/go-kvolt/kvolt/pkg/config"
)

type DatabaseConfig struct {
    Host     string `mapstructure:"host" env:"DB_HOST"`
    Port     int    `mapstructure:"port" env:"DB_PORT"`
    User     string `mapstructure:"user" env:"DB_USER"`
    Password string `mapstructure:"password" env:"DB_PASSWORD"`
    Name     string `mapstructure:"name" env:"DB_NAME"`
}

type Config struct {
    AppName  string         `mapstructure:"app_name" env:"APP_NAME"`
    Port     int            `mapstructure:"port"     env:"PORT"`
    Database DatabaseConfig `mapstructure:"database"`
}

func main() {
    var cfg Config

    // Load config (merged from config.yaml and ENV variables)
    if err := config.Load(&cfg); err != nil {
        log.Fatalf("❌ Failed to load config: %v", err)
    }

    // Use Config
    dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", 
        cfg.Database.User, 
        cfg.Database.Password, 
        cfg.Database.Host, 
        cfg.Database.Port, 
        cfg.Database.Name,
    )

    fmt.Printf("✅ Loaded %s\n", cfg.AppName)
    fmt.Printf("🔌 Connecting to DB: %s\n", dsn)
    // db.Connect(dsn)...
}
```

This allows you to run locally with `config.yaml`, and deploy to Docker/Kubernetes supplying `DB_HOST`, `DB_PASSWORD`, etc., as environment variables which will automatically override the file values.
