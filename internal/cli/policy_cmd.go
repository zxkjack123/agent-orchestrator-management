package cli

import (
	"fmt"
	"strings"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/provider"
)

func (r Runner) executePolicy(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("policy subcommand is required (try: aom policy list)")
	}
	switch args[0] {
	case "list":
		return r.executePolicyList(args[1:])
	default:
		return fmt.Errorf("unknown policy command %q", strings.Join(args, " "))
	}
}

func (r Runner) executePolicyList(args []string) error {
	taskID := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--task" {
			i++
			if i >= len(args) {
				return fmt.Errorf("--task requires a value")
			}
			taskID = strings.TrimSpace(args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	pol := result.Policy.Policy

	fmt.Fprintln(r.stdout, "Project Policy")
	fmt.Fprintln(r.stdout, "==============")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "  Yolo mode:       %s\n", pol.SessionDefaults.YoloMode)
	fmt.Fprintf(r.stdout, "  Approval scope:  %s\n", pol.SessionDefaults.ApprovalScope)
	fmt.Fprintln(r.stdout, "")

	if len(pol.DenyCommands) == 0 {
		fmt.Fprintln(r.stdout, "  Deny commands: none configured")
	} else {
		fmt.Fprintf(r.stdout, "  Deny commands (%d):\n", len(pol.DenyCommands))
		for _, cmd := range pol.DenyCommands {
			fmt.Fprintf(r.stdout, "    BLOCK  %s\n", cmd)
		}
	}

	if len(pol.RequireApproval) > 0 {
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "  Require approval (%d):\n", len(pol.RequireApproval))
		for _, cmd := range pol.RequireApproval {
			fmt.Fprintf(r.stdout, "    GATE   %s\n", cmd)
		}
	}

	if taskID == "" {
		return nil
	}

	// Per-task: show assigned agent and runtime enforcement level.
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", taskID)

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	rec, err := taskService.Get(taskID)
	if err != nil || rec == nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	agentName := rec.PreferredAgent
	if agentName == "" {
		agentName = rec.PreferredRole
	}
	fmt.Fprintf(r.stdout, "  Assigned agent: %s\n", func() string {
		if agentName == "" {
			return "unassigned"
		}
		return agentName
	}())

	// Look up the agent's runtime from the project agents list.
	runtime := ""
	for _, a := range result.Agents {
		if a.Name == agentName {
			runtime = a.Runtime
			break
		}
	}
	if runtime == "" {
		fmt.Fprintln(r.stdout, "  Runtime enforcement: agent runtime unknown")
		return nil
	}

	fmt.Fprintf(r.stdout, "  Runtime: %s\n", runtime)
	switch r.registry.Lookup(runtime).PolicyEnforcementLevel() {
	case provider.PolicyEnforcementRuntimeFlag:
		fmt.Fprintf(r.stdout, "  Enforcement: deny commands passed via --disallowed-tools (runtime-level)\n")
	case provider.PolicyEnforcementWrapperScript:
		fmt.Fprintf(r.stdout, "  Enforcement: deny commands enforced via PATH wrapper scripts (shell-level)\n")
	default:
		fmt.Fprintf(r.stdout, "  Enforcement: instruction-only — %s has no runtime enforcement flag\n", runtime)
	}

	return nil
}
