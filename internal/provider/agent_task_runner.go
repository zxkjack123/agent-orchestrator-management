package provider

import (
	"fmt"
	"os/exec"
)

// agentTaskRunnerProvider implements the Provider contract for agent-task-runner.
//
// Unlike other providers that embed agent CLI backends (claude, codex, etc.),
// this provider delegates to the agent-task-runner Python package which runs
// the full PM→Worker→Reviewer loop. The LaunchShellSpec returns an empty
// ShellSpec because agent-task-runner is not launched via tmux/spawn — it is
// invoked by AOM's pipeline-loop command directly as a subprocess.
//
// Integration path:
//
//	AOM pipeline-loop → generate task_card.json → python -m loop_kit run
//	  → outcome.json → AOM reads outcome → task status updated
type agentTaskRunnerProvider struct{}

func (p *agentTaskRunnerProvider) Name() string            { return "agent-task-runner" }
func (p *agentTaskRunnerProvider) IdentityFilename() string { return "AGENTS.md" }

// LaunchShellSpec returns an empty ShellSpec because agent-task-runner is not
// launched through the tmux session spawn path. Instead, the pipeline-loop
// command executes it as a subprocess directly.
const agentRunnerClarificationMsg = "agent-task-runner is not launched via tmux/spawn — use pipeline-loop command instead"

func (p *agentTaskRunnerProvider) LaunchShellSpec(_ LaunchSpec, _ func(string) (string, error)) (ShellSpec, error) {
	return ShellSpec{}, fmt.Errorf(agentRunnerClarificationMsg)
}

func (p *agentTaskRunnerProvider) ResumeInfo() ResumeInfo { return ResumeInfo{Supported: false} }

// MCPConfigStyle returns MCPStyleJSONFile. The agent-task-runner runtime
// consumes MCP configuration via JSON files in its .loop/ directory.
func (p *agentTaskRunnerProvider) MCPConfigStyle() MCPStyle { return MCPStyleJSONFile }

// PolicyEnforcementLevel returns PolicyEnforcementRuntimeFlag.
// agent-task-runner injects preflight policy (forbidden commands, size limits)
// directly into the worker prompt at runtime.
func (p *agentTaskRunnerProvider) PolicyEnforcementLevel() PolicyEnforcement {
	return PolicyEnforcementRuntimeFlag
}

func (p *agentTaskRunnerProvider) NativeSessionDetection() *NativeSessionStrategy { return nil }
func (p *agentTaskRunnerProvider) StartupDialogResponse() string                  { return "" }

// ModelHint explains that the model is determined by agent-task-runner's
// --worker-backend / --reviewer-backend flags, not by an agent runtime model.
func (p *agentTaskRunnerProvider) ModelHint() string {
	return "agent-task-runner uses --worker-backend and --reviewer-backend CLI flags to select models"
}

func (p *agentTaskRunnerProvider) KnownModels() []string { return nil }

// ---------- CLI integration helpers ----------

// ResolveLoopKitBinary returns the path to the loop_kit Python module.
// Falls back to "python3" + "-m loop_kit" if the direct binary is not found.
func ResolveLoopKitBinary() (pythonExe string, loopArgs []string) {
	if path, err := exec.LookPath("python3"); err == nil {
		return path, []string{"-m", "loop_kit"}
	}
	if path, err := exec.LookPath("python"); err == nil {
		return path, []string{"-m", "loop_kit"}
	}
	return "python3", []string{"-m", "loop_kit"} // best-effort fallback
}

// LoopKitCommand builds the agent-task-runner invocation.
//
// Usage:
//
//	taskCardPath := "/path/to/task_card.json"
//	outcomePath := "/path/to/outcome.json"
//	args := LoopKitRunArgs{
//	    TaskCardPath:    taskCardPath,
//	    OutcomeFile:     outcomePath,
//	    WorkerBackend:   "opencode",
//	    ReviewerBackend: "opencode",
//	    MaxRounds:       5,
//	    Timeout:         900,
//	}
//	cmd := LoopKitCommand(args)
//	cmd.Dir = worktreePath
//	err := cmd.Run()
type LoopKitRunArgs struct {
	TaskCardPath    string
	OutcomeFile     string
	WorkerBackend   string
	ReviewerBackend string
	MaxRounds       int
	Timeout         int
	DispatchTimeout int
	ArtifactTimeout int
	AllowDirty      bool
	MaxParallelWorkers int
}

// LoopKitCommand assembles the command for agent-task-runner.
// Returns a *exec.Cmd that should be run in the task's worktree directory.
func LoopKitCommand(args LoopKitRunArgs) *exec.Cmd {
	python, loopArgs := ResolveLoopKitBinary()

	if args.WorkerBackend == "" {
		args.WorkerBackend = "opencode"
	}
	if args.ReviewerBackend == "" {
		args.ReviewerBackend = "opencode"
	}
	if args.MaxRounds <= 0 {
		args.MaxRounds = 5
	}
	if args.Timeout <= 0 {
		args.Timeout = 900
	}
	if args.DispatchTimeout <= 0 {
		args.DispatchTimeout = 900
	}
	if args.ArtifactTimeout <= 0 {
		args.ArtifactTimeout = 300
	}
	if args.MaxParallelWorkers <= 0 {
		args.MaxParallelWorkers = 1
	}

	cmdArgs := append(loopArgs, "run",
		"--task", args.TaskCardPath,
		"--worker-backend", args.WorkerBackend,
		"--reviewer-backend", args.ReviewerBackend,
		"--auto-dispatch",
		"--max-rounds", fmt.Sprintf("%d", args.MaxRounds),
		"--timeout", fmt.Sprintf("%d", args.Timeout),
		"--dispatch-timeout", fmt.Sprintf("%d", args.DispatchTimeout),
		"--artifact-timeout", fmt.Sprintf("%d", args.ArtifactTimeout),
		"--max-parallel-workers", fmt.Sprintf("%d", args.MaxParallelWorkers),
		"--outcome-file", args.OutcomeFile,
	)

	if args.AllowDirty {
		cmdArgs = append(cmdArgs, "--allow-dirty")
	}

	return exec.Command(python, cmdArgs...)
}
