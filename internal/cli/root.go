package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/app"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/events"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/provider"
	aomruntime "github.com/lattapon-aek/agent-orchestrator-management/internal/runtime"
)

var newApp = app.New
var newLaunchBuilder = aomruntime.NewBuilder
var newRegistry = func() provider.Registry { return provider.DefaultRegistry() }

// Runner executes top-level CLI behavior.
type Runner struct {
	app      *app.App
	bus      *events.Bus
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
	isTTY    func(io.Reader) bool
	registry provider.Registry
}

// Execute runs the AOM CLI using the provided arguments and streams.
func Execute(args []string, stdout, stderr io.Writer) error {
	bus := &events.Bus{}
	bus.SubscribeAsync(hookRunnerSubscriber())
	r := Runner{
		app:      newApp(),
		bus:      bus,
		stdin:    os.Stdin,
		stdout:   stdout,
		stderr:   stderr,
		isTTY:    isTTYReader,
		registry: newRegistry(),
	}
	return r.Execute(args)
}

// Execute dispatches a command line invocation.
func (r Runner) Execute(args []string) (retErr error) {
	// Catch unexpected panics so the CLI always exits with a structured error
	// instead of a raw stack trace. Panics in handler code (e.g. nil pointer
	// dereference in step update) are surfaced as actionable messages rather
	// than crashing the process mid-operation.
	defer func() {
		if p := recover(); p != nil {
			retErr = fmt.Errorf("internal error: %v\n(this is a bug — please report it)", p)
		}
	}()

	_ = r.app

	if len(args) == 0 {
		r.printHelp()
		return nil
	}

	switch args[0] {
	case "--version", "-v":
		return r.executeVersion()
	case "help", "--help", "-h":
		r.printHelp()
		return nil
	case "version":
		return r.executeVersion()
	case "install":
		return r.executeInstall(args[1:])
	case "uninstall":
		return r.executeUninstall()
	case "update":
		return r.executeUpdate(args[1:])
	case "agent":
		return r.executeAgent(args[1:])
	case "attach":
		return r.executeAttach(args[1:])
	case "switch":
		return r.executeSwitch(args[1:])
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
	case "dashboard":
		return r.executeDashboard(args[1:])
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
	case "policy":
		return r.executePolicy(args[1:])
	case "outbox":
		return r.executeOutbox(args[1:])
	case "team":
		return r.executeTeam(args[1:])
	case "task":
		return r.executeTask(args[1:])
	case "events":
		return r.executeEvents(args[1:])
	case "watch":
		return r.executeWatch(args[1:])
	case "run-pipeline":
		return r.executeRunPipeline(args[1:])
	case "goal":
		return r.executeGoal(args[1:])
	case "orchestrate":
		return r.executeOrchestrate(args[1:])
	case "orchestrator":
		return r.executeOrchestratorMode(args[1:])
	case "worktree":
		return r.executeWorktree(args[1:])
	case "memory":
		return r.executeMemory(args[1:])
	case "claim":
		return r.executeClaim(args[1:])
	case "token-usage":
		return r.executeTokenUsage(args[1:])
	case "project":
		return r.executeProject(args[1:])
	case "role":
		return r.executeRole(args[1:])
	case "class":
		return r.executeClass(args[1:])
	case "system-template":
		return r.executeSystemTemplateShow(args[1:])
	case "serve":
		return r.executeServe(args[1:])
	default:
		return fmt.Errorf("unknown command %q", strings.Join(args, " "))
	}
}

func (r Runner) executeTask(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task subcommand is required")
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			r.printHelp()
			return nil
		}
	}
	switch args[0] {
	case "create":
		return r.executeTaskCreate(args[1:])
	case "update":
		return r.executeTaskUpdate(args[1:])
	case "close":
		return r.executeTaskClose(args[1:])
	case "accept":
		return r.executeTaskAccept(args[1:])
	case "show":
		return r.executeTaskShow(args[1:])
	case "list":
		return r.executeTaskList(args[1:])
	case "claim":
		return r.executeTaskClaim(args[1:])
	case "reanalyze":
		return r.executeTaskReanalyze(args[1:])
	case "link":
		return r.executeTaskLink(args[1:])
	case "unlink":
		return r.executeTaskUnlink(args[1:])
	case "record-result":
		return r.executeTaskRecordResult(args[1:])
	case "ready":
		return r.executeTaskReady(args[1:])
	case "request":
		return r.executeTaskRequest(args[1:])
	case "list-requests":
		return r.executeTaskListRequests(args[1:])
	case "approve-request":
		return r.executeTaskApproveRequest(args[1:])
	case "reject-request":
		return r.executeTaskRejectRequest(args[1:])
	case "cancel":
		return r.executeTaskCancel(args[1:])
	case "verify":
		return r.executeTaskVerify(args[1:])
	case "signal":
		return r.executeTaskSignal(args[1:])
	case "propose-plan":
		return r.executeTaskProposePlan(args[1:])
	case "plan-approve":
		return r.executeTaskPlanApprove(args[1:])
	case "plan-reject":
		return r.executeTaskPlanReject(args[1:])
	case "templates":
		return r.executeTaskTemplates(args[1:])
	default:
		return fmt.Errorf("unknown task command %q", strings.Join(args, " "))
	}
}

func (r Runner) executeStep(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("step subcommand is required")
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			r.printHelp()
			return nil
		}
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
	for _, a := range args {
		if a == "--help" || a == "-h" {
			r.printHelp()
			return nil
		}
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
	case "cleanup":
		return r.executeSessionCleanup(args[1:])
	case "recover":
		return r.executeSessionRecover(args[1:])
	case "watch":
		return r.executeSessionWatch(args[1:])
	default:
		return fmt.Errorf("unknown session command %q", strings.Join(args, " "))
	}
}

func (r Runner) executeProject(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("project subcommand is required")
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			r.printHelp()
			return nil
		}
	}
	switch args[0] {
	case "init":
		return r.executeProjectInit(args[1:])
	case "resources":
		return r.executeProjectResources(args[1:])
	case "share":
		return r.executeProjectShare(args[1:])
	case "layout":
		return r.executeProjectLayout()
	default:
		return fmt.Errorf("unknown project command %q", strings.Join(args, " "))
	}
}

func (r Runner) executeWorktree(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("worktree subcommand is required")
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			r.printHelp()
			return nil
		}
	}
	switch args[0] {
	case "repair":
		return r.executeWorktreeRepair(args[1:])
	case "read-file":
		return r.executeWorktreeReadFile(args[1:])
	case "commit":
		return r.executeWorktreeCommit(args[1:])
	case "prune":
		return r.executeWorktreePrune(args[1:])
	default:
		return fmt.Errorf("unknown worktree command %q", strings.Join(args, " "))
	}
}

// ── M15: merge coordination ──────────────────────────────────────────────────

func (r Runner) executeMerge(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("merge subcommand is required (check | prepare | commit)")
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			r.printHelp()
			return nil
		}
	}
	switch args[0] {
	case "check":
		return r.executeMergeCheck(args[1:])
	case "prepare":
		return r.executeMergePrepare(args[1:])
	case "commit":
		return r.executeMergeCommit(args[1:])
	case "continue":
		return r.executeMergeContinue(args[1:])
	case "abort":
		return r.executeMergeAbort(args[1:])
	default:
		return fmt.Errorf("unknown merge command %q", args[0])
	}
}

// ── M16: communication & feedback upgrade ────────────────────────────────────

func (r Runner) executeMessage(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("message subcommand is required (send | read | clear | watch | reply)")
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			r.printHelp()
			return nil
		}
	}
	switch args[0] {
	case "send":
		return r.executeMessageSend(args[1:])
	case "read":
		return r.executeMessageRead(args[1:])
	case "clear":
		return r.executeMessageClear(args[1:])
	case "watch":
		return r.executeMessageWatch(args[1:])
	case "reply":
		return r.executeMessageReply(args[1:])
	default:
		return fmt.Errorf("unknown message command %q", args[0])
	}
}

func (r Runner) executeChannel(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("channel subcommand is required: append, read")
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			r.printHelp()
			return nil
		}
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

// wrapProjectNotFound wraps errors from Projects.Open that indicate no project
// has been initialized yet, adding a hint about how to fix it.
func wrapProjectNotFound(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "no AOM project found") {
		return fmt.Errorf("%w\n\nRun `aom project init <name> --repo .` to initialise a project first, then `aom open`.", err)
	}
	return err
}

func (r Runner) printHelp() {
	fmt.Fprintln(r.stdout, "AOM is a project-local control plane for agent sessions, tasks, worktrees, and durable markdown artifacts.")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Operator workflow")
	fmt.Fprintln(r.stdout, "The operator drives the project — you can orchestrate directly or delegate to an agent.")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "  Option A: operator as orchestrator (no orchestrator agent needed)")
	fmt.Fprintln(r.stdout, "  1. aom agent add <name> --role <role> --class <class> --runtime <runtime>")
	fmt.Fprintln(r.stdout, "  2. aom task create \"work summary\" --role <role> --agent <agent>")
	fmt.Fprintln(r.stdout, "  3. aom step list <task-id> ; aom step update <step-id> --status confirmed")
	fmt.Fprintln(r.stdout, "  4. aom session spawn <agent> --task <task-id> --mock|--real")
	fmt.Fprintln(r.stdout, "  5. aom session send <session-id> \"brief for the worker\"")
	fmt.Fprintln(r.stdout, "  6. aom capture <session-id>")
	fmt.Fprintln(r.stdout, "  7. aom task close <task-id>")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "  Option B: spawn an orchestrator agent and let it manage the team")
	fmt.Fprintln(r.stdout, "  1. aom agent add orchestrator --role orchestrator --class orchestrator --runtime claude")
	fmt.Fprintln(r.stdout, "  2. aom task create \"build <feature>\" --agent orchestrator")
	fmt.Fprintln(r.stdout, "  3. aom session spawn orchestrator --task <task-id> --real")
	fmt.Fprintln(r.stdout, "     (the orchestrator agent will add sub-agents and assign tasks autonomously)")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Project")
	fmt.Fprintln(r.stdout, "aom project init <name> --repo <path> : create .aom config, db, and starter agents")
	fmt.Fprintln(r.stdout, "aom project resources : show role bindings, skills, MCP servers, and policy")
	fmt.Fprintln(r.stdout, "aom project share <file> : broadcast a file to all active agent worktrees (e.g. repo-layout.md)")
	fmt.Fprintln(r.stdout, "aom project layout : write .aom/shared/repo-layout.md and push to active worktrees")
	fmt.Fprintln(r.stdout, "aom open : load project state and reconcile tmux/worktree/session state")
	fmt.Fprintln(r.stdout, "aom status [--active] [--graph] [--json] : show project, tasks, sessions, worktrees, and next-action hints")
	fmt.Fprintln(r.stdout, "aom status --action-items : show only items that need operator attention (approvals, completions, spawns)")
	fmt.Fprintln(r.stdout, "aom dashboard [--interval <dur>] : live auto-refreshing view of sessions, action items, and team channel (default 5s)")
	fmt.Fprintln(r.stdout, "aom metrics : velocity report — task throughput, step completion rate, and agent activity from log events")
	fmt.Fprintln(r.stdout, "aom next : show the highest-priority Ready task that has no active session")
	fmt.Fprintln(r.stdout, "aom pause-all : send pause signal to all active agent sessions")
	fmt.Fprintln(r.stdout, "aom resume-all : send resume signal to all paused sessions")
	fmt.Fprintln(r.stdout, "aom serve [--port <port>] [--host <host>] : start web UI server on http://localhost:7777 (browser dashboard)")
	fmt.Fprintln(r.stdout, "aom serve stop    : stop the running web UI server")
	fmt.Fprintln(r.stdout, "aom serve restart : restart the web UI server (re-reads embedded frontend)")
	fmt.Fprintln(r.stdout, "aom version : print build metadata for the installed binary")
	fmt.Fprintln(r.stdout, "aom install [--test] [--dry] : build and install from the current checkout")
	fmt.Fprintln(r.stdout, "aom update [--test] : pull latest source and reinstall from the current checkout")
	fmt.Fprintln(r.stdout, uninstallHelpLine())
	fmt.Fprintln(r.stdout, "aom plan \"work\" [--create] : draft a task plan and optionally persist it")
	fmt.Fprintln(r.stdout, "aom doctor : validate environment (tmux, config, runtimes, db, worktrees)")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Agent")
	fmt.Fprintln(r.stdout, "aom agent list : list all configured agents with role, runtime, and profile path")
	fmt.Fprintln(r.stdout, "aom agent add <name> --role <role> --class <class> --runtime <runtime> : add agent and seed role profile")
	fmt.Fprintln(r.stdout, "  --class controls which profile template is used (.aom/templates/profiles/<class>.md.tmpl)")
	fmt.Fprintln(r.stdout, "  built-in classes: builder, frontend, reviewer, orchestrator, researcher, generic  (add your own by dropping a .md.tmpl file)")
	fmt.Fprintln(r.stdout, "aom agent show <name> : show agent config and full profile content")
	fmt.Fprintln(r.stdout, "aom agent profile show <name> : print agent profile markdown")
	fmt.Fprintln(r.stdout, "aom agent profile update <name> [--responsibilities <text>] [--constraints <text>] : update profile sections")
	fmt.Fprintln(r.stdout, "aom agent profile set-instructions <name> \"<text>\" : set custom instructions for agent (operator-controlled section; warns if agent is active)")
	fmt.Fprintln(r.stdout, "aom agent profile set-instructions <name> --file <path> : read instructions from file (use - for stdin)")
	fmt.Fprintln(r.stdout, "aom agent profile set-instructions <name> --clear : clear custom instructions")
	fmt.Fprintln(r.stdout, "aom agent set-model <name> <model> : update agent model in agents.yaml without overwriting other fields")
	fmt.Fprintln(r.stdout, "aom agent provision <name> : create permanent workspace at .aom/agents/<name>/workspace/ on branch agents/<name>")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Role & Class")
	fmt.Fprintln(r.stdout, "aom role list : list all roles defined in agents.yaml with class, worktree_mode, and assigned agents")
	fmt.Fprintln(r.stdout, "aom role show <name> : show full role config and agents using it")
	fmt.Fprintln(r.stdout, "aom role create <name> --class <class> [--worktree-mode dedicated-writer|read-only] [--checkpoint required|optional] [--description <text>] : create a new role")
	fmt.Fprintln(r.stdout, "aom role update <name> [--class <class>] [--worktree-mode <mode>] [--checkpoint <exp>] [--description <text>] : update role config")
	fmt.Fprintln(r.stdout, "aom role delete <name> : delete a role (fails if any agent is using it)")
	fmt.Fprintln(r.stdout, "aom role preview <name> [--runtime claude|codex] : render a full profile preview for a role")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "aom class list : list all class templates (built-in and custom project overrides)")
	fmt.Fprintln(r.stdout, "aom class show <name> : print the raw template content for a class")
	fmt.Fprintln(r.stdout, "aom class create <name> [--from <existing-class>] : create a new custom class template in .aom/templates/profiles/")
	fmt.Fprintln(r.stdout, "aom class edit <name> : open a custom class template in $EDITOR (use 'override' first for built-in classes)")
	fmt.Fprintln(r.stdout, "aom class override <name> : copy a built-in class to a project-level file so it can be edited")
	fmt.Fprintln(r.stdout, "aom class delete <name> : delete a custom class or revert a built-in override to the embedded default")
	fmt.Fprintln(r.stdout, "aom class preview <name> [--runtime claude|codex] [--role <role>] [--agent <name>] : render full profile preview for a class")
	fmt.Fprintln(r.stdout, "aom system-template show : print the AOM system protocol template (base.md.tmpl, read-only)")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Runtime")
	fmt.Fprintln(r.stdout, "aom runtime list : list configured runtimes with binary availability")
	fmt.Fprintln(r.stdout, "aom runtime inspect <runtime> : show runtime capabilities, agents, and resume support")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Task")
	fmt.Fprintln(r.stdout, "aom task create <title> [--role <role>] [--agent <agent>] : create a task")
	fmt.Fprintln(r.stdout, "aom task show <task-id> : inspect task state, artifacts, and ownership")
	fmt.Fprintln(r.stdout, "aom task update <task-id> [flags] : change task mode, owner, or status")
	fmt.Fprintln(r.stdout, "aom task ready <task-id> : confirm all Proposed steps and transition Planned task to Ready in one shot")
	fmt.Fprintln(r.stdout, "aom task close <task-id> : mark a task complete (task must be InProgress; all steps must be terminal)")
	fmt.Fprintln(r.stdout, "aom task cancel <task-id> : cancel a Draft/Planned/Ready task (archives it and cancels all pending steps)")
	fmt.Fprintln(r.stdout, "aom task accept <task-id> [--auto] [--interval <dur>] [--timeout <dur>] : accept agent work — complete all pending steps and close the task")
	fmt.Fprintln(r.stdout, "  --auto : poll every --interval (default 15s) until all verify checks pass, then accept automatically; timeout default 30m")
	fmt.Fprintln(r.stdout, "aom task verify <task-id> [--watch] [--interval <dur>] [--timeout <dur>] : check completion readiness; --watch polls until all checks pass")
	fmt.Fprintln(r.stdout, "aom task signal <type> --task <id> [--summary <text>] [--step <step-id>] : record a signal event (task.completed, handoff.prepared, checkpoint.created, step.completed, escalation.required)")
	fmt.Fprintln(r.stdout, "aom task list [--status <status>] [--role <role>] [--agent <agent>] : list tasks with optional filters")
	fmt.Fprintln(r.stdout, "aom task link <task-id> --blocked-by <blocker-id> : declare that task-id cannot start until blocker-id is done")
	fmt.Fprintln(r.stdout, "aom task unlink <task-id> --blocked-by <blocker-id> : remove a dependency edge")
	fmt.Fprintln(r.stdout, "aom task record-result <task-id> \"<summary>\" : record a task outcome or result summary to the artifact log")
	fmt.Fprintln(r.stdout, "aom task request \"<description>\" [--agent <name>] : submit a task request (agent self-service); operator approves or rejects")
	fmt.Fprintln(r.stdout, "aom task list-requests : list pending task requests from agents awaiting operator approval")
	fmt.Fprintln(r.stdout, "aom task approve-request <id> : approve a pending agent task request and create the task")
	fmt.Fprintln(r.stdout, "aom task reject-request <id> [--reason <why>] : reject a pending agent task request")
	fmt.Fprintln(r.stdout, "aom review <task-id> [--mock|--real] : prepare or start review flow")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Step")
	fmt.Fprintln(r.stdout, "aom step list <task-id> [--ids-only] : list task steps; --ids-only prints one step ID per line for scripting")
	fmt.Fprintln(r.stdout, "aom step update <step-id> --status <status> : advance one step explicitly")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Session")
	fmt.Fprintln(r.stdout, "aom session spawn <agent> [--task <task-id>] [--mock|--real] [--fresh] : start a worker session")
	fmt.Fprintln(r.stdout, "  --fresh : force a new context even when a previous native session exists for this task")
	fmt.Fprintln(r.stdout, "aom session send <session-id> <message>        : deliver a prompt into a live session")
	fmt.Fprintln(r.stdout, "aom session send <session-id> --file <path>    : deliver a prompt from file (mutually exclusive with inline message; avoids shell escaping)")

	fmt.Fprintln(r.stdout, "aom session list [--active] : list known sessions (--active shows only running sessions)")
	fmt.Fprintln(r.stdout, "aom session show <session-id> : inspect one session and its bindings")
	fmt.Fprintln(r.stdout, "aom session stop <session-id> : stop a live session and keep continuity state")
	fmt.Fprintln(r.stdout, "aom session stop --all        : stop every active session in the project")
	fmt.Fprintln(r.stdout, "aom session archive <session-id> : archive an inactive session record")
	fmt.Fprintln(r.stdout, "aom session resume <session-id> --task <task-id> : rebind an Idle or WaitingHandoff session to a new task (reuses native context)")
	fmt.Fprintln(r.stdout, "aom session replace <session-id> --agent <agent> --reason <why> [--mock|--real] : spawn a replacement in the same context")
	fmt.Fprintln(r.stdout, "aom session set-agent-id <session-id> <native-id> : register the agent CLI's own session ID for resume on next spawn")
	fmt.Fprintln(r.stdout, "aom session wait <session-id> --event <type> [--timeout 30m] : block until event appears in task log (e.g. handoff.prepared, task.completed)")
	fmt.Fprintln(r.stdout, "aom session watch [--auto-spawn] [--grid] [--interval <dur>] [--timeout <dur>] [--real|--mock] : poll for Ready tasks with no session; --auto-spawn spawns them automatically; --grid places new sessions in team window")
	fmt.Fprintln(r.stdout, "aom session health <session-id> : diagnose session health and print recommended recovery action")
	fmt.Fprintln(r.stdout, "aom session recover <session-id> : diagnose and recover a failed or stuck session")
	fmt.Fprintln(r.stdout, "aom session isolate <session-id> : move agent pane from shared team window to its own dedicated session")
	fmt.Fprintln(r.stdout, "aom session cleanup --stale [--dry-run] : remove orphan policy wrapper dirs and capture state files for inactive sessions")
	fmt.Fprintln(r.stdout, "aom task reanalyze <task-id> : refresh task artifacts from current state and print recommended next action")
	fmt.Fprintln(r.stdout, "aom capture <session-id>                        : read worker output through AOM")
	fmt.Fprintln(r.stdout, "aom capture --all                               : capture every active session in one view")
	fmt.Fprintln(r.stdout, "aom capture --all --summary                     : signal-only summary for each active session")
	fmt.Fprintln(r.stdout, "aom capture --all --follow [--interval <dur>]   : stream new lines from all sessions (default interval 2s)")
	fmt.Fprintln(r.stdout, "aom attach <session-id>   : attach manually and log operator intervention")
	fmt.Fprintln(r.stdout, "aom switch <agent-name>   : jump to an agent's live session by name (no session-id needed); logs operator.intervention automatically")
	fmt.Fprintln(r.stdout, "aom checkpoint <session-id> : refresh task artifacts and record a checkpoint")
	fmt.Fprintln(r.stdout, "aom handoff <session-id> --to <role-or-agent> [--reason <why>] : prepare handoff state")
	fmt.Fprintln(r.stdout, "aom approve <session-id> : approve a pending WaitingApproval session request")
	fmt.Fprintln(r.stdout, "aom deny <session-id> [--reason <why>] : deny a pending WaitingApproval session request")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Team collaboration")
	fmt.Fprintln(r.stdout, "aom watch [--task <task-id>] [--event <type>] [--timeout 30m] : stream log events across all active tasks (or one task with --task)")
	fmt.Fprintln(r.stdout, "aom orchestrate [--layout tiled|even-horizontal|even-vertical] [--mock] : spawn all enabled agents into the shared team tmux window")
	fmt.Fprintln(r.stdout, "aom team view [--layout <layout>]  : join all active agent panes into one tmux window and attach")
	fmt.Fprintln(r.stdout, "aom team status                    : show each agent's current location (team / dedicated / shared-window)")
	fmt.Fprintln(r.stdout, "aom team isolate                   : move all agents from team session back to dedicated sessions (keeps processes running)")
	fmt.Fprintln(r.stdout, "aom team brief [--push]            : generate shared project brief from current tasks, sessions, and agents → .aom/team-brief.md")
	fmt.Fprintln(r.stdout, "  --push : also broadcast to team channel and copy to all active agent worktrees")
	fmt.Fprintln(r.stdout, "aom team roster                    : print agents defined in agents.yaml with their status")
	fmt.Fprintln(r.stdout, "aom session spawn <agent> --grid [--layout <layout>] : place pane in team window instead of own window")

	fmt.Fprintln(r.stdout, "aom broadcast \"<message>\" [--from <agent>] [--exclude-self] [--sessions <id,...>] [--file <path>] : push to all live sessions + channel.md; omit --sessions to reach everyone")
	fmt.Fprintln(r.stdout, "aom policy list [--task <task-id>] : show project deny_commands and enforcement level; add --task to see per-task agent enforcement")
	fmt.Fprintln(r.stdout, "aom channel append \"<message>\" [--agent <name>] : append a message to the shared .aom/channel.md")
	fmt.Fprintln(r.stdout, "aom channel read : print current shared channel contents")
	fmt.Fprintln(r.stdout, "aom message send <agent-name> \"<message>\" [--from <sender>] : write a direct message to an agent's mailbox")
	fmt.Fprintln(r.stdout, "aom message read <agent-name> : print an agent's unread mailbox messages")
	fmt.Fprintln(r.stdout, "aom message watch <agent-name> [--timeout <dur>] : block and wait for an incoming message from the agent")
	fmt.Fprintln(r.stdout, "aom message reply <msg-id> \"<reply>\" : reply to a specific message by ID")
	fmt.Fprintln(r.stdout, "aom message clear <agent-name> : archive and clear an agent's mailbox")
	fmt.Fprintln(r.stdout, "aom outbox flush : route all staged outbox messages (from sandbox agents) to channel/mailbox")
	fmt.Fprintln(r.stdout, "aom outbox list : show pending outbox messages waiting to be flushed")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Worktree")
	fmt.Fprintln(r.stdout, "aom worktree repair <task-id> : repair a missing or stale task worktree")
	fmt.Fprintln(r.stdout, "aom worktree read-file <task-id> <path> : read a file from another task's worktree (cross-worktree read)")
	fmt.Fprintln(r.stdout, "aom worktree commit <task-id> -m <msg> [--deliverables-only] : stage all changes and commit (--deliverables-only excludes .agent/ .aom/ AGENTS.md and other AOM artifacts)")
	fmt.Fprintln(r.stdout, "aom worktree prune : remove stale task worktrees and orphaned git worktree entries")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Hooks (automation)")
	fmt.Fprintln(r.stdout, ".aom/hooks/on-task-done.sh     — called when a task is closed or accepted; args: task_id task_title status")
	fmt.Fprintln(r.stdout, ".aom/hooks/on-task-ready.sh    — called when a task transitions to Ready; args: task_id task_title status")
	fmt.Fprintln(r.stdout, ".aom/hooks/on-session-spawn.sh — called after a session is successfully spawned; args: session_id agent_name task_id")
	fmt.Fprintln(r.stdout, "See .aom/hooks/on-task-done.sh.example for a template. Copy and chmod +x to activate.")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Merge")
	fmt.Fprintln(r.stdout, "aom merge check <task-id> : check for conflicts before merging")
	fmt.Fprintln(r.stdout, "aom merge prepare <task-id> : generate a merge plan artifact")
	fmt.Fprintln(r.stdout, "aom merge commit <task-id> [--into <branch>] : merge task branch; runs merge check first")
	fmt.Fprintln(r.stdout, "aom merge continue <task-id> : complete a merge paused by conflicts (after git add of resolved files)")
	fmt.Fprintln(r.stdout, "aom merge abort <task-id> : abort a conflicted merge and restore HEAD")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Goal & Orchestrator Mode")
	fmt.Fprintln(r.stdout, "aom goal set \"<text>\"      — set the project goal for the orchestrator agent")
	fmt.Fprintln(r.stdout, "aom goal show             — print current goal and status")
	fmt.Fprintln(r.stdout, "aom goal complete         — mark the current goal as complete")
	fmt.Fprintln(r.stdout, "aom orchestrator start [--goal \"<text>\"] [--real|--mock] [--no-grid]")
	fmt.Fprintln(r.stdout, "  Spawn the orchestrator agent in the team grid. Workers it spawns appear in the same grid.")
	fmt.Fprintln(r.stdout, "aom orchestrator view [--layout tiled|even-horizontal|even-vertical]")
	fmt.Fprintln(r.stdout, "  Attach to the team grid — see orchestrator + workers at once (Ctrl+B+arrows to navigate)")
	fmt.Fprintln(r.stdout, "aom orchestrator status   — goal + channel summary without attaching")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Automation (Phase 5 — Guided Autonomy)")
	fmt.Fprintln(r.stdout, "aom run-pipeline <task-id> [--agent <name>] [--timeout <dur>] [--real|--mock] [--skip-merge]")
	fmt.Fprintln(r.stdout, "  Runs the full pipeline in one command: spawn → wait(task.completed) → verify → accept → merge")
	fmt.Fprintln(r.stdout, "  --skip-merge : stop after accept; merge manually later with: aom merge commit <task-id>")
	fmt.Fprintln(r.stdout, "aom task accept --auto <task-id> [--interval 15s] [--timeout 30m] : poll + auto-accept when all verify checks pass")
	fmt.Fprintln(r.stdout, "aom session watch --auto-spawn [--interval 15s] [--timeout 60m] [--real|--mock] : auto-spawn agents for Ready tasks")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Key rules")
	fmt.Fprintln(r.stdout, "- Never edit .aom/ files directly; use the CLI so state changes stay canonical.")
	fmt.Fprintln(r.stdout, "- Use aom capture <session-id> to read agent output; do not inspect tmux directly as your primary interface.")
	fmt.Fprintln(r.stdout, "- .agent/*.md artifacts inside the task worktree are the source of truth for worker continuity.")
	fmt.Fprintln(r.stdout, "- Session status Idle means ready for the next prompt or task; Working means the agent is busy.")
}
