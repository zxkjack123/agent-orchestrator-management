package provider

import "fmt"

type geminiProvider struct{}

func (p *geminiProvider) Name() string            { return "gemini" }
func (p *geminiProvider) IdentityFilename() string { return "GEMINI.md" }

func (p *geminiProvider) LaunchShellSpec(_ LaunchSpec, _ func(string) (string, error)) (ShellSpec, error) {
	return ShellSpec{}, fmt.Errorf("real launch for runtime %q is not yet implemented: CLI flags unconfirmed", "gemini")
}

func (p *geminiProvider) ResumeInfo() ResumeInfo                         { return ResumeInfo{Supported: false} }
func (p *geminiProvider) MCPConfigStyle() MCPStyle                       { return MCPStyleNone }
func (p *geminiProvider) PolicyEnforcementLevel() PolicyEnforcement      { return PolicyEnforcementInstructionOnly }
func (p *geminiProvider) NativeSessionDetection() *NativeSessionStrategy { return nil }
