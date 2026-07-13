package config

import "testing"

func TestDevAPIEnabledDefault(t *testing.T) {
	t.Setenv("DEV_API_ENABLED", "")
	cfg := Load()
	if cfg.DevAPIEnabled {
		t.Fatal("want DevAPIEnabled false by default")
	}
}

func TestDevAPIEnabledTrue(t *testing.T) {
	t.Setenv("DEV_API_ENABLED", "true")
	cfg := Load()
	if !cfg.DevAPIEnabled {
		t.Fatal("want DevAPIEnabled true for DEV_API_ENABLED=true")
	}
}

func TestDevAPIEnabledOne(t *testing.T) {
	t.Setenv("DEV_API_ENABLED", "1")
	cfg := Load()
	if !cfg.DevAPIEnabled {
		t.Fatal("want DevAPIEnabled true for DEV_API_ENABLED=1")
	}
}
