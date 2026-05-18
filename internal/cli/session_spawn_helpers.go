package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/agent"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/artifact"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/project"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/provider"
	aomruntime "github.com/lattapon-aek/agents-orchestrator-management-private/internal/runtime"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/session"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/task"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/worktree"
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

	modelHint := r.registry.Lookup(agentRecord.Runtime).ModelHint()
	if err := artifact.MaterializeModelHint(agentRecord.Name, agentRecord.Runtime, agentRecord.Model, modelHint, worktreePath, result.Project.RepoPath); err != nil {
		return fmt.Errorf("materialize model hint: %w", err)
	}

	if worktreePath != "" {
		identityFile := r.registry.Lookup(agentRecord.Runtime).IdentityFilename()
		if identityFile != "" {
			idPath := filepath.Join(worktreePath, identityFile)
			if f, err := os.OpenFile(idPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644); err == nil {
				// Project board pointer.
				boardPath := filepath.Join(result.Project.RepoPath, ".aom", "project-board.md")
				_, _ = fmt.Fprintf(f, "\n## Project Board\nFull team task board: %s\nUse `aom task list` to see live task state.\n", boardPath)

				// Team roster — query task assignments silently; failures are non-fatal.
				rosterNote := r.buildTeamRosterNote(result, agentRecord.Name)
				if rosterNote != "" {
					_, _ = f.WriteString(rosterNote)
				}

				// Channel snapshot — last 10 messages at spawn time so the agent
				// immediately knows team decisions, API contracts, and broadcast context
				// without having to fetch channel.md separately.
				channelNote := buildChannelSnapshotNote(result.Project.RepoPath)
				if channelNote != "" {
					_, _ = f.WriteString(channelNote)
				}

				_ = f.Close()
			}
		}
	}

	return nil
}

// buildTeamRosterNote returns a markdown "Your Team" section listing all project agents
// with their current task assignments, for injection into the spawned agent's identity file.
func (r Runner) buildTeamRosterNote(result *project.OpenResult, selfName string) string {
	if len(result.Agents) == 0 {
		return ""
	}

	// Build agent → task title map from task service (best-effort, non-fatal).
	agentTask := make(map[string]string)
	if taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath); err == nil {
		defer sqlDB.Close()
		if tasks, err := taskService.ListByProject(result.Project.ID); err == nil {
			for _, t := range tasks {
				if t.PreferredAgent != "" && t.Status != "Done" && t.Status != "Archived" {
					agentTask[t.PreferredAgent] = fmt.Sprintf("%s — %s", t.ID, t.Title)
				}
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("\n## Your Team\n\n")
	sb.WriteString("| Agent | Role | Runtime | Current Task |\n")
	sb.WriteString("|-------|------|---------|-------------|\n")
	for _, a := range result.Agents {
		assignment := agentTask[a.Name]
		if assignment == "" {
			assignment = "(unassigned)"
		}
		marker := ""
		if a.Name == selfName {
			marker = " *(you)*"
		}
		fmt.Fprintf(&sb, "| %s%s | %s | %s | %s |\n", a.Name, marker, a.Role, a.Runtime, assignment)
	}
	sb.WriteString("\nTo message a teammate directly:\n")
	for _, a := range result.Agents {
		if a.Name != selfName {
			fmt.Fprintf(&sb, "  aom message send %s \"your message\"\n", a.Name)
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// buildChannelSnapshotNote returns a markdown section with the last 10 channel
// messages at spawn time. This gives agents immediate visibility into team
// decisions, API contracts, and broadcast context without reading channel.md.
func buildChannelSnapshotNote(repoPath string) string {
	msgs := lastChannelMessages(repoPath, 10)
	if len(msgs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## Team Channel (snapshot at spawn time)\n\n")
	sb.WriteString("These are the most recent broadcast messages from the team. ")
	sb.WriteString("Read the full channel: cat .aom/channel.md\n\n")
	for _, m := range msgs {
		sb.WriteString(m)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// enforcePolicyDefaults surfaces policy information at spawn time.
// For runtimes with PolicyEnforcementRuntimeFlag (e.g. claude): deny_commands are enforced
// via --disallowed-tools. For all other runtimes: they are written to the identity file as
// instructions only.
func (r Runner) enforcePolicyDefaults(result *project.OpenResult, agentRuntime, agentModel string) {
	policy := result.Policy.Policy
	if policy.SessionDefaults.YoloMode == "enabled" {
		fmt.Fprintln(r.stderr, "Warning: project policy has yolo_mode=enabled — agent runs without approval gates")
	}
	if n := len(policy.DenyCommands); n > 0 {
		switch r.registry.Lookup(agentRuntime).PolicyEnforcementLevel() {
		case provider.PolicyEnforcementRuntimeFlag:
			fmt.Fprintf(r.stderr, "Policy: %d deny command(s) enforced via --disallowed-tools (runtime-level, %s)\n", n, agentRuntime)
		case provider.PolicyEnforcementWrapperScript:
			fmt.Fprintf(r.stderr, "Policy: %d deny command(s) enforced via PATH wrapper scripts (shell-level, %s)\n", n, agentRuntime)
		default:
			fmt.Fprintf(r.stderr, "Policy: %d deny command(s) written to identity file (instruction-only — %s has no runtime enforcement flag)\n", n, agentRuntime)
		}
	}
	if agentModel != "" {
		p := r.registry.Lookup(agentRuntime)
		known := p.KnownModels()
		if len(known) > 0 && !sliceContains(known, agentModel) {
			fmt.Fprintf(r.stderr, "Warning: model %q is not in the known model list for %s — verify the slug is correct (%s)\n",
				agentModel, agentRuntime, p.ModelHint())
		}
	}
}

func sliceContains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
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

// detectUniqueVendorSessionID polls the provider's native session detection and
// skips any session ID already registered to a live sibling session in the same
// project. This prevents duplicate assignment when two sessions are spawned close
// together and the active session file for the first session is still being written
// to (keeping its mtime fresh).
func (r Runner) detectUniqueVendorSessionID(
	strategy *provider.NativeSessionStrategy,
	svc *session.Service,
	record session.Record,
	spawnedAt time.Time,
) string {
	const totalTimeout = 90 * time.Second
	const retryInterval = 5 * time.Second
	deadline := time.Now().Add(totalTimeout)

	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		pollTimeout := remaining
		if pollTimeout > retryInterval {
			pollTimeout = retryInterval
		}
		sid, _ := strategy.DetectFn(record.WorktreePath, spawnedAt, pollTimeout)
		if sid == "" {
			continue
		}
		active, err := svc.IsVendorSessionIDActive(record.ProjectID, sid)
		if err != nil || active {
			// sid belongs to a sibling session — keep polling for a distinct one
			continue
		}
		return sid
	}
	return ""
}
