package config

import (
	"fmt"
	"os"
)

// LoadFromPath loads the YAML config after verifying the file exists. Missing files get a hint to run ghr init.
func LoadFromPath(path string) (*Config, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s\n\nRun `ghr init` to create ~/.ghr, or add %s in the current directory, or set %s or use --config",
				path, LocalConfigRelative(), EnvVarConfigPath)
		}
		return nil, fmt.Errorf("config file: %w", err)
	}
	return Load(path)
}
