package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/agent"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/project"
)

var knownAgentRuntimes = map[string]struct{}{
	"claude": {},
	"codex":  {},
	"gemini": {},
	"kiro":   {},
}

func (r Runner) executeAgent(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent subcommand is required: list, add, show, profile")
	}
	switch args[0] {
	case "list":
		return r.executeAgentList(args[1:])
	case "add":
		return r.executeAgentAdd(args[1:])
	case "show":
		return r.executeAgentShow(args[1:])
	case "profile":
		return r.executeAgentProfile(args[1:])
	default:
		return fmt.Errorf("unknown agent subcommand %q", args[0])
	}
}

func (r Runner) executeAgentList(args []string) error {
	_ = args
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	agents := sortedAgents(result.Agents)

	fmt.Fprintln(r.stdout, "Agents")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "  %-22s %-14s %-8s %-8s %s\n", "NAME", "ROLE", "RUNTIME", "ENABLED", "PROFILE")
	fmt.Fprintln(r.stdout, "  "+strings.Repeat("-", 76))
	for _, a := range agents {
		profilePath := project.AgentProfilePath(result.AOMPath, a.Name)
		profileStatus := profilePath
		if _, err := os.Stat(profilePath); os.IsNotExist(err) {
			profileStatus = "(not seeded — run: aom open)"
		}
		enabled := "yes"
		if !a.Enabled {
			enabled = "no"
		}
		fmt.Fprintf(r.stdout, "  %-22s %-14s %-8s %-8s %s\n", a.Name, a.Role, a.Runtime, enabled, profileStatus)
	}
	return nil
}

func (r Runner) executeAgentAdd(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent name is required")
	}
	name := strings.TrimSpace(args[0])
	if name == "" {
		return fmt.Errorf("agent name is required")
	}

	var role, runtime string
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--role":
			i++
			if i >= len(args) {
				return fmt.Errorf("--role requires a value")
			}
			role = strings.TrimSpace(args[i])
		case "--runtime":
			i++
			if i >= len(args) {
				return fmt.Errorf("--runtime requires a value")
			}
			runtime = strings.ToLower(strings.TrimSpace(args[i]))
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if role == "" {
		return fmt.Errorf("--role is required")
	}
	if runtime == "" {
		return fmt.Errorf("--runtime is required")
	}
	if _, ok := knownAgentRuntimes[runtime]; !ok {
		return fmt.Errorf("unknown runtime %q; supported: claude, codex, gemini, kiro", runtime)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	if err := project.AddAgentToConfig(result.AOMPath, project.AddAgentParams{
		Name:    name,
		Role:    role,
		Runtime: runtime,
	}); err != nil {
		return err
	}

	// Re-open to sync DB state (idempotent; also seeds profile).
	if _, err := r.app.Projects.Open("."); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Agent added")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Name:    %s\n", name)
	fmt.Fprintf(r.stdout, "Role:    %s\n", role)
	fmt.Fprintf(r.stdout, "Runtime: %s\n", runtime)
	fmt.Fprintf(r.stdout, "Profile: %s\n", project.AgentProfilePath(result.AOMPath, name))
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Next: aom agent show "+name)
	return nil
}

func (r Runner) executeAgentShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent name is required")
	}
	name := strings.TrimSpace(args[0])

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	var found *agent.Record
	for i := range result.Agents {
		if result.Agents[i].Name == name {
			found = &result.Agents[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("agent %q not found", name)
	}

	roleCfg := result.RoleConfigs[found.Role]
	profilePath := project.AgentProfilePath(result.AOMPath, name)
	profile, err := project.ReadAgentProfile(result.AOMPath, name)
	if err != nil {
		return err
	}

	enabled := "yes"
	if !found.Enabled {
		enabled = "no"
	}

	fmt.Fprintf(r.stdout, "Agent: %s\n\n", name)
	fmt.Fprintf(r.stdout, "Role:          %s\n", found.Role)
	fmt.Fprintf(r.stdout, "Role class:    %s\n", emptyFallback(roleCfg.Class))
	fmt.Fprintf(r.stdout, "Runtime:       %s\n", found.Runtime)
	fmt.Fprintf(r.stdout, "Enabled:       %s\n", enabled)
	fmt.Fprintf(r.stdout, "Profile path:  %s\n", profilePath)
	fmt.Fprintln(r.stdout, "")

	if profile == "" {
		fmt.Fprintln(r.stdout, "Profile: (not seeded — run: aom open)")
	} else {
		fmt.Fprintln(r.stdout, "── Profile ──────────────────────────────────────────────────────────")
		fmt.Fprint(r.stdout, profile)
	}
	return nil
}

func (r Runner) executeAgentProfile(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent profile subcommand is required: show, update")
	}
	switch args[0] {
	case "show":
		return r.executeAgentProfileShow(args[1:])
	case "update":
		return r.executeAgentProfileUpdate(args[1:])
	default:
		return fmt.Errorf("unknown agent profile subcommand %q", args[0])
	}
}

func (r Runner) executeAgentProfileShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent name is required")
	}
	name := strings.TrimSpace(args[0])

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	profile, err := project.ReadAgentProfile(result.AOMPath, name)
	if err != nil {
		return err
	}
	if profile == "" {
		return fmt.Errorf("profile not found for agent %q — run: aom open", name)
	}

	fmt.Fprint(r.stdout, profile)
	return nil
}

func (r Runner) executeAgentProfileUpdate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent name is required")
	}
	name := strings.TrimSpace(args[0])

	var responsibilities, constraints string
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--responsibilities":
			i++
			if i >= len(args) {
				return fmt.Errorf("--responsibilities requires a value")
			}
			responsibilities = args[i]
		case "--constraints":
			i++
			if i >= len(args) {
				return fmt.Errorf("--constraints requires a value")
			}
			constraints = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if responsibilities == "" && constraints == "" {
		return fmt.Errorf("at least one of --responsibilities or --constraints is required")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	var found bool
	for _, a := range result.Agents {
		if a.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("agent %q not found", name)
	}

	profile, err := project.ReadAgentProfile(result.AOMPath, name)
	if err != nil {
		return err
	}
	if profile == "" {
		return fmt.Errorf("profile not found for agent %q — run: aom open", name)
	}

	if responsibilities != "" {
		profile = project.UpdateProfileSection(profile, "Responsibilities", "- "+responsibilities)
	}
	if constraints != "" {
		profile = project.UpdateProfileSection(profile, "Constraints", "- "+constraints)
	}

	if err := project.WriteAgentProfile(result.AOMPath, name, profile); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Profile updated")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Agent:   %s\n", name)
	fmt.Fprintf(r.stdout, "Profile: %s\n", project.AgentProfilePath(result.AOMPath, name))
	if responsibilities != "" {
		fmt.Fprintln(r.stdout, "Responsibilities: updated")
	}
	if constraints != "" {
		fmt.Fprintln(r.stdout, "Constraints: updated")
	}
	return nil
}

func sortedAgents(agents []agent.Record) []agent.Record {
	sorted := make([]agent.Record, len(agents))
	copy(sorted, agents)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}
