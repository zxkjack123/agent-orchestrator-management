package provider

import "fmt"

type kiroProvider struct{}

func (p *kiroProvider) Name() string            { return "kiro" }
func (p *kiroProvider) IdentityFilename() string { return "" } // unknown CLI, skip identity file

// LaunchShellSpec is not yet implemented — kiro CLI flags are unconfirmed.
// When implemented, ExecCmd MUST start with NiceExecPrefix so the kiro process
// and all child processes run at niceness 10 and cannot starve the host UI.
func (p *kiroProvider) LaunchShellSpec(_ LaunchSpec, _ func(string) (string, error)) (ShellSpec, error) {
	return ShellSpec{}, fmt.Errorf("real launch for runtime %q is not yet implemented: CLI flags unconfirmed", "kiro")
}

func (p *kiroProvider) ResumeInfo() ResumeInfo                         { return ResumeInfo{Supported: false} }
func (p *kiroProvider) MCPConfigStyle() MCPStyle                       { return MCPStyleNone }
func (p *kiroProvider) PolicyEnforcementLevel() PolicyEnforcement      { return PolicyEnforcementInstructionOnly }
func (p *kiroProvider) NativeSessionDetection() *NativeSessionStrategy { return nil }
func (p *kiroProvider) StartupDialogResponse() string { return "" }
func (p *kiroProvider) ModelHint() string             { return "" }
func (p *kiroProvider) KnownModels() []string         { return nil }
