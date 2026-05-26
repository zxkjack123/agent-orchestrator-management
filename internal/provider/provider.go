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
	Model          string // optional; empty means use the CLI's default model
	// BypassSandbox instructs a provider to skip its internal sandbox layer.
	// For codex this replaces --sandbox danger-full-access with
	// --dangerously-bypass-approvals-and-sandbox, which prevents bwrap from
	// being invoked.  Required on WSL2 where bwrap overlay causes git to spin
	// at 60–100% CPU indefinitely.  Set via policy.yaml codex_bypass_sandbox: true.
	BypassSandbox bool
	// WorktreePath is the agent's workspace directory. When non-empty, providers
	// that tend to navigate away from their CWD (e.g. codex) prepend a
	// "cd <WorktreePath>" statement to the preamble so the agent starts in the
	// correct directory even if the runtime later discovers a git root elsewhere.
	WorktreePath string
}

// NiceExecPrefix is the standard exec prefix for all agent runtimes.
// It runs the runtime process at niceness 10, below interactive processes
// (nice=0) but above idle background tasks (nice=19). This prevents any
// agent — and all child processes it spawns — from starving the host UI
// or other user applications under CPU load.
//
// Usage in every provider's LaunchShellSpec:
//
//	execCmd = NiceExecPrefix + "myruntime --flag ..."
//
// All future providers must use this prefix. Do not inline "exec nice -n 10"
// directly; use this constant so the niceness level is tuned in one place.
const NiceExecPrefix = "exec nice -n 10 "

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
	PolicyEnforcementWrapperScript                      // e.g. codex PATH-based shell wrappers
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
	// StartupDialogResponse returns the key to send to auto-accept the initial
	// startup dialog shown by this runtime (e.g. bypass-permissions for claude,
	// directory trust for codex). Returns "" when no key should be sent.
	StartupDialogResponse() string
	// ModelHint returns a human-readable description of how to choose a model
	// for this provider. Intended for spawn-time output and identity file injection
	// so operators and orchestrator agents can make informed model selections.
	// Returns "" for providers that do not support model selection.
	ModelHint() string
	// KnownModels returns a list of recognized model slugs for this provider.
	// An empty slice means the provider does not maintain a static model list.
	// Used for soft-warn validation at spawn time.
	KnownModels() []string
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
func (p *fallbackProvider) StartupDialogResponse() string { return "" }
func (p *fallbackProvider) ModelHint() string             { return "" }
func (p *fallbackProvider) KnownModels() []string         { return nil }
