package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// EnvVarConfigPath overrides auto config path resolution when --config is not set.
	EnvVarConfigPath = "GH_SR_CONFIG"
)

// SrDir returns the directory ~/.gh-sr (or $HOME/.gh-sr).
func SrDir() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	return filepath.Join(h, ".gh-sr"), nil
}

// EnvFilePath returns ~/.gh-sr/env.
func EnvFilePath() (string, error) {
	d, err := SrDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "env"), nil
}

// UserRunnersPath returns ~/.gh-sr/runners.yml.
func UserRunnersPath() (string, error) {
	d, err := SrDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "runners.yml"), nil
}

// ResolveConfigPath picks the config file path when --config is empty (auto mode).
// Order: non-empty flag, else GH_SR_CONFIG, else ~/.gh-sr/runners.yml.
func ResolveConfigPath(cfgFlag string) (string, error) {
	if cfgFlag != "" {
		return filepath.Abs(cfgFlag)
	}
	if v := os.Getenv(EnvVarConfigPath); v != "" {
		return filepath.Abs(v)
	}
	return UserRunnersPath()
}

// BootstrapEnv loads ~/.gh-sr/env into the process environment if the file exists.
func BootstrapEnv() error {
	p, err := EnvFilePath()
	if err != nil {
		return err
	}
	return ApplyEnvFile(p)
}
