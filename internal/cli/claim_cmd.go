package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
)

// claimRecord is the on-disk representation of a file-lock claim.
type claimRecord struct {
	Agent     string   `json:"agent"`
	TaskID    string   `json:"task_id,omitempty"`
	Paths     []string `json:"paths"`
	ClaimedAt string   `json:"claimed_at"`
}

func claimsDir(repoPath string) string {
	return filepath.Join(repoPath, ".aom", "claims")
}

func claimFilePath(repoPath, agentName string) string {
	return filepath.Join(claimsDir(repoPath), agentName+".json")
}

// executeClaim dispatches aom claim subcommands.
// Default (no subcommand): claim files.
// Usage:
//
//	aom claim <paths...> [--agent <name>] [--task <id>]
//	aom claim release [--agent <name>]
//	aom claim list
func (r Runner) executeClaim(args []string) error {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			r.printHelp()
			return nil
		}
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: aom claim <paths...> [--agent <name>] [--task <id>]\n       aom claim release [--agent <name>]\n       aom claim list")
	}
	switch args[0] {
	case "release":
		return r.executeClaimRelease(args[1:])
	case "list":
		return r.executeClaimList(args[1:])
	default:
		return r.executeClaimAdd(args)
	}
}

// executeClaimAdd claims one or more paths for an agent.
// Usage: aom claim <paths...> [--agent <name>] [--task <id>]
func (r Runner) executeClaimAdd(args []string) error {
	var paths []string
	var agentName, taskID string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			agentName = strings.TrimSpace(args[i])
		case "--task":
			i++
			if i >= len(args) {
				return fmt.Errorf("--task requires a value")
			}
			taskID = strings.TrimSpace(args[i])
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown flag %q", args[i])
			}
			paths = append(paths, strings.TrimSpace(args[i]))
		}
	}

	if len(paths) == 0 {
		return fmt.Errorf("at least one path is required")
	}
	if agentName == "" {
		agentName = resolvedActor()
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// Check for overlapping claims from other agents.
	existingClaims, err := loadAllClaims(result.Project.RepoPath)
	if err != nil {
		return err
	}
	for _, existing := range existingClaims {
		if existing.Agent == agentName {
			continue
		}
		for _, newPath := range paths {
			for _, claimedPath := range existing.Paths {
				if filepath.Clean(newPath) == filepath.Clean(claimedPath) {
					fmt.Fprintf(r.stderr, "warning: %q is already claimed by agent %q (task %s)\n",
						newPath, existing.Agent, existing.TaskID)
				}
			}
		}
	}

	if err := os.MkdirAll(claimsDir(result.Project.RepoPath), 0o755); err != nil {
		return fmt.Errorf("create claims dir: %w", err)
	}

	rec := claimRecord{
		Agent:     agentName,
		TaskID:    taskID,
		Paths:     paths,
		ClaimedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal claim: %w", err)
	}
	if err := os.WriteFile(claimFilePath(result.Project.RepoPath, agentName), data, 0o644); err != nil {
		return fmt.Errorf("write claim: %w", err)
	}

	fmt.Fprintf(r.stdout, "Claimed %d path(s) for agent %q.\n", len(paths), agentName)
	for _, p := range paths {
		fmt.Fprintf(r.stdout, "  %s\n", p)
	}
	if taskID != "" {
		fmt.Fprintf(r.stdout, "Task: %s\n", taskID)
	}
	fmt.Fprintf(r.stdout, "To release: aom claim release --agent %s\n", agentName)
	return nil
}

// executeClaimRelease removes the claim lock for an agent.
// Usage: aom claim release [--agent <name>]
func (r Runner) executeClaimRelease(args []string) error {
	agentName := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			agentName = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if agentName == "" {
		agentName = resolvedActor()
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	claimPath := claimFilePath(result.Project.RepoPath, agentName)
	if _, statErr := os.Stat(claimPath); os.IsNotExist(statErr) {
		fmt.Fprintf(r.stdout, "No active claim for agent %q.\n", agentName)
		return nil
	}

	if err := os.Remove(claimPath); err != nil {
		return fmt.Errorf("release claim: %w", err)
	}

	fmt.Fprintf(r.stdout, "Claim released for agent %q.\n", agentName)
	return nil
}

// executeClaimList shows all active claims across all agents.
// Usage: aom claim list
func (r Runner) executeClaimList(_ []string) error {
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	claims, err := loadAllClaims(result.Project.RepoPath)
	if err != nil {
		return err
	}
	if len(claims) == 0 {
		fmt.Fprintln(r.stdout, "No active file claims.")
		return nil
	}

	fmt.Fprintln(r.stdout, "Active file claims:")
	fmt.Fprintln(r.stdout, "")
	for _, c := range claims {
		taskLabel := ""
		if c.TaskID != "" {
			taskLabel = "  task: " + c.TaskID
		}
		fmt.Fprintf(r.stdout, "Agent: %s%s  (claimed %s)\n", c.Agent, taskLabel, c.ClaimedAt)
		for _, p := range c.Paths {
			fmt.Fprintf(r.stdout, "  %s\n", p)
		}
	}
	return nil
}

// warnOnClaimOverlap prints a warning when other agents have active file claims.
// This is a soft warning — it never blocks spawn.
func (r Runner) warnOnClaimOverlap(result *project.OpenResult, spawningAgent string) {
	claims, err := loadAllClaims(result.Project.RepoPath)
	if err != nil || len(claims) == 0 {
		return
	}
	for _, c := range claims {
		if c.Agent == spawningAgent {
			continue
		}
		taskSuffix := ""
		if c.TaskID != "" {
			taskSuffix = " (task " + c.TaskID + ")"
		}
		fmt.Fprintf(r.stderr, "warning: agent %q holds %d file claim(s)%s — verify no overlap before proceeding\n",
			c.Agent, len(c.Paths), taskSuffix)
		for _, p := range c.Paths {
			fmt.Fprintf(r.stderr, "  claimed: %s\n", p)
		}
	}
}

// loadAllClaims reads every .json file in the claims directory.
func loadAllClaims(repoPath string) ([]claimRecord, error) {
	dir := claimsDir(repoPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read claims dir: %w", err)
	}

	var records []claimRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var rec claimRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		records = append(records, rec)
	}
	return records, nil
}
