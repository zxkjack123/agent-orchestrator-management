package app

import (
	"database/sql"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/db"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/session"
)

// OpenSessionService opens the project database and returns a session service bound to it.
func (a *App) OpenSessionService(dbPath string) (*session.Service, *sql.DB, error) {
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return nil, nil, err
	}

	return session.NewService(sqlDB), sqlDB, nil
}
