package server

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/handler"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/server/ws"
)

// Server is the AOM HTTP server. It owns the registry and wires all handlers
// onto a single mux. The app dependency is passed through to each handler so
// they can call domain services directly (no separate service layer needed here).
type Server struct {
	app      *app.App
	registry *Registry
	mux      *http.ServeMux
}

// New creates a Server, loads the project registry, and registers all routes.
func New(a *app.App) (*Server, error) {
	reg, err := NewRegistry()
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}

	s := &Server{
		app:      a,
		registry: reg,
		mux:      http.NewServeMux(),
	}
	s.registerRoutes()
	return s, nil
}

// Handler returns the composed HTTP handler (middleware + mux).
func (s *Server) Handler() http.Handler {
	return loggingMiddleware(corsMiddleware(s.mux))
}

// registerRoutes wires every REST and WebSocket endpoint onto the mux.
// One line per route — easy to scan and add new endpoints.
func (s *Server) registerRoutes() {
	projects := handler.NewProjectsHandler(s.registry)
	agents := handler.NewAgentsHandler(s.app, s.registry)
	sessions := handler.NewSessionsHandler(s.app, s.registry)
	status := handler.NewStatusHandler(s.app, s.registry)

	// REST — Projects registry
	s.mux.HandleFunc("GET /api/v1/projects", projects.List)
	s.mux.HandleFunc("POST /api/v1/projects", projects.Add)
	s.mux.HandleFunc("DELETE /api/v1/projects/{id}", projects.Remove)

	// REST — Per-project resources
	s.mux.HandleFunc("GET /api/v1/projects/{id}/agents", agents.List)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/sessions", sessions.List)
	s.mux.HandleFunc("DELETE /api/v1/projects/{id}/sessions/{sid}", sessions.Stop)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/status", status.Get)

	// WebSocket — tmux terminal stream: /ws/terminal/{pane-id}
	terminal := ws.NewTerminalHandler(s.app.Tmux)
	s.mux.Handle("/ws/terminal/{pane}", terminal)

	// WebSocket — project events stream (channel.md tail): /ws/events/{project-id}
	events := ws.NewEventsHandler(s.channelPath)
	s.mux.Handle("/ws/events/{project}", events)

	// WebSocket — agent mailbox stream: /ws/mailbox/{project-id}/{agent}
	mailbox := ws.NewMailboxHandler(s.mailboxPath)
	s.mux.Handle("/ws/mailbox/{project}/{agent}", mailbox)

	// SPA — serve embedded frontend for all other paths (must be last).
	s.mux.Handle("/", newSPAHandler())
}

// channelPath resolves the .aom/channel.md path for a project ID.
func (s *Server) channelPath(projectID string) (string, bool) {
	proj, ok := s.registry.Get(projectID)
	if !ok {
		return "", false
	}
	return filepath.Join(proj.Path, ".aom", "channel.md"), true
}

// mailboxPath resolves the .aom/mailbox/<agent>.md path for a project + agent.
func (s *Server) mailboxPath(projectID, agentName string) (string, bool) {
	proj, ok := s.registry.Get(projectID)
	if !ok {
		return "", false
	}
	return filepath.Join(proj.Path, ".aom", "mailbox", agentName+".md"), true
}
