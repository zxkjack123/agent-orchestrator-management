package cli

import (
	"fmt"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
)

func (r Runner) executeRole(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("role subcommand is required: list, show, create, update, delete, preview")
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			r.printHelp()
			return nil
		}
	}
	switch args[0] {
	case "list":
		return r.executeRoleList(args[1:])
	case "show":
		return r.executeRoleShow(args[1:])
	case "create":
		return r.executeRoleCreate(args[1:])
	case "update":
		return r.executeRoleUpdate(args[1:])
	case "delete":
		return r.executeRoleDelete(args[1:])
	case "preview":
		return r.executeRolePreview(args[1:])
	default:
		return fmt.Errorf("unknown role subcommand %q — valid: list, show, create, update, delete, preview", args[0])
	}
}

func (r Runner) executeRoleList(_ []string) error {
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}
	roles, err := project.ListRoles(result.AOMPath)
	if err != nil {
		return err
	}
	fmt.Fprintln(r.stdout, "Roles")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "  %-20s %-16s %-18s %-12s %s\n", "NAME", "CLASS", "WORKTREE_MODE", "CHECKPOINT", "AGENTS")
	fmt.Fprintln(r.stdout, "  "+strings.Repeat("-", 90))
	for _, ro := range roles {
		agents := strings.Join(ro.AgentsUsing, ", ")
		if agents == "" {
			agents = "-"
		}
		fmt.Fprintf(r.stdout, "  %-20s %-16s %-18s %-12s %s\n",
			ro.Name, ro.Class, ro.WorktreeMode, ro.CheckpointExpectation, agents)
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "  %d role(s) defined\n", len(roles))
	return nil
}

func (r Runner) executeRoleShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aom role show <name>")
	}
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}
	ro, err := project.GetRole(result.AOMPath, args[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "Role: %s\n\n", ro.Name)
	fmt.Fprintf(r.stdout, "  class:                 %s\n", ro.Class)
	fmt.Fprintf(r.stdout, "  worktree_mode:         %s\n", ro.WorktreeMode)
	fmt.Fprintf(r.stdout, "  checkpoint_expectation:%s\n", " "+ro.CheckpointExpectation)
	fmt.Fprintf(r.stdout, "  default_session_mode:  %s\n", ro.DefaultSessionMode)
	fmt.Fprintln(r.stdout, "")
	if len(ro.AgentsUsing) > 0 {
		fmt.Fprintf(r.stdout, "  Agents using this role: %s\n", strings.Join(ro.AgentsUsing, ", "))
	} else {
		fmt.Fprintln(r.stdout, "  Agents using this role: (none)")
	}
	return nil
}

func (r Runner) executeRoleCreate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aom role create <name> [--class <class>] [--worktree-mode <mode>] [--checkpoint <exp>]")
	}
	name := args[0]
	params := project.RoleCreateParams{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--class":
			if i+1 >= len(args) {
				return fmt.Errorf("--class requires a value")
			}
			params.Class = args[i+1]
			i++
		case "--worktree-mode":
			if i+1 >= len(args) {
				return fmt.Errorf("--worktree-mode requires a value")
			}
			params.WorktreeMode = args[i+1]
			i++
		case "--checkpoint":
			if i+1 >= len(args) {
				return fmt.Errorf("--checkpoint requires a value")
			}
			params.CheckpointExpectation = args[i+1]
			i++
		case "--session-mode":
			if i+1 >= len(args) {
				return fmt.Errorf("--session-mode requires a value")
			}
			params.DefaultSessionMode = args[i+1]
			i++
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}
	if err := project.CreateRole(result.AOMPath, name, params); err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "Role %q created (class: %s, worktree_mode: %s)\n",
		name, coalesceStr(params.Class, "generic"), coalesceStr(params.WorktreeMode, "dedicated-writer"))
	fmt.Fprintf(r.stdout, "Next steps:\n")
	fmt.Fprintf(r.stdout, "  aom class show %s          # view class template\n", coalesceStr(params.Class, "generic"))
	fmt.Fprintf(r.stdout, "  aom agent add <name> --role %s --runtime claude\n", name)
	return nil
}

func (r Runner) executeRoleUpdate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aom role update <name> [--class <class>] [--worktree-mode <mode>] [--checkpoint <exp>]")
	}
	name := args[0]
	params := project.RoleUpdateParams{}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--class":
			if i+1 >= len(args) {
				return fmt.Errorf("--class requires a value")
			}
			v := args[i+1]
			params.Class = &v
			i++
		case "--worktree-mode":
			if i+1 >= len(args) {
				return fmt.Errorf("--worktree-mode requires a value")
			}
			v := args[i+1]
			params.WorktreeMode = &v
			i++
		case "--checkpoint":
			if i+1 >= len(args) {
				return fmt.Errorf("--checkpoint requires a value")
			}
			v := args[i+1]
			params.CheckpointExpectation = &v
			i++
		case "--session-mode":
			if i+1 >= len(args) {
				return fmt.Errorf("--session-mode requires a value")
			}
			v := args[i+1]
			params.DefaultSessionMode = &v
			i++
		}
	}
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}
	if err := project.UpdateRole(result.AOMPath, name, params); err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "Role %q updated.\n", name)
	return nil
}

func (r Runner) executeRoleDelete(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aom role delete <name>")
	}
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}
	if err := project.DeleteRole(result.AOMPath, args[0]); err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "Role %q deleted.\n", args[0])
	return nil
}

func (r Runner) executeRolePreview(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aom role preview <name> [--runtime claude|codex]")
	}
	roleName := args[0]
	runtime := "claude"
	for i := 1; i < len(args); i++ {
		if args[i] == "--runtime" && i+1 < len(args) {
			runtime = args[i+1]
			i++
		}
	}
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}
	rendered, err := project.PreviewRoleProfile(result.AOMPath, roleName, runtime)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "--- Profile preview for role %q (runtime: %s) ---\n\n", roleName, runtime)
	fmt.Fprintln(r.stdout, rendered)
	return nil
}

func coalesceStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
