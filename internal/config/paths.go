package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// EnvVarConfigPath is the environment variable that overrides auto config path resolution
	// when --config is not set.
	EnvVarConfigPath = "GHR_CONFIG"
)

// LocalConfigRelative is the relative path under the current working directory for project-local config.
func LocalConfigRelative() string {
	return filepath.Join("config", "runners.yml")
}

// DefaultPath returns the legacy default relative path (cwd/config/runners.yml). Prefer ResolveConfigPath for auto.
func DefaultPath() string {
	return LocalConfigRelative()
}

// GhrDir returns the directory ~/.ghr (or $HOME/.ghr).
func GhrDir() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	return filepath.Join(h, ".ghr"), nil
}

// EnvFilePath returns ~/.ghr/env.
func EnvFilePath() (string, error) {
	d, err := GhrDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "env"), nil
}

// UserRunnersPath returns ~/.ghr/runners.yml.
func UserRunnersPath() (string, error) {
	d, err := GhrDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "runners.yml"), nil
}

// ResolveConfigPath picks the config file path when --config is empty (auto mode).
// Order: explicit flag (caller must pass non-empty when set), else GHR_CONFIG, else ./config/runners.yml if it exists, else ~/.ghr/runners.yml.
func ResolveConfigPath(cfgFlag string) (string, error) {
	if cfgFlag != "" {
		return filepath.Abs(cfgFlag)
	}
	if v := os.Getenv(EnvVarConfigPath); v != "" {
		return filepath.Abs(v)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	local := filepath.Join(wd, LocalConfigRelative())
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}
	return UserRunnersPath()
}

// BootstrapEnv loads ~/.ghr/env into the process environment if the file exists.
func BootstrapEnv() error {
	p, err := EnvFilePath()
	if err != nil {
		return err
	}
	return ApplyEnvFile(p)
}
