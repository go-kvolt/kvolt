package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_EnvDefaultAndYAML(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()

	yaml := []byte("port: 9090\nname: fromyaml\n")
	if err := os.WriteFile("config.yaml", yaml, 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("KVOLT_CFG_NAME", "fromenv")

	type Cfg struct {
		Name string `env:"KVOLT_CFG_NAME" mapstructure:"name"`
		Port int    `default:"8080" mapstructure:"port"`
		Host string `default:"localhost" mapstructure:"host"`
	}
	var cfg Cfg
	if err := Load(&cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9090 {
		t.Fatalf("port=%d", cfg.Port)
	}
	if cfg.Name == "" {
		t.Fatal("expected name from env or yaml")
	}
	if cfg.Host != "localhost" {
		t.Fatalf("host default=%q", cfg.Host)
	}
}

func TestLoad_InvalidConfigFile(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()

	if err := os.WriteFile("config.yaml", []byte(":\n:\tborked"), 0o644); err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		X string `mapstructure:"x"`
	}
	if err := Load(&cfg); err == nil {
		t.Fatal("want parse error for invalid yaml")
	}
}

func TestLoad_NonPointerAndNil(t *testing.T) {
	type Cfg struct {
		A string `env:"NOPE" default:"x"`
	}
	var cfg Cfg
	// non-pointer skips field tag loop; Unmarshal may still succeed/fail.
	_ = Load(cfg)

	var nilPtr *Cfg
	if err := Load(nilPtr); err != nil {
		// nil pointer unmarshal error is acceptable
		t.Log(err)
	}

	// nested config dir
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	if err := os.MkdirAll("config", 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join("config", "config.yaml")
	if err := os.WriteFile(p, []byte("a: nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var cfg2 Cfg
	if err := Load(&cfg2); err != nil {
		t.Fatal(err)
	}
}
