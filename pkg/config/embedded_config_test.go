package config

import (
	"os"
	"testing"
)

type ServerMeta struct {
	Host string `mapstructure:"host" env:"SERVER_HOST" default:"0.0.0.0"`
	Port int    `mapstructure:"port" env:"SERVER_PORT" default:"9000"`
}

type AppConfig struct {
	ServerMeta
	AppName string `mapstructure:"app_name" env:"APP_NAME" default:"KVoltApp"`
}

func TestConfig_EmbeddedStructEnvAndDefaults(t *testing.T) {
	_ = os.Setenv("SERVER_HOST", "127.0.0.1")
	defer func() { _ = os.Unsetenv("SERVER_HOST") }()

	var cfg AppConfig
	if err := Load(&cfg); err != nil {
		t.Fatalf("unexpected Load error: %v", err)
	}

	if cfg.Host != "127.0.0.1" {
		t.Fatalf("expected Host to be read from SERVER_HOST env var, got %s", cfg.Host)
	}
	if cfg.Port != 9000 {
		t.Fatalf("expected Port default=9000, got %d", cfg.Port)
	}
	if cfg.AppName != "KVoltApp" {
		t.Fatalf("expected AppName default=KVoltApp, got %s", cfg.AppName)
	}
}
