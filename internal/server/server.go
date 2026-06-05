package server

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

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
	go s.cleanupStaleSessions()
	return s, nil
}

// cleanupStaleSessions periodically kills aom-ws-* grouped sessions that were
// created by the War Room WebSocket handler but not cleaned up (e.g. browser
// crash, hard refresh). Skips sessions that still have an active client attached.
func (s *Server) cleanupStaleSessions() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		stale, _ := s.app.Tmux.ListSessionsByPrefix("aom-ws-")
		for _, name := range stale {
			if s.app.Tmux.SessionHasClients(name) {
				continue // still in use — leave it alone
			}
			if err := s.app.Tmux.KillSession(name); err == nil {
				log.Printf("[server] cleaned up stale session %s", name)
			}
		}
	}
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
	tasks := handler.NewTasksHandler(s.app, s.registry)

	// REST — Projects registry
	s.mux.HandleFunc("GET /api/v1/projects", projects.List)
	s.mux.HandleFunc("POST /api/v1/projects", projects.Add)
	s.mux.HandleFunc("DELETE /api/v1/projects/{id}", projects.Remove)

	// REST — Per-project resources
	s.mux.HandleFunc("GET /api/v1/projects/{id}/agents", agents.List)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/agents", agents.Add)
	s.mux.HandleFunc("PUT /api/v1/projects/{id}/agents/{name}", agents.Update)
	s.mux.HandleFunc("DELETE /api/v1/projects/{id}/agents/{name}", agents.Remove)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/agents/{name}/provision", agents.Provision)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/agents/{name}/profile", agents.GetProfile)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/agents/{name}/instructions", agents.GetInstructions)
	s.mux.HandleFunc("PUT /api/v1/projects/{id}/agents/{name}/instructions", agents.SetInstructions)

	s.mux.HandleFunc("GET /api/v1/projects/{id}/sessions", sessions.List)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/sessions", sessions.Spawn)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/sessions/isolate", sessions.Isolate)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/sessions/{sid}", sessions.Get)
	s.mux.HandleFunc("DELETE /api/v1/projects/{id}/sessions/{sid}", sessions.Stop)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/sessions/{sid}/archive", sessions.Archive)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/sessions/{sid}/send", sessions.Send)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/sessions/{sid}/resume", sessions.Resume)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/sessions/{sid}/approve", sessions.Approve)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/sessions/{sid}/deny", sessions.Deny)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/sessions/{sid}/recover", sessions.Recover)

	s.mux.HandleFunc("GET /api/v1/projects/{id}/status", status.Get)

	// REST — Tasks
	s.mux.HandleFunc("GET /api/v1/projects/{id}/tasks", tasks.List)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/tasks", tasks.Create)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/tasks/{tid}", tasks.GetOne)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/tasks/{tid}/signal", tasks.Signal)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/tasks/{tid}/accept", tasks.Accept)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/tasks/{tid}/close", tasks.Close)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/tasks/{tid}/cancel", tasks.Cancel)

	// REST — Project-wide actions
	actions := handler.NewProjectActionsHandler(s.app, s.registry)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/channel", actions.ChannelHistory)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/channel", actions.Channel)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/mailbox/{agent}", actions.MailboxHistory)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/broadcast", actions.Broadcast)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/pause-all", actions.PauseAll)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/resume-all", actions.ResumeAll)

	// REST — Extras (task artifact, requests, metrics, doctor, team-brief, merge)
	extras := handler.NewExtrasHandler(s.registry)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/tasks/{tid}/artifact", extras.TaskArtifact)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/requests", extras.ListRequests)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/requests/{rid}/approve", extras.ApproveRequest)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/requests/{rid}/reject", extras.RejectRequest)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/metrics", extras.Metrics)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/doctor", extras.Doctor)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/team-brief", extras.TeamBriefGet)
	s.mux.HandleFunc("PUT /api/v1/projects/{id}/team-brief", extras.TeamBriefPut)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/team-brief/generate", extras.TeamBriefGenerate)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/team-brief/push", extras.TeamBriefPush)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/merge/check", extras.MergeCheck)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/merge/prepare", extras.MergePrepare)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/merge/commit", extras.MergeCommit)

	// REST — Roles and Classes
	roles := handler.NewRolesHandler(s.app, s.registry)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/roles", roles.ListRoles)
	s.mux.HandleFunc("POST /api/v1/projects/{id}/roles", roles.CreateRole)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/roles/{name}", roles.GetRole)
	s.mux.HandleFunc("PUT /api/v1/projects/{id}/roles/{name}", roles.UpdateRole)
	s.mux.HandleFunc("DELETE /api/v1/projects/{id}/roles/{name}", roles.DeleteRole)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/roles/{name}/preview", roles.PreviewRole)

	s.mux.HandleFunc("GET /api/v1/projects/{id}/classes", roles.ListClasses)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/classes/{name}", roles.GetClass)
	s.mux.HandleFunc("PUT /api/v1/projects/{id}/classes/{name}", roles.SetClass)
	s.mux.HandleFunc("DELETE /api/v1/projects/{id}/classes/{name}", roles.DeleteClass)
	s.mux.HandleFunc("GET /api/v1/projects/{id}/classes/{name}/preview", roles.PreviewClass)

	// REST — System template (read-only embedded base.md.tmpl)
	s.mux.HandleFunc("GET /api/v1/system-template", handler.GetSystemTemplate)

	// REST — Filesystem browser (directory picker)
	s.mux.HandleFunc("GET /api/v1/fs/browse", handler.FsBrowse)
	s.mux.HandleFunc("POST /api/v1/fs/mkdir", handler.FsMkdir)

	// REST — Project init (non-interactive aom project init)
	s.mux.HandleFunc("POST /api/v1/projects/init", projects.InitProject)

	// REST — Terminal pane scrollback history
	s.mux.HandleFunc("GET /api/v1/terminal/{pane}/history", handler.TerminalHistory)

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
