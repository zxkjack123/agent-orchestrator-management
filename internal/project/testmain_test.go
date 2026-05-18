package project

import (
	"os"
	"testing"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/config"
)

// TestMain seeds the known-runtime registries with the built-in provider names
// before any test in this package runs. In production code this registration
// happens via internal/app init(), which the project package cannot import.
func TestMain(m *testing.M) {
	for _, name := range []string{"claude", "codex", "gemini", "kiro"} {
		RegisterKnownInitRuntime(name)
		config.RegisterKnownRuntime(name)
	}
	os.Exit(m.Run())
}
