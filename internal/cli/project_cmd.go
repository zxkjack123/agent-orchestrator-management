package cli

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/artifact"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/plan"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/project"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/session"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/task"
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

func (r Runner) executeStatus(args []string) error {
	activeOnly := false
	graphMode := false
	for _, arg := range args {
		switch arg {
		case "--active":
			activeOnly = true
		case "--graph":
			graphMode = true
		default:
			return fmt.Errorf("unknown flag %q", arg)
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
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

	r.printProjectSummary("Project status", result, nil, sessions, taskCount, taskViews)
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

