package app

import (
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/config"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/project"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/provider"
)

func init() {
	for name := range provider.DefaultRegistry() {
		config.RegisterKnownRuntime(name)
		project.RegisterKnownInitRuntime(name)
	}
}
