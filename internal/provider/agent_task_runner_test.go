package provider

import (
	"strings"
	"testing"
)

func TestAgentTaskRunnerProviderImplementsInterface(t *testing.T) {
	p := &agentTaskRunnerProvider{}

	// Verify all required methods exist
	if p.Name() != "agent-task-runner" {
		t.Fatalf("Name() = %q, want agent-task-runner", p.Name())
	}

	if p.IdentityFilename() != "AGENTS.md" {
		t.Fatalf("IdentityFilename() = %q, want AGENTS.md", p.IdentityFilename())
	}

	// MCP style should be JSON file (agent-task-runner uses .loop/ for config)
	if p.MCPConfigStyle() != MCPStyleJSONFile {
		t.Fatalf("MCPConfigStyle() = %v, want MCPStyleJSONFile", p.MCPConfigStyle())
	}

	// Policy enforcement via runtime flags (preflight injected into worker prompt)
	if p.PolicyEnforcementLevel() != PolicyEnforcementRuntimeFlag {
		t.Fatalf("PolicyEnforcementLevel() = %v, want PolicyEnforcementRuntimeFlag", p.PolicyEnforcementLevel())
	}

	// Resume is not supported (agent-task-runner uses fresh loop run each time)
	info := p.ResumeInfo()
	if info.Supported {
		t.Fatal("ResumeInfo().Supported = true, want false")
	}

	// Native session detection not applicable (no tmux session)
	if p.NativeSessionDetection() != nil {
		t.Fatal("NativeSessionDetection() should return nil")
	}
}

func TestAgentTaskRunnerProviderLaunchShellSpecReturnsError(t *testing.T) {
	p := &agentTaskRunnerProvider{}

	spec, err := p.LaunchShellSpec(LaunchSpec{}, nil)
	if err == nil {
		t.Fatal("LaunchShellSpec should return an error for agent-task-runner")
	}
	if spec.ExecCmd != "" {
		t.Fatalf("LaunchShellSpec ExecCmd should be empty, got %q", spec.ExecCmd)
	}
	if !strings.Contains(err.Error(), "tmux/spawn") {
		t.Fatalf("LaunchShellSpec error should mention tmux/spawn, got: %v", err)
	}
}

func TestAgentTaskRunnerProviderKnownModels(t *testing.T) {
	p := &agentTaskRunnerProvider{}
	models := p.KnownModels()
	if models != nil {
		t.Fatalf("KnownModels should return nil (models determined by --worker-backend flag), got %v", models)
	}
}

func TestAgentTaskRunnerProviderStartupDialogResponse(t *testing.T) {
	p := &agentTaskRunnerProvider{}
	if p.StartupDialogResponse() != "" {
		t.Fatal("StartupDialogResponse should return empty string")
	}
}

func TestAgentTaskRunnerProviderModelHint(t *testing.T) {
	p := &agentTaskRunnerProvider{}
	hint := p.ModelHint()
	if hint == "" {
		t.Fatal("ModelHint should return a non-empty description")
	}
}

func TestAgentTaskRunnerProviderInDefaultRegistry(t *testing.T) {
	reg := DefaultRegistry()
	p := reg.Lookup("agent-task-runner")
	if p == nil {
		t.Fatal("DefaultRegistry should contain agent-task-runner")
	}
	if p.Name() != "agent-task-runner" {
		t.Fatalf("Lookup returned provider with Name()=%q, want agent-task-runner", p.Name())
	}
	// Verify it's not the fallback
	if _, ok := p.(*agentTaskRunnerProvider); !ok {
		t.Fatalf("Lookup should return *agentTaskRunnerProvider, got %T", p)
	}
}

func TestResolveLoopKitBinary(t *testing.T) {
	python, args := ResolveLoopKitBinary()
	if python == "" {
		t.Fatal("ResolveLoopKitBinary should return a non-empty python executable")
	}
	if len(args) < 2 {
		t.Fatalf("ResolveLoopKitBinary should return at least 2 args, got %v", args)
	}
	if args[0] != "-m" || args[1] != "loop_kit" {
		t.Fatalf("ResolveLoopKitBinary args should be [-m, loop_kit], got %v", args)
	}
}
