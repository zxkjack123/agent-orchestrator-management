package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/agent"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/artifact"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/provider"
	aomruntime "github.com/lattapon-aek/agent-orchestrator-management/internal/runtime"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/session"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/task"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/worktree"
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

func (r Runner) resolveTaskExecutionPath(result *project.OpenResult, agentRecord *agent.Record, taskRecord task.Record) (*worktree.Record, string, error) {
	// Per-Agent Workspace: if this agent has a provisioned workspace, always
	// use it — the agent never needs to change CWD between tasks.
	if agentRecord != nil && strings.TrimSpace(agentRecord.WorkspacePath) != "" {
		return nil, agentRecord.WorkspacePath, nil
	}

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

	if err := artifact.MaterializeCodexConfig(agentRecord.Name, agentRecord.Runtime, worktreePath); err != nil {
		return fmt.Errorf("materialize codex config: %w", err)
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

		// Inject repo layout if the operator has generated one (aom project layout).
		// Copied into .agent/shared/repo-layout.md so agents can read it without
		// needing access to the .aom/ operator directory.
		layoutSrc := filepath.Join(result.Project.RepoPath, ".aom", "shared", "repo-layout.md")
		if layoutData, readErr := os.ReadFile(layoutSrc); readErr == nil {
			agentSharedDir := filepath.Join(worktreePath, ".agent", "shared")
			if mkErr := os.MkdirAll(agentSharedDir, 0o755); mkErr == nil {
				_ = os.WriteFile(filepath.Join(agentSharedDir, "repo-layout.md"), layoutData, 0o644)
			}
		}

		// Write standalone .agent/team-roster.md so the agent can refresh
		// their team view at any point during work with `aom team roster`.
		r.writeTeamRosterArtifact(result, agentRecord, worktreePath)
	}

	return nil
}

// buildTeamRosterNote returns a markdown "Your Team" section listing all project agents
// with their current task assignments and session status, for injection into the spawned
// agent's identity file. Non-fatal: returns "" on any DB error.
func (r Runner) buildTeamRosterNote(result *project.OpenResult, selfName string) string {
	if len(result.Agents) == 0 {
		return ""
	}

	// Build agent → task title map (non-fatal).
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

	// Build agent → live session status map (non-fatal).
	agentStatus := make(map[string]string)
	if sessService, sessDB, err := r.app.OpenSessionService(result.DBPath); err == nil {
		defer sessDB.Close()
		if sessions, err := sessService.ListByProject(result.Project.ID); err == nil {
			for _, s := range sessions {
				if s.AgentName != "" && isActiveSessionStatus(s.Status) {
					agentStatus[s.AgentName] = s.Status
				}
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("\n## Your Team\n\n")
	sb.WriteString("| Agent | Role | Runtime | Session | Current Task |\n")
	sb.WriteString("|-------|------|---------|---------|-------------|\n")
	for _, a := range result.Agents {
		assignment := agentTask[a.Name]
		if assignment == "" {
			assignment = "(unassigned)"
		}
		status := agentStatus[a.Name]
		if status == "" {
			status = "—"
		}
		marker := ""
		if a.Name == selfName {
			marker = " *(you)*"
		}
		fmt.Fprintf(&sb, "| %s%s | %s | %s | %s | %s |\n", a.Name, marker, a.Role, a.Runtime, status, assignment)
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

// writeTeamRosterArtifact writes .agent/team-roster.md into the worktree so the agent
// has a standalone, refreshable view of the team and their task dependencies.
// Silently skipped when worktreePath is empty or any write fails (non-fatal).
func (r Runner) writeTeamRosterArtifact(result *project.OpenResult, agentRecord *agent.Record, worktreePath string) {
	if strings.TrimSpace(worktreePath) == "" {
		return
	}
	content := r.buildTeamRosterFileContent(result, agentRecord)
	if content == "" {
		return
	}
	agentDir := filepath.Join(worktreePath, ".agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(agentDir, "team-roster.md"), []byte(content), 0o644)
}

// buildTeamRosterFileContent generates the full content for .agent/team-roster.md
// for a given agent: team table with session status, dependency graph, and
// communication quick-reference. All DB queries are non-fatal.
func (r Runner) buildTeamRosterFileContent(result *project.OpenResult, agentRecord *agent.Record) string {
	if agentRecord == nil || len(result.Agents) == 0 {
		return ""
	}

	// 1. Agent → task map + locate self task (all in one DB open).
	agentTask := make(map[string]string) // agent name → "TASK-xxx — title"
	var selfTaskID, selfTaskTitle string
	var blockedBy, unblocks []task.Record

	if taskSvc, db, err := r.app.OpenTaskService(result.DBPath); err == nil {
		defer db.Close()
		if tasks, err := taskSvc.ListByProject(result.Project.ID); err == nil {
			for _, t := range tasks {
				if t.PreferredAgent != "" && t.Status != "Done" && t.Status != "Archived" {
					agentTask[t.PreferredAgent] = fmt.Sprintf("%s — %s", t.ID, t.Title)
					if t.PreferredAgent == agentRecord.Name {
						selfTaskID = t.ID
						selfTaskTitle = t.Title
					}
				}
			}
		}
		if selfTaskID != "" {
			blockedBy, _ = taskSvc.BlockedBy(selfTaskID)
			unblocks, _ = taskSvc.Unblocks(selfTaskID)
		}
	}

	// 2. Agent → live session status map (non-fatal).
	agentStatus := make(map[string]string)
	if sessService, sessDB, err := r.app.OpenSessionService(result.DBPath); err == nil {
		defer sessDB.Close()
		if sessions, err := sessService.ListByProject(result.Project.ID); err == nil {
			for _, s := range sessions {
				if s.AgentName != "" && isActiveSessionStatus(s.Status) {
					agentStatus[s.AgentName] = s.Status
				}
			}
		}
	}

	var sb strings.Builder
	now := time.Now()

	// Header.
	fmt.Fprintf(&sb, "# Team Roster\n")
	fmt.Fprintf(&sb, "- Generated: %s\n", now.Format(time.RFC3339))
	fmt.Fprintf(&sb, "- Agent: %s\n", agentRecord.Name)
	if selfTaskID != "" {
		fmt.Fprintf(&sb, "- Task: %s — %s\n", selfTaskID, selfTaskTitle)
	}
	fmt.Fprintf(&sb, "- Refresh anytime: `aom team roster`\n\n")

	// Your Place section.
	fmt.Fprintf(&sb, "## Your Place in the Team\n\n")
	fmt.Fprintf(&sb, "You are **%s** — role: %s, runtime: %s\n", agentRecord.Name, agentRecord.Role, agentRecord.Runtime)
	if selfTaskID != "" {
		fmt.Fprintf(&sb, "Working on: %s — %s\n\n", selfTaskID, selfTaskTitle)
	} else {
		fmt.Fprintf(&sb, "Working on: (unassigned — run `aom next` to see available tasks)\n\n")
	}

	// Teammates table (skip self).
	fmt.Fprintf(&sb, "## Teammates\n\n")
	hasTeammates := false
	for _, a := range result.Agents {
		if a.Name != agentRecord.Name {
			hasTeammates = true
			break
		}
	}
	if !hasTeammates {
		fmt.Fprintf(&sb, "No other agents configured.\n\n")
	} else {
		fmt.Fprintf(&sb, "| Agent | Role | Runtime | Session | Current Task |\n")
		fmt.Fprintf(&sb, "|-------|------|---------|---------|-------------|\n")
		for _, a := range result.Agents {
			if a.Name == agentRecord.Name {
				continue
			}
			assignment := agentTask[a.Name]
			if assignment == "" {
				assignment = "(unassigned)"
			}
			status := agentStatus[a.Name]
			if status == "" {
				status = "—"
			}
			fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s |\n", a.Name, a.Role, a.Runtime, status, assignment)
		}
		sb.WriteString("\n")
	}

	// Dependencies section.
	fmt.Fprintf(&sb, "## Your Dependencies\n\n")

	activeBlockers := make([]task.Record, 0, len(blockedBy))
	for _, b := range blockedBy {
		if b.Status != "Done" && b.Status != "Archived" {
			activeBlockers = append(activeBlockers, b)
		}
	}
	if len(activeBlockers) == 0 {
		fmt.Fprintf(&sb, "Blocked by: none — you can start work immediately.\n\n")
	} else {
		fmt.Fprintf(&sb, "You are blocked by (wait for these to finish first):\n")
		for _, b := range activeBlockers {
			owner := b.PreferredAgent
			if owner == "" {
				owner = b.PreferredRole
			}
			if owner == "" {
				owner = "unassigned"
			}
			fmt.Fprintf(&sb, "  - %s — %s (owner: %s, status: %s)\n", b.ID, b.Title, owner, b.Status)
			if owner != "unassigned" {
				fmt.Fprintf(&sb, "    → check status: aom message send %s \"Status update on %s?\"\n", owner, b.ID)
			}
		}
		sb.WriteString("\n")
	}

	activeUnblocks := make([]task.Record, 0, len(unblocks))
	for _, u := range unblocks {
		if u.Status != "Done" && u.Status != "Archived" {
			activeUnblocks = append(activeUnblocks, u)
		}
	}
	if len(activeUnblocks) == 0 {
		fmt.Fprintf(&sb, "Your output unblocks: none.\n\n")
	} else {
		fmt.Fprintf(&sb, "Your output unblocks (notify them when you are done):\n")
		for _, u := range activeUnblocks {
			owner := u.PreferredAgent
			if owner == "" {
				owner = u.PreferredRole
			}
			if owner == "" {
				owner = "unassigned"
			}
			fmt.Fprintf(&sb, "  - %s — %s (owner: %s)\n", u.ID, u.Title, owner)
			if owner != "unassigned" {
				fmt.Fprintf(&sb, "    → when done: aom message send %s \"READY: <one-line summary>\"\n", owner)
			}
		}
		sb.WriteString("\n")
	}

	// Communication quick-reference.
	fmt.Fprintf(&sb, "## Communication Quick Reference\n\n")
	fmt.Fprintf(&sb, "  aom channel append \"%s: <message>\"    # broadcast to team\n", agentRecord.Name)
	fmt.Fprintf(&sb, "  aom message read %s                       # check your inbox\n", agentRecord.Name)
	for _, a := range result.Agents {
		if a.Name == agentRecord.Name {
			continue
		}
		fmt.Fprintf(&sb, "  aom message send %s \"<message>\"   # DM %s\n", a.Name, a.Name)
	}
	fmt.Fprintf(&sb, "  aom team roster                            # refresh this file\n")
	fmt.Fprintf(&sb, "  aom next                                   # see available tasks\n")

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

// isActiveSessionStatus reports whether the session is expected to have a live pane.
func isActiveSessionStatus(status string) bool {
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
			time.Sleep(time.Second)
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

// writeCurrentTaskFile writes .agent/current-task.md into the agent workspace
// so the agent always knows which task it is currently working on.
func writeCurrentTaskFile(workspacePath, taskID, taskTitle string) {
	agentDir := filepath.Join(workspacePath, ".agent")
	_ = os.MkdirAll(agentDir, 0o755)
	content := fmt.Sprintf("# Current Task\n\nTask: %s\nTitle: %s\nArtifacts: .agent/\n", taskID, taskTitle)
	_ = os.WriteFile(filepath.Join(agentDir, "current-task.md"), []byte(content), 0o644)
}
