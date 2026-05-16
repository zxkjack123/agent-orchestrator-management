package provider

import "fmt"

type kiroProvider struct{}

func (p *kiroProvider) Name() string            { return "kiro" }
func (p *kiroProvider) IdentityFilename() string { return "" } // unknown CLI, skip identity file

func (p *kiroProvider) LaunchCommand(_ LaunchSpec, _ func(string) (string, error)) (string, error) {
	return "", fmt.Errorf("real launch for runtime %q is not yet implemented: CLI flags unconfirmed", "kiro")
}

func (p *kiroProvider) ResumeInfo() ResumeInfo                         { return ResumeInfo{Supported: false} }
func (p *kiroProvider) MCPConfigStyle() MCPStyle                       { return MCPStyleNone }
func (p *kiroProvider) PolicyEnforcementLevel() PolicyEnforcement      { return PolicyEnforcementInstructionOnly }
func (p *kiroProvider) NativeSessionDetection() *NativeSessionStrategy { return nil }
