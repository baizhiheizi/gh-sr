package cache

import (
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
)

func TestSettingsFromConfig_nilConfig(t *testing.T) {
	t.Parallel()
	if got := SettingsFromConfig(nil); got != nil {
		t.Errorf("nil cfg should map to nil Settings, got %+v", got)
	}
}

func TestSettingsFromConfig_disabled(t *testing.T) {
	t.Parallel()
	disabled := false
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Enabled:          &disabled,
			Port:             12345,
			BindAddr:         "10.0.0.1",
			StoragePath:      "/srv/cache",
			RetentionDays:    7,
			MaxSizeBytes:     1024,
			MaxUsagePercent:  50,
			Image:            "img:tag",
			ManagementAPIKey: "key",
			URLOverride:      "http://override/",
		},
	}
	if got := SettingsFromConfig(cfg); got != nil {
		t.Errorf("disabled cfg should map to nil Settings, got %+v", got)
	}
}

func TestSettingsFromConfig_enabledMapsAllFields(t *testing.T) {
	t.Parallel()
	enabled := true
	cfg := &config.Config{
		Cache: config.CacheConfig{
			Enabled:          &enabled,
			Port:             12345,
			BindAddr:         "10.0.0.1",
			StoragePath:      "/srv/cache",
			RetentionDays:    7,
			MaxSizeBytes:     1024,
			MaxUsagePercent:  50,
			Image:            "img:tag",
			ManagementAPIKey: "key",
			URLOverride:      "http://override/",
		},
	}

	got := SettingsFromConfig(cfg)
	if got == nil {
		t.Fatal("enabled cfg should produce non-nil Settings")
	}
	if !got.Enabled {
		t.Error("Enabled should be true")
	}
	if got.Port != 12345 {
		t.Errorf("Port: got %d want 12345", got.Port)
	}
	if got.BindAddr != "10.0.0.1" {
		t.Errorf("BindAddr: got %q", got.BindAddr)
	}
	if got.StoragePath != "/srv/cache" {
		t.Errorf("StoragePath: got %q", got.StoragePath)
	}
	if got.RetentionDays != 7 {
		t.Errorf("RetentionDays: got %d", got.RetentionDays)
	}
	if got.MaxSizeBytes != 1024 {
		t.Errorf("MaxSizeBytes: got %d", got.MaxSizeBytes)
	}
	if got.MaxUsagePercent != 50 {
		t.Errorf("MaxUsagePercent: got %d", got.MaxUsagePercent)
	}
	if got.Image != "img:tag" {
		t.Errorf("Image: got %q", got.Image)
	}
	if got.ManagementAPIKey != "key" {
		t.Errorf("ManagementAPIKey: got %q", got.ManagementAPIKey)
	}
	if got.URLOverride != "http://override/" {
		t.Errorf("URLOverride: got %q", got.URLOverride)
	}
}

func TestSettingsFromConfig_zeroValueConfigIsDisabled(t *testing.T) {
	t.Parallel()
	// A zero-value *Config has not been through Load/applyDefaults; CacheEnabled
	// returns false, so SettingsFromConfig must return nil — callers can rely on
	// this to detect "no cache section configured".
	if got := SettingsFromConfig(&config.Config{}); got != nil {
		t.Errorf("zero-value cfg should map to nil Settings, got %+v", got)
	}
}
