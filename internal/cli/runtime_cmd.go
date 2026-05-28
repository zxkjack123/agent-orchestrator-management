package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/config"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/provider"
)

func (r Runner) executeRuntime(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("runtime subcommand is required: list, inspect")
	}

	switch args[0] {
	case "list":
		return r.executeRuntimeList(args[1:])
	case "inspect":
		return r.executeRuntimeInspect(args[1:])
	default:
		return fmt.Errorf("unknown runtime command %q", args[0])
	}
}

func (r Runner) executeRuntimeList(_ []string) error {
	cfg, err := config.LoadProjectConfig(".")
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}

	runtimeAgents := buildRuntimeAgentMap(cfg)
	if len(runtimeAgents) == 0 {
		fmt.Fprintln(r.stdout, "No agent runtimes configured.")
		return nil
	}

	fmt.Fprintln(r.stdout, "Configured runtimes")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "  %-8s  %-10s  %s\n", "RUNTIME", "AVAILABLE", "AGENTS")
	fmt.Fprintln(r.stdout, "  "+strings.Repeat("-", 52))

	for _, rt := range sortedKeys(runtimeAgents) {
		agents := runtimeAgents[rt]
		avail := "yes"
		if _, err := exec.LookPath(rt); err != nil {
			avail = "no"
		}
		fmt.Fprintf(r.stdout, "  %-8s  %-10s  %s\n", rt, avail, strings.Join(agents, ", "))
	}

	return nil
}

func (r Runner) executeRuntimeInspect(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("runtime name is required")
	}
	target := strings.TrimSpace(args[0])

	cfg, err := config.LoadProjectConfig(".")
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}

	// Collect agents that use this runtime.
	var agentNames []string
	for agentName, agentCfg := range cfg.Agents.Agents {
		if agentCfg.Runtime == target {
			agentNames = append(agentNames, agentName)
		}
	}

	fmt.Fprintf(r.stdout, "Runtime: %s\n", target)
	fmt.Fprintln(r.stdout, "")

	// Binary availability.
	binaryPath, err := exec.LookPath(target)
	if err != nil {
		fmt.Fprintf(r.stdout, "  Binary:        not found in PATH\n")
		fmt.Fprintf(r.stdout, "  Available:     no\n")
	} else {
		fmt.Fprintf(r.stdout, "  Binary:        %s\n", binaryPath)
		fmt.Fprintf(r.stdout, "  Available:     yes\n")
	}

	// Launch modes.
	fmt.Fprintln(r.stdout, "  Launch modes:  placeholder, mock, real")

	// Resume support.
	resumeSupport, freshArgs, resumeArgs := runtimeResumeInfo(target, r.registry)
	fmt.Fprintf(r.stdout, "  Resume:        %v\n", resumeSupport)
	if resumeSupport {
		fmt.Fprintf(r.stdout, "  Fresh start:   %s\n", freshArgs)
		fmt.Fprintf(r.stdout, "  Resume start:  %s\n", resumeArgs)
	}

	// Agents.
	fmt.Fprintln(r.stdout, "")
	if len(agentNames) == 0 {
		fmt.Fprintf(r.stdout, "  Agents:        (none configured for this runtime)\n")
	} else {
		// Sort agent names.
		for i := 1; i < len(agentNames); i++ {
			for j := i; j > 0 && agentNames[j] < agentNames[j-1]; j-- {
				agentNames[j], agentNames[j-1] = agentNames[j-1], agentNames[j]
			}
		}
		fmt.Fprintf(r.stdout, "  Agents:        %s\n", strings.Join(agentNames, ", "))
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "  %-20s  %-10s  %s\n", "AGENT", "ROLE", "ENABLED")
		fmt.Fprintln(r.stdout, "  "+strings.Repeat("-", 46))
		for _, name := range agentNames {
			agentCfg := cfg.Agents.Agents[name]
			enabled := "yes"
			if !agentCfg.Enabled {
				enabled = "no"
			}
			fmt.Fprintf(r.stdout, "  %-20s  %-10s  %s\n", name, agentCfg.Role, enabled)
		}
	}

	return nil
}

// runtimeResumeInfo returns resume support flag and example CLI invocations.
func runtimeResumeInfo(rt string, registry provider.Registry) (supported bool, freshExample, resumeExample string) {
	info := registry.Lookup(rt).ResumeInfo()
	return info.Supported, info.FreshExample, info.ResumeExample
}
