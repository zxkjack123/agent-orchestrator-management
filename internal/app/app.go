package app

import (
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/plan"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/project"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/tmux"
)

// App holds top-level application dependencies as the CLI grows.
type App struct {
	Planner  *plan.Service
	Projects *project.Service
	Tmux     *tmux.Manager
}

// New creates a new application container with default wiring.
func New() *App {
	return &App{
		Planner:  plan.NewService(),
		Projects: project.NewService(),
		Tmux:     tmux.NewManager(),
	}
}
