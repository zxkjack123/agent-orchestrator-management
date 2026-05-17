package provider

import (
	"fmt"
	"strings"
	"time"
)

// LaunchSpec carries the data needed by LaunchShellSpec. It mirrors runtime.SessionSpec
// but lives here so the provider package does not import internal/runtime.
type LaunchSpec struct {
	SessionID      string
	AgentName      string
	RoleName       string
	AgentSessionID string
	DenyCommands   []string
}

// ShellSpec is the structured output of LaunchShellSpec. The Builder assembles
// the final sh -lc command from these parts, injecting any operator-level
// environment (e.g. PATH) before the provider preamble.
type ShellSpec struct {
	// Preamble contains shell statements executed before the main command,
	// e.g. "export AOM_RUNTIME=codex" or "unset SOME_VAR 2>/dev/null".
	// Statements must NOT contain trailing semicolons — the Builder joins them.
	Preamble []string
	// ExecCmd is the main exec invocation, e.g. "exec claude --dangerously-skip-permissions".
	ExecCmd string
}

// ResumeInfo describes session resume support for a runtime.
type ResumeInfo struct {
	Supported     bool
	FreshExample  string
	ResumeExample string
}

// MCPStyle enumerates MCP configuration delivery strategies.
type MCPStyle int

const (
	MCPStyleNone           MCPStyle = iota
	MCPStyleMarkdownAppend          // append ## MCP Servers section to a .md file
	MCPStyleJSONFile                // write a JSON config file (.codex/mcp.json)
)

// PolicyEnforcement enumerates deny_commands enforcement strategies.
type PolicyEnforcement int

const (
	PolicyEnforcementInstructionOnly PolicyEnforcement = iota
	PolicyEnforcementRuntimeFlag                        // e.g. claude --disallowed-tools
)

// NativeSessionStrategy describes how to detect a runtime's own session ID after spawn.
type NativeSessionStrategy struct {
	DetectFn func(worktreePath string, spawnedAt time.Time, timeout time.Duration) (string, error)
}

// Provider describes the behavioural contract for one agent CLI runtime.
// All methods must be pure — no I/O, no state.
//
// Adding a new provider only requires implementing this interface. Cross-cutting
// concerns (PATH injection, shell assembly) are handled by runtime.Builder — not
// here — so new providers never need to repeat that logic.
type Provider interface {
	Name() string
	IdentityFilename() string
	// LaunchShellSpec returns the structured shell components for this provider.
	// The Builder assembles the final sh -lc command, injecting operator env first.
	LaunchShellSpec(spec LaunchSpec, lookPath func(string) (string, error)) (ShellSpec, error)
	ResumeInfo() ResumeInfo
	MCPConfigStyle() MCPStyle
	PolicyEnforcementLevel() PolicyEnforcement
	NativeSessionDetection() *NativeSessionStrategy
}

// Registry maps runtime names to Provider implementations.
type Registry map[string]Provider

// DefaultRegistry returns a Registry pre-populated with all built-in providers.
func DefaultRegistry() Registry {
	providers := []Provider{
		&claudeProvider{},
		&codexProvider{},
		&geminiProvider{},
		&kiroProvider{},
	}
	r := make(Registry, len(providers))
	for _, p := range providers {
		r[p.Name()] = p
	}
	return r
}

// Lookup returns the Provider for the given runtime name.
// Returns a fallbackProvider for unknown runtimes — never returns nil.
func (r Registry) Lookup(runtimeName string) Provider {
	name := strings.TrimSpace(strings.ToLower(runtimeName))
	if p, ok := r[name]; ok {
		return p
	}
	return &fallbackProvider{name: name}
}

// fallbackProvider is returned for unrecognized runtime names.
type fallbackProvider struct{ name string }

func (p *fallbackProvider) Name() string { return p.name }
func (p *fallbackProvider) IdentityFilename() string { return "" }
func (p *fallbackProvider) LaunchShellSpec(_ LaunchSpec, _ func(string) (string, error)) (ShellSpec, error) {
	return ShellSpec{}, fmt.Errorf("real launch mode does not support runtime %q in the current milestone", p.name)
}
func (p *fallbackProvider) ResumeInfo() ResumeInfo                         { return ResumeInfo{} }
func (p *fallbackProvider) MCPConfigStyle() MCPStyle                       { return MCPStyleNone }
func (p *fallbackProvider) PolicyEnforcementLevel() PolicyEnforcement      { return PolicyEnforcementInstructionOnly }
func (p *fallbackProvider) NativeSessionDetection() *NativeSessionStrategy { return nil }
