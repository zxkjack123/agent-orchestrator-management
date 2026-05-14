package cli

import (
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
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/plan"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/project"
	aomruntime "github.com/lattapon-aek/Agents-Orchestfator-Management/internal/runtime"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/session"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/step"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/task"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/tmux"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/worktree"
)

var newApp = app.New
var newLaunchBuilder = aomruntime.NewBuilder

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
	case "checkpoint":
		return r.executeCheckpoint(args[1:])
	case "handoff":
		return r.executeHandoff(args[1:])
	case "open":
		return r.executeOpen(args[1:])
	case "plan":
		return r.executePlan(args[1:])
	case "review":
		return r.executeReview(args[1:])
	case "step":
		return r.executeStep(args[1:])
	case "session":
		return r.executeSession(args[1:])
	case "status":
		return r.executeStatus(args[1:])
	case "task":
		return r.executeTask(args[1:])
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
	case "show":
		return r.executeSessionShow(args[1:])
	case "replace":
		return r.executeSessionReplace(args[1:])
	case "stop":
		return r.executeSessionStop(args[1:])
	case "archive":
		return r.executeSessionArchive(args[1:])
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

func (r Runner) executeWorktree(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("worktree subcommand is required")
	}

	switch args[0] {
	case "repair":
		return r.executeWorktreeRepair(args[1:])
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
	name          string
	repo          string
	defaultBranch string
	sessionPrefix string
	templateName  string
	templateDir   string
}

func (p projectInitParams) toInitParams() project.InitParams {
	return project.InitParams{
		Name:          p.name,
		RepoPath:      p.repo,
		DefaultBranch: p.defaultBranch,
		SessionPrefix: p.sessionPrefix,
		TemplateName:  p.templateName,
		TemplateDir:   p.templateDir,
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

	createResult, err := taskService.Create(task.CreateParams{
		ProjectID:      result.Project.ID,
		Title:          params.title,
		Mode:           params.mode,
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
	title string
	mode  string
	role  string
	agent string
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
	id     string
	mode   string
	status string
	role   string
	agent  string
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

	launchCommand, err := newLaunchBuilder().Build(aomruntime.SessionSpec{
		SessionID: record.ID,
		AgentName: record.AgentName,
		RoleName:  record.RoleName,
		Runtime:   record.Runtime,
	}, params.launchMode)
	if err != nil {
		return nil, r.failTaskBoundSessionSpawn(result, sessionService, record, taskRecord, params.stepID, "session launch validation failed before session became interactive", err)
	}

	paneBinding, err := r.app.Tmux.CreatePane(workspace.Target, executionPath, launchCommand)
	if err != nil {
		return nil, r.failTaskBoundSessionSpawn(result, sessionService, record, taskRecord, params.stepID, "pane creation failed before session became interactive", err)
	}

	record.Status = "Idle"
	record.TmuxWindow = paneBinding.WindowID
	record.TmuxPane = paneBinding.PaneID
	record.TmuxSessionName = workspace.Name

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
		return fmt.Errorf("task identifier is required")
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
	fmt.Fprintln(r.stdout, "Tasks:")
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
				item.Task.Status,
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
					item.Worktree.Status,
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
					taskStep.Status,
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

func (r Runner) printHelp() {
	fmt.Fprintln(r.stdout, "AOM")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Milestone 3 workflow scaffolding is in progress.")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Planned commands:")
	fmt.Fprintln(r.stdout, "  aom project init")
	fmt.Fprintln(r.stdout, "  aom attach")
	fmt.Fprintln(r.stdout, "  aom capture")
	fmt.Fprintln(r.stdout, "  aom checkpoint")
	fmt.Fprintln(r.stdout, "  aom handoff")
	fmt.Fprintln(r.stdout, "  aom open")
	fmt.Fprintln(r.stdout, "  aom plan")
	fmt.Fprintln(r.stdout, "  aom review")
	fmt.Fprintln(r.stdout, "  aom step list")
	fmt.Fprintln(r.stdout, "  aom step update")
	fmt.Fprintln(r.stdout, "  aom session show")
	fmt.Fprintln(r.stdout, "  aom session spawn")
	fmt.Fprintln(r.stdout, "  aom session list")
	fmt.Fprintln(r.stdout, "  aom session stop")
	fmt.Fprintln(r.stdout, "  aom session archive")
	fmt.Fprintln(r.stdout, "  aom status")
	fmt.Fprintln(r.stdout, "  aom task close")
	fmt.Fprintln(r.stdout, "  aom task create")
	fmt.Fprintln(r.stdout, "  aom task show")
	fmt.Fprintln(r.stdout, "  aom task update")
	fmt.Fprintln(r.stdout, "  aom worktree repair")
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
			"task %q already has active writer session %q (%s, status=%s); stop or replace it before starting another dedicated writer",
			params.taskID,
			item.ID,
			item.AgentName,
			item.Status,
		)
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
	case "Created", "Booting", "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Blocked", "Detached":
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
	params := artifact.SyncParams{
		Task:                  view.Task,
		Steps:                 view.Steps,
		ActiveSession:         activeSession,
		Worktree:              view.Worktree,
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
