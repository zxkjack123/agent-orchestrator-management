package app

import (
	"github.com/lattapon-aek/agent-orchestrator-management/internal/config"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/provider"
)

func init() {
	for name := range provider.DefaultRegistry() {
		config.RegisterKnownRuntime(name)
		project.RegisterKnownInitRuntime(name)
	}
}
