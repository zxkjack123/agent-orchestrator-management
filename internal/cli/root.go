package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/agent"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/app"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/artifact"
	aommerge "github.com/lattapon-aek/Agents-Orchestfator-Management/internal/merge"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/plan"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/project"
	aomruntime "github.com/lattapon-aek/Agents-Orchestfator-Management/internal/runtime"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/session"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/step"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/task"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/tmux"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/worktree"
	"github.com/mattn/go-isatty"
)

var newApp = app.New
var newLaunchBuilder = aomruntime.NewBuilder

// Runner executes top-level CLI behavior.
type Runner struct {
	app    *app.App
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	isTTY  func(io.Reader) bool
}

// Execute runs the AOM CLI using the provided arguments and streams.
func Execute(args []string, stdout, stderr io.Writer) error {
	r := Runner{
		app:    newApp(),
		stdin:  os.Stdin,
		stdout: stdout,
		stderr: stderr,
		isTTY:  isTTYReader,
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
	case "approve":
		return r.executeApprove(args[1:])
	case "broadcast":
		return r.executeBroadcast(args[1:])
	case "capture":
		return r.executeCapture(args[1:])
	case "channel":
		return r.executeChannel(args[1:])
	case "checkpoint":
		return r.executeCheckpoint(args[1:])
	case "deny":
		return r.executeDeny(args[1:])
	case "doctor":
		return r.executeDoctor(args[1:])
	case "handoff":
		return r.executeHandoff(args[1:])
	case "open":
		return r.executeOpen(args[1:])
	case "plan":
		return r.executePlan(args[1:])
	case "review":
		return r.executeReview(args[1:])
	case "runtime":
		return r.executeRuntime(args[1:])
	case "step":
		return r.executeStep(args[1:])
	case "session":
		return r.executeSession(args[1:])
	case "status":
		return r.executeStatus(args[1:])
	case "merge":
		return r.executeMerge(args[1:])
	case "metrics":
		return r.executeMetrics(args[1:])
	case "message":
		return r.executeMessage(args[1:])
	case "pause-all":
		return r.executePauseAll(args[1:])
	case "resume-all":
		return r.executeResumeAll(args[1:])
	case "next":
		return r.executeNext(args[1:])
	case "team":
		return r.executeTeam(args[1:])
	case "task":
		return r.executeTask(args[1:])
	case "watch":
		return r.executeWatch(args[1:])
	case "worktree":
		return r.executeWorktree(args[1:])
	case "project":
		return r.executeProject(args[1:])
	default:
		return fmt.Errorf("unknown command %q", strings.Join(args, " "))
	}
}

func (r Runner) executeTask(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task subcommand is required")
	}

	switch args[0] {
	case "create":
		return r.executeTaskCreate(args[1:])
	case "update":
		return r.executeTaskUpdate(args[1:])
	case "close":
		return r.executeTaskClose(args[1:])
	case "show":
		return r.executeTaskShow(args[1:])
	case "reanalyze":
		return r.executeTaskReanalyze(args[1:])
	case "link":
		return r.executeTaskLink(args[1:])
	case "unlink":
		return r.executeTaskUnlink(args[1:])
	case "record-result":
		return r.executeTaskRecordResult(args[1:])
	case "request":
		return r.executeTaskRequest(args[1:])
	case "list-requests":
		return r.executeTaskListRequests(args[1:])
	case "approve-request":
		return r.executeTaskApproveRequest(args[1:])
	case "reject-request":
		return r.executeTaskRejectRequest(args[1:])
	default:
		return fmt.Errorf("unknown task command %q", strings.Join(args, " "))
	}
}

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

func (r Runner) executeStep(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("step subcommand is required")
	}

	switch args[0] {
	case "list":
		return r.executeStepList(args[1:])
	case "update":
		return r.executeStepUpdate(args[1:])
	default:
		return fmt.Errorf("unknown step command %q", strings.Join(args, " "))
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
	case "send":
		return r.executeSessionSend(args[1:])
	case "show":
		return r.executeSessionShow(args[1:])
	case "replace":
		return r.executeSessionReplace(args[1:])
	case "stop":
		return r.executeSessionStop(args[1:])
	case "archive":
		return r.executeSessionArchive(args[1:])
	case "resume":
		return r.executeSessionResume(args[1:])
	case "rebind":
		return r.executeSessionRebind(args[1:])
	case "set-agent-id":
		return r.executeSessionSetAgentID(args[1:])
	case "wait":
		return r.executeSessionWait(args[1:])
	case "health":
		return r.executeSessionHealth(args[1:])
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
	case "resources":
		return r.executeProjectResources(args[1:])
	default:
		return fmt.Errorf("unknown project command %q", strings.Join(args, " "))
	}
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

func (r Runner) executeWorktree(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("worktree subcommand is required")
	}

	switch args[0] {
	case "repair":
		return r.executeWorktreeRepair(args[1:])
	case "read-file":
		return r.executeWorktreeReadFile(args[1:])
	default:
		return fmt.Errorf("unknown worktree command %q", strings.Join(args, " "))
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

func (r Runner) executeWorktreeRepair(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("worktree repair does not accept extra positional arguments in the current milestone")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskRecord, err := r.loadTaskByID(result, strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if taskRecord == nil {
		return fmt.Errorf("task %q not found", strings.TrimSpace(args[0]))
	}

	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer worktreeDB.Close()

	record, err := worktreeService.Repair(taskRecord.ID, result.Project.RepoPath)
	if err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, taskRecord.ID, artifact.Event{
		Type:        "worktree.repaired",
		Actor:       "operator",
		Summary:     "Worktree continuity was explicitly repaired",
		StateEffect: fmt.Sprintf("Worktree %s", record.Status),
	}, false); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Worktree repaired")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", taskRecord.ID)
	fmt.Fprintf(r.stdout, "Status: %s\n", record.Status)
	fmt.Fprintf(r.stdout, "Branch: %s\n", record.BranchName)
	fmt.Fprintf(r.stdout, "Path: %s\n", record.WorktreePath)

	return nil
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

	taskCount, err := r.loadTaskCount(result)
	if err != nil {
		return err
	}

	taskViews, err := r.loadTaskViews(result, sessions)
	if err != nil {
		return err
	}

	r.printProjectSummary("Project status", result, nil, sessions, taskCount, taskViews)
	return nil
}

func (r Runner) executeTaskCreate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task title is required")
	}

	params := taskCreateParams{title: args[0]}
	for i := 1; i < len(args); i++ {
		switch args[i] {
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
		case "--priority":
			i++
			if i >= len(args) {
				return fmt.Errorf("--priority requires a value")
			}
			params.priority = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	if err := r.validateTaskProvisioning(result); err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	priority, err := task.NormalizePriority(params.priority)
	if err != nil {
		return err
	}

	createResult, err := taskService.Create(task.CreateParams{
		ProjectID:      result.Project.ID,
		Title:          params.title,
		Mode:           params.mode,
		Priority:       priority,
		PreferredRole:  params.role,
		PreferredAgent: params.agent,
	})
	if err != nil {
		return err
	}

	if _, err := r.ensurePlannedWorktree(result, createResult.Task); err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, createResult.Task.ID, artifact.Event{
		Type:        "task.created",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task created in %s mode", createResult.Task.Mode),
		StateEffect: fmt.Sprintf("Task %s", createResult.Task.Status),
	}, true); err != nil {
		return err
	}

	recommendedNext := "confirm the initial step and move the task to Ready"
	if createResult.Task.PreferredRole != "" || createResult.Task.PreferredAgent != "" {
		recommendedNext = "confirm the initial step owner and move the task to Ready"
	}

	fmt.Fprintln(r.stdout, "Task created")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", createResult.Task.ID)
	fmt.Fprintf(r.stdout, "Title: %s\n", createResult.Task.Title)
	fmt.Fprintf(r.stdout, "Mode: %s\n", createResult.Task.Mode)
	fmt.Fprintf(r.stdout, "Status: %s\n", createResult.Task.Status)
	fmt.Fprintf(r.stdout, "Initial steps: %d\n", len(createResult.Steps))
	fmt.Fprintf(r.stdout, "Recommended next step: %s\n", recommendedNext)

	return nil
}

type taskCreateParams struct {
	title    string
	mode     string
	role     string
	agent    string
	priority string
}

func (r Runner) executeTaskShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("task show does not accept extra positional arguments in the current milestone")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", strings.TrimSpace(args[0]))
	}

	fmt.Fprintln(r.stdout, "Task")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "ID: %s\n", view.Task.ID)
	fmt.Fprintf(r.stdout, "Title: %s\n", view.Task.Title)
	fmt.Fprintf(r.stdout, "Mode: %s\n", view.Task.Mode)
	fmt.Fprintf(r.stdout, "Status: %s\n", view.Task.Status)
	fmt.Fprintf(r.stdout, "Preferred role: %s\n", emptyFallback(view.Task.PreferredRole))
	fmt.Fprintf(r.stdout, "Preferred agent: %s\n", emptyFallback(view.Task.PreferredAgent))
	if view.Worktree != nil {
		fmt.Fprintf(r.stdout, "Worktree status: %s\n", view.Worktree.Status)
		fmt.Fprintf(r.stdout, "Worktree branch: %s\n", view.Worktree.BranchName)
		fmt.Fprintf(r.stdout, "Worktree path: %s\n", view.Worktree.WorktreePath)
		if hint := worktreeHint(view.Task.ID, view.Worktree, view.WorktreeDrift); hint != "" {
			fmt.Fprintf(r.stdout, "Worktree hint: %s\n", hint)
		}
	}
	fmt.Fprintf(r.stdout, "Artifact root: %s\n", taskArtifactRoot(result.Project.RepoPath, result.StateDir, view.Task.ID, view.Worktree))
	fmt.Fprintf(r.stdout, "Task log: %s\n", taskArtifactLogPath(result.Project.RepoPath, result.StateDir, view.Task.ID, view.Worktree))
	fmt.Fprintf(r.stdout, "Unresolved review items: %d\n", view.UnresolvedReviewItems)
	fmt.Fprintf(r.stdout, "Review owner hint: %s\n", reviewOwnerHintDisplay(view.ReviewOwnerHint, view.ReviewOwnerAmbiguous))
	fmt.Fprintf(r.stdout, "Steps: %d\n", len(view.Steps))
	fmt.Fprintf(r.stdout, "Recommended next action: %s\n", recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous))

	return nil
}

func (r Runner) executeTaskUpdate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}

	params := taskUpdateParams{id: strings.TrimSpace(args[0])}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--mode":
			i++
			if i >= len(args) {
				return fmt.Errorf("--mode requires a value")
			}
			params.mode = args[i]
		case "--status":
			i++
			if i >= len(args) {
				return fmt.Errorf("--status requires a value")
			}
			params.status = args[i]
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
		case "--priority":
			i++
			if i >= len(args) {
				return fmt.Errorf("--priority requires a value")
			}
			params.priority = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	record, err := taskService.Update(params.id, task.UpdateParams{
		Mode:           params.mode,
		Status:         params.status,
		Priority:       params.priority,
		PreferredRole:  params.role,
		PreferredAgent: params.agent,
	})
	if err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, record.ID, artifact.Event{
		Type:        mapTaskEventType(params.status, params.mode),
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task updated to mode=%s status=%s", record.Mode, record.Status),
		StateEffect: fmt.Sprintf("Task %s", record.Status),
	}, false); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Task updated")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", record.ID)
	fmt.Fprintf(r.stdout, "Mode: %s\n", record.Mode)
	fmt.Fprintf(r.stdout, "Status: %s\n", record.Status)
	fmt.Fprintf(r.stdout, "Preferred role: %s\n", emptyFallback(record.PreferredRole))
	fmt.Fprintf(r.stdout, "Preferred agent: %s\n", emptyFallback(record.PreferredAgent))

	return nil
}

type taskUpdateParams struct {
	id       string
	mode     string
	status   string
	role     string
	agent    string
	priority string
}

func (r Runner) executeTaskClose(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("task close does not accept extra positional arguments in the current milestone")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	record, err := taskService.Close(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, record.ID, artifact.Event{
		Type:        "task.closed",
		Actor:       "operator",
		Summary:     "Task closed explicitly by operator",
		StateEffect: fmt.Sprintf("Task %s", record.Status),
	}, false); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Task closed")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", record.ID)
	fmt.Fprintf(r.stdout, "Status: %s\n", record.Status)

	return nil
}

func (r Runner) executeStepList(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("step list does not accept extra positional arguments in the current milestone")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	taskRecord, err := taskService.Get(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if taskRecord == nil {
		return fmt.Errorf("task %q not found", strings.TrimSpace(args[0]))
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	steps, err := stepService.ListByTask(taskRecord.ID)
	if err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Steps")
	fmt.Fprintln(r.stdout, "")
	if len(steps) == 0 {
		fmt.Fprintf(r.stdout, "No steps for %s\n", taskRecord.ID)
		return nil
	}

	for _, item := range steps {
		fmt.Fprintf(
			r.stdout,
			"  - %s | type=%s | title=%s | role=%s | agent=%s | status=%s | dependencies=%s\n",
			item.ID,
			item.StepType,
			item.Title,
			emptyFallback(item.RoleName),
			emptyFallback(item.AgentName),
			item.Status,
			formatDependencies(item.Dependencies),
		)
	}

	return nil
}

func (r Runner) executeStepUpdate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("step identifier is required")
	}

	params := stepUpdateParams{id: strings.TrimSpace(args[0])}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--status":
			i++
			if i >= len(args) {
				return fmt.Errorf("--status requires a value")
			}
			params.status = args[i]
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

	stepService, sqlDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	record, err := stepService.Update(params.id, step.UpdateParams{
		Status:    params.status,
		RoleName:  params.role,
		AgentName: params.agent,
	})
	if err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, record.TaskID, artifact.Event{
		Type:        mapStepEventType(record.Status),
		Actor:       "operator",
		StepID:      record.ID,
		Summary:     fmt.Sprintf("Step updated to %s", record.Status),
		StateEffect: fmt.Sprintf("Step %s", record.Status),
	}, false); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Step updated")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Step: %s\n", record.ID)
	fmt.Fprintf(r.stdout, "Status: %s\n", record.Status)
	fmt.Fprintf(r.stdout, "Role: %s\n", emptyFallback(record.RoleName))
	fmt.Fprintf(r.stdout, "Agent: %s\n", emptyFallback(record.AgentName))

	return nil
}

type stepUpdateParams struct {
	id     string
	status string
	role   string
	agent  string
}

func (r Runner) executeSessionSpawn(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent name is required")
	}

	params := sessionSpawnParams{
		agentName:  strings.TrimSpace(args[0]),
		launchMode: aomruntime.LaunchModePlaceholder,
	}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--task":
			i++
			if i >= len(args) {
				return fmt.Errorf("--task requires a value")
			}
			params.taskID = strings.TrimSpace(args[i])
		case "--step":
			i++
			if i >= len(args) {
				return fmt.Errorf("--step requires a value")
			}
			params.stepID = strings.TrimSpace(args[i])
		case "--mock":
			if err := setLaunchMode(&params.launchMode, aomruntime.LaunchModeMock); err != nil {
				return err
			}
		case "--real":
			if err := setLaunchMode(&params.launchMode, aomruntime.LaunchModeReal); err != nil {
				return err
			}
		case "--fresh":
			params.freshStart = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	agentRecord, err := findAgent(result.Agents, params.agentName)
	if err != nil {
		return err
	}
	_, err = r.executeResolvedSessionSpawn(result, agentRecord, params)
	return err
}

func (r Runner) executeResolvedSessionSpawn(result *project.OpenResult, agentRecord *agent.Record, params sessionSpawnParams) (*session.Record, error) {
	var err error
	var taskRecord *task.Record
	if params.taskID != "" {
		taskRecord, err = r.loadTaskByID(result, params.taskID)
		if err != nil {
			return nil, err
		}
		if taskRecord == nil {
			return nil, fmt.Errorf("task %q not found", params.taskID)
		}
	}

	executionPath := result.Project.RepoPath
	var taskWorktree *worktree.Record
	if taskRecord != nil {
		taskWorktree, executionPath, err = r.resolveTaskExecutionPath(result, *taskRecord)
		if err != nil {
			return nil, err
		}
	}

	var stepRecord *step.Record
	if params.stepID != "" {
		if taskRecord == nil {
			return nil, fmt.Errorf("--step requires --task in the current milestone")
		}
		stepRecord, err = r.loadStepByID(result, params.stepID)
		if err != nil {
			return nil, err
		}
		if stepRecord == nil {
			return nil, fmt.Errorf("step %q not found", params.stepID)
		}
		if stepRecord.TaskID != taskRecord.ID {
			return nil, fmt.Errorf("step %q does not belong to task %q", stepRecord.ID, taskRecord.ID)
		}
	}

	if err := r.enforceWriterWorktreeBoundary(result, agentRecord, params); err != nil {
		return nil, err
	}

	if _, err := newLaunchBuilder().Build(aomruntime.SessionSpec{
		SessionID: "",
		AgentName: agentRecord.Name,
		RoleName:  agentRecord.Role,
		Runtime:   agentRecord.Runtime,
	}, params.launchMode); err != nil {
		return nil, err
	}

	workspace, err := r.app.Tmux.EnsureWorkspace(result.SessionPrefix, result.Project.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("ensure tmux workspace: %w", err)
	}

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	record, err := sessionService.Create(session.CreateParams{
		ProjectID:       result.Project.ID,
		AgentID:         agentRecord.ID,
		AgentName:       agentRecord.Name,
		RoleName:        agentRecord.Role,
		TaskID:          params.taskID,
		Runtime:         agentRecord.Runtime,
		Status:          "Booting",
		RepoPath:        result.Project.RepoPath,
		WorktreePath:    executionPath,
		TmuxSessionName: workspace.Name,
	})
	if err != nil {
		return nil, err
	}

	if taskRecord != nil {
		if err := r.syncTaskArtifactsWithSessionEvents(result, taskRecord.ID, false, record, artifact.Event{
			Type:        "session.created",
			Actor:       "aom",
			StepID:      params.stepID,
			SessionID:   record.ID,
			Summary:     fmt.Sprintf("Session record created for %s using %s launch mode", agentRecord.Name, params.launchMode),
			StateEffect: "Session Booting",
		}); err != nil {
			return nil, err
		}
	}

	agentSessionID := ""
	if params.taskID != "" && !params.freshStart {
		agentSessionID, _ = sessionService.LatestVendorSessionID(params.taskID, agentRecord.Name)
	}

	launchCommand, err := newLaunchBuilder().Build(aomruntime.SessionSpec{
		SessionID:      record.ID,
		AgentName:      record.AgentName,
		RoleName:       record.RoleName,
		Runtime:        record.Runtime,
		AgentSessionID: agentSessionID,
	}, params.launchMode)
	if err != nil {
		return nil, r.failTaskBoundSessionSpawn(result, sessionService, record, taskRecord, params.stepID, "session launch validation failed before session became interactive", err)
	}

	if err := materializeAgentContext(result, agentRecord, executionPath); err != nil {
		return nil, fmt.Errorf("materialize agent context: %w", err)
	}

	r.enforcePolicyDefaults(result)

	paneBinding, err := r.app.Tmux.CreatePane(workspace.Target, executionPath, launchCommand)
	if err != nil {
		return nil, r.failTaskBoundSessionSpawn(result, sessionService, record, taskRecord, params.stepID, "pane creation failed before session became interactive", err)
	}

	record.Status = "Idle"
	record.TmuxWindow = paneBinding.WindowID
	record.TmuxPane = paneBinding.PaneID
	record.TmuxSessionName = workspace.Name

	// Label the window with the agent name so operators can identify sessions at a glance.
	_ = r.app.Tmux.RenameWindow(paneBinding.WindowID, agentRecord.Name)

	record, err = sessionService.Save(*record)
	if err != nil {
		return nil, err
	}

	if taskRecord != nil {
		worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
		if err != nil {
			return nil, err
		}
		taskWorktree, err = worktreeService.Reconcile(taskRecord.ID, result.Project.RepoPath, true)
		worktreeDB.Close()
		if err != nil {
			return nil, err
		}
	}

	if taskRecord != nil {
		if err := r.syncTaskArtifactsWithSessionEvents(result, taskRecord.ID, false, record, artifact.Event{
			Type:        "session.ready",
			Actor:       "aom",
			StepID:      params.stepID,
			SessionID:   record.ID,
			Summary:     fmt.Sprintf("Session pane attached for %s and ready for operator inspection", agentRecord.Name),
			StateEffect: fmt.Sprintf("Session %s", record.Status),
		}); err != nil {
			return nil, err
		}
		if err := r.ensureTaskHandoffTemplate(result, taskRecord.ID, record); err != nil {
			return nil, err
		}
	}

	if err := r.app.Tmux.AnnotatePane(record.TmuxPane, map[string]string{
		"@aom_session_id": record.ID,
		"@aom_agent":      record.AgentName,
		"@aom_role":       record.RoleName,
	}); err != nil {
		return nil, r.failTaskBoundSessionSpawn(result, sessionService, record, taskRecord, params.stepID, "pane annotation failed after session launch", err)
	}

	fmt.Fprintln(r.stdout, "Session spawned")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Session: %s\n", record.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", record.AgentName)
	fmt.Fprintf(r.stdout, "Role: %s\n", record.RoleName)
	if taskRecord != nil {
		fmt.Fprintf(r.stdout, "Task: %s\n", taskRecord.ID)
	}
	if stepRecord != nil {
		fmt.Fprintf(r.stdout, "Step: %s\n", stepRecord.ID)
	}
	fmt.Fprintf(r.stdout, "Runtime: %s\n", record.Runtime)
	fmt.Fprintf(r.stdout, "Launch mode: %s\n", params.launchMode)
	if taskWorktree != nil {
		fmt.Fprintf(r.stdout, "Worktree status: %s\n", taskWorktree.Status)
	}
	fmt.Fprintf(r.stdout, "Worktree path: %s\n", record.WorktreePath)
	fmt.Fprintf(r.stdout, "Workspace: %s\n", workspace.Target)
	fmt.Fprintf(r.stdout, "Window: %s\n", record.TmuxWindow)
	fmt.Fprintf(r.stdout, "Pane: %s\n", record.TmuxPane)

	if params.taskID != "" {
		fmt.Fprintln(r.stdout, "")
		if agentSessionID != "" {
			fmt.Fprintf(r.stdout, "Continuity: resuming previous session %s\n", agentSessionID)
			fmt.Fprintln(r.stdout, "            Use --fresh to start a clean context if this is unrelated work.")
		} else if params.freshStart {
			fmt.Fprintln(r.stdout, "Continuity: fresh start (--fresh flag set — previous context ignored)")
		} else {
			fmt.Fprintln(r.stdout, "Continuity: fresh start (no previous session found for this task)")
		}

		artifactRoot := taskArtifactRoot(result.Project.RepoPath, result.StateDir, params.taskID, taskWorktree)
		taskMDPath := filepath.Join(artifactRoot, "task.md")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "Context: %s\n", taskMDPath)
		fmt.Fprintln(r.stdout, "         Read this file before starting work on the task.")
		if params.launchMode == aomruntime.LaunchModeReal {
			fmt.Fprintf(r.stdout, "         To send it: aom session send %s \"@%s\"\n", record.ID, taskMDPath)
		}
	}

	if params.launchMode == aomruntime.LaunchModeReal && record.Runtime == "claude" {
		if agentSessionID != "" {
			fmt.Fprintf(r.stdout, "Native session ID: %s (resumed)\n", agentSessionID)
		} else {
			spawnedAt := time.Now()
			fmt.Fprintln(r.stdout, "")
			fmt.Fprintln(r.stdout, "Detecting native session ID (this may take up to 45s)...")
			time.Sleep(3 * time.Second)
			_ = r.app.Tmux.SendKeys(record.TmuxPane, "2")
			if sid, _ := claudeSessionForWorktree(record.WorktreePath, spawnedAt, 45*time.Second); sid != "" {
				if updated, err := sessionService.SetVendorSessionID(record.ID, sid); err == nil {
					record = updated
				}
				fmt.Fprintf(r.stdout, "Native session ID: %s (auto-detected)\n", sid)
			} else {
				fmt.Fprintln(r.stdout, "Native session ID not yet available")
				fmt.Fprintf(r.stdout, "To register manually: aom session set-agent-id %s <uuid>\n", record.ID)
			}
		}
	}

	return record, nil
}

func (r Runner) executeSessionReplace(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	params := sessionReplaceParams{
		id:         strings.TrimSpace(args[0]),
		launchMode: aomruntime.LaunchModePlaceholder,
	}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			params.agentName = strings.TrimSpace(args[i])
		case "--reason":
			i++
			if i >= len(args) {
				return fmt.Errorf("--reason requires a value")
			}
			params.reason = strings.TrimSpace(args[i])
		case "--mock":
			if err := setLaunchMode(&params.launchMode, aomruntime.LaunchModeMock); err != nil {
				return err
			}
		case "--real":
			if err := setLaunchMode(&params.launchMode, aomruntime.LaunchModeReal); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if params.agentName == "" {
		return fmt.Errorf("--agent is required")
	}

	oldRecord, err := r.loadSessionByIdentifier(params.id)
	if err != nil {
		return err
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	agentRecord, err := findAgent(result.Agents, params.agentName)
	if err != nil {
		return err
	}

	if err := r.replaceSession(result, *oldRecord, agentRecord, params); err != nil {
		return err
	}
	return nil
}

func (r Runner) failTaskBoundSessionSpawn(
	result *project.OpenResult,
	sessionService *session.Service,
	record *session.Record,
	taskRecord *task.Record,
	stepID string,
	summary string,
	cause error,
) error {
	if record == nil {
		return cause
	}

	record.Status = "Failed"
	if _, err := sessionService.Save(*record); err != nil {
		return fmt.Errorf("%w (also failed to persist failed session state: %v)", cause, err)
	}

	if taskRecord != nil {
		appendErr := r.syncTaskArtifactsWithSessionEvents(result, taskRecord.ID, false, record, artifact.Event{
			Type:        "session.failed",
			Actor:       "aom",
			StepID:      stepID,
			SessionID:   record.ID,
			Summary:     summary,
			StateEffect: "Session Failed",
		})
		if appendErr != nil {
			return fmt.Errorf("%w (also failed to append task log event: %v)", cause, appendErr)
		}
	}

	return cause
}

type sessionSpawnParams struct {
	agentName       string
	taskID          string
	stepID          string
	ignoreSessionID string
	launchMode      aomruntime.LaunchMode
	freshStart      bool
}

type sessionReplaceParams struct {
	id         string
	agentName  string
	reason     string
	launchMode aomruntime.LaunchMode
}

func (r Runner) executeSessionList(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("session list does not accept positional arguments in the current milestone")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	sessions, err := r.loadProjectSessions(result)
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
			"  - %s | agent=%s | role=%s | task=%s | runtime=%s | status=%s | tmux=%s %s %s\n",
			item.ID,
			item.AgentName,
			item.RoleName,
			emptyFallback(item.TaskID),
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
	fmt.Fprintf(r.stdout, "Task: %s\n", emptyFallback(sessionRecord.TaskID))
	fmt.Fprintf(r.stdout, "Runtime: %s\n", sessionRecord.Runtime)
	fmt.Fprintf(r.stdout, "Status: %s\n", sessionRecord.Status)
	fmt.Fprintf(r.stdout, "Repo: %s\n", sessionRecord.RepoPath)
	fmt.Fprintf(r.stdout, "Worktree: %s\n", sessionRecord.WorktreePath)
	fmt.Fprintf(r.stdout, "Tmux session: %s\n", sessionRecord.TmuxSessionName)
	fmt.Fprintf(r.stdout, "Tmux window: %s\n", sessionRecord.TmuxWindow)
	fmt.Fprintf(r.stdout, "Tmux pane: %s\n", sessionRecord.TmuxPane)
	if sessionRecord.VendorSessionID != "" {
		fmt.Fprintf(r.stdout, "Native session ID: %s\n", sessionRecord.VendorSessionID)
	}

	return nil
}

func (r Runner) replaceSession(result *project.OpenResult, oldRecord session.Record, agentRecord *agent.Record, params sessionReplaceParams) error {
	if agentRecord == nil {
		return fmt.Errorf("replacement agent is required")
	}

	spawnParams := sessionSpawnParams{
		agentName:       agentRecord.Name,
		taskID:          oldRecord.TaskID,
		ignoreSessionID: oldRecord.ID,
		launchMode:      params.launchMode,
	}

	taskRecord, err := r.loadTaskByID(result, oldRecord.TaskID)
	if err != nil {
		return err
	}
	if oldRecord.TaskID != "" && taskRecord == nil {
		return fmt.Errorf("task %q not found", oldRecord.TaskID)
	}

	oldStatusBefore := oldRecord.Status
	newSession, err := r.executeResolvedSessionSpawn(result, agentRecord, spawnParams)
	if err != nil {
		return err
	}

	stoppedStatus := oldRecord.Status
	oldSessionOutcome := fmt.Sprintf("left running (%s requires operator intervention)", oldRecord.Status)
	oldSessionHint := ""
	stopWarning := ""
	if archivableReplacementSession(oldRecord.Status) {
		stopped, warning, err := r.stopSessionRecord(result, oldRecord, false)
		if err != nil {
			return err
		}
		archived, err := r.archiveSessionRecord(result, *stopped, false, "Superseded session archived automatically after replacement")
		if err != nil {
			return err
		}
		stoppedStatus = archived.Status
		stopWarning = warning
		oldSessionOutcome = fmt.Sprintf("archived (%s)", archived.Status)
		if warning != "" {
			oldSessionOutcome = fmt.Sprintf("archived with warning (%s)", archived.Status)
		}
	} else if stoppableReplacementSession(oldRecord.Status) {
		stopped, warning, err := r.stopSessionRecord(result, oldRecord, false)
		if err != nil {
			return err
		}
		stoppedStatus = stopped.Status
		stopWarning = warning
		oldSessionOutcome = fmt.Sprintf("stopped (%s)", stopped.Status)
		if warning != "" {
			oldSessionOutcome = fmt.Sprintf("stopped with warning (%s)", stopped.Status)
		}
	} else {
		oldSessionHint = fmt.Sprintf("run \"aom session stop %s\" after inspecting whether the previous session should be shut down", oldRecord.ID)
	}

	if strings.TrimSpace(oldRecord.TaskID) != "" {
		reason := params.reason
		if reason == "" {
			reason = "operator requested replacement"
		}
		if err := r.syncTaskArtifactsWithSessionEvents(result, oldRecord.TaskID, false, newSession, artifact.Event{
			Type:        "session.replaced",
			Actor:       "operator",
			SessionID:   newSession.ID,
			Summary:     fmt.Sprintf("Session %s replaced %s using agent %s (%s)", newSession.ID, oldRecord.ID, agentRecord.Name, reason),
			StateEffect: fmt.Sprintf("Old session %s, new session %s", stoppedStatus, newSession.Status),
		}); err != nil {
			return err
		}
	}

	fmt.Fprintln(r.stdout, "Session replaced")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Old session: %s\n", oldRecord.ID)
	fmt.Fprintf(r.stdout, "Old status: %s\n", oldStatusBefore)
	fmt.Fprintf(r.stdout, "Old session result: %s\n", oldSessionOutcome)
	fmt.Fprintf(r.stdout, "New session: %s\n", newSession.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", newSession.AgentName)
	fmt.Fprintf(r.stdout, "Reason: %s\n", emptyFallback(params.reason))
	if stopWarning != "" {
		fmt.Fprintf(r.stdout, "Warning: %s\n", stopWarning)
	}
	if oldSessionHint != "" {
		fmt.Fprintf(r.stdout, "Action hint: %s\n", oldSessionHint)
	}
	fmt.Fprintln(r.stdout, "Continuity quality: artifact-backed")
	fmt.Fprintln(r.stdout, "Next recommended action: inspect the replacement session and continue work from the same task context")

	return nil
}

func (r Runner) stopSessionRecord(result *project.OpenResult, record session.Record, print bool) (*session.Record, string, error) {
	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return nil, "", err
	}
	defer sqlDB.Close()

	recordRefreshed, err := r.reconcileSessionRecord(sessionService, record)
	if err != nil {
		return nil, "", err
	}
	record = *recordRefreshed

	warning := ""
	if strings.TrimSpace(record.TmuxPane) != "" && record.Status != "Detached" {
		paneExists, err := r.app.Tmux.PaneExists(record.TmuxPane)
		if err != nil {
			return nil, "", err
		}
		if paneExists {
			if err := r.app.Tmux.KillPane(record.TmuxPane); err != nil {
				warning = fmt.Sprintf("tmux pane cleanup failed for %s: %v", record.TmuxPane, err)
			}
		}
	}

	stopped, err := sessionService.Stop(record)
	if err != nil {
		return nil, warning, err
	}

	if strings.TrimSpace(stopped.TaskID) != "" {
		summary := "Session stopped explicitly by operator"
		stateEffect := "Session Stopped"
		if warning != "" {
			summary = fmt.Sprintf("%s (tmux cleanup warning: %s)", summary, warning)
			stateEffect = "Session Stopped with tmux cleanup warning"
		}
		if err := r.syncTaskArtifacts(result, stopped.TaskID, artifact.Event{
			Type:        "session.stopped",
			Actor:       "operator",
			SessionID:   stopped.ID,
			Summary:     summary,
			StateEffect: stateEffect,
		}, false); err != nil {
			return nil, warning, err
		}
	}

	if print {
		fmt.Fprintln(r.stdout, "Session stopped")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "Session: %s\n", stopped.ID)
		fmt.Fprintf(r.stdout, "Status: %s\n", stopped.Status)
		fmt.Fprintf(r.stdout, "Task: %s\n", emptyFallback(stopped.TaskID))
		if warning != "" {
			fmt.Fprintf(r.stdout, "Warning: %s\n", warning)
		}
	}

	return stopped, warning, nil
}

func (r Runner) archiveSessionRecord(result *project.OpenResult, record session.Record, print bool, summary string) (*session.Record, error) {
	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	archived, err := sessionService.Archive(record)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(archived.TaskID) != "" {
		if err := r.syncTaskArtifacts(result, archived.TaskID, artifact.Event{
			Type:        "session.archived",
			Actor:       "operator",
			SessionID:   archived.ID,
			Summary:     summary,
			StateEffect: "Session Archived",
		}, false); err != nil {
			return nil, err
		}
	}

	if print {
		fmt.Fprintln(r.stdout, "Session archived")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "Session: %s\n", archived.ID)
		fmt.Fprintf(r.stdout, "Status: %s\n", archived.Status)
		fmt.Fprintf(r.stdout, "Task: %s\n", emptyFallback(archived.TaskID))
	}

	return archived, nil
}

func stoppableReplacementSession(status string) bool {
	switch status {
	case "Idle", "WaitingHandoff":
		return true
	default:
		return false
	}
}

func archivableReplacementSession(status string) bool {
	return status == "Detached"
}

func (r Runner) executeSessionStop(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("session stop does not accept extra positional arguments in the current milestone")
	}

	record, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}
	_, _, err = r.stopSessionRecord(result, *record, true)
	return err
}

func (r Runner) executeSessionArchive(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("session archive does not accept extra positional arguments in the current milestone")
	}

	record, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	_, err = r.archiveSessionRecord(result, *record, true, "Session archived explicitly by operator")
	return err
}

func (r Runner) executeSessionResume(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	sessionIdentifier := strings.TrimSpace(args[0])
	newTaskID := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--task":
			i++
			if i >= len(args) {
				return fmt.Errorf("--task requires a value")
			}
			newTaskID = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if newTaskID == "" {
		return fmt.Errorf("--task is required")
	}

	record, err := r.loadSessionByIdentifier(sessionIdentifier)
	if err != nil {
		return err
	}
	if record.Status != "WaitingHandoff" && record.Status != "Idle" {
		return fmt.Errorf("session %q cannot be resumed for a new task (status: %s); session must be Idle or WaitingHandoff", record.ID, record.Status)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	newTaskRecord, err := r.loadTaskByID(result, newTaskID)
	if err != nil {
		return err
	}

	agentRecord, err := findAgent(result.Agents, record.AgentName)
	if err != nil {
		return err
	}

	if err := r.enforceWriterWorktreeBoundary(result, agentRecord, sessionSpawnParams{
		taskID:          newTaskID,
		ignoreSessionID: record.ID,
	}); err != nil {
		return err
	}

	newTaskWorktree, newExecutionPath, err := r.resolveTaskExecutionPath(result, *newTaskRecord)
	if err != nil {
		return err
	}

	oldTaskID := record.TaskID

	record.TaskID = newTaskID
	record.WorktreePath = newExecutionPath
	record.Status = "Idle"

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	updated, err := sessionService.Save(*record)
	if err != nil {
		return err
	}

	// Change the pane's working directory to the new task's worktree.
	if strings.TrimSpace(record.TmuxPane) != "" && strings.TrimSpace(newExecutionPath) != "" {
		_ = r.app.Tmux.SendKeys(record.TmuxPane, "cd "+newExecutionPath)
	}

	// Deliver agent context (identity, skills, MCP) into the new worktree.
	if agentRec := findAgentByName(result.Agents, record.AgentName); agentRec != nil {
		_ = materializeAgentContext(result, agentRec, newExecutionPath)
	}

	// Advance new task's worktree to Active.
	if newTaskWorktree != nil {
		worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
		if err == nil {
			_, _ = worktreeService.Reconcile(newTaskID, result.Project.RepoPath, true)
			worktreeDB.Close()
		}
	}

	// Record the session's departure from the old task.
	if strings.TrimSpace(oldTaskID) != "" && oldTaskID != newTaskID {
		_ = r.syncTaskArtifacts(result, oldTaskID, artifact.Event{
			Type:        "operator.intervention",
			Actor:       "operator",
			SessionID:   record.ID,
			Summary:     fmt.Sprintf("Session %s (%s) transferred to task %s", record.ID, record.AgentName, newTaskID),
			StateEffect: "Session transferred out",
		}, false)
	}

	// Bind session to the new task's artifacts.
	if err := r.syncTaskArtifactsWithSessionEvents(result, newTaskID, false, updated, artifact.Event{
		Type:        "session.ready",
		Actor:       "aom",
		SessionID:   updated.ID,
		Summary:     fmt.Sprintf("Session %s (%s) resumed and bound to this task", updated.ID, updated.AgentName),
		StateEffect: fmt.Sprintf("Session %s", updated.Status),
	}); err != nil {
		return err
	}

	_ = r.ensureTaskHandoffTemplate(result, newTaskID, updated)

	fmt.Fprintln(r.stdout, "Session resumed")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Session: %s\n", updated.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", updated.AgentName)
	fmt.Fprintf(r.stdout, "Task: %s\n", updated.TaskID)
	if strings.TrimSpace(oldTaskID) != "" && oldTaskID != newTaskID {
		fmt.Fprintf(r.stdout, "Previous task: %s\n", oldTaskID)
	}
	fmt.Fprintf(r.stdout, "Status: %s\n", updated.Status)
	fmt.Fprintf(r.stdout, "Worktree: %s\n", updated.WorktreePath)
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Next: aom session send %s \"read .agent/task.md and begin\"\n", updated.ID)
	return nil
}

// executeSessionRebind reconnects a Detached session to a live tmux pane
// without replacing the session record or re-spawning the runtime. If the
// existing pane is still alive the session is simply un-detached. If the pane
// is gone a new placeholder pane is created in the project workspace.
func (r Runner) executeSessionRebind(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	record, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if record.Status != "Detached" {
		return fmt.Errorf("session %q is %s; rebind only applies to Detached sessions", record.ID, record.Status)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// If the existing pane is still alive, just un-detach.
	if strings.TrimSpace(record.TmuxPane) != "" {
		if alive, _ := r.app.Tmux.PaneExists(record.TmuxPane); alive {
			sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			record.Status = "Idle"
			if _, err := sessionService.Save(*record); err != nil {
				return err
			}

			fmt.Fprintln(r.stdout, "Session rebound (pane still alive)")
			fmt.Fprintln(r.stdout, "")
			fmt.Fprintf(r.stdout, "Session: %s\n", record.ID)
			fmt.Fprintf(r.stdout, "Pane: %s\n", record.TmuxPane)
			fmt.Fprintf(r.stdout, "Status: Idle\n")
			return nil
		}
	}

	// Pane is gone — create a new one in the project workspace.
	workspace, err := r.app.Tmux.EnsureWorkspace(result.SessionPrefix, result.Project.RepoPath)
	if err != nil {
		return fmt.Errorf("ensure workspace for rebind: %w", err)
	}

	launchCommand, err := newLaunchBuilder().Build(aomruntime.SessionSpec{
		SessionID:      record.ID,
		AgentName:      record.AgentName,
		RoleName:       record.RoleName,
		Runtime:        record.Runtime,
		AgentSessionID: record.VendorSessionID,
	}, aomruntime.LaunchModePlaceholder)
	if err != nil {
		return fmt.Errorf("build launch command for rebind: %w", err)
	}

	executionPath := record.WorktreePath
	if strings.TrimSpace(executionPath) == "" {
		executionPath = result.Project.RepoPath
	}

	paneBinding, err := r.app.Tmux.CreatePane(workspace.Target, executionPath, launchCommand)
	if err != nil {
		return fmt.Errorf("create pane for rebind: %w", err)
	}

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	record.Status = "Idle"
	record.TmuxSessionName = workspace.Name
	record.TmuxWindow = paneBinding.WindowID
	record.TmuxPane = paneBinding.PaneID

	if _, err := sessionService.Save(*record); err != nil {
		return err
	}

	_ = r.app.Tmux.RenameWindow(paneBinding.WindowID, record.AgentName)

	fmt.Fprintln(r.stdout, "Session rebound (new pane)")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Session: %s\n", record.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", record.AgentName)
	fmt.Fprintf(r.stdout, "Pane: %s\n", record.TmuxPane)
	fmt.Fprintf(r.stdout, "Window: %s\n", record.TmuxWindow)
	fmt.Fprintf(r.stdout, "Status: Idle\n")
	return nil
}

func (r Runner) executeSessionWait(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	sessionIdentifier := strings.TrimSpace(args[0])
	eventType := ""
	waitTimeout := 30 * time.Minute

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--event":
			i++
			if i >= len(args) {
				return fmt.Errorf("--event requires a value")
			}
			eventType = strings.TrimSpace(args[i])
		case "--timeout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--timeout requires a value")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("--timeout value %q is not a valid duration (e.g. 30m, 1h): %w", args[i], err)
			}
			waitTimeout = d
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if eventType == "" {
		return fmt.Errorf("--event is required (e.g. --event handoff.prepared)")
	}

	record, err := r.loadSessionByIdentifier(sessionIdentifier)
	if err != nil {
		return err
	}
	if strings.TrimSpace(record.TaskID) == "" {
		return fmt.Errorf("session %q is not bound to a task; session wait requires a task-bound session", record.ID)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, record.TaskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", record.TaskID)
	}

	logPath := taskArtifactLogPath(result.Project.RepoPath, result.StateDir, record.TaskID, view.Worktree)

	fmt.Fprintf(r.stdout, "Waiting for event %q\n", eventType)
	fmt.Fprintf(r.stdout, "Session: %s  Task: %s\n", record.ID, record.TaskID)
	fmt.Fprintf(r.stdout, "Log: %s\n", logPath)
	fmt.Fprintf(r.stdout, "Timeout: %s\n", waitTimeout)
	fmt.Fprintln(r.stdout, "")

	line, err := waitForLogEvent(logPath, eventType, waitTimeout)
	if err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Event detected")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Event: %s\n", eventType)
	fmt.Fprintf(r.stdout, "Entry: %s\n", line)
	return nil
}

func (r Runner) executeTaskReanalyze(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	taskID := strings.TrimSpace(args[0])

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	nextAction := recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous)

	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:    "reanalysis.completed",
		Actor:   "aom",
		Summary: fmt.Sprintf("Artifacts re-synchronized from current system state; recommended next action: %s", nextAction),
		StateEffect: fmt.Sprintf("Task %s", view.Task.Status),
	}, false); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Task re-analyzed")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", view.Task.ID)
	fmt.Fprintf(r.stdout, "Title: %s\n", view.Task.Title)
	fmt.Fprintf(r.stdout, "Status: %s\n", view.Task.Status)
	if view.Worktree != nil {
		if view.WorktreeDrift != "" {
			fmt.Fprintf(r.stdout, "Worktree: %s (%s)\n", view.Worktree.Status, view.WorktreeDrift)
		} else {
			fmt.Fprintf(r.stdout, "Worktree: %s\n", view.Worktree.Status)
		}
	}
	if view.UnresolvedReviewItems > 0 {
		fmt.Fprintf(r.stdout, "Unresolved review items: %d\n", view.UnresolvedReviewItems)
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Recommended next action: %s\n", nextAction)
	return nil
}

func (r Runner) executeTaskLink(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	dependentID := strings.TrimSpace(args[0])
	var blockingID string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--blocks":
			i++
			if i >= len(args) {
				return fmt.Errorf("--blocks requires a value")
			}
			blockingID = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if blockingID == "" {
		return fmt.Errorf("--blocks <task-id> is required")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	if err := taskService.AddDependency(dependentID, blockingID); err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, dependentID, artifact.Event{
		Type:        "task.linked",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task %s is now blocked by %s", dependentID, blockingID),
		StateEffect: "dependency added",
	}, false); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Linked: %s is blocked by %s\n", dependentID, blockingID)
	return nil
}

func (r Runner) executeTaskUnlink(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	dependentID := strings.TrimSpace(args[0])
	var blockingID string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--blocks":
			i++
			if i >= len(args) {
				return fmt.Errorf("--blocks requires a value")
			}
			blockingID = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if blockingID == "" {
		return fmt.Errorf("--blocks <task-id> is required")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	if err := taskService.RemoveDependency(dependentID, blockingID); err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, dependentID, artifact.Event{
		Type:        "task.unlinked",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task %s is no longer blocked by %s", dependentID, blockingID),
		StateEffect: "dependency removed",
	}, false); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Unlinked: %s is no longer blocked by %s\n", dependentID, blockingID)
	return nil
}

func (r Runner) executeNext(args []string) error {
	_ = args

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	unblocked, err := taskService.ListUnblocked(result.Project.ID)
	if err != nil {
		return err
	}

	all, err := taskService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	// Collect blocked tasks (have at least one active blocker).
	unblockedIDs := make(map[string]bool)
	for _, t := range unblocked {
		unblockedIDs[t.ID] = true
	}

	var blocked []task.Record
	for _, t := range all {
		if t.Status == "Done" || t.Status == "Archived" {
			continue
		}
		if !unblockedIDs[t.ID] {
			blocked = append(blocked, t)
		}
	}

	fmt.Fprintln(r.stdout, "Next tasks")
	fmt.Fprintln(r.stdout, "")

	if len(unblocked) == 0 {
		fmt.Fprintln(r.stdout, "Unblocked: none")
	} else {
		fmt.Fprintln(r.stdout, "Unblocked (work on these next):")
		for i, t := range unblocked {
			priority := task.PriorityLabel(t.Priority)
			owner := emptyFallback(t.PreferredAgent)
			if owner == "-" {
				owner = emptyFallback(t.PreferredRole)
			}
			fmt.Fprintf(r.stdout, "  %d. [%s] %s  %s  owner=%s\n", i+1, priority, t.ID, t.Title, owner)
		}
	}

	if len(blocked) > 0 {
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintln(r.stdout, "Blocked (waiting on dependencies):")
		for _, t := range blocked {
			blockers, _ := taskService.BlockedBy(t.ID)
			blockerIDs := make([]string, 0, len(blockers))
			for _, b := range blockers {
				if b.Status != "Done" && b.Status != "Archived" {
					blockerIDs = append(blockerIDs, b.ID)
				}
			}
			fmt.Fprintf(r.stdout, "  %s  %s  waiting on: %s\n", t.ID, t.Title, strings.Join(blockerIDs, ", "))
		}
	}

	return nil
}

func (r Runner) executeSessionSetAgentID(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: aom session set-agent-id <session-id> <native-session-id>")
	}
	sessID := strings.TrimSpace(args[0])
	vendorID := strings.TrimSpace(args[1])

	record, err := r.loadSessionByIdentifier(sessID)
	if err != nil {
		return err
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

	updated, err := sessionService.SetVendorSessionID(record.ID, vendorID)
	if err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Agent session ID registered")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Session: %s\n", updated.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", updated.AgentName)
	fmt.Fprintf(r.stdout, "Native session ID: %s\n", updated.VendorSessionID)
	fmt.Fprintf(r.stdout, "Next spawn for this agent on task %s will resume this session.\n", emptyFallback(updated.TaskID))
	return nil
}

func (r Runner) executeWatch(args []string) error {
	taskID := ""
	eventType := ""
	watchTimeout := 30 * time.Minute

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			i++
			if i >= len(args) {
				return fmt.Errorf("--task requires a value")
			}
			taskID = strings.TrimSpace(args[i])
		case "--event":
			i++
			if i >= len(args) {
				return fmt.Errorf("--event requires a value")
			}
			eventType = strings.TrimSpace(args[i])
		case "--timeout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--timeout requires a value")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("--timeout value %q is not a valid duration (e.g. 30m, 2h): %w", args[i], err)
			}
			watchTimeout = d
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	if taskID != "" {
		return r.executeWatchSingleTask(result, taskID, eventType, watchTimeout)
	}

	return r.executeWatchAllTasks(result, eventType, watchTimeout)
}

// executeWatchSingleTask is the original single-task watch path.
func (r Runner) executeWatchSingleTask(result *project.OpenResult, taskID, eventType string, watchTimeout time.Duration) error {
	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	logPath := taskArtifactLogPath(result.Project.RepoPath, result.StateDir, taskID, view.Worktree)

	if eventType != "" {
		fmt.Fprintf(r.stdout, "Watching for event %q\n", eventType)
		fmt.Fprintf(r.stdout, "Task: %s\n", taskID)
		fmt.Fprintf(r.stdout, "Log: %s\n", logPath)
		fmt.Fprintf(r.stdout, "Timeout: %s\n\n", watchTimeout)

		line, err := waitForLogEvent(logPath, eventType, watchTimeout)
		if err != nil {
			return err
		}

		fmt.Fprintln(r.stdout, "Event detected")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "Event: %s\n", eventType)
		fmt.Fprintf(r.stdout, "Entry: %s\n", line)
		return nil
	}

	fmt.Fprintf(r.stdout, "Watching task %s (tail mode, timeout %s)\n", taskID, watchTimeout)
	fmt.Fprintf(r.stdout, "Log: %s\n\n", logPath)

	return tailLogEvents(r.stdout, logPath, watchTimeout)
}

// executeWatchAllTasks watches all active tasks simultaneously.
func (r Runner) executeWatchAllTasks(result *project.OpenResult, eventType string, watchTimeout time.Duration) error {
	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	allTasks, err := taskService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer worktreeDB.Close()

	activeStatuses := map[string]bool{
		"InProgress":     true,
		"Blocked":        true,
		"NeedsAttention": true,
		"Ready":          true,
	}

	var entries []taskLogEntry
	for _, t := range allTasks {
		if !activeStatuses[t.Status] {
			continue
		}
		mapping, _ := worktreeService.GetByTask(t.ID)
		logPath := taskArtifactLogPath(result.Project.RepoPath, result.StateDir, t.ID, mapping)
		entries = append(entries, taskLogEntry{TaskID: t.ID, LogPath: logPath})
	}

	if len(entries) == 0 {
		fmt.Fprintln(r.stdout, "No active tasks to watch.")
		return nil
	}

	if eventType != "" {
		fmt.Fprintf(r.stdout, "Watching %d active task(s) for event %q (timeout %s)\n", len(entries), eventType, watchTimeout)
		for _, e := range entries {
			fmt.Fprintf(r.stdout, "  %s → %s\n", e.TaskID, e.LogPath)
		}
		fmt.Fprintln(r.stdout, "")

		matchedTask, matchedLine, err := waitForMultiTaskLogEvent(entries, eventType, watchTimeout)
		if err != nil {
			return err
		}

		fmt.Fprintln(r.stdout, "Event detected")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "Task: %s\n", matchedTask)
		fmt.Fprintf(r.stdout, "Event: %s\n", eventType)
		fmt.Fprintf(r.stdout, "Entry: %s\n", matchedLine)
		return nil
	}

	fmt.Fprintf(r.stdout, "Watching %d active task(s) (tail mode, timeout %s)\n", len(entries), watchTimeout)
	for _, e := range entries {
		fmt.Fprintf(r.stdout, "  %s → %s\n", e.TaskID, e.LogPath)
	}
	fmt.Fprintln(r.stdout, "")

	return tailMultiTaskLogEvents(r.stdout, entries, watchTimeout)
}

// ── M14: task request / team brief ──────────────────────────────────────────

func (r Runner) executeTaskRequest(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task title is required")
	}

	title := strings.TrimSpace(args[0])
	var fromSession, priorityFlag string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--from-session":
			i++
			if i >= len(args) {
				return fmt.Errorf("--from-session requires a value")
			}
			fromSession = strings.TrimSpace(args[i])
		case "--priority":
			i++
			if i >= len(args) {
				return fmt.Errorf("--priority requires a value")
			}
			priorityFlag = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if priorityFlag == "" {
		priorityFlag = "normal"
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	reqID := generateRequestID(time.Now())

	requestedBy := "operator"
	if fromSession != "" {
		requestedBy = fromSession
	}

	rec := RequestRecord{
		ID:          reqID,
		Title:       title,
		RequestedBy: requestedBy,
		Priority:    priorityFlag,
		Status:      "pending",
	}

	if err := writeRequestArtifact(result.Project.RepoPath, rec); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Request filed\n\n")
	fmt.Fprintf(r.stdout, "Request: %s\n", reqID)
	fmt.Fprintf(r.stdout, "Title:   %s\n", title)
	fmt.Fprintf(r.stdout, "Status:  pending\n")
	fmt.Fprintf(r.stdout, "Next:    aom task list-requests\n")
	return nil
}

func (r Runner) executeTaskListRequests(args []string) error {
	_ = args

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	records, err := readPendingRequests(result.Project.RepoPath)
	if err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Pending task requests")
	fmt.Fprintln(r.stdout, "")

	if len(records) == 0 {
		fmt.Fprintln(r.stdout, "No pending requests.")
		return nil
	}

	for _, rec := range records {
		fmt.Fprintf(r.stdout, "  %s  [%s]  %s  from=%s\n",
			rec.ID, rec.Priority, rec.Title, emptyFallback(rec.RequestedBy))
	}
	fmt.Fprintf(r.stdout, "\nApprove: aom task approve-request <id>\n")
	return nil
}

func (r Runner) executeTaskApproveRequest(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("request id is required")
	}

	reqID := strings.TrimSpace(args[0])

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	all, err := readAllRequests(result.Project.RepoPath)
	if err != nil {
		return err
	}

	var rec *RequestRecord
	for i := range all {
		if all[i].ID == reqID {
			rec = &all[i]
			break
		}
	}
	if rec == nil {
		return fmt.Errorf("request %q not found", reqID)
	}
	if rec.Status != "pending" {
		return fmt.Errorf("request %q is already %s", reqID, rec.Status)
	}

	if err := r.validateTaskProvisioning(result); err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	priority, err := task.NormalizePriority(rec.Priority)
	if err != nil {
		priority = task.PriorityNormal
	}

	createResult, err := taskService.Create(task.CreateParams{
		ProjectID: result.Project.ID,
		Title:     rec.Title,
		Priority:  priority,
	})
	if err != nil {
		return err
	}

	if _, err := r.ensurePlannedWorktree(result, createResult.Task); err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, createResult.Task.ID, artifact.Event{
		Type:        "task.created",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Task created from approved request %s", reqID),
		StateEffect: fmt.Sprintf("Task %s", createResult.Task.Status),
	}, true); err != nil {
		return err
	}

	// Mark request approved.
	rec.Status = "approved"
	if err := writeRequestArtifact(result.Project.RepoPath, *rec); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Request approved\n\n")
	fmt.Fprintf(r.stdout, "Request: %s\n", reqID)
	fmt.Fprintf(r.stdout, "Task:    %s\n", createResult.Task.ID)
	fmt.Fprintf(r.stdout, "Title:   %s\n", createResult.Task.Title)
	return nil
}

func (r Runner) executeTaskRejectRequest(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("request id is required")
	}

	reqID := strings.TrimSpace(args[0])
	var reason string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--reason":
			i++
			if i >= len(args) {
				return fmt.Errorf("--reason requires a value")
			}
			reason = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	all, err := readAllRequests(result.Project.RepoPath)
	if err != nil {
		return err
	}

	var rec *RequestRecord
	for i := range all {
		if all[i].ID == reqID {
			rec = &all[i]
			break
		}
	}
	if rec == nil {
		return fmt.Errorf("request %q not found", reqID)
	}
	if rec.Status != "pending" {
		return fmt.Errorf("request %q is already %s", reqID, rec.Status)
	}

	rec.Status = "rejected"
	rec.Reason = reason
	if err := writeRequestArtifact(result.Project.RepoPath, *rec); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Request rejected\n\n")
	fmt.Fprintf(r.stdout, "Request: %s\n", reqID)
	if reason != "" {
		fmt.Fprintf(r.stdout, "Reason:  %s\n", reason)
	}
	return nil
}

func (r Runner) executeTeam(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("team subcommand is required (try: team brief)")
	}

	switch args[0] {
	case "brief":
		return r.executeTeamBrief(args[1:])
	default:
		return fmt.Errorf("unknown team command %q", args[0])
	}
}

func (r Runner) executeTeamBrief(args []string) error {
	_ = args

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	allTasks, err := taskService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	sessionService, sessDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sessDB.Close()

	sessions, err := sessionService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	// Build session status index by task.
	taskSessionStatus := make(map[string]string)
	for _, s := range sessions {
		if s.TaskID != "" {
			taskSessionStatus[s.TaskID] = s.Status
		}
	}

	// Build agent session status index.
	agentSessionStatus := make(map[string]string)
	for _, s := range sessions {
		if s.AgentName != "" {
			agentSessionStatus[s.AgentName] = s.Status
		}
	}

	var briefTasks []artifact.TeamBriefTask
	for _, t := range allTasks {
		if t.Status == "Done" || t.Status == "Archived" {
			continue
		}
		blockers, _ := taskService.BlockedBy(t.ID)
		blockerIDs := make([]string, 0, len(blockers))
		for _, b := range blockers {
			if b.Status != "Done" && b.Status != "Archived" {
				blockerIDs = append(blockerIDs, b.ID)
			}
		}
		owner := t.PreferredAgent
		if owner == "" {
			owner = t.PreferredRole
		}
		briefTasks = append(briefTasks, artifact.TeamBriefTask{
			ID:        t.ID,
			Title:     t.Title,
			Status:    t.Status,
			Priority:  task.PriorityLabel(t.Priority),
			Agent:     owner,
			BlockedBy: blockerIDs,
		})
	}

	// Pending requests.
	pendingReqs, _ := readPendingRequests(result.Project.RepoPath)
	var reqLines []string
	for _, req := range pendingReqs {
		reqLines = append(reqLines, fmt.Sprintf("%s: \"%s\" from %s [%s]",
			req.ID, req.Title, emptyFallback(req.RequestedBy), req.Priority))
	}

	// Last 5 channel messages (raw lines from channel.md).
	channelTail := lastChannelMessages(result.Project.RepoPath, 5)

	// Agents from config.
	var briefAgents []artifact.TeamBriefAgent
	for _, a := range result.Agents {
		status := agentSessionStatus[a.Name]
		briefAgents = append(briefAgents, artifact.TeamBriefAgent{
			Name:          a.Name,
			Role:          a.Role,
			Runtime:       a.Runtime,
			SessionStatus: status,
		})
	}

	svc := artifact.NewService(result.Project.RepoPath, result.StateDir)
	briefPath, err := svc.GenerateTeamBrief(artifact.TeamBriefParams{
		ProjectName:     result.Project.Name,
		Tasks:           briefTasks,
		PendingRequests: reqLines,
		ChannelTail:     channelTail,
		Agents:          briefAgents,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Team brief generated\n\n")
	fmt.Fprintf(r.stdout, "Path:    %s\n", briefPath)
	fmt.Fprintf(r.stdout, "Tasks:   %d active\n", len(briefTasks))
	fmt.Fprintf(r.stdout, "Pending requests: %d\n", len(pendingReqs))
	return nil
}

// ── M15: merge coordination ──────────────────────────────────────────────────

func (r Runner) executeMerge(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("merge subcommand is required (check | prepare)")
	}

	switch args[0] {
	case "check":
		return r.executeMergeCheck(args[1:])
	case "prepare":
		return r.executeMergePrepare(args[1:])
	default:
		return fmt.Errorf("unknown merge command %q", args[0])
	}
}

func (r Runner) executeMergeCheck(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	taskID := strings.TrimSpace(args[0])
	var againstFlag string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--against":
			i++
			if i >= len(args) {
				return fmt.Errorf("--against requires a value")
			}
			againstFlag = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if view.Worktree == nil {
		return fmt.Errorf("task %q has no worktree", taskID)
	}

	sourceBranch := view.Worktree.BranchName

	// Resolve --against: accept a task ID or a branch name.
	otherBranch := againstFlag
	if strings.HasPrefix(againstFlag, "TASK-") {
		otherView, err := r.loadTaskView(result, againstFlag)
		if err != nil {
			return err
		}
		if otherView == nil {
			return fmt.Errorf("task %q not found", againstFlag)
		}
		if otherView.Worktree == nil {
			return fmt.Errorf("task %q has no worktree", againstFlag)
		}
		otherBranch = otherView.Worktree.BranchName
	}

	if otherBranch == "" {
		otherBranch = result.Project.DefaultBranch
	}

	base := result.Project.DefaultBranch

	checkResult, err := aommerge.CheckOverlaps(result.Project.RepoPath, sourceBranch, otherBranch, base)
	if err != nil {
		return fmt.Errorf("merge check: %w", err)
	}

	fmt.Fprintf(r.stdout, "Merge check: %s → %s\n", taskID, base)
	fmt.Fprintf(r.stdout, "Source branch: %s\n", sourceBranch)
	fmt.Fprintf(r.stdout, "Against:       %s\n", otherBranch)
	fmt.Fprintf(r.stdout, "Conflict score: %s (%d overlapping files)\n\n", checkResult.Score, len(checkResult.Overlaps))

	if len(checkResult.Overlaps) == 0 {
		fmt.Fprintln(r.stdout, "No overlapping files. Safe to merge.")
	} else {
		fmt.Fprintln(r.stdout, "Overlapping files:")
		for _, o := range checkResult.Overlaps {
			fmt.Fprintf(r.stdout, "  %s   (also in: %s)\n", o.Path, o.OtherBranch)
		}
		fmt.Fprintln(r.stdout, "\nReview overlapping files before merging.")
	}

	return nil
}

func (r Runner) executeMergePrepare(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	taskID := strings.TrimSpace(args[0])
	intoFlag := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--into":
			i++
			if i >= len(args) {
				return fmt.Errorf("--into requires a value")
			}
			intoFlag = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if view.Worktree == nil {
		return fmt.Errorf("task %q has no worktree", taskID)
	}

	targetBranch := intoFlag
	if targetBranch == "" {
		targetBranch = result.Project.DefaultBranch
	}

	sourceBranch := view.Worktree.BranchName
	base := result.Project.DefaultBranch

	checkResult, err := aommerge.CheckOverlaps(result.Project.RepoPath, sourceBranch, targetBranch, base)
	if err != nil {
		return fmt.Errorf("merge check: %w", err)
	}

	// Write merge-plan.md.
	svc := artifact.NewService(result.Project.RepoPath, result.StateDir)
	overlaps := make([]artifact.MergePlanOverlap, 0, len(checkResult.Overlaps))
	for _, o := range checkResult.Overlaps {
		overlaps = append(overlaps, artifact.MergePlanOverlap{
			Path:        o.Path,
			OtherBranch: o.OtherBranch,
		})
	}

	if err := svc.WriteMergePlan(artifact.SyncParams{
		Task:  view.Task,
		Steps: view.Steps,
	}, artifact.MergePlanParams{
		TaskID:        taskID,
		TargetBranch:  targetBranch,
		ConflictScore: string(checkResult.Score),
		Overlaps:      overlaps,
	}); err != nil {
		return err
	}

	// Create an integration step owned by operator role.
	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	_, err = stepService.Create(step.CreateParams{
		ProjectID: result.Project.ID,
		TaskID:    taskID,
		StepType:  "integration",
		Title:     fmt.Sprintf("Merge %s into %s", sourceBranch, targetBranch),
		RoleName:  "operator",
	})
	if err != nil {
		return err
	}

	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        "merge.prepared",
		Actor:       "operator",
		Summary:     fmt.Sprintf("Merge plan prepared: %s → %s (score: %s)", sourceBranch, targetBranch, checkResult.Score),
		StateEffect: "integration step created",
	}, false); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Merge plan prepared\n\n")
	fmt.Fprintf(r.stdout, "Task:           %s\n", taskID)
	fmt.Fprintf(r.stdout, "Target branch:  %s\n", targetBranch)
	fmt.Fprintf(r.stdout, "Conflict score: %s\n", checkResult.Score)
	fmt.Fprintf(r.stdout, "Overlapping files: %d\n", len(checkResult.Overlaps))
	fmt.Fprintf(r.stdout, "merge-plan.md written to task artifact root.\n")
	return nil
}

// ── M16: communication & feedback upgrade ────────────────────────────────────

func (r Runner) executeMessage(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("message subcommand is required (send | read | clear)")
	}

	switch args[0] {
	case "send":
		return r.executeMessageSend(args[1:])
	case "read":
		return r.executeMessageRead(args[1:])
	case "clear":
		return r.executeMessageClear(args[1:])
	default:
		return fmt.Errorf("unknown message command %q", args[0])
	}
}

func (r Runner) executeMessageSend(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: aom message send <agent-name> \"<message>\" [--from <sender>]")
	}

	agentName := strings.TrimSpace(args[0])
	message := strings.TrimSpace(args[1])
	fromSender := "operator"

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--from":
			i++
			if i >= len(args) {
				return fmt.Errorf("--from requires a value")
			}
			fromSender = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	if err := appendMailboxMessage(result.Project.RepoPath, agentName, message, fromSender, time.Now()); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Message sent to %s\n", agentName)
	return nil
}

func (r Runner) executeMessageRead(args []string) error {
	var agentName string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			agentName = strings.TrimSpace(args[i])
		default:
			if agentName == "" {
				agentName = strings.TrimSpace(args[i])
			} else {
				return fmt.Errorf("unknown flag %q", args[i])
			}
		}
	}

	if agentName == "" {
		return fmt.Errorf("agent name is required (--agent <name>)")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	content, err := readMailbox(result.Project.RepoPath, agentName)
	if err != nil {
		return err
	}

	if content == "" {
		fmt.Fprintf(r.stdout, "Mailbox for %s is empty.\n", agentName)
		return nil
	}

	fmt.Fprint(r.stdout, content)
	return nil
}

func (r Runner) executeMessageClear(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent name is required")
	}

	agentName := strings.TrimSpace(args[0])

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	if err := clearMailbox(result.Project.RepoPath, agentName); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Mailbox for %s cleared (archived).\n", agentName)
	return nil
}

func (r Runner) executeTaskRecordResult(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task id is required")
	}

	taskID := strings.TrimSpace(args[0])
	var passed, failed bool
	var summary, ciURL string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--passed":
			passed = true
		case "--failed":
			failed = true
		case "--summary":
			i++
			if i >= len(args) {
				return fmt.Errorf("--summary requires a value")
			}
			summary = strings.TrimSpace(args[i])
		case "--url":
			i++
			if i >= len(args) {
				return fmt.Errorf("--url requires a value")
			}
			ciURL = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if !passed && !failed {
		return fmt.Errorf("--passed or --failed is required")
	}
	if passed && failed {
		return fmt.Errorf("--passed and --failed are mutually exclusive")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	eventType := "test.passed"
	eventSummary := "CI tests passed"
	if failed {
		eventType = "test.failed"
		eventSummary = "CI tests failed"
	}
	if summary != "" {
		eventSummary = eventSummary + ": " + summary
	}
	if ciURL != "" {
		eventSummary = eventSummary + " (" + ciURL + ")"
	}

	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        eventType,
		Actor:       "ci",
		Summary:     eventSummary,
		StateEffect: eventType,
	}, false); err != nil {
		return err
	}

	// Move task to NeedsAttention on failure.
	if failed {
		taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
		if err != nil {
			return err
		}
		defer sqlDB.Close()

		rec, err := taskService.Get(taskID)
		if err != nil {
			return err
		}
		if rec != nil && (rec.Status == "InProgress" || rec.Status == "Ready") {
			if _, err := taskService.Update(taskID, task.UpdateParams{Status: "NeedsAttention"}); err != nil {
				// Non-fatal: log event already appended.
				fmt.Fprintf(r.stderr, "warning: could not transition task to NeedsAttention: %v\n", err)
			}
		}
	}

	status := "passed"
	if failed {
		status = "failed"
	}
	fmt.Fprintf(r.stdout, "Result recorded: %s — %s\n", status, eventSummary)
	return nil
}

func (r Runner) executeSessionHealth(args []string) error {
	showAll := false
	sessionID := ""

	for _, arg := range args {
		switch arg {
		case "--all":
			showAll = true
		default:
			if sessionID == "" {
				sessionID = strings.TrimSpace(arg)
			}
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	sessionService, sessDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sessDB.Close()

	sessions, err := sessionService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	now := time.Now()

	if showAll {
		fmt.Fprintln(r.stdout, "Session health")
		fmt.Fprintln(r.stdout, "")
		active := 0
		for _, s := range sessions {
			switch s.Status {
			case "Booting", "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Blocked":
			default:
				continue
			}
			active++
			logPath := r.resolveTaskLogPath(result, s.TaskID)
			h := computeSessionHealth(logPath, s.ID, now)
			warning := ""
			if h.CheckpointWarning {
				warning += " [checkpoint overdue]"
			}
			fmt.Fprintf(r.stdout, "  %s  agent=%-20s  status=%-20s  since-checkpoint=%-8s%s\n",
				s.ID, s.AgentName, s.Status, h.TimeSinceCheckpoint, warning)
		}
		if active == 0 {
			fmt.Fprintln(r.stdout, "No active sessions.")
		}
		return nil
	}

	if sessionID == "" {
		return fmt.Errorf("session id is required (or use --all)")
	}

	var targetSession *session.Record
	for i := range sessions {
		if sessions[i].ID == sessionID {
			targetSession = &sessions[i]
			break
		}
	}
	if targetSession == nil {
		return fmt.Errorf("session %q not found", sessionID)
	}

	logPath := r.resolveTaskLogPath(result, targetSession.TaskID)
	h := computeSessionHealth(logPath, sessionID, now)

	fmt.Fprintf(r.stdout, "Session health: %s\n\n", sessionID)
	fmt.Fprintf(r.stdout, "Agent:               %s\n", emptyFallback(targetSession.AgentName))
	fmt.Fprintf(r.stdout, "Status:              %s\n", targetSession.Status)
	fmt.Fprintf(r.stdout, "Time since checkpoint: %s\n", h.TimeSinceCheckpoint)
	if h.CheckpointWarning {
		fmt.Fprintf(r.stdout, "Warning: context may be stale — consider: aom checkpoint %s\n", sessionID)
	}

	return nil
}

// resolveTaskLogPath returns the log.md path for a task, falling back gracefully.
func (r Runner) resolveTaskLogPath(result *project.OpenResult, taskID string) string {
	if taskID == "" {
		return ""
	}
	view, err := r.loadTaskView(result, taskID)
	if err != nil || view == nil {
		return filepath.Join(result.Project.RepoPath, result.StateDir, "tasks", taskID, "log.md")
	}
	svc := artifact.NewService(result.Project.RepoPath, result.StateDir)
	return svc.TaskLogPath(artifact.SyncParams{Task: view.Task, Worktree: view.Worktree})
}

func (r Runner) executePauseAll(args []string) error {
	reason := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--reason":
			i++
			if i >= len(args) {
				return fmt.Errorf("--reason requires a value")
			}
			reason = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	sessionService, sessDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sessDB.Close()

	sessions, err := sessionService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	pauseMsg := "PAUSE: operator has paused all agents"
	if reason != "" {
		pauseMsg += " — " + reason
	}

	var paused []string
	for i := range sessions {
		s := sessions[i]
		if s.Status != "Working" {
			continue
		}

		s.Status = "WaitingApproval"
		if _, err := sessionService.Save(s); err != nil {
			fmt.Fprintf(r.stderr, "warning: could not pause session %s: %v\n", s.ID, err)
			continue
		}

		_ = r.app.Tmux.SendKeys(s.TmuxPane, pauseMsg)

		if s.TaskID != "" {
			_ = r.syncTaskArtifacts(result, s.TaskID, artifact.Event{
				Type:        "operator.pause",
				Actor:       "operator",
				Summary:     pauseMsg,
				StateEffect: "session WaitingApproval",
			}, false)
		}

		paused = append(paused, s.ID)
	}

	fmt.Fprintf(r.stdout, "Paused %d session(s)\n", len(paused))
	for _, id := range paused {
		fmt.Fprintf(r.stdout, "  %s  → WaitingApproval  (resume: aom approve %s)\n", id, id)
	}
	if len(paused) == 0 {
		fmt.Fprintln(r.stdout, "No Working sessions found to pause.")
	}
	return nil
}

func (r Runner) executeResumeAll(args []string) error {
	_ = args

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	sessionService, sessDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sessDB.Close()

	sessions, err := sessionService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	var resumed []string
	for i := range sessions {
		s := sessions[i]
		if s.Status != "WaitingApproval" {
			continue
		}

		s.Status = "Idle"
		if _, err := sessionService.Save(s); err != nil {
			fmt.Fprintf(r.stderr, "warning: could not resume session %s: %v\n", s.ID, err)
			continue
		}

		if s.TaskID != "" {
			_ = r.syncTaskArtifacts(result, s.TaskID, artifact.Event{
				Type:        "operator.resume",
				Actor:       "operator",
				Summary:     "Operator resumed all paused sessions",
				StateEffect: "session Idle",
			}, false)
		}

		resumed = append(resumed, s.ID)
	}

	fmt.Fprintf(r.stdout, "Resumed %d session(s)\n", len(resumed))
	for _, id := range resumed {
		fmt.Fprintf(r.stdout, "  %s  → Idle\n", id)
	}
	if len(resumed) == 0 {
		fmt.Fprintln(r.stdout, "No WaitingApproval sessions found to resume.")
	}
	return nil
}

// ── M17: observability ───────────────────────────────────────────────────────

func (r Runner) executeWorktreeReadFile(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: aom worktree read-file <task-id> <relative-path>")
	}

	taskID := strings.TrimSpace(args[0])
	relPath := filepath.Clean(strings.TrimSpace(args[1]))

	// Reject obvious traversal attempts before hitting the filesystem.
	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("path %q escapes the worktree root", args[1])
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	worktreeService, wDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer wDB.Close()

	mapping, err := worktreeService.GetByTask(taskID)
	if err != nil {
		return err
	}
	if mapping == nil {
		return fmt.Errorf("task %q has no worktree", taskID)
	}
	if mapping.Status != "Ready" && mapping.Status != "Active" {
		return fmt.Errorf("worktree for task %q is not available (status: %s)", taskID, mapping.Status)
	}

	// Final path validation: resolved path must stay inside worktree root.
	worktreeRoot := filepath.Clean(mapping.WorktreePath)
	targetPath := filepath.Join(worktreeRoot, relPath)
	targetPath = filepath.Clean(targetPath)

	if !strings.HasPrefix(targetPath, worktreeRoot+string(filepath.Separator)) && targetPath != worktreeRoot {
		return fmt.Errorf("path %q escapes the worktree root", args[1])
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Audit trail.
	_ = r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:    "worktree.read",
		Actor:   "operator",
		Summary: fmt.Sprintf("Read file %s from worktree of task %s", relPath, taskID),
	}, false)

	fmt.Fprint(r.stdout, string(data))
	return nil
}

func (r Runner) executeMetrics(args []string) error {
	days := 7
	filterTaskID := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--days":
			i++
			if i >= len(args) {
				return fmt.Errorf("--days requires a value")
			}
			n := 0
			if _, err := fmt.Sscanf(args[i], "%d", &n); err != nil || n <= 0 {
				return fmt.Errorf("--days must be a positive integer")
			}
			days = n
		case "--task":
			i++
			if i >= len(args) {
				return fmt.Errorf("--task requires a value")
			}
			filterTaskID = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	allTasks, err := taskService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	if filterTaskID != "" {
		filtered := allTasks[:0]
		for _, t := range allTasks {
			if t.ID == filterTaskID {
				filtered = append(filtered, t)
				break
			}
		}
		allTasks = filtered
	}

	logDir := logDirForTask(result.Project.RepoPath, result.StateDir)
	report := BuildVelocityReport(allTasks, logDir, days, time.Now())
	PrintVelocityReport(r.stdout, report)
	return nil
}

// lastChannelMessages returns the last n message blocks from channel.md.
func lastChannelMessages(repoPath string, n int) []string {
	content, err := readChannelFile(repoPath)
	if err != nil || content == "" {
		return nil
	}

	// Split on "### " to get individual message blocks.
	parts := strings.Split(content, "\n### ")
	if len(parts) <= 1 {
		return nil
	}

	// Drop the header part (before first ###).
	msgs := parts[1:]
	if len(msgs) > n {
		msgs = msgs[len(msgs)-n:]
	}

	result := make([]string, 0, len(msgs))
	for _, m := range msgs {
		result = append(result, "### "+strings.TrimSpace(m))
	}
	return result
}

func (r Runner) executeChannel(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("channel subcommand is required: append, read")
	}
	switch args[0] {
	case "append":
		return r.executeChannelAppend(args[1:])
	case "read":
		return r.executeChannelRead(args[1:])
	default:
		return fmt.Errorf("unknown channel command %q", args[0])
	}
}

func (r Runner) executeChannelAppend(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("message is required")
	}

	agentName := "operator"
	var msgParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			agentName = strings.TrimSpace(args[i])
		default:
			msgParts = append(msgParts, args[i])
		}
	}

	message := strings.TrimSpace(strings.Join(msgParts, " "))
	if message == "" {
		return fmt.Errorf("message is required")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	if err := appendChannelMessage(result.Project.RepoPath, agentName, message, time.Now()); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Message appended to channel")
	fmt.Fprintf(r.stdout, "Agent: %s\n", agentName)
	fmt.Fprintf(r.stdout, "Message: %s\n", message)
	fmt.Fprintf(r.stdout, "Channel: %s\n", channelFilePath(result.Project.RepoPath))
	return nil
}

func (r Runner) executeChannelRead(args []string) error {
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	content, err := readChannelFile(result.Project.RepoPath)
	if err != nil {
		return err
	}

	if content == "" {
		fmt.Fprintln(r.stdout, "Channel is empty")
		fmt.Fprintf(r.stdout, "Channel: %s\n", channelFilePath(result.Project.RepoPath))
		return nil
	}

	fmt.Fprint(r.stdout, content)
	return nil
}

func (r Runner) executeBroadcast(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("message is required")
	}

	var sessionIDs []string
	var msgParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--sessions":
			i++
			if i >= len(args) {
				return fmt.Errorf("--sessions requires a value")
			}
			for _, id := range strings.Split(args[i], ",") {
				if trimmed := strings.TrimSpace(id); trimmed != "" {
					sessionIDs = append(sessionIDs, trimmed)
				}
			}
		default:
			msgParts = append(msgParts, args[i])
		}
	}

	message := strings.TrimSpace(strings.Join(msgParts, " "))
	if message == "" {
		return fmt.Errorf("message is required")
	}
	if len(sessionIDs) == 0 {
		return fmt.Errorf("--sessions is required (e.g. --sessions SESS-001,SESS-002)")
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

	fmt.Fprintf(r.stdout, "Broadcasting to %d sessions\n\n", len(sessionIDs))

	delivered := 0
	failed := 0
	for _, id := range sessionIDs {
		record, err := sessionService.Get(id)
		if err != nil || record == nil {
			fmt.Fprintf(r.stdout, "  %-30s not found\n", id)
			failed++
			continue
		}
		if !sendableSessionStatus(record.Status) || strings.TrimSpace(record.TmuxPane) == "" {
			fmt.Fprintf(r.stdout, "  %-30s no live pane (status: %s)\n", id, record.Status)
			failed++
			continue
		}
		if err := r.app.Tmux.SendKeys(record.TmuxPane, message); err != nil {
			fmt.Fprintf(r.stdout, "  %-30s send failed: %v\n", id, err)
			failed++
			continue
		}
		fmt.Fprintf(r.stdout, "  %-30s delivered (%s)\n", id, record.AgentName)
		delivered++
	}

	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Delivered: %d  Failed: %d\n", delivered, failed)
	fmt.Fprintf(r.stdout, "Message: %s\n", message)
	return nil
}

func (r Runner) executeApprove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	record, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if record.Status != "WaitingApproval" {
		return fmt.Errorf("session %q is not waiting for approval (status: %s)", record.ID, record.Status)
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

	record.Status = "Idle"
	updated, err := sessionService.Save(*record)
	if err != nil {
		return err
	}

	if strings.TrimSpace(record.TaskID) != "" {
		_ = r.syncTaskArtifacts(result, record.TaskID, artifact.Event{
			Type:        "approval.approved",
			Actor:       "operator",
			SessionID:   record.ID,
			Summary:     fmt.Sprintf("Operator approved pending request for session %s", record.ID),
			StateEffect: "Session Idle",
		}, false)
	}

	fmt.Fprintln(r.stdout, "Approved")
	fmt.Fprintf(r.stdout, "Session: %s\n", updated.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", updated.AgentName)
	fmt.Fprintf(r.stdout, "Status: %s\n", updated.Status)
	return nil
}

func (r Runner) executeDeny(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	sessionIdentifier := strings.TrimSpace(args[0])
	reason := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--reason":
			i++
			if i >= len(args) {
				return fmt.Errorf("--reason requires a value")
			}
			reason = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	record, err := r.loadSessionByIdentifier(sessionIdentifier)
	if err != nil {
		return err
	}
	if record.Status != "WaitingApproval" {
		return fmt.Errorf("session %q is not waiting for approval (status: %s)", record.ID, record.Status)
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

	record.Status = "Blocked"
	updated, err := sessionService.Save(*record)
	if err != nil {
		return err
	}

	if strings.TrimSpace(record.TaskID) != "" {
		summary := fmt.Sprintf("Operator denied pending request for session %s", record.ID)
		if reason != "" {
			summary += ": " + reason
		}
		_ = r.syncTaskArtifacts(result, record.TaskID, artifact.Event{
			Type:        "approval.denied",
			Actor:       "operator",
			SessionID:   record.ID,
			Summary:     summary,
			StateEffect: "Session Blocked",
		}, false)
	}

	fmt.Fprintln(r.stdout, "Denied")
	fmt.Fprintf(r.stdout, "Session: %s\n", updated.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", updated.AgentName)
	fmt.Fprintf(r.stdout, "Status: %s\n", updated.Status)
	if reason != "" {
		fmt.Fprintf(r.stdout, "Reason: %s\n", reason)
	}
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
	if err := r.app.Tmux.AttachPane(sessionRecord.TmuxSessionName, sessionRecord.TmuxPane); err != nil {
		return err
	}

	if strings.TrimSpace(sessionRecord.TaskID) == "" {
		return nil
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	return r.syncTaskArtifactsWithSession(result, sessionRecord.TaskID, artifact.Event{
		Type:        "operator.intervention",
		Actor:       "operator",
		SessionID:   sessionRecord.ID,
		Summary:     fmt.Sprintf("Operator attached directly to session %s for live intervention", sessionRecord.ID),
		StateEffect: "Re-analysis required",
	}, false, sessionRecord)
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

func (r Runner) executeSessionSend(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) == 1 {
		return fmt.Errorf("message is required")
	}

	sessionRecord, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if !sendableSessionStatus(sessionRecord.Status) || strings.TrimSpace(sessionRecord.TmuxPane) == "" {
		return fmt.Errorf("session %q does not have a live tmux pane binding", sessionRecord.ID)
	}

	message := strings.TrimSpace(strings.Join(args[1:], " "))
	if message == "" {
		return fmt.Errorf("message is required")
	}

	// Interpret shell-style escape sequences so callers can embed newlines with \n.
	message = interpretEscapes(message)

	if err := r.app.Tmux.SendKeys(sessionRecord.TmuxPane, message); err != nil {
		return err
	}

	if strings.TrimSpace(sessionRecord.TaskID) != "" {
		result, err := r.app.Projects.Open(".")
		if err != nil {
			return err
		}

		actor := "operator"
		if env := strings.TrimSpace(os.Getenv("AOM_ACTOR")); env != "" {
			actor = env
		}

		if err := r.syncTaskArtifactsWithSession(result, sessionRecord.TaskID, artifact.Event{
			Type:        "orchestrator.prompt",
			Actor:       actor,
			SessionID:   sessionRecord.ID,
			Summary:     fmt.Sprintf("Prompt delivered to session %s: %s", sessionRecord.ID, briefSummary(message)),
			StateEffect: "Session Prompted",
		}, false, sessionRecord); err != nil {
			return err
		}
	}

	fmt.Fprintln(r.stdout, "Prompt delivered")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Session: %s\n", sessionRecord.ID)
	fmt.Fprintf(r.stdout, "Pane: %s\n", sessionRecord.TmuxPane)
	fmt.Fprintf(r.stdout, "Message: %s\n", message)
	return nil
}

type checkpointParams struct {
	sessionID string
}

type handoffParams struct {
	sessionID string
	target    string
	reason    string
}

type reviewParams struct {
	taskID     string
	agentName  string
	launchMode aomruntime.LaunchMode
}

func (r Runner) executeCheckpoint(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("checkpoint does not accept extra positional arguments in the current milestone")
	}

	sessionRecord, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if strings.TrimSpace(sessionRecord.TaskID) == "" {
		return fmt.Errorf("session %q is not bound to a task", sessionRecord.ID)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, sessionRecord.TaskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", sessionRecord.TaskID)
	}

	checkpointID := newCheckpointID()
	summary := fmt.Sprintf("Checkpoint %s created for session %s", checkpointID, sessionRecord.ID)
	if err := r.syncTaskArtifactsWithSession(result, sessionRecord.TaskID, artifact.Event{
		Type:        "checkpoint.created",
		Actor:       "operator",
		StepID:      activeStepID(view.Steps),
		SessionID:   sessionRecord.ID,
		Summary:     summary,
		StateEffect: "Checkpoint Ready",
	}, false, sessionRecord); err != nil {
		return err
	}
	if err := r.refreshTaskArtifacts(result, sessionRecord.TaskID, sessionRecord); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Checkpoint created")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Checkpoint: %s\n", checkpointID)
	fmt.Fprintf(r.stdout, "Task: %s\n", sessionRecord.TaskID)
	fmt.Fprintf(r.stdout, "Step: %s\n", emptyFallback(activeStepID(view.Steps)))
	fmt.Fprintf(r.stdout, "Owner: %s\n", sessionRecord.AgentName)
	fmt.Fprintf(r.stdout, "Changed files: %s\n", changedFilesSummary(sessionRecord.WorktreePath, sessionRecord.RepoPath))
	fmt.Fprintf(r.stdout, "Next action: %s\n", recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous))

	return nil
}

func (r Runner) executeHandoff(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	params := handoffParams{sessionID: strings.TrimSpace(args[0])}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--to":
			i++
			if i >= len(args) {
				return fmt.Errorf("--to requires a value")
			}
			params.target = strings.TrimSpace(args[i])
		case "--reason":
			i++
			if i >= len(args) {
				return fmt.Errorf("--reason requires a value")
			}
			params.reason = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if params.target == "" {
		return fmt.Errorf("--to is required")
	}

	sessionRecord, err := r.loadSessionByIdentifier(params.sessionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(sessionRecord.TaskID) == "" {
		return fmt.Errorf("session %q is not bound to a task", sessionRecord.ID)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, sessionRecord.TaskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", sessionRecord.TaskID)
	}

	targetRole, targetAgent, suggestedRuntime, err := resolveHandoffTarget(result, params.target)
	if err != nil {
		return err
	}
	reason := params.reason
	if reason == "" {
		reason = "Operator requested ownership transfer"
	}

	if err := r.transferHandoffOwnership(result, view, targetRole, targetAgent); err != nil {
		return err
	}

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	if sessionRecord.Status != "WaitingHandoff" {
		sessionRecord.Status = "WaitingHandoff"
		updated, err := sessionService.Save(*sessionRecord)
		if err != nil {
			return err
		}
		sessionRecord = updated
	}

	if err := r.writeHandoffArtifact(result, view, *sessionRecord, targetRole, targetAgent, suggestedRuntime, reason); err != nil {
		return err
	}

	if err := r.syncTaskArtifactsWithSession(result, sessionRecord.TaskID, artifact.Event{
		Type:        "handoff.prepared",
		Actor:       "operator",
		StepID:      activeStepID(view.Steps),
		SessionID:   sessionRecord.ID,
		Summary:     fmt.Sprintf("Handoff prepared from %s to %s", sessionRecord.AgentName, ownershipSummary(targetRole, targetAgent)),
		StateEffect: "Handoff Ready and ownership transferred",
	}, false, sessionRecord); err != nil {
		return err
	}
	if err := r.refreshTaskArtifacts(result, sessionRecord.TaskID, sessionRecord); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Handoff prepared")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", sessionRecord.TaskID)
	fmt.Fprintf(r.stdout, "From session: %s\n", sessionRecord.ID)
	fmt.Fprintf(r.stdout, "To role: %s\n", targetRole)
	fmt.Fprintf(r.stdout, "To agent: %s\n", emptyFallback(targetAgent))
	fmt.Fprintf(r.stdout, "Suggested runtime: %s\n", emptyFallback(suggestedRuntime))
	fmt.Fprintf(r.stdout, "Readiness: ready\n")
	fmt.Fprintf(r.stdout, "Ownership transferred: %s\n", ownershipSummary(targetRole, targetAgent))
	fmt.Fprintf(r.stdout, "Next action: %s\n", handoffNextAction(sessionRecord.TaskID, targetRole, targetAgent))

	return nil
}

func (r Runner) executeReview(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required (or use 'close <task-id>')")
	}

	if args[0] == "close" {
		return r.executeReviewClose(args[1:])
	}

	params := reviewParams{
		taskID:     strings.TrimSpace(args[0]),
		launchMode: aomruntime.LaunchModePlaceholder,
	}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			params.agentName = strings.TrimSpace(args[i])
		case "--mock":
			if err := setLaunchMode(&params.launchMode, aomruntime.LaunchModeMock); err != nil {
				return err
			}
		case "--real":
			if err := setLaunchMode(&params.launchMode, aomruntime.LaunchModeReal); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, params.taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", params.taskID)
	}

	reviewerAgent, err := r.resolveReviewerAgent(result, params.agentName)
	if err != nil {
		return err
	}

	reviewStep, err := r.ensureReviewStep(result, view, reviewerAgent)
	if err != nil {
		return err
	}
	if err := r.prepareReviewState(result, view.Task.ID, reviewStep.ID); err != nil {
		return err
	}
	view, err = r.loadTaskView(result, params.taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found after review step preparation", params.taskID)
	}

	service := artifact.NewService(result.Project.RepoPath, result.StateDir)
	syncParams := artifact.SyncParams{
		Task:                  view.Task,
		Steps:                 view.Steps,
		Worktree:              view.Worktree,
		CreatedBy:             "operator",
		UpdatedBy:             "operator",
		ReviewOwnerHint:       view.ReviewOwnerHint,
		ReviewOwnerAmbiguous:  view.ReviewOwnerAmbiguous,
		RecommendedNextAction: recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous),
	}
	if err := service.EnsureReviewNotesTemplate(syncParams, reviewerAgent.Name, ""); err != nil {
		return err
	}
	if changed, unresolvedCount, err := r.reconcileReviewFindings(result, view.Task.ID, reviewStep.ID); err != nil {
		return err
	} else if changed {
		view, err = r.loadTaskView(result, params.taskID)
		if err != nil {
			return err
		}
		if view == nil {
			return fmt.Errorf("task %q not found after review findings reconciliation", params.taskID)
		}
		fmt.Fprintf(r.stdout, "Review findings detected: %d\n", unresolvedCount)
	}
	if err := r.syncTaskArtifacts(result, view.Task.ID, artifact.Event{
		Type:        "review.prepared",
		Actor:       "operator",
		StepID:      reviewStep.ID,
		Summary:     fmt.Sprintf("Review context prepared for %s", reviewerAgent.Name),
		StateEffect: "Review Ready",
	}, false); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Review prepared")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", view.Task.ID)
	fmt.Fprintf(r.stdout, "Review step: %s\n", reviewStep.ID)
	fmt.Fprintf(r.stdout, "Reviewer agent: %s\n", reviewerAgent.Name)
	fmt.Fprintf(r.stdout, "Review notes: %s\n", filepath.Join(taskArtifactRoot(result.Project.RepoPath, result.StateDir, view.Task.ID, view.Worktree), "review-notes.md"))
	fmt.Fprintf(r.stdout, "Unresolved review items: %d\n", artifact.CountUnresolvedReviewItems(filepath.Join(taskArtifactRoot(result.Project.RepoPath, result.StateDir, view.Task.ID, view.Worktree), "review-notes.md")))

	if !r.app.Tmux.Availability().Available {
		fmt.Fprintln(r.stdout, "Next action: tmux is unavailable here; inspect review-notes.md and spawn the reviewer later from a supported environment")
		if err := r.refreshTaskArtifacts(result, view.Task.ID, nil); err != nil {
			return err
		}
		return nil
	}

	sessionRecord, reused, err := r.resolveReviewSession(result, view.Task.ID, reviewStep.ID, reviewerAgent, params.launchMode)
	if err != nil {
		return err
	}
	if err := r.activateReviewState(result, view.Task.ID, reviewStep.ID); err != nil {
		return err
	}
	view, err = r.loadTaskView(result, params.taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found after activating review state", params.taskID)
	}

	if err := service.EnsureReviewNotesTemplate(artifact.SyncParams{
		Task:                  view.Task,
		Steps:                 view.Steps,
		ActiveSession:         sessionRecord,
		Worktree:              view.Worktree,
		CreatedBy:             "operator",
		UpdatedBy:             "operator",
		ReviewOwnerHint:       view.ReviewOwnerHint,
		ReviewOwnerAmbiguous:  view.ReviewOwnerAmbiguous,
		RecommendedNextAction: recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous),
	}, reviewerAgent.Name, sessionRecord.ID); err != nil {
		return err
	}
	if err := r.refreshTaskArtifacts(result, view.Task.ID, sessionRecord); err != nil {
		return err
	}
	if reused {
		fmt.Fprintf(r.stdout, "Session reused: %s\n", sessionRecord.ID)
	} else {
		fmt.Fprintf(r.stdout, "Session spawned: %s\n", sessionRecord.ID)
	}

	return nil
}

// executeReviewClose marks the active review step Completed, transitions the
// task back to InProgress, and records a review.closed event. If the review
// notes contain an unambiguous owner hint, it is applied to the task.
func (r Runner) executeReviewClose(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}
	taskID := strings.TrimSpace(args[0])

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	// Find the active review step.
	var reviewStep *step.Record
	for i := range view.Steps {
		s := &view.Steps[i]
		if s.StepType == "review" && (s.Status == "InProgress" || s.Status == "Ready" || s.Status == "NeedsAttention") {
			reviewStep = s
			break
		}
	}
	if reviewStep == nil {
		return fmt.Errorf("no active review step found for task %q", taskID)
	}

	// Complete the review step. The step machine requires InProgress before Completed,
	// so advance through InProgress if the step hasn't entered it yet.
	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	if reviewStep.Status == "Ready" || reviewStep.Status == "NeedsAttention" {
		if _, err := stepService.Update(reviewStep.ID, step.UpdateParams{Status: "in-progress"}); err != nil {
			return fmt.Errorf("advance review step to in-progress: %w", err)
		}
	}
	if _, err := stepService.Update(reviewStep.ID, step.UpdateParams{Status: "completed"}); err != nil {
		return fmt.Errorf("complete review step: %w", err)
	}

	// Transition task back to InProgress.
	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	updateParams := task.UpdateParams{Status: "in-progress"}
	if !view.ReviewOwnerAmbiguous && strings.TrimSpace(view.ReviewOwnerHint) != "" {
		updateParams.PreferredAgent = view.ReviewOwnerHint
	}
	if _, err := taskService.Update(taskID, updateParams); err != nil {
		return fmt.Errorf("transition task to in-progress: %w", err)
	}

	// Record the review.closed event in the artifact log.
	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        "review.closed",
		Actor:       "operator",
		StepID:      reviewStep.ID,
		Summary:     fmt.Sprintf("Review step %s closed; task returned to in-progress", reviewStep.ID),
		StateEffect: "Review Closed",
	}, false); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Review closed")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", taskID)
	fmt.Fprintf(r.stdout, "Review step: %s\n", reviewStep.ID)
	fmt.Fprintf(r.stdout, "Task status: in-progress\n")
	if updateParams.PreferredAgent != "" {
		fmt.Fprintf(r.stdout, "Preferred agent: %s\n", updateParams.PreferredAgent)
	}
	return nil
}

type taskView struct {
	Task                  task.Record
	Steps                 []step.Record
	Worktree              *worktree.Record
	WorktreeDrift         string
	UnresolvedReviewItems int
	ReviewOwnerHint       string
	ReviewOwnerAmbiguous  bool
}

func (r Runner) printProjectSummary(title string, result *project.OpenResult, workspace *tmux.Workspace, sessions []session.Record, taskCount int, taskViews []taskView) {
	terminalAvailability := r.app.Tmux.Availability()
	workspaceName := r.app.Tmux.ProjectSessionName(result.SessionPrefix)

	fmt.Fprintln(r.stdout, title)
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Name: %s\n", result.Project.Name)
	fmt.Fprintf(r.stdout, "Repo: %s\n", result.Project.RepoPath)
	fmt.Fprintf(r.stdout, "Default branch: %s\n", result.Project.DefaultBranch)
	fmt.Fprintf(r.stdout, "DB: %s\n", result.DBPath)
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, sectionLabel("Terminal:", r.stdout))
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
	fmt.Fprintln(r.stdout, sectionLabel("Agents:", r.stdout))
	for _, agent := range result.Agents {
		fmt.Fprintf(r.stdout, "  - %s | role=%s | runtime=%s | enabled=%t\n", agent.Name, agent.Role, agent.Runtime, agent.Enabled)
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, sectionLabel("Sessions:", r.stdout))
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
				colorStatus(item.Status, r.stdout),
				item.TmuxSessionName,
				item.TmuxWindow,
				item.TmuxPane,
			)
		}
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, sectionLabel("Tasks:", r.stdout))
	if len(taskViews) == 0 {
		fmt.Fprintln(r.stdout, "  None")
	} else {
		for _, item := range taskViews {
			fmt.Fprintf(
				r.stdout,
				"  - %s | title=%s | mode=%s | status=%s | role=%s | agent=%s | steps=%d\n",
				item.Task.ID,
				item.Task.Title,
				item.Task.Mode,
				colorStatus(item.Task.Status, r.stdout),
				emptyFallback(item.Task.PreferredRole),
				emptyFallback(item.Task.PreferredAgent),
				len(item.Steps),
			)
			fmt.Fprintf(r.stdout, "    reviews=open:%d\n", item.UnresolvedReviewItems)
			fmt.Fprintf(r.stdout, "    review-owner=%s\n", reviewOwnerHintDisplay(item.ReviewOwnerHint, item.ReviewOwnerAmbiguous))
			if item.Worktree != nil {
				fmt.Fprintf(
					r.stdout,
					"    worktree=%s | branch=%s | path=%s\n",
					colorStatus(item.Worktree.Status, r.stdout),
					item.Worktree.BranchName,
					item.Worktree.WorktreePath,
				)
				if hint := worktreeHint(item.Task.ID, item.Worktree, item.WorktreeDrift); hint != "" {
					label := "note"
					if item.Worktree.Status == worktree.StatusNeedsRepair {
						label = "repair"
					}
					fmt.Fprintf(r.stdout, "    %s=%s\n", label, hint)
				}
			}
			fmt.Fprintf(
				r.stdout,
				"    artifacts=%s | log=%s\n",
				taskArtifactRoot(result.Project.RepoPath, result.StateDir, item.Task.ID, item.Worktree),
				taskArtifactLogPath(result.Project.RepoPath, result.StateDir, item.Task.ID, item.Worktree),
			)
			fmt.Fprintf(r.stdout, "    next=%s\n", recommendTaskAction(item.Task.Status, item.Steps, item.Worktree, item.WorktreeDrift, item.UnresolvedReviewItems, item.ReviewOwnerHint, item.ReviewOwnerAmbiguous))
			for _, taskStep := range item.Steps {
				fmt.Fprintf(
					r.stdout,
					"    * %s | type=%s | title=%s | status=%s | role=%s | agent=%s | dependencies=%s\n",
					taskStep.ID,
					taskStep.StepType,
					taskStep.Title,
					colorStatus(taskStep.Status, r.stdout),
					emptyFallback(taskStep.RoleName),
					emptyFallback(taskStep.AgentName),
					formatDependencies(taskStep.Dependencies),
				)
			}
		}
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Counts:")
	fmt.Fprintf(r.stdout, "  Tasks: %d\n", taskCount)
	fmt.Fprintf(r.stdout, "  Sessions: %d\n", len(sessions))
}

// interpretEscapes converts literal escape sequences in s to their real byte
// equivalents (\n → newline, \t → tab). This lets callers embed multi-line
// content in a single shell argument without requiring ANSI-C quoting.
func interpretEscapes(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")
	return s
}

// briefSummary returns a single-line summary of a brief for use in log events.
// Multi-line briefs are truncated to their first non-empty line so that the
// log scanner never mistakes embedded log-format templates for real events.
func briefSummary(message string) string {
	const maxLen = 200
	for _, line := range strings.Split(message, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			if len(trimmed) > maxLen {
				return trimmed[:maxLen] + "..."
			}
			return trimmed
		}
	}
	return strings.TrimSpace(message)
}

func (r Runner) printHelp() {
	fmt.Fprintln(r.stdout, "AOM is a project-local control plane for agent sessions, tasks, worktrees, and durable markdown artifacts.")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Operator workflow")
	fmt.Fprintln(r.stdout, "1. aom task create \"work summary\" --role <role> --agent <agent>")
	fmt.Fprintln(r.stdout, "2. aom step list <task-id> ; aom step update <step-id> --status confirmed")
	fmt.Fprintln(r.stdout, "3. aom session spawn <agent> --task <task-id> --mock|--real")
	fmt.Fprintln(r.stdout, "4. aom session send <session-id> \"brief for the worker\"")
	fmt.Fprintln(r.stdout, "5. aom capture <session-id>")
	fmt.Fprintln(r.stdout, "6. aom task close <task-id>")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Project")
	fmt.Fprintln(r.stdout, "aom project init <name> --repo <path> : create .aom config, db, and starter agents")
	fmt.Fprintln(r.stdout, "aom project resources : show role bindings, skills, MCP servers, and policy")
	fmt.Fprintln(r.stdout, "aom open : load project state and reconcile tmux/worktree/session state")
	fmt.Fprintln(r.stdout, "aom status : show project, tasks, sessions, worktrees, and next-action hints")
	fmt.Fprintln(r.stdout, "aom plan \"work\" [--create] : draft a task plan and optionally persist it")
	fmt.Fprintln(r.stdout, "aom doctor : validate environment (tmux, config, runtimes, db, worktrees)")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Runtime")
	fmt.Fprintln(r.stdout, "aom runtime list : list configured runtimes with binary availability")
	fmt.Fprintln(r.stdout, "aom runtime inspect <runtime> : show runtime capabilities, agents, and resume support")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Task")
	fmt.Fprintln(r.stdout, "aom task create <title> [--role <role>] [--agent <agent>] : create a task")
	fmt.Fprintln(r.stdout, "aom task show <task-id> : inspect task state, artifacts, and ownership")
	fmt.Fprintln(r.stdout, "aom task update <task-id> [flags] : change task mode, owner, or status")
	fmt.Fprintln(r.stdout, "aom task close <task-id> : mark a task complete")
	fmt.Fprintln(r.stdout, "aom review <task-id> [--mock|--real] : prepare or start review flow")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Step")
	fmt.Fprintln(r.stdout, "aom step list <task-id> : list task steps and their owners/statuses")
	fmt.Fprintln(r.stdout, "aom step update <step-id> --status <status> : advance one step explicitly")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Session")
	fmt.Fprintln(r.stdout, "aom session spawn <agent> [--task <task-id>] [--mock|--real] [--fresh] : start a worker session")
	fmt.Fprintln(r.stdout, "  --fresh : force a new context even when a previous native session exists for this task")
	fmt.Fprintln(r.stdout, "aom session send <session-id> <message> : deliver a prompt into a live session")
	fmt.Fprintln(r.stdout, "aom session list : list known sessions")
	fmt.Fprintln(r.stdout, "aom session show <session-id> : inspect one session and its bindings")
	fmt.Fprintln(r.stdout, "aom session stop <session-id> : stop a live session and keep continuity state")
	fmt.Fprintln(r.stdout, "aom session archive <session-id> : archive an inactive session record")
	fmt.Fprintln(r.stdout, "aom session resume <session-id> --task <task-id> : rebind an Idle or WaitingHandoff session to a new task (reuses native context)")
	fmt.Fprintln(r.stdout, "aom session replace <session-id> --agent <agent> --reason <why> [--mock|--real] : spawn a replacement in the same context")
	fmt.Fprintln(r.stdout, "aom session set-agent-id <session-id> <native-id> : register the agent CLI's own session ID for resume on next spawn")
	fmt.Fprintln(r.stdout, "aom session wait <session-id> --event <type> [--timeout 30m] : block until event appears in task log (e.g. handoff.prepared, task.completed)")
	fmt.Fprintln(r.stdout, "aom task reanalyze <task-id> : refresh task artifacts from current state and print recommended next action")
	fmt.Fprintln(r.stdout, "aom capture <session-id> : read worker output through AOM")
	fmt.Fprintln(r.stdout, "aom attach <session-id> : attach manually and log operator intervention")
	fmt.Fprintln(r.stdout, "aom checkpoint <session-id> : refresh task artifacts and record a checkpoint")
	fmt.Fprintln(r.stdout, "aom handoff <session-id> --to <role-or-agent> [--reason <why>] : prepare handoff state")
	fmt.Fprintln(r.stdout, "aom approve <session-id> : approve a pending WaitingApproval session request")
	fmt.Fprintln(r.stdout, "aom deny <session-id> [--reason <why>] : deny a pending WaitingApproval session request")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Team collaboration")
	fmt.Fprintln(r.stdout, "aom watch [--task <task-id>] [--event <type>] [--timeout 30m] : stream log events across all active tasks (or one task with --task)")

	fmt.Fprintln(r.stdout, "aom broadcast \"<message>\" --sessions <id,id,...> : deliver the same prompt to multiple sessions at once")
	fmt.Fprintln(r.stdout, "aom channel append \"<message>\" [--agent <name>] : append a message to the shared .aom/channel.md")
	fmt.Fprintln(r.stdout, "aom channel read : print current shared channel contents")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Worktree")
	fmt.Fprintln(r.stdout, "aom worktree repair <task-id> : repair a missing or stale task worktree")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Key rules")
	fmt.Fprintln(r.stdout, "- Never edit .aom/ files directly; use the CLI so state changes stay canonical.")
	fmt.Fprintln(r.stdout, "- Use aom capture <session-id> to read agent output; do not inspect tmux directly as your primary interface.")
	fmt.Fprintln(r.stdout, "- .agent/*.md artifacts inside the task worktree are the source of truth for worker continuity.")
	fmt.Fprintln(r.stdout, "- Session status Idle means ready for the next prompt or task; Working means the agent is busy.")
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

func (r Runner) resolveReviewerAgent(result *project.OpenResult, name string) (*agent.Record, error) {
	if result == nil {
		return nil, fmt.Errorf("project context is required")
	}
	if strings.TrimSpace(name) != "" {
		agentRecord, err := findAgent(result.Agents, name)
		if err != nil {
			return nil, err
		}
		if agentRecord.Role != "reviewer" {
			return nil, fmt.Errorf("agent %q does not use reviewer role", name)
		}
		return agentRecord, nil
	}

	for _, item := range result.Agents {
		if item.Enabled && item.Role == "reviewer" {
			agentCopy := item
			return &agentCopy, nil
		}
	}

	return nil, fmt.Errorf("no enabled reviewer agent is configured for this project")
}

func (r Runner) reconcileReviewFindings(result *project.OpenResult, taskID, reviewStepID string) (bool, int, error) {
	if result == nil {
		return false, 0, fmt.Errorf("project context is required")
	}

	reviewNotesPath := filepath.Join(taskArtifactRoot(result.Project.RepoPath, result.StateDir, taskID, nil), "review-notes.md")
	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return false, 0, err
	}
	if view == nil {
		return false, 0, fmt.Errorf("task %q not found", taskID)
	}
	reviewNotesPath = filepath.Join(taskArtifactRoot(result.Project.RepoPath, result.StateDir, taskID, view.Worktree), "review-notes.md")
	unresolvedCount := artifact.CountUnresolvedReviewItems(reviewNotesPath)
	if unresolvedCount == 0 {
		return false, 0, nil
	}
	suggestedOwner := artifact.SuggestedReviewOwner(reviewNotesPath)
	suggestedAgent := resolveRoleHintAgent(result.Agents, suggestedOwner)

	changed := false

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return false, unresolvedCount, err
	}
	defer taskDB.Close()

	taskRecord, err := taskService.Get(taskID)
	if err != nil {
		return false, unresolvedCount, err
	}
	if taskRecord == nil {
		return false, unresolvedCount, fmt.Errorf("task %q not found", taskID)
	}
	if suggestedOwner != "" && (!strings.EqualFold(taskRecord.PreferredRole, suggestedOwner) || !strings.EqualFold(strings.TrimSpace(taskRecord.PreferredAgent), strings.TrimSpace(suggestedAgent))) {
		if _, err := taskService.AssignOwner(taskID, suggestedOwner, suggestedAgent); err != nil {
			return false, unresolvedCount, err
		}
		changed = true
	}
	if taskRecord.Status == "InProgress" {
		if _, err := taskService.Update(taskID, task.UpdateParams{Status: "needs-attention"}); err != nil {
			return false, unresolvedCount, err
		}
		changed = true
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return false, unresolvedCount, err
	}
	defer stepDB.Close()

	stepRecord, err := stepService.Get(reviewStepID)
	if err != nil {
		return false, unresolvedCount, err
	}
	if stepRecord == nil {
		return false, unresolvedCount, fmt.Errorf("step %q not found", reviewStepID)
	}
	if suggestedOwner != "" {
		if followupStep := selectReviewFollowupStep(view.Steps, reviewStepID); followupStep != nil {
			if !strings.EqualFold(followupStep.RoleName, suggestedOwner) || !strings.EqualFold(strings.TrimSpace(followupStep.AgentName), strings.TrimSpace(suggestedAgent)) {
				if _, err := stepService.AssignOwner(followupStep.ID, suggestedOwner, suggestedAgent); err != nil {
					return false, unresolvedCount, err
				}
				changed = true
			}
		}
	}
	if stepRecord.Status == "InProgress" || stepRecord.Status == "Blocked" {
		if _, err := stepService.Update(reviewStepID, step.UpdateParams{Status: "needs-attention"}); err != nil {
			return false, unresolvedCount, err
		}
		changed = true
	}

	if !changed {
		return false, unresolvedCount, nil
	}

	view, err = r.loadTaskView(result, taskID)
	if err != nil {
		return false, unresolvedCount, err
	}
	if view == nil {
		return false, unresolvedCount, fmt.Errorf("task %q not found after review findings reconciliation", taskID)
	}
	if err := r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:        "review.findings_detected",
		Actor:       "operator",
		StepID:      reviewStepID,
		Summary:     reviewFindingsSummary(unresolvedCount, suggestedOwner, suggestedAgent),
		StateEffect: reviewFindingsStateEffect(suggestedOwner, suggestedAgent),
	}, false); err != nil {
		return false, unresolvedCount, err
	}

	return true, unresolvedCount, nil
}

func reviewFindingsSummary(unresolvedCount int, suggestedOwner, suggestedAgent string) string {
	if strings.TrimSpace(suggestedOwner) != "" {
		return fmt.Sprintf("Review findings detected with %d unresolved item(s); preferred follow-up owner is %s", unresolvedCount, reviewOwnerHintDisplay(buildReviewOwnerHint(suggestedOwner, suggestedAgent), false))
	}
	return fmt.Sprintf("Review findings detected with %d unresolved item(s)", unresolvedCount)
}

func reviewFindingsStateEffect(suggestedOwner, suggestedAgent string) string {
	if strings.TrimSpace(suggestedOwner) != "" {
		return fmt.Sprintf("Task and review step moved to NeedsAttention; preferred owner and follow-up step hint reset to %s", reviewOwnerHintDisplay(buildReviewOwnerHint(suggestedOwner, suggestedAgent), false))
	}
	return "Task and review step moved to NeedsAttention"
}

func selectReviewFollowupStep(steps []step.Record, reviewStepID string) *step.Record {
	for i := len(steps) - 1; i >= 0; i-- {
		item := steps[i]
		if item.ID == reviewStepID || item.StepType == "review" {
			continue
		}
		switch item.Status {
		case "Canceled", "Skipped":
			continue
		default:
			stepCopy := item
			return &stepCopy
		}
	}
	return nil
}

func (r Runner) prepareReviewState(result *project.OpenResult, taskID, stepID string) error {
	if result == nil {
		return fmt.Errorf("project context is required")
	}
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(stepID) == "" {
		return fmt.Errorf("review state requires task and step identifiers")
	}

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	taskRecord, err := taskService.Get(taskID)
	if err != nil {
		return err
	}
	if taskRecord == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if taskRecord.Status == "Planned" {
		if _, err := taskService.Update(taskID, task.UpdateParams{Status: "ready"}); err != nil {
			return err
		}
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	stepRecord, err := stepService.Get(stepID)
	if err != nil {
		return err
	}
	if stepRecord == nil {
		return fmt.Errorf("step %q not found", stepID)
	}
	if stepRecord.Status == "Confirmed" || stepRecord.Status == "Blocked" || stepRecord.Status == "NeedsAttention" {
		if _, err := stepService.Update(stepID, step.UpdateParams{Status: "ready"}); err != nil {
			return err
		}
	}

	return nil
}

func (r Runner) activateReviewState(result *project.OpenResult, taskID, stepID string) error {
	if result == nil {
		return fmt.Errorf("project context is required")
	}
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(stepID) == "" {
		return fmt.Errorf("review activation requires task and step identifiers")
	}

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	taskRecord, err := taskService.Get(taskID)
	if err != nil {
		return err
	}
	if taskRecord == nil {
		return fmt.Errorf("task %q not found", taskID)
	}
	if taskRecord.Status == "Ready" {
		if _, err := taskService.Update(taskID, task.UpdateParams{Status: "in-progress"}); err != nil {
			return err
		}
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	stepRecord, err := stepService.Get(stepID)
	if err != nil {
		return err
	}
	if stepRecord == nil {
		return fmt.Errorf("step %q not found", stepID)
	}
	if stepRecord.Status == "Ready" {
		if _, err := stepService.Update(stepID, step.UpdateParams{Status: "in-progress"}); err != nil {
			return err
		}
	}

	return nil
}

func (r Runner) ensureReviewStep(result *project.OpenResult, view *taskView, reviewerAgent *agent.Record) (*step.Record, error) {
	if result == nil || view == nil || reviewerAgent == nil {
		return nil, fmt.Errorf("review step context is required")
	}

	if existing := findReusableReviewStep(view.Steps); existing != nil {
		stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
		if err != nil {
			return nil, err
		}
		defer stepDB.Close()

		updated, err := stepService.AssignOwner(existing.ID, reviewerAgent.Role, reviewerAgent.Name)
		if err != nil {
			return nil, err
		}
		if updated.Status == "Confirmed" || updated.Status == "Blocked" || updated.Status == "NeedsAttention" {
			return stepService.Update(updated.ID, step.UpdateParams{Status: "ready"})
		}
		return updated, nil
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer stepDB.Close()

	return stepService.Create(step.CreateParams{
		ProjectID:    view.Task.ProjectID,
		TaskID:       view.Task.ID,
		StepType:     "review",
		Title:        "Review " + view.Task.Title,
		Status:       "ready",
		RoleName:     reviewerAgent.Role,
		AgentName:    reviewerAgent.Name,
		Dependencies: reviewStepDependencies(view.Steps),
	})
}

func (r Runner) resolveReviewSession(
	result *project.OpenResult,
	taskID string,
	stepID string,
	reviewerAgent *agent.Record,
	launchMode aomruntime.LaunchMode,
) (*session.Record, bool, error) {
	if reusable, err := r.findReusableReviewSession(result, taskID, reviewerAgent.Name); err != nil {
		return nil, false, err
	} else if reusable != nil {
		return reusable, true, nil
	}

	record, err := r.executeResolvedSessionSpawn(result, reviewerAgent, sessionSpawnParams{
		agentName:  reviewerAgent.Name,
		taskID:     taskID,
		stepID:     stepID,
		launchMode: launchMode,
	})
	if err != nil {
		return nil, false, err
	}
	return record, false, nil
}

func (r Runner) findReusableReviewSession(result *project.OpenResult, taskID, agentName string) (*session.Record, error) {
	if result == nil {
		return nil, fmt.Errorf("project context is required")
	}
	sessions, err := r.loadProjectSessions(result)
	if err != nil {
		return nil, err
	}
	for _, item := range sessions {
		if item.TaskID != taskID || item.AgentName != agentName {
			continue
		}
		if strings.TrimSpace(item.TmuxSessionName) == "" || strings.TrimSpace(item.TmuxPane) == "" {
			continue
		}
		switch item.Status {
		case "Idle", "WaitingHandoff":
			recordCopy := item
			return &recordCopy, nil
		}
	}
	return nil, nil
}

func findReusableReviewStep(steps []step.Record) *step.Record {
	for i := len(steps) - 1; i >= 0; i-- {
		item := steps[i]
		if item.StepType != "review" {
			continue
		}
		switch item.Status {
		case "Ready", "InProgress", "Confirmed", "Blocked", "NeedsAttention":
			stepCopy := item
			return &stepCopy
		}
	}
	return nil
}

func reviewStepDependencies(steps []step.Record) []string {
	for i := len(steps) - 1; i >= 0; i-- {
		item := steps[i]
		if item.StepType == "review" {
			continue
		}
		switch item.Status {
		case "Completed", "InProgress", "Ready", "Confirmed", "Blocked", "NeedsAttention":
			return []string{item.ID}
		}
	}
	return nil
}

func (r Runner) transferHandoffOwnership(result *project.OpenResult, view *taskView, roleName, agentName string) error {
	if result == nil || view == nil {
		return fmt.Errorf("handoff ownership context is required")
	}

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	updatedTask, err := taskService.AssignOwner(view.Task.ID, roleName, agentName)
	if err != nil {
		return err
	}
	if shouldResetRoleOnlyHandoffStatus(agentName, updatedTask.Status) {
		if _, err := taskService.Update(view.Task.ID, task.UpdateParams{Status: "ready"}); err != nil {
			return err
		}
	}

	activeStep := selectHandoffStep(view.Steps)
	if activeStep == nil {
		return nil
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	updatedStep, err := stepService.AssignOwner(activeStep.ID, roleName, agentName)
	if err != nil {
		return err
	}
	if shouldResetRoleOnlyHandoffStatus(agentName, updatedStep.Status) {
		if _, err := stepService.Update(activeStep.ID, step.UpdateParams{Status: "ready"}); err != nil {
			return err
		}
	}
	return nil
}

func selectHandoffStep(steps []step.Record) *step.Record {
	for _, status := range []string{"InProgress", "Ready", "Confirmed", "Proposed", "Blocked", "NeedsAttention"} {
		for _, item := range steps {
			if item.Status == status {
				stepCopy := item
				return &stepCopy
			}
		}
	}
	return nil
}

func ownershipSummary(roleName, agentName string) string {
	if strings.TrimSpace(agentName) != "" {
		return fmt.Sprintf("%s (%s)", agentName, roleName)
	}
	return roleName
}

func shouldResetRoleOnlyHandoffStatus(agentName, status string) bool {
	if strings.TrimSpace(agentName) != "" {
		return false
	}
	switch strings.TrimSpace(status) {
	case "InProgress", "Blocked":
		return true
	default:
		return false
	}
}

func emptyFallback(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}

	return value
}

func taskArtifactRoot(repoPath, stateDir, taskID string, mapping *worktree.Record) string {
	if mapping != nil && strings.TrimSpace(mapping.WorktreePath) != "" {
		switch mapping.Status {
		case worktree.StatusReady, worktree.StatusActive:
			return filepath.Join(mapping.WorktreePath, ".agent")
		}
	}

	return filepath.Join(repoPath, ".aom", stateDir, taskID)
}

func taskArtifactLogPath(repoPath, stateDir, taskID string, mapping *worktree.Record) string {
	return filepath.Join(taskArtifactRoot(repoPath, stateDir, taskID, mapping), "log.md")
}

func taskHandoffPath(repoPath, stateDir, taskID string, mapping *worktree.Record) string {
	return filepath.Join(taskArtifactRoot(repoPath, stateDir, taskID, mapping), "handoff.md")
}

func setLaunchMode(current *aomruntime.LaunchMode, next aomruntime.LaunchMode) error {
	if current == nil {
		return fmt.Errorf("launch mode target is required")
	}
	if *current == "" || *current == aomruntime.LaunchModePlaceholder {
		*current = next
		return nil
	}
	if *current == next {
		return nil
	}

	return fmt.Errorf("--mock and --real cannot be used together")
}

func formatDependencies(values []string) string {
	if len(values) == 0 {
		return "-"
	}

	return strings.Join(values, ",")
}

func parseCommaSeparatedValues(value string) []string {
	parts := strings.Split(value, ",")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		normalized = append(normalized, item)
	}
	return normalized
}

func isTTYReader(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(file.Fd()) || isatty.IsCygwinTerminal(file.Fd())
}

func buildPlanStepSeeds(steps []plan.StepProposal) []task.StepSeed {
	if len(steps) == 0 {
		return nil
	}

	seeds := make([]task.StepSeed, 0, len(steps))
	for _, item := range steps {
		seed := task.StepSeed{
			Type:      item.Type,
			Title:     item.Title,
			RoleName:  item.RoleName,
			AgentName: item.AgentName,
		}
		seeds = append(seeds, seed)
	}
	return seeds
}

func activeStepID(steps []step.Record) string {
	for _, item := range steps {
		switch item.Status {
		case "InProgress", "Ready", "Confirmed", "Blocked", "NeedsAttention", "Proposed":
			return item.ID
		}
	}
	return ""
}

func activeStepTitle(steps []step.Record) string {
	for _, item := range steps {
		switch item.Status {
		case "InProgress", "Ready", "Confirmed", "Blocked", "NeedsAttention", "Proposed":
			return item.Title
		}
	}
	return ""
}

func changedFilesSummary(worktreePath, repoPath string) string {
	target := strings.TrimSpace(worktreePath)
	if target == "" {
		target = strings.TrimSpace(repoPath)
	}
	if target == "" {
		return "unavailable in current milestone"
	}

	if _, err := exec.LookPath("git"); err != nil {
		return "unavailable in current milestone"
	}

	output, err := exec.Command("git", "-C", target, "status", "--short").CombinedOutput()
	if err != nil {
		return "unavailable in current milestone"
	}
	lines := strings.FieldsFunc(strings.TrimSpace(string(output)), func(r rune) bool { return r == '\n' })
	if len(lines) == 0 || strings.TrimSpace(string(output)) == "" {
		return "no local changes detected"
	}
	if len(lines) == 1 {
		return strings.TrimSpace(lines[0])
	}
	return fmt.Sprintf("%d changed paths", len(lines))
}

func newCheckpointID() string {
	return "CHK-" + fmt.Sprintf("%d", time.Now().UnixNano())
}

func resolveHandoffTarget(result *project.OpenResult, target string) (role string, agentName string, suggestedRuntime string, err error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", "", "", fmt.Errorf("handoff target is required")
	}

	for _, agentRecord := range result.Agents {
		if agentRecord.Name == target {
			return agentRecord.Role, agentRecord.Name, agentRecord.Runtime, nil
		}
	}
	if _, ok := result.RoleConfigs[target]; ok {
		return target, "", "", nil
	}

	return "", "", "", fmt.Errorf("handoff target %q did not match a known agent or role", target)
}

func handoffNextAction(taskID, role, agentName string) string {
	if strings.TrimSpace(agentName) != "" {
		return fmt.Sprintf("run \"aom session spawn %s --task %s\" to continue from the prepared handoff", agentName, taskID)
	}
	return fmt.Sprintf("choose an agent for role %s and spawn it against task %s", role, taskID)
}

func sendableSessionStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "Booting", "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Blocked":
		return true
	default:
		return false
	}
}

func (r Runner) ensurePlannedWorktree(result *project.OpenResult, taskRecord task.Record) (*worktree.Record, error) {
	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer worktreeDB.Close()

	record, err := worktreeService.CreatePlanned(worktree.CreateParams{
		ProjectID:     result.Project.ID,
		TaskID:        taskRecord.ID,
		TaskTitle:     taskRecord.Title,
		RepoPath:      result.Project.RepoPath,
		DefaultBranch: result.Project.DefaultBranch,
	})
	if err != nil {
		return nil, err
	}

	return worktreeService.EnsureProvisioned(record.TaskID, result.Project.RepoPath)
}

func (r Runner) validateTaskProvisioning(result *project.OpenResult) error {
	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer worktreeDB.Close()

	return worktreeService.ValidateProvisioningPreconditions(
		result.Project.RepoPath,
		result.Project.DefaultBranch,
	)
}

func (r Runner) enforceWriterWorktreeBoundary(result *project.OpenResult, agentRecord *agent.Record, params sessionSpawnParams) error {
	if result == nil || agentRecord == nil {
		return nil
	}
	if strings.TrimSpace(params.taskID) == "" {
		return nil
	}
	if roleWorktreeMode(result, agentRecord.Role) != "dedicated-writer" {
		return nil
	}

	sessions, err := r.loadProjectSessions(result)
	if err != nil {
		return err
	}

	for _, item := range sessions {
		if item.TaskID != params.taskID {
			continue
		}
		if strings.TrimSpace(params.ignoreSessionID) != "" && item.ID == params.ignoreSessionID {
			continue
		}
		if roleWorktreeMode(result, item.RoleName) != "dedicated-writer" {
			continue
		}
		if !writerSessionOccupiesWorktree(item.Status) {
			continue
		}
		return fmt.Errorf(
			"task %q already has active writer session %q (%s, status=%s); run \"aom session replace %s --agent <name> --reason <why>\" to hand off, or \"aom session stop %s\" to stop it first",
			params.taskID,
			item.ID,
			item.AgentName,
			item.Status,
			item.ID,
			item.ID,
		)
	}

	return nil
}

func findAgentByName(agents []agent.Record, name string) *agent.Record {
	for i := range agents {
		if agents[i].Name == name {
			return &agents[i]
		}
	}
	return nil
}

func roleWorktreeMode(result *project.OpenResult, roleName string) string {
	if result == nil {
		return ""
	}
	role, ok := result.RoleConfigs[strings.TrimSpace(roleName)]
	if !ok {
		return ""
	}
	return strings.TrimSpace(role.WorktreeMode)
}

func writerSessionOccupiesWorktree(status string) bool {
	switch strings.TrimSpace(status) {
	case "Created", "Booting", "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Blocked":
		return true
	default:
		return false
	}
}

func (r Runner) resolveTaskExecutionPath(result *project.OpenResult, taskRecord task.Record) (*worktree.Record, string, error) {
	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return nil, "", err
	}
	defer worktreeDB.Close()

	mapping, err := worktreeService.GetByTask(taskRecord.ID)
	if err != nil {
		return nil, "", err
	}
	if mapping == nil {
		return nil, result.Project.RepoPath, nil
	}
	if mapping, err = worktreeService.Reconcile(mapping.TaskID, result.Project.RepoPath, false); err != nil {
		return nil, "", err
	}
	if mapping != nil && mapping.Status == worktree.StatusNeedsRepair {
		driftKind, err := worktreeService.DriftKind(taskRecord.ID, result.Project.RepoPath)
		if err != nil {
			return nil, "", err
		}
		return mapping, "", fmt.Errorf("worktree for task %q needs repair: %s", taskRecord.ID, worktreeHint(taskRecord.ID, mapping, driftKind))
	}
	if mapping.Status != worktree.StatusReady && mapping.Status != worktree.StatusActive {
		mapping, err = worktreeService.EnsureProvisioned(mapping.TaskID, result.Project.RepoPath)
		if err != nil {
			return nil, "", err
		}
	}
	if mapping == nil || (mapping.Status != worktree.StatusReady && mapping.Status != worktree.StatusActive) || strings.TrimSpace(mapping.WorktreePath) == "" {
		return mapping, result.Project.RepoPath, nil
	}

	return mapping, mapping.WorktreePath, nil
}

// materializeAgentContext consolidates all three spawn-time context injections:
// identity file, role skill files, and MCP server config. Both spawn sites call
// this so they cannot diverge.
func materializeAgentContext(result *project.OpenResult, agentRecord *agent.Record, worktreePath string) error {
	profilePath := filepath.Join(result.Project.RepoPath, ".aom", "agents", agentRecord.Name, "profile.md")
	if err := artifact.MaterializeIdentityFile(agentRecord.Name, agentRecord.Runtime, worktreePath, profilePath); err != nil {
		return fmt.Errorf("materialize identity file: %w", err)
	}

	roleRes := result.Resources.ResourcesForRole(agentRecord.Role, agentRecord.Runtime)

	if err := artifact.MaterializeSkillFiles(agentRecord.Name, roleRes.Skills, result.Project.RepoPath, worktreePath); err != nil {
		return fmt.Errorf("materialize skill files: %w", err)
	}

	if err := artifact.MaterializeMCPConfig(agentRecord.Name, agentRecord.Runtime, roleRes.MCPServers, worktreePath); err != nil {
		return fmt.Errorf("materialize mcp config: %w", err)
	}

	if err := artifact.MaterializePolicyConstraints(agentRecord.Name, agentRecord.Runtime, result.Policy.Policy.DenyCommands, worktreePath); err != nil {
		return fmt.Errorf("materialize policy constraints: %w", err)
	}

	return nil
}

// enforcePolicyDefaults surfaces policy information at spawn time.
// Full command interception is deferred to M10 (runtime adapter layer).
func (r Runner) enforcePolicyDefaults(result *project.OpenResult) {
	policy := result.Policy.Policy
	if policy.SessionDefaults.YoloMode == "enabled" {
		fmt.Fprintln(r.stderr, "Warning: project policy has yolo_mode=enabled — agent runs without approval gates")
	}
}

func recommendTaskAction(status string, steps []step.Record, mapping *worktree.Record, driftKind string, unresolvedReviewItems int, reviewOwnerHint string, reviewOwnerAmbiguous bool) string {
	if mapping != nil && mapping.Status == worktree.StatusNeedsRepair {
		switch driftKind {
		case worktree.DriftMissingPath:
			return "recreate the missing task worktree before continuing"
		case worktree.DriftUnregisteredArtifactOnlyPath:
			return "run worktree repair to recreate the unregistered task worktree"
		case worktree.DriftUnregisteredDirtyPath:
			return "inspect the existing task worktree path and clean it up manually before continuing"
		default:
			return "repair the task worktree before continuing"
		}
	}
	if status == "Done" {
		return "task is closed; archive later if needed"
	}
	if status == "NeedsAttention" {
		return "operator review is needed before work continues"
	}
	if unresolvedReviewItems > 0 {
		if reviewOwnerAmbiguous {
			return "review findings have mixed owners; operator must choose the follow-up owner before continuing"
		}
		if strings.TrimSpace(reviewOwnerHint) != "" {
			return fmt.Sprintf("address unresolved review items and route follow-up work to %s", reviewOwnerHint)
		}
		return "address unresolved review items before continuing implementation"
	}
	for _, item := range steps {
		if item.Status == "NeedsAttention" {
			return "resolve the step that needs operator attention"
		}
	}
	for _, item := range steps {
		if item.Status == "Blocked" {
			return "unblock the blocked step or move it to NeedsAttention"
		}
	}
	if status == "Planned" && len(steps) > 0 && steps[0].Status == "Proposed" {
		return "confirm the proposed step and move the task to Ready"
	}
	for _, item := range steps {
		if item.Status == "Ready" {
			return fmt.Sprintf("start step %s", item.ID)
		}
		if item.Status == "InProgress" {
			return fmt.Sprintf("continue step %s", item.ID)
		}
	}

	return "inspect steps and choose the next operator action"
}

func unresolvedReviewItemsForTask(repoPath, stateDir, taskID string, mapping *worktree.Record) int {
	return artifact.CountUnresolvedReviewItems(filepath.Join(taskArtifactRoot(repoPath, stateDir, taskID, mapping), "review-notes.md"))
}

func reviewOwnerHintForTask(repoPath, stateDir, taskID string, mapping *worktree.Record) (string, bool) {
	return artifact.ReviewOwnerHint(filepath.Join(taskArtifactRoot(repoPath, stateDir, taskID, mapping), "review-notes.md"))
}

func reviewOwnerHintDisplay(owner string, ambiguous bool) string {
	if ambiguous {
		return "mixed owners - operator must choose"
	}
	if strings.TrimSpace(owner) == "" {
		return "-"
	}
	return owner
}

func buildReviewOwnerHint(roleName, agentName string) string {
	roleName = strings.TrimSpace(roleName)
	agentName = strings.TrimSpace(agentName)
	if roleName == "" {
		return ""
	}
	if agentName == "" {
		return roleName
	}
	return fmt.Sprintf("%s (%s)", agentName, roleName)
}

func resolveRoleHintAgent(agents []agent.Record, roleName string) string {
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return ""
	}

	matches := make([]string, 0, 1)
	for _, item := range agents {
		if !item.Enabled || !strings.EqualFold(item.Role, roleName) {
			continue
		}
		matches = append(matches, item.Name)
		if len(matches) > 1 {
			return ""
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}

func (r Runner) loadTaskViews(result *project.OpenResult, sessions []session.Record) ([]taskView, error) {
	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer taskDB.Close()

	tasks, err := taskService.ListByProject(result.Project.ID)
	if err != nil {
		return nil, err
	}

	if len(tasks) == 0 {
		return nil, nil
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer stepDB.Close()

	views := make([]taskView, 0, len(tasks))
	for _, item := range tasks {
		steps, err := stepService.ListByTask(item.ID)
		if err != nil {
			return nil, err
		}
		views = append(views, taskView{
			Task:  item,
			Steps: steps,
		})
	}

	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer worktreeDB.Close()

	for i := range views {
		mapping, err := worktreeService.Reconcile(views[i].Task.ID, result.Project.RepoPath, hasActiveTaskSession(sessions, views[i].Task.ID))
		if err != nil {
			return nil, err
		}
		if mapping == nil {
			mapping, err = worktreeService.GetByTask(views[i].Task.ID)
			if err != nil {
				return nil, err
			}
		}
		views[i].Worktree = mapping
		if mapping != nil {
			views[i].WorktreeDrift, err = worktreeService.DriftKind(views[i].Task.ID, result.Project.RepoPath)
			if err != nil {
				return nil, err
			}
		}
		views[i].UnresolvedReviewItems = unresolvedReviewItemsForTask(result.Project.RepoPath, result.StateDir, views[i].Task.ID, views[i].Worktree)
		rawReviewOwnerHint, ambiguous := reviewOwnerHintForTask(result.Project.RepoPath, result.StateDir, views[i].Task.ID, views[i].Worktree)
		views[i].ReviewOwnerHint = buildReviewOwnerHint(rawReviewOwnerHint, resolveRoleHintAgent(result.Agents, rawReviewOwnerHint))
		views[i].ReviewOwnerAmbiguous = ambiguous
	}

	return views, nil
}

func worktreeHint(taskID string, mapping *worktree.Record, driftKind string) string {
	if mapping == nil {
		return ""
	}

	switch mapping.Status {
	case worktree.StatusNeedsRepair:
		switch driftKind {
		case worktree.DriftMissingPath:
			return fmt.Sprintf("run \"aom worktree repair %s\" to recreate the missing git worktree path before continuing", taskID)
		case worktree.DriftUnregisteredArtifactOnlyPath:
			return fmt.Sprintf("run \"aom worktree repair %s\" to recreate the unregistered worktree path; the existing path only contains AOM-owned content", taskID)
		case worktree.DriftUnregisteredDirtyPath:
			return fmt.Sprintf("inspect the existing worktree path and clean up non-artifact content manually before running \"aom worktree repair %s\"", taskID)
		default:
			return fmt.Sprintf("run \"aom worktree repair %s\" or inspect the git worktree path before continuing", taskID)
		}
	case worktree.StatusActive:
		return "task worktree is currently bound to a live session"
	default:
		return ""
	}
}

// loadBlockedBy fetches the tasks that block taskID. Errors are silently ignored
// so that artifact sync never fails due to a dependency lookup problem.
func (r Runner) loadBlockedBy(result *project.OpenResult, taskID string) []task.Record {
	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return nil
	}
	defer sqlDB.Close()

	records, err := taskService.BlockedBy(taskID)
	if err != nil {
		return nil
	}
	return records
}

func hasActiveTaskSession(sessions []session.Record, taskID string) bool {
	for _, item := range sessions {
		if item.TaskID != taskID {
			continue
		}
		switch item.Status {
		case "Booting", "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Blocked":
			return true
		}
	}
	return false
}

func (r Runner) syncTaskArtifacts(result *project.OpenResult, taskID string, event artifact.Event, seed bool) error {
	return r.syncTaskArtifactsWithSessionEvents(result, taskID, seed, nil, event)
}

func (r Runner) syncTaskArtifactsWithSession(result *project.OpenResult, taskID string, event artifact.Event, seed bool, activeSession *session.Record) error {
	return r.syncTaskArtifactsWithSessionEvents(result, taskID, seed, activeSession, event)
}

func (r Runner) refreshTaskArtifacts(result *project.OpenResult, taskID string, activeSession *session.Record) error {
	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found for artifact refresh", taskID)
	}

	service := artifact.NewService(result.Project.RepoPath, result.StateDir)
	params := artifact.SyncParams{
		Task:                  view.Task,
		Steps:                 view.Steps,
		ActiveSession:         activeSession,
		Worktree:              view.Worktree,
		CreatedBy:             "operator",
		UpdatedBy:             "aom",
		ReviewOwnerHint:       view.ReviewOwnerHint,
		ReviewOwnerAmbiguous:  view.ReviewOwnerAmbiguous,
		RecommendedNextAction: recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous),
	}
	return service.RefreshTaskArtifacts(params)
}

func (r Runner) ensureTaskHandoffTemplate(result *project.OpenResult, taskID string, activeSession *session.Record) error {
	if activeSession == nil {
		return fmt.Errorf("active session is required for handoff template seeding")
	}

	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found for handoff template seeding", taskID)
	}

	service := artifact.NewService(result.Project.RepoPath, result.StateDir)
	params := artifact.SyncParams{
		Task:                  view.Task,
		Steps:                 view.Steps,
		ActiveSession:         activeSession,
		Worktree:              view.Worktree,
		CreatedBy:             "operator",
		UpdatedBy:             "aom",
		ReviewOwnerHint:       view.ReviewOwnerHint,
		ReviewOwnerAmbiguous:  view.ReviewOwnerAmbiguous,
		RecommendedNextAction: recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous),
	}
	return service.EnsureHandoffTemplate(params, *activeSession)
}

func (r Runner) syncTaskArtifactsWithSessionEvents(result *project.OpenResult, taskID string, seed bool, activeSession *session.Record, events ...artifact.Event) error {
	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found for artifact sync", taskID)
	}

	service := artifact.NewService(result.Project.RepoPath, result.StateDir)
	updatedBy := "aom"
	if len(events) > 0 {
		updatedBy = events[len(events)-1].Actor
	}
	blockedByRecords := r.loadBlockedBy(result, taskID)

	params := artifact.SyncParams{
		Task:                  view.Task,
		Steps:                 view.Steps,
		ActiveSession:         activeSession,
		Worktree:              view.Worktree,
		BlockedBy:             blockedByRecords,
		CreatedBy:             "operator",
		UpdatedBy:             updatedBy,
		ReviewOwnerHint:       view.ReviewOwnerHint,
		ReviewOwnerAmbiguous:  view.ReviewOwnerAmbiguous,
		RecommendedNextAction: recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous),
	}
	if seed {
		return service.SeedTaskArtifacts(params)
	}
	if err := service.RefreshTaskArtifacts(params); err != nil {
		return err
	}
	return service.AppendEvents(params, events)
}

func (r Runner) writeHandoffArtifact(result *project.OpenResult, view *taskView, sessionRecord session.Record, targetRole, targetAgent, suggestedRuntime, reason string) error {
	if result == nil || view == nil {
		return fmt.Errorf("handoff artifact context is required")
	}

	path := taskHandoffPath(result.Project.RepoPath, result.StateDir, view.Task.ID, view.Worktree)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create handoff dir: %w", err)
	}

	content := fmt.Sprintf(`# Handoff

## Transfer
- From Role: %s
- From Agent: %s
- From Session: %s
- From Runtime: %s
- To Role: %s
- Suggested Runtime: %s
- Task: %s
- Step: %s
- Reason: %s

## Completed
- Review current state in state.md before continuing
- Continue from the current worktree and latest task log

## Remaining
- Continue the assigned step from the latest task context
- Validate the current diff and task state before making new changes

## Touched Files
- %s

## Constraints
- Preserve the current task scope and worktree continuity
- Start from AOM artifacts rather than transcript memory alone

## Warnings
- Review unresolved notes in log.md and state.md before continuing

## Exact Next Action
%s

## Do Not Redo
- Do not restart the task from scratch
- Do not ignore the existing worktree and artifact context
`,
		sessionRecord.RoleName,
		sessionRecord.AgentName,
		sessionRecord.ID,
		sessionRecord.Runtime,
		targetRole,
		emptyFallback(suggestedRuntime),
		view.Task.ID,
		emptyFallback(activeStepID(view.Steps)),
		reason,
		changedFilesSummary(sessionRecord.WorktreePath, sessionRecord.RepoPath),
		handoffNextAction(view.Task.ID, targetRole, targetAgent),
	)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write handoff.md: %w", err)
	}

	return nil
}

func (r Runner) loadTaskByID(result *project.OpenResult, taskID string) (*task.Record, error) {
	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer taskDB.Close()

	return taskService.Get(taskID)
}

func (r Runner) loadStepByID(result *project.OpenResult, stepID string) (*step.Record, error) {
	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer stepDB.Close()

	return stepService.Get(stepID)
}

func (r Runner) loadTaskView(result *project.OpenResult, taskID string) (*taskView, error) {
	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer taskDB.Close()

	record, err := taskService.Get(taskID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer stepDB.Close()

	steps, err := stepService.ListByTask(record.ID)
	if err != nil {
		return nil, err
	}

	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer worktreeDB.Close()

	sessions, err := r.loadProjectSessions(result)
	if err != nil {
		return nil, err
	}

	mapping, err := worktreeService.Reconcile(record.ID, result.Project.RepoPath, hasActiveTaskSession(sessions, record.ID))
	if err != nil {
		return nil, err
	}
	if mapping == nil {
		mapping, err = worktreeService.GetByTask(record.ID)
		if err != nil {
			return nil, err
		}
	}
	driftKind := worktree.DriftNone
	if mapping != nil {
		driftKind, err = worktreeService.DriftKind(record.ID, result.Project.RepoPath)
		if err != nil {
			return nil, err
		}
	}

	return &taskView{
		Task:          *record,
		Steps:         steps,
		Worktree:      mapping,
		WorktreeDrift: driftKind,
		UnresolvedReviewItems: unresolvedReviewItemsForTask(
			result.Project.RepoPath,
			result.StateDir,
			record.ID,
			mapping,
		),
		ReviewOwnerHint: func() string {
			owner, _ := reviewOwnerHintForTask(result.Project.RepoPath, result.StateDir, record.ID, mapping)
			return buildReviewOwnerHint(owner, resolveRoleHintAgent(result.Agents, owner))
		}(),
		ReviewOwnerAmbiguous: func() bool {
			_, ambiguous := reviewOwnerHintForTask(result.Project.RepoPath, result.StateDir, record.ID, mapping)
			return ambiguous
		}(),
	}, nil
}

func mapTaskEventType(status, mode string) string {
	if strings.TrimSpace(mode) != "" {
		return "task.mode_changed"
	}
	if strings.EqualFold(strings.TrimSpace(status), "done") {
		return "task.closed"
	}
	return "task.updated"
}

func mapStepEventType(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "confirmed":
		return "step.confirmed"
	case "completed":
		return "step.completed"
	default:
		return "step.updated"
	}
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
		return r.reconcileSessionRecord(sessionService, *record)
	}

	sessions, err := sessionService.ListByProject(result.Project.ID)
	if err != nil {
		return nil, err
	}

	for _, item := range sessions {
		if item.AgentName == identifier {
			return r.reconcileSessionRecord(sessionService, item)
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

	sessions, err := sessionService.ListByProject(result.Project.ID)
	if err != nil {
		return nil, err
	}

	reconciled := make([]session.Record, 0, len(sessions))
	for _, item := range sessions {
		record, err := r.reconcileSessionRecord(sessionService, item)
		if err != nil {
			return nil, err
		}
		reconciled = append(reconciled, *record)
	}

	return reconciled, nil
}

func (r Runner) reconcileSessionRecord(sessionService *session.Service, record session.Record) (*session.Record, error) {
	if sessionService == nil {
		return nil, fmt.Errorf("session service is required")
	}

	availability := r.app.Tmux.Availability()
	if !availability.Available {
		return &record, nil
	}

	paneExists, err := r.app.Tmux.PaneExists(record.TmuxPane)
	if err != nil {
		return nil, err
	}

	return sessionService.ReconcileBinding(record, paneExists)
}

func (r Runner) loadTaskCount(result *project.OpenResult) (int, error) {
	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return 0, err
	}
	defer sqlDB.Close()

	return taskService.CountByProject(result.Project.ID)
}
