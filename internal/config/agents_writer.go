package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SaveAgentsFile writes the agents configuration back to .aom/agents.yaml.
func SaveAgentsFile(aomPath string, f AgentsFile) error {
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshal agents: %w", err)
	}
	return os.WriteFile(filepath.Join(aomPath, "agents.yaml"), data, 0o644)
}
