package config

import (
	"testing"
)

func TestLoad_ValidStruct(t *testing.T) {
	type Cfg struct {
		AppName string `mapstructure:"app_name" default:"kvolt-erp"`
		Port    int    `mapstructure:"port" env:"TEST_PORT" default:"9000"`
	}
	t.Setenv("TEST_PORT", "8888")
	var cfg Cfg
	err := Load(&cfg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AppName != "kvolt-erp" {
		t.Errorf("want default app_name kvolt-erp, got %s", cfg.AppName)
	}
	if cfg.Port != 8888 {
		t.Errorf("want env port 8888, got %d", cfg.Port)
	}
}

func TestLoad_NonStructPointer(t *testing.T) {
	var s string
	// Must not panic on non-struct pointer
	_ = Load(&s)
	var m map[string]string
	_ = Load(&m)
}
