package config

import (
	"fmt"
	"os"
)

// LoadFromPath loads the YAML config after verifying the file exists. Missing files get a hint to run gh sr init.
func LoadFromPath(path string) (*Config, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s\n\nRun `gh sr init` to create ~/.gh-sr/runners.yml, or set %s to a YAML file, or use --config",
				path, EnvVarConfigPath)
		}
		return nil, fmt.Errorf("config file: %w", err)
	}
	return Load(path)
}
