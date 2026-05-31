package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/agent"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/artifact"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
	aomruntime "github.com/lattapon-aek/agent-orchestrator-management/internal/runtime"
)

// findOrchestratorAgent returns the first enabled agent whose role class is
// "orchestrator", or nil when none is configured.
func findOrchestratorAgent(result *project.OpenResult) *agent.Record {
	for i := range result.Agents {
		a := &result.Agents[i]
		if !a.Enabled {
			continue
		}
		roleCfg, ok := result.RoleConfigs[a.Role]
		if ok && roleCfg.Class == "orchestrator" {
			return a
		}
	}
	return nil
}

// executeOrchestratorMode handles the `aom orchestrator` subcommand group.
// Note: `aom orchestrate` (no r) is a separate command that spawns all agents
// into a team grid. This command group manages autonomous orchestrator mode.
func (r Runner) executeOrchestratorMode(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("orchestrator subcommand required: start | view | status")
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Fprintln(r.stdout, "aom orchestrator start [--goal \"<text>\"] [--real|--mock] [--no-grid]")
			fmt.Fprintln(r.stdout, "  Spawn the orchestrator agent in autonomous mode inside the team grid.")
			fmt.Fprintln(r.stdout, "  The orchestrator reads .aom/goal.json, drives the worker team, and")
			fmt.Fprintln(r.stdout, "  reports back via channel when done or blocked.")
			fmt.Fprintln(r.stdout, "  --goal    : set the project goal before spawning")
			fmt.Fprintln(r.stdout, "  --real    : spawn a real agent session (default)")
			fmt.Fprintln(r.stdout, "  --mock    : spawn a mock session (for testing)")
			fmt.Fprintln(r.stdout, "  --no-grid : spawn in own window instead of team grid")
			fmt.Fprintln(r.stdout, "")
			fmt.Fprintln(r.stdout, "aom orchestrator view [--layout tiled|even-horizontal|even-vertical]")
			fmt.Fprintln(r.stdout, "  Attach to the team grid — see orchestrator + all worker panes at once.")
			fmt.Fprintln(r.stdout, "  Ctrl+B then arrow keys to switch between panes.")
			fmt.Fprintln(r.stdout, "")
			fmt.Fprintln(r.stdout, "aom orchestrator status")
			fmt.Fprintln(r.stdout, "  Show orchestrator session health, current goal, and recent channel.")
			return nil
		}
	}
	switch args[0] {
	case "start":
		return r.executeOrchestratorStart(args[1:])
	case "view":
		return r.executeOrchestratorView(args[1:])
	case "status":
		return r.executeOrchestratorStatus(args[1:])
	default:
		return fmt.Errorf("unknown orchestrator command %q", args[0])
	}
}

// executeOrchestratorStart provisions and spawns the orchestrator agent into
// the shared team grid so workers it spawns later appear in the same window.
//
// Usage: aom orchestrator start [--goal "<text>"] [--real|--mock] [--no-grid]
func (r Runner) executeOrchestratorStart(args []string) error {
	goalText := ""
	launchMode := aomruntime.LaunchModeReal
	useGrid := true // default: spawn into the team grid

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--goal":
			i++
			if i >= len(args) {
				return fmt.Errorf("--goal requires a value")
			}
			goalText = args[i]
		case "--mock":
			launchMode = aomruntime.LaunchModeMock
		case "--real":
			launchMode = aomruntime.LaunchModeReal
		case "--no-grid":
			useGrid = false
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return wrapProjectNotFound(err)
	}

	// Find the first enabled orchestrator-class agent.
	orchAgent := findOrchestratorAgent(result)
	if orchAgent == nil {
		return fmt.Errorf("no enabled orchestrator-class agent found\n\nAdd one with:\n  aom agent add orchestrator --class orchestrator --runtime claude")
	}

	// Write goal if provided on the command line.
	if strings.TrimSpace(goalText) != "" {
		goalPath, wErr := artifact.WriteGoalFile(result.Project.RepoPath, goalText)
		if wErr != nil {
			return fmt.Errorf("write goal: %w", wErr)
		}
		fmt.Fprintf(r.stdout, "Goal written: %s\n", goalPath)
	}

	// Report current goal state.
	if rec, gErr := artifact.ReadGoalFile(result.Project.RepoPath); gErr == nil {
		fmt.Fprintf(r.stdout, "Goal [%s]: %s\n", rec.Status, rec.Text)
	} else if strings.TrimSpace(goalText) == "" {
		fmt.Fprintln(r.stdout, "No goal set. The orchestrator will start without a goal.")
		fmt.Fprintln(r.stdout, "Set one with: aom goal set \"<text>\"  or  aom orchestrator start --goal \"<text>\"")
		fmt.Fprintln(r.stdout, "")
	}

	// Spawn the orchestrator — into the team grid by default so workers it
	// spawns later (also with --grid) appear in the same tmux window.
	rec, spawnErr := r.executeResolvedSessionSpawn(result, orchAgent, sessionSpawnParams{
		agentName:  orchAgent.Name,
		launchMode: launchMode,
		gridMode:   useGrid,
		gridLayout: "tiled",
	})
	if spawnErr != nil {
		return spawnErr
	}

	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Orchestrator session: %s\n", rec.ID)
	fmt.Fprintf(r.stdout, "Agent:                %s  (%s)\n", orchAgent.Name, orchAgent.Runtime)
	if useGrid {
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintln(r.stdout, "The orchestrator is running in the team grid.")
		fmt.Fprintln(r.stdout, "Workers it spawns will appear in the same grid automatically.")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintln(r.stdout, "Watch the team:")
		fmt.Fprintln(r.stdout, "  aom orchestrator view          ← attach to grid (Ctrl+B+arrows to navigate)")
		fmt.Fprintln(r.stdout, "  aom orchestrator status        ← goal + channel summary")
		fmt.Fprintln(r.stdout, "  aom dashboard                  ← live action-item feed")
	} else {
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintln(r.stdout, "Monitor with:")
		fmt.Fprintln(r.stdout, "  aom orchestrator status")
		fmt.Fprintln(r.stdout, "  aom dashboard")
		fmt.Fprintf(r.stdout, "  aom switch %s\n", orchAgent.Name)
	}
	return nil
}

// executeOrchestratorView attaches the operator to the team tmux grid where
// the orchestrator and all worker panes live side-by-side.
//
// Usage: aom orchestrator view [--layout tiled|even-horizontal|even-vertical]
func (r Runner) executeOrchestratorView(args []string) error {
	layout := "tiled"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--layout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--layout requires a value")
			}
			layout = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return wrapProjectNotFound(err)
	}

	workspace, err := r.app.Tmux.EnsureWorkspace(result.SessionPrefix, result.Project.RepoPath)
	if err != nil {
		return fmt.Errorf("ensure tmux workspace: %w", err)
	}

	const teamWindowName = "team"
	windowTarget, _, err := r.app.Tmux.EnsureTeamWindow(workspace.Target, teamWindowName)
	if err != nil {
		return fmt.Errorf("ensure team window: %w", err)
	}

	if layout != "" {
		if err := r.app.Tmux.SelectLayout(windowTarget, layout); err != nil {
			fmt.Fprintf(r.stderr, "warning: select-layout %q: %v\n", layout, err)
		}
	}

	fmt.Fprintln(r.stdout, "Attaching to team grid...")
	fmt.Fprintln(r.stdout, "  Ctrl+B + arrow keys  — switch between panes")
	fmt.Fprintln(r.stdout, "  Ctrl+B + z           — zoom into one pane (toggle)")
	fmt.Fprintln(r.stdout, "  Ctrl+B + d           — detach (leave agents running)")
	fmt.Fprintln(r.stdout, "")

	return r.app.Tmux.AttachWindow(workspace.Target, windowTarget)
}

// executeOrchestratorStatus shows orchestrator session health, goal, and recent channel.
func (r Runner) executeOrchestratorStatus(args []string) error {
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return wrapProjectNotFound(err)
	}

	// Goal.
	fmt.Fprintln(r.stdout, "Goal")
	if rec, gErr := artifact.ReadGoalFile(result.Project.RepoPath); gErr == nil {
		fmt.Fprintf(r.stdout, "  Status: %s\n", rec.Status)
		fmt.Fprintf(r.stdout, "  Set at: %s\n", rec.SetAt.UTC().Format("2006-01-02 15:04:05 UTC"))
		fmt.Fprintf(r.stdout, "  Text:   %s\n", rec.Text)
	} else {
		fmt.Fprintln(r.stdout, "  (no goal set — run: aom goal set \"<text>\")")
	}
	fmt.Fprintln(r.stdout, "")

	// Orchestrator sessions.
	sessions, err := r.loadProjectSessions(result)
	if err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Orchestrator Sessions")
	found := 0
	for _, s := range sessions {
		roleCfg, ok := result.RoleConfigs[s.RoleName]
		if !ok || roleCfg.Class != "orchestrator" {
			continue
		}
		found++
		fmt.Fprintf(r.stdout, "  %s  agent=%-20s  status=%-18s  task=%s\n",
			s.ID, s.AgentName, s.Status, s.TaskID)
	}
	if found == 0 {
		fmt.Fprintln(r.stdout, "  (no orchestrator sessions — run: aom orchestrator start)")
	}
	fmt.Fprintln(r.stdout, "")

	// Recent channel activity.
	fmt.Fprintln(r.stdout, "Recent Channel Activity")
	channelPath := result.Project.RepoPath + "/.aom/channel.md"
	data, readErr := os.ReadFile(channelPath)
	if readErr != nil {
		fmt.Fprintln(r.stdout, "  (no channel activity)")
	} else {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		start := 0
		if len(lines) > 10 {
			start = len(lines) - 10
		}
		for _, l := range lines[start:] {
			if strings.TrimSpace(l) != "" {
				fmt.Fprintln(r.stdout, "  "+l)
			}
		}
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Commands:")
	fmt.Fprintln(r.stdout, "  aom orchestrator view    ← attach to team grid")
	fmt.Fprintln(r.stdout, "  aom dashboard            ← live action-item feed")
	return nil
}
