package plan

import (
	"fmt"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/agent"
)

// Params describes one orchestrator planning request.
type Params struct {
	WorkDescription string
	Mode            string
	PreferredRole   string
	PreferredAgent  string
	Agents          []agent.Record
}

// StepProposal is one proposed workflow step.
type StepProposal struct {
	Type      string
	Title     string
	RoleName  string
	AgentName string
}

// Result is the orchestrator planning response for one request.
type Result struct {
	Mode                  string
	Steps                 []StepProposal
	RecommendedRole       string
	RecommendedAgent      string
	RecommendedNextAction string
}

// Service owns simple orchestrator planning behavior for Milestone 3.
type Service struct{}

// NewService creates a planning service.
func NewService() *Service {
	return &Service{}
}

// Build produces a lightweight orchestrator recommendation without persisting state.
func (s *Service) Build(params Params) (*Result, error) {
	work := strings.TrimSpace(params.WorkDescription)
	if work == "" {
		return nil, fmt.Errorf("work description is required")
	}

	mode, err := normalizeMode(params.Mode, work)
	if err != nil {
		return nil, err
	}

	role, agentName, err := resolveOwner(params.PreferredRole, params.PreferredAgent, params.Agents, mode)
	if err != nil {
		return nil, err
	}

	steps := buildSteps(work, mode, role, agentName)
	return &Result{
		Mode:                  mode,
		Steps:                 steps,
		RecommendedRole:       role,
		RecommendedAgent:      agentName,
		RecommendedNextAction: recommendNextAction(mode),
	}, nil
}

func normalizeMode(explicitMode, work string) (string, error) {
	if strings.TrimSpace(explicitMode) != "" {
		switch strings.ToLower(strings.TrimSpace(explicitMode)) {
		case "direct":
			return "Direct", nil
		case "bugfix":
			return "Bugfix", nil
		case "requirements-first":
			return "Requirements-first", nil
		case "design-first":
			return "Design-first", nil
		default:
			return "", fmt.Errorf("plan mode %q is not recognized", explicitMode)
		}
	}

	lower := strings.ToLower(work)
	switch {
	case containsAny(lower, "bug", "fix", "broken", "error", "regression", "issue"):
		return "Bugfix", nil
	case containsAny(lower, "requirement", "requirements", "spec", "prd"):
		return "Requirements-first", nil
	case containsAny(lower, "design", "architecture", "constraint", "schema"):
		return "Design-first", nil
	default:
		return "Direct", nil
	}
}

func resolveOwner(preferredRole, preferredAgent string, agents []agent.Record, mode string) (string, string, error) {
	agentName := strings.TrimSpace(preferredAgent)
	role := strings.TrimSpace(preferredRole)

	if agentName != "" {
		for _, item := range agents {
			if item.Name != agentName {
				continue
			}
			if !item.Enabled {
				return "", "", fmt.Errorf("agent %q is disabled", agentName)
			}
			if role == "" {
				role = item.Role
			}
			return role, item.Name, nil
		}
		return "", "", fmt.Errorf("agent %q not found", agentName)
	}

	if role != "" {
		for _, item := range agents {
			if item.Role == role && item.Enabled {
				return role, item.Name, nil
			}
		}
		return role, "", nil
	}

	defaultRole := "backend"
	if mode == "Design-first" {
		defaultRole = "orchestrator"
	}
	for _, item := range agents {
		if item.Role == defaultRole && item.Enabled {
			return defaultRole, item.Name, nil
		}
	}

	return defaultRole, "", nil
}

func buildSteps(work, mode, role, agentName string) []StepProposal {
	switch mode {
	case "Bugfix":
		return []StepProposal{
			{Type: "research", Title: "Confirm current behavior and likely root cause", RoleName: role, AgentName: agentName},
			{Type: "implementation", Title: "Apply the fix for " + work, RoleName: role, AgentName: agentName},
		}
	case "Requirements-first":
		return []StepProposal{
			{Type: "research", Title: "Capture requirements for " + work, RoleName: role, AgentName: agentName},
			{Type: "coordination", Title: "Turn accepted requirements into implementation steps", RoleName: role, AgentName: agentName},
		}
	case "Design-first":
		return []StepProposal{
			{Type: "research", Title: "Lock design constraints for " + work, RoleName: role, AgentName: agentName},
			{Type: "coordination", Title: "Convert the accepted design into execution steps", RoleName: role, AgentName: agentName},
		}
	default:
		return []StepProposal{
			{Type: "implementation", Title: work, RoleName: role, AgentName: agentName},
		}
	}
}

func recommendNextAction(mode string) string {
	switch mode {
	case "Bugfix":
		return "review the proposed diagnosis step, then create the task if the bug framing looks right"
	case "Requirements-first":
		return "confirm the requirements-first mode, then create the task and capture the first requirement step"
	case "Design-first":
		return "confirm the design-first mode, then create the task and lock the first design step"
	default:
		return "create the task in Direct mode if the proposed scope looks right"
	}
}

func containsAny(value string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(value, pattern) {
			return true
		}
	}

	return false
}
