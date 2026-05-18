package plan

import (
	"testing"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/agent"
)

func TestBuildDefaultsToDirectMode(t *testing.T) {
	service := NewService()
	result, err := service.Build(Params{
		WorkDescription: "implement login validation",
		Agents: []agent.Record{
			{Name: "backend-main", Role: "backend", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if result.Mode != "Direct" {
		t.Fatalf("Mode = %q, want Direct", result.Mode)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(result.Steps))
	}
	if result.RecommendedAgent != "backend-main" {
		t.Fatalf("RecommendedAgent = %q, want backend-main", result.RecommendedAgent)
	}
}

func TestBuildInfersBugfixMode(t *testing.T) {
	service := NewService()
	result, err := service.Build(Params{
		WorkDescription: "fix login bug in callback flow",
		Agents: []agent.Record{
			{Name: "backend-main", Role: "backend", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if result.Mode != "Bugfix" {
		t.Fatalf("Mode = %q, want Bugfix", result.Mode)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(result.Steps))
	}
}

func TestBuildRespectsExplicitAgent(t *testing.T) {
	service := NewService()
	result, err := service.Build(Params{
		WorkDescription: "define architecture constraints",
		PreferredAgent:  "reviewer-main",
		Agents: []agent.Record{
			{Name: "reviewer-main", Role: "reviewer", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if result.RecommendedAgent != "reviewer-main" {
		t.Fatalf("RecommendedAgent = %q, want reviewer-main", result.RecommendedAgent)
	}
	if result.RecommendedRole != "reviewer" {
		t.Fatalf("RecommendedRole = %q, want reviewer", result.RecommendedRole)
	}
}
