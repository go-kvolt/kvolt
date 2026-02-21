package config

import (
	"testing"
)

func TestLoad_ValidStruct(t *testing.T) {
	type Cfg struct {
		AppName string `mapstructure:"app_name"`
		Port    int    `mapstructure:"port"`
	}
	var cfg Cfg
	err := Load(&cfg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// No config file in test; defaults/zero values are fine
	_ = cfg.AppName
	_ = cfg.Port
}

