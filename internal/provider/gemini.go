package provider

import "fmt"

type geminiProvider struct{}

func (p *geminiProvider) Name() string            { return "gemini" }
func (p *geminiProvider) IdentityFilename() string { return "GEMINI.md" }

// LaunchShellSpec is not yet implemented — gemini CLI flags are unconfirmed.
// When implemented, ExecCmd MUST start with NiceExecPrefix so the gemini process
// and all child processes run at niceness 10 and cannot starve the host UI.
func (p *geminiProvider) LaunchShellSpec(_ LaunchSpec, _ func(string) (string, error)) (ShellSpec, error) {
	return ShellSpec{}, fmt.Errorf("real launch for runtime %q is not yet implemented: CLI flags unconfirmed", "gemini")
}

func (p *geminiProvider) ResumeInfo() ResumeInfo                         { return ResumeInfo{Supported: false} }
func (p *geminiProvider) MCPConfigStyle() MCPStyle                       { return MCPStyleNone }
func (p *geminiProvider) PolicyEnforcementLevel() PolicyEnforcement      { return PolicyEnforcementInstructionOnly }
func (p *geminiProvider) NativeSessionDetection() *NativeSessionStrategy { return nil }
func (p *geminiProvider) StartupDialogResponse() string { return "" }
func (p *geminiProvider) ModelHint() string             { return "" }
func (p *geminiProvider) KnownModels() []string {
	return []string{"gemini-2.5-pro", "gemini-2.5-flash"}
}
