package provider

import "fmt"

type codexProvider struct{}

func (p *codexProvider) Name() string            { return "codex" }
func (p *codexProvider) IdentityFilename() string { return "AGENTS.md" }

func (p *codexProvider) LaunchCommand(spec LaunchSpec, lookPath func(string) (string, error)) (string, error) {
	if _, err := lookPath("codex"); err != nil {
		return "", fmt.Errorf("real launch for runtime %q requires the %q CLI in PATH", "codex", "codex")
	}
	if spec.AgentSessionID != "" {
		return fmt.Sprintf("sh -lc 'exec codex resume %s --sandbox workspace-write'", spec.AgentSessionID), nil
	}
	return "sh -lc 'exec codex --sandbox workspace-write'", nil
}

func (p *codexProvider) ResumeInfo() ResumeInfo {
	return ResumeInfo{
		Supported:     true,
		FreshExample:  "codex --sandbox workspace-write",
		ResumeExample: "codex resume <session-id> --sandbox workspace-write",
	}
}

func (p *codexProvider) MCPConfigStyle() MCPStyle                       { return MCPStyleJSONFile }
func (p *codexProvider) PolicyEnforcementLevel() PolicyEnforcement      { return PolicyEnforcementInstructionOnly }
func (p *codexProvider) NativeSessionDetection() *NativeSessionStrategy { return nil }
