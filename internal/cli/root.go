package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/agent"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/app"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/project"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/session"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/tmux"
)

var newApp = app.New

// Runner executes top-level CLI behavior.
type Runner struct {
	app    *app.App
	stdout io.Writer
	stderr io.Writer
}

// Execute runs the AOM CLI using the provided arguments and streams.
func Execute(args []string, stdout, stderr io.Writer) error {
	r := Runner{
		app:    newApp(),
		stdout: stdout,
		stderr: stderr,
	}

	return r.Execute(args)
}

// Execute dispatches a command line invocation.
func (r Runner) Execute(args []string) error {
	_ = r.app

	if len(args) == 0 {
		r.printHelp()
		return nil
	}

	switch args[0] {
	case "help", "--help", "-h":
		r.printHelp()
		return nil
	case "attach":
		return r.executeAttach(args[1:])
	case "capture":
		return r.executeCapture(args[1:])
	case "open":
		return r.executeOpen(args[1:])
	case "session":
		return r.executeSession(args[1:])
	case "status":
		return r.executeStatus(args[1:])
	case "project":
		return r.executeProject(args[1:])
	default:
		return fmt.Errorf("unknown command %q", strings.Join(args, " "))
	}
}

func (r Runner) executeSession(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session subcommand is required")
	}

	switch args[0] {
	case "spawn":
		return r.executeSessionSpawn(args[1:])
	case "list":
		return r.executeSessionList(args[1:])
	case "show":
		return r.executeSessionShow(args[1:])
	default:
		return fmt.Errorf("unknown session command %q", strings.Join(args, " "))
	}
}

func (r Runner) executeProject(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("project subcommand is required")
	}

	switch args[0] {
	case "init":
		return r.executeProjectInit(args[1:])
	default:
		return fmt.Errorf("unknown project command %q", strings.Join(args, " "))
	}
}

func (r Runner) executeProjectInit(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("project name is required")
	}

	params := projectInitParams{
		name: args[0],
	}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			i++
			if i >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			params.repo = args[i]
		case "--default-branch":
			i++
			if i >= len(args) {
				return fmt.Errorf("--default-branch requires a value")
			}
			params.defaultBranch = args[i]
		case "--session-prefix":
			i++
			if i >= len(args) {
				return fmt.Errorf("--session-prefix requires a value")
			}
			params.sessionPrefix = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if strings.TrimSpace(params.repo) == "" {
		return fmt.Errorf("--repo is required")
	}

	result, err := r.app.Projects.Init(params.toInitParams())
	if err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Project initialized")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Name: %s\n", result.ProjectName)
	fmt.Fprintf(r.stdout, "Repo: %s\n", result.RepoPath)
	fmt.Fprintf(r.stdout, "AOM: %s\n", result.AOMPath)
	fmt.Fprintf(r.stdout, "DB: %s\n", result.DBPath)
	fmt.Fprintf(r.stdout, "Config: %s\n", filepath.Join(result.AOMPath, "project.yaml"))

	return nil
}

type projectInitParams struct {
	name          string
	repo          string
	defaultBranch string
	sessionPrefix string
}

func (p projectInitParams) toInitParams() project.InitParams {
	return project.InitParams{
		Name:          p.name,
		RepoPath:      p.repo,
		DefaultBranch: p.defaultBranch,
		SessionPrefix: p.sessionPrefix,
	}
}

func (r Runner) executeOpen(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("open does not accept positional arguments in the current milestone")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	workspace, err := r.app.Tmux.EnsureWorkspace(result.SessionPrefix, result.Project.RepoPath)
	if err != nil {
		return fmt.Errorf("ensure tmux workspace: %w", err)
	}

	sessions, err := r.loadProjectSessions(result)
	if err != nil {
		return err
	}

	r.printProjectSummary("Project opened", result, workspace, sessions)
	return nil
}

func (r Runner) executeStatus(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("status does not accept positional arguments in the current milestone")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	sessions, err := r.loadProjectSessions(result)
	if err != nil {
		return err
	}

	r.printProjectSummary("Project status", result, nil, sessions)
	return nil
}

func (r Runner) executeSessionSpawn(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent name is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("session spawn does not accept extra positional arguments in the current milestone")
	}

	agentName := strings.TrimSpace(args[0])
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	agentRecord, err := findAgent(result.Agents, agentName)
	if err != nil {
		return err
	}

	workspace, err := r.app.Tmux.EnsureWorkspace(result.SessionPrefix, result.Project.RepoPath)
	if err != nil {
		return fmt.Errorf("ensure tmux workspace: %w", err)
	}

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	record, err := sessionService.Create(session.CreateParams{
		ProjectID:       result.Project.ID,
		AgentID:         agentRecord.ID,
		AgentName:       agentRecord.Name,
		RoleName:        agentRecord.Role,
		Runtime:         agentRecord.Runtime,
		Status:          "Booting",
		RepoPath:        result.Project.RepoPath,
		WorktreePath:    result.Project.RepoPath,
		TmuxSessionName: workspace.Name,
	})
	if err != nil {
		return err
	}

	paneBinding, err := r.app.Tmux.CreatePane(workspace.Target, result.Project.RepoPath, placeholderShellCommand(*record))
	if err != nil {
		return err
	}

	record.Status = "Idle"
	record.TmuxWindow = paneBinding.WindowID
	record.TmuxPane = paneBinding.PaneID
	record.TmuxSessionName = workspace.Name

	record, err = sessionService.Save(*record)
	if err != nil {
		return err
	}

	if err := r.app.Tmux.AnnotatePane(record.TmuxPane, map[string]string{
		"@aom_session_id": record.ID,
		"@aom_agent":      record.AgentName,
		"@aom_role":       record.RoleName,
	}); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Session spawned")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Session: %s\n", record.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", record.AgentName)
	fmt.Fprintf(r.stdout, "Role: %s\n", record.RoleName)
	fmt.Fprintf(r.stdout, "Runtime: %s\n", record.Runtime)
	fmt.Fprintf(r.stdout, "Workspace: %s\n", workspace.Target)
	fmt.Fprintf(r.stdout, "Window: %s\n", record.TmuxWindow)
	fmt.Fprintf(r.stdout, "Pane: %s\n", record.TmuxPane)

	return nil
}

func (r Runner) executeSessionList(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("session list does not accept positional arguments in the current milestone")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	sessions, err := sessionService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Sessions")
	fmt.Fprintln(r.stdout, "")
	if len(sessions) == 0 {
		fmt.Fprintln(r.stdout, "No sessions")
		return nil
	}

	for _, item := range sessions {
		fmt.Fprintf(
			r.stdout,
			"  - %s | agent=%s | role=%s | runtime=%s | status=%s | tmux=%s %s %s\n",
			item.ID,
			item.AgentName,
			item.RoleName,
			item.Runtime,
			item.Status,
			item.TmuxSessionName,
			item.TmuxWindow,
			item.TmuxPane,
		)
	}

	return nil
}

func (r Runner) executeSessionShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("session show does not accept extra positional arguments in the current milestone")
	}

	sessionRecord, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Session")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "ID: %s\n", sessionRecord.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", sessionRecord.AgentName)
	fmt.Fprintf(r.stdout, "Role: %s\n", sessionRecord.RoleName)
	fmt.Fprintf(r.stdout, "Runtime: %s\n", sessionRecord.Runtime)
	fmt.Fprintf(r.stdout, "Status: %s\n", sessionRecord.Status)
	fmt.Fprintf(r.stdout, "Repo: %s\n", sessionRecord.RepoPath)
	fmt.Fprintf(r.stdout, "Worktree: %s\n", sessionRecord.WorktreePath)
	fmt.Fprintf(r.stdout, "Tmux session: %s\n", sessionRecord.TmuxSessionName)
	fmt.Fprintf(r.stdout, "Tmux window: %s\n", sessionRecord.TmuxWindow)
	fmt.Fprintf(r.stdout, "Tmux pane: %s\n", sessionRecord.TmuxPane)

	return nil
}

func (r Runner) executeAttach(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("attach does not accept extra positional arguments in the current milestone")
	}

	sessionRecord, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if strings.TrimSpace(sessionRecord.TmuxSessionName) == "" || strings.TrimSpace(sessionRecord.TmuxPane) == "" {
		return fmt.Errorf("session %q does not have a live tmux binding", sessionRecord.ID)
	}

	fmt.Fprintf(r.stdout, "Attaching to %s (%s)\n", sessionRecord.ID, sessionRecord.TmuxPane)
	return r.app.Tmux.AttachPane(sessionRecord.TmuxSessionName, sessionRecord.TmuxPane)
}

func (r Runner) executeCapture(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("capture does not accept extra positional arguments in the current milestone")
	}

	sessionRecord, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if strings.TrimSpace(sessionRecord.TmuxPane) == "" {
		return fmt.Errorf("session %q does not have a live tmux pane binding", sessionRecord.ID)
	}

	output, err := r.app.Tmux.CapturePane(sessionRecord.TmuxPane)
	if err != nil {
		return err
	}

	fmt.Fprint(r.stdout, output)
	return nil
}

func (r Runner) printProjectSummary(title string, result *project.OpenResult, workspace *tmux.Workspace, sessions []session.Record) {
	terminalAvailability := r.app.Tmux.Availability()
	workspaceName := r.app.Tmux.ProjectSessionName(result.SessionPrefix)

	fmt.Fprintln(r.stdout, title)
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Name: %s\n", result.Project.Name)
	fmt.Fprintf(r.stdout, "Repo: %s\n", result.Project.RepoPath)
	fmt.Fprintf(r.stdout, "Default branch: %s\n", result.Project.DefaultBranch)
	fmt.Fprintf(r.stdout, "DB: %s\n", result.DBPath)
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Terminal:")
	fmt.Fprintf(r.stdout, "  Driver: %s\n", result.TerminalDriver)
	fmt.Fprintf(r.stdout, "  Available: %t\n", terminalAvailability.Available)
	if terminalAvailability.Available {
		fmt.Fprintf(r.stdout, "  Binary: %s\n", terminalAvailability.BinaryPath)
	} else {
		fmt.Fprintln(r.stdout, "  Binary: not found")
	}
	fmt.Fprintf(r.stdout, "  Workspace: %s\n", workspaceName)
	if workspace != nil {
		state := "reused"
		if workspace.Created {
			state = "created"
		}
		fmt.Fprintf(r.stdout, "  Workspace state: %s\n", state)
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Agents:")
	for _, agent := range result.Agents {
		fmt.Fprintf(r.stdout, "  - %s | role=%s | runtime=%s | enabled=%t\n", agent.Name, agent.Role, agent.Runtime, agent.Enabled)
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Sessions:")
	if len(sessions) == 0 {
		fmt.Fprintln(r.stdout, "  None")
	} else {
		for _, item := range sessions {
			fmt.Fprintf(
				r.stdout,
				"  - %s | agent=%s | role=%s | runtime=%s | status=%s | tmux=%s %s %s\n",
				item.ID,
				item.AgentName,
				item.RoleName,
				item.Runtime,
				item.Status,
				item.TmuxSessionName,
				item.TmuxWindow,
				item.TmuxPane,
			)
		}
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Counts:")
	fmt.Fprintf(r.stdout, "  Tasks: %d\n", 0)
	fmt.Fprintf(r.stdout, "  Sessions: %d\n", len(sessions))
}

func (r Runner) printHelp() {
	fmt.Fprintln(r.stdout, "AOM")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Milestone 2 terminal scaffolding is in progress.")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Planned commands:")
	fmt.Fprintln(r.stdout, "  aom project init")
	fmt.Fprintln(r.stdout, "  aom attach")
	fmt.Fprintln(r.stdout, "  aom capture")
	fmt.Fprintln(r.stdout, "  aom open")
	fmt.Fprintln(r.stdout, "  aom session show")
	fmt.Fprintln(r.stdout, "  aom session spawn")
	fmt.Fprintln(r.stdout, "  aom session list")
	fmt.Fprintln(r.stdout, "  aom status")
}

func findAgent(agents []agent.Record, name string) (*agent.Record, error) {
	for _, item := range agents {
		if item.Name != name {
			continue
		}
		if !item.Enabled {
			return nil, fmt.Errorf("agent %q is disabled", name)
		}

		agentCopy := item
		return &agentCopy, nil
	}

	return nil, fmt.Errorf("agent %q not found", name)
}

func placeholderShellCommand(record session.Record) string {
	return fmt.Sprintf(
		"sh -lc 'printf \"AOM session %s\\nagent=%s\\nrole=%s\\nruntime=%s\\n\"; exec ${SHELL:-sh}'",
		record.ID,
		record.AgentName,
		record.RoleName,
		record.Runtime,
	)
}

func (r Runner) loadSessionByIdentifier(identifier string) (*session.Record, error) {
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return nil, err
	}

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	record, err := sessionService.Get(identifier)
	if err != nil {
		return nil, err
	}
	if record != nil {
		return record, nil
	}

	sessions, err := sessionService.ListByProject(result.Project.ID)
	if err != nil {
		return nil, err
	}

	for _, item := range sessions {
		if item.AgentName == identifier {
			sessionCopy := item
			return &sessionCopy, nil
		}
	}

	return nil, fmt.Errorf("session %q not found", identifier)
}

func (r Runner) loadProjectSessions(result *project.OpenResult) ([]session.Record, error) {
	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	return sessionService.ListByProject(result.Project.ID)
}
