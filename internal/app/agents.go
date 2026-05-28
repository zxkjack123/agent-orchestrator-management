package app

import (
	"database/sql"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/agent"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/db"
)

// OpenAgentRepository opens the project database and returns an agent repository bound to it.
func (a *App) OpenAgentRepository(dbPath string) (*agent.Repository, *sql.DB, error) {
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return nil, nil, err
	}

	return agent.NewRepository(sqlDB), sqlDB, nil
}
