package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/artifact"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/plan"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/session"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/task"
)

func (r Runner) executePlan(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("work description is required")
	}

	params := planParams{workDescription: args[0]}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--create":
			params.createTask = true
		case "--mode":
			i++
			if i >= len(args) {
				return fmt.Errorf("--mode requires a value")
			}
			params.mode = args[i]
		case "--role":
			i++
			if i >= len(args) {
				return fmt.Errorf("--role requires a value")
			}
			params.role = args[i]
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			params.agent = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	planResult, err := r.app.Planner.Build(plan.Params{
		WorkDescription: params.workDescription,
		Mode:            params.mode,
		PreferredRole:   params.role,
		PreferredAgent:  params.agent,
		Agents:          result.Agents,
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Plan")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Work: %s\n", params.workDescription)
	fmt.Fprintf(r.stdout, "Mode: %s\n", planResult.Mode)
	fmt.Fprintf(r.stdout, "Recommended role: %s\n", emptyFallback(planResult.RecommendedRole))
	fmt.Fprintf(r.stdout, "Recommended agent: %s\n", emptyFallback(planResult.RecommendedAgent))
	fmt.Fprintln(r.stdout, "Proposed steps:")
	for i, item := range planResult.Steps {
		fmt.Fprintf(
			r.stdout,
			"  %d. type=%s | title=%s | role=%s | agent=%s\n",
			i+1,
			item.Type,
			item.Title,
			emptyFallback(item.RoleName),
			emptyFallback(item.AgentName),
		)
	}
	fmt.Fprintf(r.stdout, "Recommended next action: %s\n", planResult.RecommendedNextAction)

	if !params.createTask {
		return nil
	}

	if err := r.validateTaskProvisioning(result); err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	createResult, err := taskService.CreateFromPlan(task.CreateParams{
		ProjectID:      result.Project.ID,
		Title:          params.workDescription,
		Mode:           planResult.Mode,
		PreferredRole:  planResult.RecommendedRole,
		PreferredAgent: planResult.RecommendedAgent,
	}, buildPlanStepSeeds(planResult.Steps))
	if err != nil {
		return err
	}

	if _, err := r.ensurePlannedWorktree(result, createResult.Task); err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, createResult.Task.ID, artifact.Event{
		Type:        "task.created",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task created from plan in %s mode", createResult.Task.Mode),
		StateEffect: fmt.Sprintf("Task %s", createResult.Task.Status),
	}, true); err != nil {
		return err
	}

	_ = r.refreshProjectBoard(result)

	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Task created from plan")
	fmt.Fprintf(r.stdout, "Task: %s\n", createResult.Task.ID)
	fmt.Fprintf(r.stdout, "Status: %s\n", createResult.Task.Status)
	fmt.Fprintf(r.stdout, "Steps created: %d\n", len(createResult.Steps))

	return nil
}

type planParams struct {
	workDescription string
	mode            string
	role            string
	agent           string
	createTask      bool
}


func (r Runner) executeProjectResources(args []string) error {
	_ = args

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Project resources: %s\n", result.Project.Name)
	fmt.Fprintln(r.stdout, "")

	res := result.Resources
	if len(res.RoleBindings) == 0 {
		fmt.Fprintln(r.stdout, "Role bindings: none (add skill and MCP server definitions to .aom/resources.yaml)")
	} else {
		fmt.Fprintln(r.stdout, "Role bindings:")
		for roleName, binding := range res.RoleBindings {
			fmt.Fprintf(r.stdout, "\n  Role: %s\n", roleName)
			fmt.Fprintln(r.stdout, "    Skills:")
			if len(binding.Skills) == 0 {
				fmt.Fprintln(r.stdout, "      none")
			} else {
				for _, skillName := range binding.Skills {
					skill, ok := res.Skills[skillName]
					if !ok {
						continue
					}
					fmt.Fprintf(r.stdout, "      %s   path=%s  runtimes=%s\n",
						skillName, skill.Path, strings.Join(skill.Runtimes, ","))
				}
			}
			fmt.Fprintln(r.stdout, "    MCP Servers:")
			if len(binding.MCPServers) == 0 {
				fmt.Fprintln(r.stdout, "      none")
			} else {
				for _, serverName := range binding.MCPServers {
					srv, ok := res.MCPServers[serverName]
					if !ok {
						continue
					}
					switch srv.Type {
					case "stdio":
						cmd := srv.Command
						if len(srv.Args) > 0 {
							cmd += " " + strings.Join(srv.Args, " ")
						}
						fmt.Fprintf(r.stdout, "      %s   type=stdio  command=%s  runtimes=%s\n",
							serverName, cmd, strings.Join(srv.Runtimes, ","))
					case "http":
						fmt.Fprintf(r.stdout, "      %s   type=http  url=%s  runtimes=%s\n",
							serverName, srv.URL, strings.Join(srv.Runtimes, ","))
					}
				}
			}
		}
	}

	fmt.Fprintln(r.stdout, "")
	pol := result.Policy.Policy
	fmt.Fprintln(r.stdout, "Policy:")
	fmt.Fprintf(r.stdout, "  Yolo mode: %s\n", pol.SessionDefaults.YoloMode)
	fmt.Fprintf(r.stdout, "  Approval scope: %s\n", pol.SessionDefaults.ApprovalScope)
	fmt.Fprintf(r.stdout, "  Deny commands: %d configured\n", len(pol.DenyCommands))
	fmt.Fprintf(r.stdout, "  Require approval: %d configured\n", len(pol.RequireApproval))
	if len(pol.DenyCommands) > 0 {
		for _, cmd := range pol.DenyCommands {
			fmt.Fprintf(r.stdout, "    deny: %s\n", cmd)
		}
	}

	return nil
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
		case "--template":
			i++
			if i >= len(args) {
				return fmt.Errorf("--template requires a value")
			}
			params.templateName = args[i]
		case "--template-dir":
			i++
			if i >= len(args) {
				return fmt.Errorf("--template-dir requires a value")
			}
			params.templateDir = args[i]
		case "--agents":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agents requires a value")
			}
			selections, err := project.ParseInitAgentSelections(parseCommaSeparatedValues(args[i]))
			if err != nil {
				return err
			}
			params.agentSelections = selections
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if strings.TrimSpace(params.repo) == "" {
		return fmt.Errorf("--repo is required")
	}
	if len(params.agentSelections) == 0 {
		selectedAgents, err := r.promptProjectInitAgents(params)
		if err != nil {
			return err
		}
		params.agentSelections = selectedAgents
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
	if result.GitInitialized && result.GitInitialCommit {
		fmt.Fprintln(r.stdout, "Git: repository initialized + initial commit created")
	} else if result.GitInitialized {
		fmt.Fprintln(r.stdout, "Git: repository initialized")
	} else if result.GitInitialCommit {
		fmt.Fprintln(r.stdout, "Git: initial commit created")
	}

	// G3: recommend per-agent workspace provisioning after init.
	// Without workspaces, agents sharing the same runtime overwrite each other's
	// identity files (CLAUDE.md / AGENTS.md) in the repo root.
	if len(params.agentSelections) > 0 {
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintln(r.stdout, "Next: provision a dedicated workspace for each agent (prevents identity-file")
		fmt.Fprintln(r.stdout, "      conflicts when multiple agents use the same runtime, e.g. two claude agents):")
		fmt.Fprintln(r.stdout, "")
		for _, sel := range params.agentSelections {
			fmt.Fprintf(r.stdout, "  aom agent provision %s\n", sel.Name)
		}
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintln(r.stdout, "Then spawn agents:")
		for _, sel := range params.agentSelections {
			fmt.Fprintf(r.stdout, "  aom session spawn %s --real\n", sel.Name)
		}
	}

	return nil
}

func (r Runner) promptProjectInitAgents(params projectInitParams) ([]project.InitAgentSelection, error) {
	if r.stdin == nil || r.isTTY == nil || !r.isTTY(r.stdin) {
		return nil, nil
	}

	options, err := r.app.Projects.PreviewInitAgents(params.toInitParams())
	if err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, nil
	}

	fmt.Fprintln(r.stdout, "Select agents to enable (comma-separated names or name:role:runtime, blank for all):")
	for _, option := range options {
		fmt.Fprintf(r.stdout, "  - %s | role=%s | runtime=%s\n", option.Name, emptyFallback(option.Role), emptyFallback(option.Runtime))
	}
	fmt.Fprint(r.stdout, "> ")

	line, err := bufio.NewReader(r.stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read agent selection: %w", err)
	}

	rawValues := parseCommaSeparatedValues(strings.TrimSpace(line))
	if len(rawValues) == 0 {
		return nil, nil
	}

	selected, err := project.ParseInitAgentSelections(rawValues)
	if err != nil {
		return nil, err
	}

	validNames := make(map[string]struct{}, len(options))
	for _, option := range options {
		validNames[option.Name] = struct{}{}
	}
	for _, item := range selected {
		if item.Inline {
			continue
		}
		if _, ok := validNames[item.Name]; !ok {
			return nil, fmt.Errorf("agent %q was not found in the selected template", item.Name)
		}
	}

	return selected, nil
}

type projectInitParams struct {
	name            string
	repo            string
	defaultBranch   string
	sessionPrefix   string
	templateName    string
	templateDir     string
	agentSelections []project.InitAgentSelection
}

func (p projectInitParams) toInitParams() project.InitParams {
	return project.InitParams{
		Name:            p.name,
		RepoPath:        p.repo,
		DefaultBranch:   p.defaultBranch,
		SessionPrefix:   p.sessionPrefix,
		TemplateName:    p.templateName,
		TemplateDir:     p.templateDir,
		AgentSelections: p.agentSelections,
	}
}

func (r Runner) executeOpen(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("open does not accept positional arguments in the current milestone")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return wrapProjectNotFound(err)
	}

	workspace, err := r.app.Tmux.EnsureWorkspace(result.SessionPrefix, result.Project.RepoPath)
	if err != nil {
		return fmt.Errorf("ensure tmux workspace: %w", err)
	}

	sessions, err := r.loadProjectSessions(result)
	if err != nil {
		return err
	}

	taskCount, err := r.loadTaskCount(result)
	if err != nil {
		return err
	}

	taskViews, err := r.loadTaskViews(result, sessions)
	if err != nil {
		return err
	}

	r.printProjectSummary("Project opened", result, workspace, sessions, taskCount, taskViews)
	return nil
}

// pushSharedFile copies src data as filename into .aom/shared/ and into
// .agent/shared/ of every Ready/Active worktree. Returns number of worktrees pushed.
func pushSharedFile(repoPath, aomPath, filename string) error {
	data, err := os.ReadFile(filepath.Join(aomPath, filename))
	if err != nil {
		return err
	}
	sharedDir := filepath.Join(repoPath, ".aom", "shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(sharedDir, filename), data, 0o644); err != nil {
		return err
	}
	// Best-effort push to agent workspaces under .aom/agents/*/workspace/.agent/shared/
	agentsDir := filepath.Join(aomPath, "agents")
	entries, _ := os.ReadDir(agentsDir)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		wsSharedDir := filepath.Join(agentsDir, e.Name(), "workspace", ".agent", "shared")
		if err := os.MkdirAll(wsSharedDir, 0o755); err != nil {
			continue
		}
		_ = os.WriteFile(filepath.Join(wsSharedDir, filename), data, 0o644)
	}
	return nil
}

// executeProjectShare copies a file into the operator-owned shared docs directory
// (.aom/shared/) and also into the .agent/shared/ directory of every active worktree,
// so all running agents can immediately read it.
func (r Runner) executeProjectShare(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("file path is required: aom project share <file>")
	}
	srcPath := strings.TrimSpace(args[0])

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	filename := filepath.Base(srcPath)

	// Always write to the operator-owned shared dir (.aom/shared/).
	sharedDir := filepath.Join(result.Project.RepoPath, ".aom", "shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		return fmt.Errorf("create shared dir: %w", err)
	}
	destMain := filepath.Join(sharedDir, filename)
	if err := os.WriteFile(destMain, data, 0o644); err != nil {
		return fmt.Errorf("write shared file: %w", err)
	}
	fmt.Fprintf(r.stdout, "shared: %s\n", destMain)

	// Also push to .agent/shared/ in every active worktree.
	worktreeService, wDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer wDB.Close()

	records, err := worktreeService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	pushed := 0
	for _, wt := range records {
		if wt.Status != "Ready" && wt.Status != "Active" {
			continue
		}
		wtSharedDir := filepath.Join(wt.WorktreePath, ".agent", "shared")
		if err := os.MkdirAll(wtSharedDir, 0o755); err != nil {
			fmt.Fprintf(r.stderr, "Warning: could not create %s: %v\n", wtSharedDir, err)
			continue
		}
		dest := filepath.Join(wtSharedDir, filename)
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			fmt.Fprintf(r.stderr, "Warning: could not write to %s: %v\n", dest, err)
			continue
		}
		fmt.Fprintf(r.stdout, "pushed:  %s  (task: %s)\n", dest, wt.TaskID)
		pushed++
	}

	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Shared %q with %d active worktree(s)\n", filename, pushed)
	fmt.Fprintf(r.stdout, "Agents can read it at: .agent/shared/%s\n", filename)
	return nil
}

// executeProjectLayout generates a repo-layout.md from the current git tree and pushes
// it to .aom/shared/ and every active worktree's .agent/shared/ directory. Agents
// injected with this layout at spawn time can avoid directory-structure drift.
func (r Runner) executeProjectLayout() error {
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// Collect top-level structure from git (gracefully handles empty repo).
	treeOut, _ := exec.Command("git", "-C", result.Project.RepoPath, "ls-tree", "--name-only", "HEAD").Output()
	entries := strings.Split(strings.TrimSpace(string(treeOut)), "\n")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Repo Layout — %s\n", result.Project.Name))
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02")))
	sb.WriteString("## Directory Structure\n\n")
	for _, e := range entries {
		if strings.TrimSpace(e) != "" {
			sb.WriteString(fmt.Sprintf("- `%s`\n", e))
		}
	}
	if strings.TrimSpace(string(treeOut)) == "" {
		sb.WriteString("_(no committed files yet — run after first commit)_\n")
	}
	sb.WriteString("\n## Layout Conventions\n\n")
	sb.WriteString("- Before creating a new top-level directory, coordinate with the team via `aom message send`.\n")
	sb.WriteString("- Keep backend and frontend code in separate subtrees (e.g. `backend/`, `frontend/`).\n")
	sb.WriteString("- Do not mix build artifacts with source files.\n")
	sb.WriteString("- Read this file before starting any task that creates new directories or top-level files.\n")

	content := []byte(sb.String())

	// Write to .aom/shared/repo-layout.md (operator-owned canonical copy).
	sharedDir := filepath.Join(result.Project.RepoPath, ".aom", "shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		return fmt.Errorf("create shared dir: %w", err)
	}
	destPath := filepath.Join(sharedDir, "repo-layout.md")
	if err := os.WriteFile(destPath, content, 0o644); err != nil {
		return fmt.Errorf("write repo-layout.md: %w", err)
	}
	fmt.Fprintf(r.stdout, "Layout written: %s\n", destPath)

	// Push to .agent/shared/ in every active worktree.
	worktreeService, wDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer wDB.Close()

	records, err := worktreeService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	pushed := 0
	for _, wt := range records {
		if wt.Status != "Ready" && wt.Status != "Active" {
			continue
		}
		wtDir := filepath.Join(wt.WorktreePath, ".agent", "shared")
		if err := os.MkdirAll(wtDir, 0o755); err != nil {
			fmt.Fprintf(r.stderr, "Warning: could not create %s: %v\n", wtDir, err)
			continue
		}
		dest := filepath.Join(wtDir, "repo-layout.md")
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			fmt.Fprintf(r.stderr, "Warning: could not write to %s: %v\n", dest, err)
			continue
		}
		pushed++
	}

	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Pushed to %d active worktree(s)\n", pushed)
	fmt.Fprintf(r.stdout, "New sessions will receive it automatically at: .agent/shared/repo-layout.md\n")
	return nil
}

func (r Runner) executeStatus(args []string) error {
	activeOnly := false
	graphMode := false
	jsonMode := false
	actionItemsMode := false
	for _, arg := range args {
		switch arg {
		case "--active":
			activeOnly = true
		case "--graph":
			graphMode = true
		case "--json", "-j":
			jsonMode = true
		case "--action-items", "--actions":
			actionItemsMode = true
		default:
			return fmt.Errorf("unknown flag %q", arg)
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return wrapProjectNotFound(err)
	}

	if graphMode {
		taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
		if err != nil {
			return err
		}
		defer sqlDB.Close()
		return printTaskGraph(r.stdout, taskService, result.Project.ID)
	}

	sessions, err := r.loadProjectSessions(result)
	if err != nil {
		return err
	}

	// Auto-stop Idle sessions whose bound task has signalled task.completed in log.md.
	// This frees orphaned background processes (codex background terminals, caffeinate)
	// the first time the operator checks status after work is done — without requiring
	// an explicit "aom session stop" or "aom task accept" call.
	sessions = r.autoStopCompletedSessions(result, sessions)

	taskCount, err := r.loadTaskCount(result)
	if err != nil {
		return err
	}

	taskViews, err := r.loadTaskViews(result, sessions)
	if err != nil {
		return err
	}

	if activeOnly {
		taskViews = filterActiveTaskViews(taskViews)
		sessions = filterActiveSessions(sessions)
	}

	if jsonMode {
		return r.printStatusJSON(result, sessions, taskCount, taskViews)
	}

	if actionItemsMode {
		return r.printActionItems(result, sessions, taskViews)
	}

	r.printProjectSummary("Project status", result, nil, sessions, taskCount, taskViews)
	return nil
}

// actionItem is one concrete thing the operator should do right now.
type actionItem struct {
	priority int    // 1=urgent, 2=normal, 3=info
	label    string // short tag shown in brackets
	detail   string // human description
	command  string // exact command to run (empty if none)
}

// buildActionItems computes the prioritised list of things the operator should
// do right now, given the current sessions and task views.
// Extracted so both printActionItems and the dashboard can share the logic.
func (r Runner) buildActionItems(result *project.OpenResult, sessions []session.Record, taskViews []taskView) []actionItem {
	var items []actionItem

	// ── Priority 0: agent process has exited (pane is at a bare shell) ───────
	// This is the highest priority because messages sent to a dead agent would
	// be executed as shell commands and silently corrupt the session.
	for _, s := range sessions {
		if s.Status == "Stopped" || s.Status == "Archived" || s.Status == "Failed" {
			continue
		}
		if strings.TrimSpace(s.TmuxPane) == "" {
			continue
		}
		if cmd := r.app.Tmux.PaneCurrentCommand(s.TmuxPane); isShellProcess(cmd) {
			items = append(items, actionItem{
				priority: 0,
				label:    "DEAD",
				detail:   fmt.Sprintf("session %s (%s) — agent process exited, pane is at shell (%s)", s.ID, s.AgentName, cmd),
				command:  fmt.Sprintf("aom session replace %s --agent %s --real", s.ID, s.AgentName),
			})
		}
	}

	// ── Priority 1: sessions waiting for approval ─────────────────────────
	for _, s := range sessions {
		if s.Status == "WaitingApproval" {
			items = append(items, actionItem{
				priority: 1,
				label:    "APPROVAL",
				detail:   fmt.Sprintf("session %s (%s) is waiting for operator approval", s.ID, s.AgentName),
				command:  fmt.Sprintf("aom approve %s", s.ID),
			})
		}
	}

	// ── Priority 2: tasks that signalled completion but are not yet accepted ─
	for i := range taskViews {
		tv := &taskViews[i]
		if tv.Task.Status == "Done" || tv.Task.Status == "Archived" {
			continue
		}
		// Quick log check — no git required.
		logPath := taskArtifactLogPath(result.Project.RepoPath, result.StateDir, tv.Task.ID, tv.Worktree)
		completed := hasTaskCompletedEvent(logPath)
		if !completed {
			agentRecord := findAgentByName(result.Agents, tv.Task.PreferredAgent)
			if agentRecord != nil && strings.TrimSpace(agentRecord.WorkspacePath) != "" {
				wsLog := filepath.Join(strings.TrimSpace(agentRecord.WorkspacePath), ".agent", "log.md")
				completed = hasTaskCompletedEvent(wsLog)
			}
		}
		if completed {
			agentLabel := tv.Task.PreferredAgent
			if agentLabel == "" {
				agentLabel = tv.Task.PreferredRole
			}
			items = append(items, actionItem{
				priority: 2,
				label:    "ACCEPT",
				detail:   fmt.Sprintf("%s  %q  (agent: %s) — task.completed signalled", tv.Task.ID, tv.Task.Title, agentLabel),
				command:  fmt.Sprintf("aom task verify %s  &&  aom task accept %s", tv.Task.ID, tv.Task.ID),
			})
		}
	}

	// ── Priority 2: tasks Ready with no active session ─────────────────────
	activeTasks := make(map[string]bool)
	for _, s := range sessions {
		if s.TaskID != "" && s.Status != "Stopped" && s.Status != "Archived" && s.Status != "Failed" && s.Status != "Detached" {
			activeTasks[s.TaskID] = true
		}
	}
	for i := range taskViews {
		tv := &taskViews[i]
		if tv.Task.Status != "Ready" || activeTasks[tv.Task.ID] {
			continue
		}
		agent := tv.Task.PreferredAgent
		if agent == "" {
			agent = "<agent>"
		}
		items = append(items, actionItem{
			priority: 2,
			label:    "SPAWN",
			detail:   fmt.Sprintf("%s  %q  — no active session", tv.Task.ID, tv.Task.Title),
			command:  fmt.Sprintf("aom session spawn %s --task %s --real", agent, tv.Task.ID),
		})
	}

	// ── Priority 3: tasks Blocked ─────────────────────────────────────────
	for i := range taskViews {
		tv := &taskViews[i]
		if tv.Task.Status == "Blocked" {
			items = append(items, actionItem{
				priority: 3,
				label:    "BLOCKED",
				detail:   fmt.Sprintf("%s  %q  — waiting on dependency (check: aom status --graph)", tv.Task.ID, tv.Task.Title),
			})
		}
	}

	return items
}

// printActionItems shows only the items that require operator attention,
// ordered by priority.  It is much shorter than the full status output.
func (r Runner) printActionItems(result *project.OpenResult, sessions []session.Record, taskViews []taskView) error {
	items := r.buildActionItems(result, sessions, taskViews)

	fmt.Fprintln(r.stdout, "Action Items")
	fmt.Fprintln(r.stdout, "")
	if len(items) == 0 {
		fmt.Fprintln(r.stdout, "  Nothing needs operator attention right now.")
		return nil
	}

	urgentCount := 0
	for _, item := range items {
		if item.priority == 1 {
			urgentCount++
		}
	}
	if urgentCount > 0 {
		fmt.Fprintf(r.stdout, colorize("  %d urgent item(s) require immediate attention\n\n", ansiRed, r.stdout), urgentCount)
	}

	for _, item := range items {
		priorityColor := ansiYellow
		if item.priority == 1 {
			priorityColor = ansiRed
		} else if item.priority == 3 {
			priorityColor = ansiDim
		}
		fmt.Fprintf(r.stdout, "%s  %s\n", colorize(fmt.Sprintf("[%s]", item.label), priorityColor, r.stdout), item.detail)
		if item.command != "" {
			fmt.Fprintf(r.stdout, "         → %s\n", item.command)
		}
		fmt.Fprintln(r.stdout, "")
	}
	fmt.Fprintf(r.stdout, "%d action item(s) total\n", len(items))
	return nil
}

// autoStopCompletedSessions stops any Idle sessions whose bound task is Done/Archived
// in the DB, or has signalled task.completed in log.md and all verify checks pass.
// Returns the updated slice with those sessions showing status=Stopped. Runs during
// aom status so that background processes are cleaned up the first time the operator
// checks status after work is done — without needing explicit action.
func (r Runner) autoStopCompletedSessions(result *project.OpenResult, sessions []session.Record) []session.Record {
	// Build task status map once to avoid N+1 DB lookups in the loop.
	taskStatusByID := map[string]string{}
	if taskSvc, sqlDB, err := r.app.OpenTaskService(result.DBPath); err == nil {
		defer sqlDB.Close()
		if tasks, err := taskSvc.ListByProject(result.Project.ID); err == nil {
			for _, t := range tasks {
				taskStatusByID[t.ID] = t.Status
			}
		}
	}

	for i, s := range sessions {
		if s.Status != "Idle" || s.TmuxPane == "" || s.TaskID == "" {
			continue
		}
		if s.Persistent {
			continue // persistent sessions are never auto-stopped
		}
		// Workspace agents (permanent per-agent worktrees) persist across tasks.
		// Auto-stopping them on task.completed breaks the step-5 "watch inbox"
		// loop and leaves the team waiting for a response that never comes.
		// They should stay alive and receive the next task via mailbox/channel.
		if strings.TrimSpace(s.WorktreePath) != "" {
			continue
		}
		if alive, _ := r.app.Tmux.PaneExists(s.TmuxPane); !alive {
			continue
		}

		// Fast path: task already accepted/archived in DB — stop without verify checks
		// (verify already passed before accept could complete).
		taskStatus := taskStatusByID[s.TaskID]
		if taskStatus == "Done" || taskStatus == "Archived" {
			stopped, _, _ := r.stopSessionRecord(result, s, false)
			if stopped != nil {
				sessions[i] = *stopped
				fmt.Fprintf(r.stdout, "ℹ  auto-stopped %s (%s): task %s — background processes cleaned up\n", s.ID, s.AgentName, taskStatus)
			}
			continue
		}

		// Slow path: task not yet accepted — check for task.completed signal in log.md
		// and require all verify checks to pass before stopping.
		if s.WorktreePath == "" {
			continue
		}
		logPath := filepath.Join(s.WorktreePath, ".agent", "log.md")
		if !hasTaskCompletedEvent(logPath) {
			continue
		}
		// Run completion checks before auto-stopping — prevents killing sessions
		// where task.completed was signalled prematurely (agent wrote the event but
		// forgot to commit, fill handoff.md, etc.).
		view, viewErr := r.loadTaskView(result, s.TaskID)
		if viewErr == nil && view != nil {
			checks := r.runTaskVerifyChecks(result, view)
			allOK := true
			for _, c := range checks {
				if !c.ok {
					allOK = false
					break
				}
			}
			if !allOK {
				continue
			}
		}
		stopped, _, _ := r.stopSessionRecord(result, s, false)
		if stopped != nil {
			sessions[i] = *stopped
			fmt.Fprintf(r.stdout, "ℹ  auto-stopped %s (%s): task.completed — background processes cleaned up\n", s.ID, s.AgentName)
		}
	}
	return sessions
}

// statusJSONActionItem is one concrete action item serialised into `aom status --json`.
type statusJSONActionItem struct {
	Priority int    `json:"priority"` // 1=urgent, 2=normal, 3=info
	Label    string `json:"label"`    // APPROVAL | ACCEPT | SPAWN | BLOCKED
	Detail   string `json:"detail"`
	Command  string `json:"command,omitempty"`
}

// statusJSONOutput is the JSON structure for `aom status --json`.
type statusJSONOutput struct {
	Project     statusJSONProject      `json:"project"`
	Agents      []statusJSONAgent      `json:"agents"`
	Sessions    []statusJSONSession    `json:"sessions"`
	Tasks       []statusJSONTask       `json:"tasks"`
	Counts      statusJSONCounts       `json:"counts"`
	ActionItems []statusJSONActionItem `json:"action_items"`
}

type statusJSONProject struct {
	Name          string `json:"name"`
	Repo          string `json:"repo"`
	DefaultBranch string `json:"defaultBranch"`
	DBPath        string `json:"dbPath"`
}

type statusJSONAgent struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Runtime string `json:"runtime"`
	Model   string `json:"model"`
	Enabled bool   `json:"enabled"`
}

type statusJSONSession struct {
	ID        string `json:"id"`
	Agent     string `json:"agent"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	Readiness string `json:"readiness"`
	TaskID    string `json:"taskId"`
	Runtime   string `json:"runtime"`
}

type statusJSONTask struct {
	ID     string           `json:"id"`
	Title  string           `json:"title"`
	Status string           `json:"status"`
	Mode   string           `json:"mode"`
	Steps  []statusJSONStep `json:"steps"`
}

type statusJSONStep struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type statusJSONCounts struct {
	Tasks    int `json:"tasks"`
	Sessions int `json:"sessions"`
}

func (r Runner) printStatusJSON(result *project.OpenResult, sessions []session.Record, taskCount int, taskViews []taskView) error {
	out := statusJSONOutput{
		Project: statusJSONProject{
			Name:          result.Project.Name,
			Repo:          result.Project.RepoPath,
			DefaultBranch: result.Project.DefaultBranch,
			DBPath:        result.DBPath,
		},
		Counts: statusJSONCounts{
			Tasks:    taskCount,
			Sessions: len(sessions),
		},
	}

	// Agents.
	out.Agents = make([]statusJSONAgent, 0, len(result.Agents))
	for _, a := range result.Agents {
		out.Agents = append(out.Agents, statusJSONAgent{
			Name:    a.Name,
			Role:    a.Role,
			Runtime: a.Runtime,
			Model:   a.Model,
			Enabled: a.Enabled,
		})
	}

	// Build task status map for accurate readiness labels.
	taskStatusMap := map[string]string{}
	for _, tv := range taskViews {
		taskStatusMap[tv.Task.ID] = tv.Task.Status
	}

	// Sessions.
	out.Sessions = make([]statusJSONSession, 0, len(sessions))
	for _, s := range sessions {
		out.Sessions = append(out.Sessions, statusJSONSession{
			ID:        s.ID,
			Agent:     s.AgentName,
			Role:      s.RoleName,
			Status:    s.Status,
			Readiness: sessionReadiness(result.Project.RepoPath, s, r.app.Tmux.CountDescendants(s.TmuxPane), taskStatusMap[s.TaskID]),
			TaskID:    s.TaskID,
			Runtime:   s.Runtime,
		})
	}

	// Tasks.
	out.Tasks = make([]statusJSONTask, 0, len(taskViews))
	for _, tv := range taskViews {
		steps := make([]statusJSONStep, 0, len(tv.Steps))
		for _, st := range tv.Steps {
			steps = append(steps, statusJSONStep{
				ID:     st.ID,
				Title:  st.Title,
				Status: st.Status,
			})
		}
		out.Tasks = append(out.Tasks, statusJSONTask{
			ID:     tv.Task.ID,
			Title:  tv.Task.Title,
			Status: tv.Task.Status,
			Mode:   tv.Task.Mode,
			Steps:  steps,
		})
	}

	// Action items — reuse buildActionItems so the orchestrator agent can
	// parse the same decision data that the human operator sees.
	rawItems := r.buildActionItems(result, sessions, taskViews)
	out.ActionItems = make([]statusJSONActionItem, 0, len(rawItems))
	for _, item := range rawItems {
		out.ActionItems = append(out.ActionItems, statusJSONActionItem{
			Priority: item.priority,
			Label:    item.label,
			Detail:   item.detail,
			Command:  item.command,
		})
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal status JSON: %w", err)
	}
	fmt.Fprintln(r.stdout, string(data))
	return nil
}

var activeTaskStatuses = map[string]bool{
	"InProgress": true, "Blocked": true, "NeedsAttention": true, "Ready": true,
}

func filterActiveTaskViews(views []taskView) []taskView {
	var out []taskView
	for _, v := range views {
		if activeTaskStatuses[v.Task.Status] {
			out = append(out, v)
		}
	}
	return out
}

func filterActiveSessions(sessions []session.Record) []session.Record {
	var out []session.Record
	for _, s := range sessions {
		if s.Status != "Archived" && s.Status != "Terminated" {
			out = append(out, s)
		}
	}
	return out
}

// printTaskGraph prints an ASCII dependency graph of tasks for a project.
func printTaskGraph(out io.Writer, taskService *task.Service, projectID string) error {
	allTasks, err := taskService.ListByProject(projectID)
	if err != nil {
		return err
	}

	if len(allTasks) == 0 {
		fmt.Fprintln(out, "No tasks to display.")
		return nil
	}

	// Build map by task ID for quick lookup.
	taskByID := make(map[string]task.Record, len(allTasks))
	for _, t := range allTasks {
		taskByID[t.ID] = t
	}

	// Build adjacency: blocker → list of tasks it blocks (blocker→dependent edges).
	// Also build reverse: task → its blockers.
	blockedBy := make(map[string][]string) // taskID → blocker IDs
	blocks := make(map[string][]string)    // blockerID → dependent IDs

	for _, t := range allTasks {
		blockers, err := taskService.BlockedBy(t.ID)
		if err != nil {
			continue
		}
		for _, b := range blockers {
			blockedBy[t.ID] = append(blockedBy[t.ID], b.ID)
			blocks[b.ID] = append(blocks[b.ID], t.ID)
		}
	}

	// Topological sort using Kahn's algorithm.
	inDegree := make(map[string]int, len(allTasks))
	for _, t := range allTasks {
		inDegree[t.ID] = len(blockedBy[t.ID])
	}

	var queue []string
	for _, t := range allTasks {
		if inDegree[t.ID] == 0 {
			queue = append(queue, t.ID)
		}
	}

	var topoOrder []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		topoOrder = append(topoOrder, cur)
		for _, dep := range blocks[cur] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}
	// Add any remaining tasks not in topoOrder (cycles or disconnected).
	inTopo := make(map[string]bool, len(topoOrder))
	for _, id := range topoOrder {
		inTopo[id] = true
	}
	for _, t := range allTasks {
		if !inTopo[t.ID] {
			topoOrder = append(topoOrder, t.ID)
		}
	}

	// Build linear chains: start from tasks with no blockers, follow blocks edges.
	visited := make(map[string]bool)
	var chains [][]string
	for _, id := range topoOrder {
		if visited[id] {
			continue
		}
		if len(blockedBy[id]) > 0 {
			continue // not a chain start
		}
		// Walk forward.
		chain := []string{id}
		visited[id] = true
		cur := id
		for {
			deps := blocks[cur]
			if len(deps) == 0 {
				break
			}
			// Pick first unvisited dependent.
			next := ""
			for _, d := range deps {
				if !visited[d] {
					next = d
					break
				}
			}
			if next == "" {
				break
			}
			chain = append(chain, next)
			visited[next] = true
			cur = next
		}
		chains = append(chains, chain)
	}
	// Collect any tasks not reached by chain walking.
	var loose []string
	for _, id := range topoOrder {
		if !visited[id] {
			loose = append(loose, id)
			visited[id] = true
		}
	}
	if len(loose) > 0 {
		chains = append(chains, loose)
	}

	// Status symbol helper.
	statusSymbol := func(status string) string {
		switch status {
		case "Done":
			return "✓"
		case "InProgress":
			return "⟳"
		case "Planned", "Ready":
			return "○"
		case "NeedsAttention", "Blocked":
			return "!"
		default:
			return "·"
		}
	}

	// Format a node label: [SYMBOL ID: title_truncated_to_20]
	formatNode := func(t task.Record) string {
		title := t.Title
		if len(title) > 20 {
			title = title[:20]
		}
		return fmt.Sprintf("[%s %s: %s]", statusSymbol(t.Status), t.ID, title)
	}

	fmt.Fprintln(out, "Task Dependency Graph")
	fmt.Fprintln(out, "=====================")
	fmt.Fprintln(out, "")

	for _, chain := range chains {
		if len(chain) == 0 {
			continue
		}
		// Build node strings and status strings.
		var nodes []string
		var statuses []string
		for _, id := range chain {
			t, ok := taskByID[id]
			if !ok {
				continue
			}
			nodes = append(nodes, formatNode(t))
			statuses = append(statuses, t.Status)
		}

		// Print chain line with arrows between nodes.
		fmt.Fprintln(out, strings.Join(nodes, " → "))

		// Print status line padded to match node widths.
		var statusParts []string
		for i, nodeStr := range nodes {
			nodeWidth := len(nodeStr)
			st := statuses[i]
			if len(st) > nodeWidth {
				st = st[:nodeWidth]
			}
			// Pad status to node width.
			statusParts = append(statusParts, fmt.Sprintf("%-*s", nodeWidth, st))
		}
		fmt.Fprintln(out, strings.Join(statusParts, "   "))
		fmt.Fprintln(out, "")
	}

	return nil
}

