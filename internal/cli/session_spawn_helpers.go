package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/agent"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/artifact"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/project"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/provider"
	aomruntime "github.com/lattapon-aek/Agents-Orchestfator-Management/internal/runtime"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/task"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/worktree"
)

func (r Runner) ensurePlannedWorktree(result *project.OpenResult, taskRecord task.Record) (*worktree.Record, error) {
	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return nil, err
	}
	defer worktreeDB.Close()

	record, err := worktreeService.CreatePlanned(worktree.CreateParams{
		ProjectID:     result.Project.ID,
		TaskID:        taskRecord.ID,
		TaskTitle:     taskRecord.Title,
		RepoPath:      result.Project.RepoPath,
		DefaultBranch: result.Project.DefaultBranch,
	})
	if err != nil {
		return nil, err
	}

	return worktreeService.EnsureProvisioned(record.TaskID, result.Project.RepoPath)
}

func (r Runner) validateTaskProvisioning(result *project.OpenResult) error {
	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer worktreeDB.Close()

	return worktreeService.ValidateProvisioningPreconditions(
		result.Project.RepoPath,
		result.Project.DefaultBranch,
	)
}

func (r Runner) enforceWriterWorktreeBoundary(result *project.OpenResult, agentRecord *agent.Record, params sessionSpawnParams) error {
	if result == nil || agentRecord == nil {
		return nil
	}
	if strings.TrimSpace(params.taskID) == "" {
		return nil
	}
	if roleWorktreeMode(result, agentRecord.Role) != "dedicated-writer" {
		return nil
	}

	sessions, err := r.loadProjectSessions(result)
	if err != nil {
		return err
	}

	for _, item := range sessions {
		if item.TaskID != params.taskID {
			continue
		}
		if strings.TrimSpace(params.ignoreSessionID) != "" && item.ID == params.ignoreSessionID {
			continue
		}
		if roleWorktreeMode(result, item.RoleName) != "dedicated-writer" {
			continue
		}
		if !writerSessionOccupiesWorktree(item.Status) {
			continue
		}
		return fmt.Errorf(
			"task %q already has active writer session %q (%s, status=%s); run \"aom session replace %s --agent <name> --reason <why>\" to hand off, or \"aom session stop %s\" to stop it first",
			params.taskID,
			item.ID,
			item.AgentName,
			item.Status,
			item.ID,
			item.ID,
		)
	}

	return nil
}

func (r Runner) resolveTaskExecutionPath(result *project.OpenResult, taskRecord task.Record) (*worktree.Record, string, error) {
	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return nil, "", err
	}
	defer worktreeDB.Close()

	mapping, err := worktreeService.GetByTask(taskRecord.ID)
	if err != nil {
		return nil, "", err
	}
	if mapping == nil {
		return nil, result.Project.RepoPath, nil
	}
	if mapping, err = worktreeService.Reconcile(mapping.TaskID, result.Project.RepoPath, false); err != nil {
		return nil, "", err
	}
	if mapping != nil && mapping.Status == worktree.StatusNeedsRepair {
		driftKind, err := worktreeService.DriftKind(taskRecord.ID, result.Project.RepoPath)
		if err != nil {
			return nil, "", err
		}
		return mapping, "", fmt.Errorf("worktree for task %q needs repair: %s", taskRecord.ID, worktreeHint(taskRecord.ID, mapping, driftKind))
	}
	if mapping.Status != worktree.StatusReady && mapping.Status != worktree.StatusActive {
		mapping, err = worktreeService.EnsureProvisioned(mapping.TaskID, result.Project.RepoPath)
		if err != nil {
			return nil, "", err
		}
	}
	if mapping == nil || (mapping.Status != worktree.StatusReady && mapping.Status != worktree.StatusActive) || strings.TrimSpace(mapping.WorktreePath) == "" {
		return mapping, result.Project.RepoPath, nil
	}

	return mapping, mapping.WorktreePath, nil
}

// materializeAgentContext consolidates all three spawn-time context injections:
// identity file, role skill files, and MCP server config. Both spawn sites call
// this so they cannot diverge.
func (r Runner) materializeAgentContext(result *project.OpenResult, agentRecord *agent.Record, worktreePath string) error {
	profilePath := filepath.Join(result.Project.RepoPath, ".aom", "agents", agentRecord.Name, "profile.md")
	if err := artifact.MaterializeIdentityFile(agentRecord.Name, agentRecord.Runtime, worktreePath, profilePath); err != nil {
		return fmt.Errorf("materialize identity file: %w", err)
	}

	roleRes := result.Resources.ResourcesForRole(agentRecord.Role, agentRecord.Runtime)

	if err := artifact.MaterializeSkillFiles(agentRecord.Name, roleRes.Skills, result.Project.RepoPath, worktreePath); err != nil {
		return fmt.Errorf("materialize skill files: %w", err)
	}

	if err := artifact.MaterializeMCPConfig(agentRecord.Name, agentRecord.Runtime, roleRes.MCPServers, worktreePath); err != nil {
		return fmt.Errorf("materialize mcp config: %w", err)
	}

	if err := artifact.MaterializePolicyConstraints(agentRecord.Name, agentRecord.Runtime, result.Policy.Policy.DenyCommands, worktreePath); err != nil {
		return fmt.Errorf("materialize policy constraints: %w", err)
	}

	// Append project-board pointer to the agent's identity file so it knows where to
	// find the shared task list. Silently skipped when the worktree or identity file is absent.
	if worktreePath != "" {
		identityFile := r.registry.Lookup(agentRecord.Runtime).IdentityFilename()
		if identityFile != "" {
			boardPath := filepath.Join(result.Project.RepoPath, ".aom", "project-board.md")
			note := fmt.Sprintf("\n## Project Board\nFull team task board: %s\nUse `aom task list` to see live task state.\n", boardPath)
			idPath := filepath.Join(worktreePath, identityFile)
			if f, err := os.OpenFile(idPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644); err == nil {
				_, _ = f.WriteString(note)
				_ = f.Close()
			}
		}
	}

	return nil
}

// enforcePolicyDefaults surfaces policy information at spawn time.
// For runtimes with PolicyEnforcementRuntimeFlag (e.g. claude): deny_commands are enforced
// via --disallowed-tools. For all other runtimes: they are written to the identity file as
// instructions only.
func (r Runner) enforcePolicyDefaults(result *project.OpenResult, agentRuntime string) {
	policy := result.Policy.Policy
	if policy.SessionDefaults.YoloMode == "enabled" {
		fmt.Fprintln(r.stderr, "Warning: project policy has yolo_mode=enabled — agent runs without approval gates")
	}
	if n := len(policy.DenyCommands); n > 0 {
		switch r.registry.Lookup(agentRuntime).PolicyEnforcementLevel() {
		case provider.PolicyEnforcementRuntimeFlag:
			fmt.Fprintf(r.stderr, "Policy: %d deny command(s) enforced via --disallowed-tools (runtime-level, %s)\n", n, agentRuntime)
		default:
			fmt.Fprintf(r.stderr, "Policy: %d deny command(s) written to identity file (instruction-only — %s has no runtime enforcement flag)\n", n, agentRuntime)
		}
	}
}

func roleWorktreeMode(result *project.OpenResult, roleName string) string {
	if result == nil {
		return ""
	}
	role, ok := result.RoleConfigs[strings.TrimSpace(roleName)]
	if !ok {
		return ""
	}
	return strings.TrimSpace(role.WorktreeMode)
}

func writerSessionOccupiesWorktree(status string) bool {
	switch strings.TrimSpace(status) {
	case "Created", "Booting", "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Blocked":
		return true
	default:
		return false
	}
}

func setLaunchMode(current *aomruntime.LaunchMode, next aomruntime.LaunchMode) error {
	if current == nil {
		return fmt.Errorf("launch mode target is required")
	}
	if *current == "" || *current == aomruntime.LaunchModePlaceholder {
		*current = next
		return nil
	}
	if *current == next {
		return nil
	}

	return fmt.Errorf("--mock and --real cannot be used together")
}

func sendableSessionStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "Booting", "Idle", "Working", "WaitingApproval", "WaitingHandoff", "Blocked":
		return true
	default:
		return false
	}
}
