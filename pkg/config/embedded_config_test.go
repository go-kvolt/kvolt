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

type OptionsConfig struct {
	Timeout int    `mapstructure:"timeout,omitempty" env:"APP_TIMEOUT" default:"30"`
	Ignored string `mapstructure:"-" env:"IGNORED" default:"secret"`
}

func TestConfig_MapstructureOptionsAndDash(t *testing.T) {
	_ = os.Setenv("APP_TIMEOUT", "45")
	defer func() { _ = os.Unsetenv("APP_TIMEOUT") }()

	var cfg OptionsConfig
	if err := Load(&cfg); err != nil {
		t.Fatalf("unexpected Load error: %v", err)
	}
	if cfg.Timeout != 45 {
		t.Fatalf("expected Timeout=45 from APP_TIMEOUT, got %d", cfg.Timeout)
	}
	if cfg.Ignored != "" {
		t.Fatalf("expected Ignored to remain empty, got %s", cfg.Ignored)
	}
}

type NestedBase struct {
	Cluster string `mapstructure:"cluster" env:"CLUSTER_NAME" default:"prod-cluster"`
}

type ContainerConfig struct {
	*NestedBase
	Replicas int `mapstructure:"replicas" default:"3"`
}

func TestConfig_EmbeddedNilPointerStruct(t *testing.T) {
	_ = os.Setenv("CLUSTER_NAME", "k8s-us-east")
	defer func() { _ = os.Unsetenv("CLUSTER_NAME") }()

	var cfg ContainerConfig
	// cfg.NestedBase is initially nil
	if err := Load(&cfg); err != nil {
		t.Fatalf("unexpected Load error: %v", err)
	}
	if cfg.NestedBase == nil {
		t.Fatal("expected NestedBase to be allocated by viper unmarshaling")
	}
	if cfg.Cluster != "k8s-us-east" {
		t.Fatalf("expected Cluster=k8s-us-east, got %s", cfg.Cluster)
	}
	if cfg.Replicas != 3 {
		t.Fatalf("expected Replicas=3, got %d", cfg.Replicas)
	}
}
