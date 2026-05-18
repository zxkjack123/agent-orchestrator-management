package app

import (
	"database/sql"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/db"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/step"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/task"
)

// OpenTaskService opens the project database and returns a task service bound to it.
func (a *App) OpenTaskService(dbPath string) (*task.Service, *sql.DB, error) {
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return nil, nil, err
	}

	return task.NewService(sqlDB), sqlDB, nil
}

// OpenStepService opens the project database and returns a step service bound to it.
func (a *App) OpenStepService(dbPath string) (*step.Service, *sql.DB, error) {
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return nil, nil, err
	}

	return step.NewService(sqlDB), sqlDB, nil
}
