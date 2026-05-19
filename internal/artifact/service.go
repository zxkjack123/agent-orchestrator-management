package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/config"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/provider"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/session"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/step"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/task"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/worktree"
)

// defaultRegistry is the package-level provider registry used by package-level
// artifact functions (MaterializeIdentityFile, MaterializeMCPConfig, etc.).
// Tests and callers that need a custom registry should use Service methods directly.
var defaultRegistry = provider.DefaultRegistry()

// Event is one canonical AOM task timeline event.
type Event struct {
	Type        string
	Actor       string
	StepID      string
	SessionID   string
	Summary     string
	StateEffect string
}

// SyncParams describes the task state needed to write continuity artifacts.
type SyncParams struct {
	Task                  task.Record
	Steps                 []step.Record
	ActiveSession         *session.Record
	Worktree              *worktree.Record
	BlockedBy             []task.Record // tasks that block this task
	Unblocks              []task.Record // tasks this task unblocks when done
	ReviewOwnerHint       string
	ReviewOwnerAmbiguous  bool
	CreatedBy             string
	UpdatedBy             string
	RecommendedNextAction string
}

// Service writes task-local operational memory artifacts.
type Service struct {
	repoPath   string
	stateDir   string
	now        func() time.Time
	eventIDGen func() string
	registry   provider.Registry
}

// NewService creates an artifact service for one project root.
func NewService(repoPath, stateDir string) *Service {
	return &Service{
		repoPath:   repoPath,
		stateDir:   stateDir,
		now:        time.Now,
		eventIDGen: defaultEventIDGenerator,
		registry:   provider.DefaultRegistry(),
	}
}

// MaterializeIdentityFile copies one agent profile into the runtime-specific
// identity filename at the worktree root when that runtime supports it.
func MaterializeIdentityFile(agentName, runtime, worktreePath string, profileSourcePath string) error {
	if strings.TrimSpace(worktreePath) == "" {
		return nil
	}

	targetName := defaultRegistry.Lookup(runtime).IdentityFilename()
	if targetName == "" {
		return nil
	}

	data, err := os.ReadFile(profileSourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read profile source for agent %q: %w", agentName, err)
	}

	targetPath := filepath.Join(worktreePath, targetName)
	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return fmt.Errorf("write runtime identity file for agent %q: %w", agentName, err)
	}

	return nil
}

// MaterializeSkillFiles copies role-bound skill markdown files into the worktree
// root so the agent can read them directly. Source paths are project-root-relative.
// Missing skill source files are silently skipped.
func MaterializeSkillFiles(agentName string, skills []config.ResolvedSkill, repoPath, worktreePath string) error {
	if strings.TrimSpace(worktreePath) == "" || len(skills) == 0 {
		return nil
	}

	for _, skill := range skills {
		src := filepath.Join(repoPath, skill.Path)
		data, err := os.ReadFile(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read skill %q for agent %q: %w", skill.Name, agentName, err)
		}

		dst := filepath.Join(worktreePath, filepath.Base(skill.Path))
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("write skill %q for agent %q: %w", skill.Name, agentName, err)
		}
	}

	return nil
}

// MaterializeMCPConfig writes runtime-specific MCP server configuration into
// the worktree. For claude, an MCP section is appended to CLAUDE.md. For codex,
// a .codex/mcp.json file is written. Other runtimes are no-ops for now.
func MaterializeMCPConfig(agentName, runtime string, mcpServers []config.ResolvedMCPServer, worktreePath string) error {
	if strings.TrimSpace(worktreePath) == "" || len(mcpServers) == 0 {
		return nil
	}

	switch defaultRegistry.Lookup(runtime).MCPConfigStyle() {
	case provider.MCPStyleMarkdownAppend:
		return appendClaudeMCPSection(agentName, mcpServers, worktreePath)
	case provider.MCPStyleJSONFile:
		return writeCodexMCPConfig(agentName, mcpServers, worktreePath)
	default:
		return nil
	}
}

func appendClaudeMCPSection(agentName string, servers []config.ResolvedMCPServer, worktreePath string) error {
	var b strings.Builder
	b.WriteString("\n## MCP Servers\n\n")
	b.WriteString("The following MCP servers are available for this session via project governance:\n\n")
	for _, s := range servers {
		switch s.Type {
		case "stdio":
			cmd := s.Command
			if len(s.Args) > 0 {
				cmd += " " + strings.Join(s.Args, " ")
			}
			fmt.Fprintf(&b, "- **%s** (stdio): `%s`\n", s.Name, cmd)
		case "http":
			fmt.Fprintf(&b, "- **%s** (http): `%s`\n", s.Name, s.URL)
		}
	}

	claudeMD := filepath.Join(worktreePath, "CLAUDE.md")
	f, err := os.OpenFile(claudeMD, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open CLAUDE.md for agent %q: %w", agentName, err)
	}
	defer f.Close()

	if _, err := f.WriteString(b.String()); err != nil {
		return fmt.Errorf("append MCP section to CLAUDE.md for agent %q: %w", agentName, err)
	}
	return nil
}

// MaterializePolicyConstraints appends the project deny_commands list to the
// runtime-specific identity file so the agent is aware of project-level rules.
// Missing or unrecognised runtimes are silently skipped.
func MaterializePolicyConstraints(agentName, runtime string, denyCommands []string, worktreePath string) error {
	if strings.TrimSpace(worktreePath) == "" || len(denyCommands) == 0 {
		return nil
	}
	targetName := defaultRegistry.Lookup(runtime).IdentityFilename()
	if targetName == "" {
		return nil
	}

	var b strings.Builder
	b.WriteString("\n## Policy Constraints\n\n")
	b.WriteString("The following commands are prohibited by project policy:\n\n")
	for _, cmd := range denyCommands {
		fmt.Fprintf(&b, "- `%s`\n", cmd)
	}

	targetPath := filepath.Join(worktreePath, targetName)
	f, err := os.OpenFile(targetPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open identity file for policy constraints (agent %q): %w", agentName, err)
	}
	defer f.Close()

	if _, err := f.WriteString(b.String()); err != nil {
		return fmt.Errorf("append policy constraints to identity file for agent %q: %w", agentName, err)
	}
	return nil
}

// MaterializeModelHint appends a ## Model Selection section to the runtime-specific
// identity file so operators and orchestrator agents can see what model this session
// is using and what other models are available for this provider.
// When worktreePath is empty, falls back to projectRootPath so orchestrators spawned
// without a dedicated worktree still see model configuration.
// Silently skipped when the provider has no model hint or both paths are empty.
func MaterializeModelHint(agentName, runtime, model, modelHint, worktreePath, projectRootPath string) error {
	effectivePath := worktreePath
	if strings.TrimSpace(effectivePath) == "" {
		effectivePath = projectRootPath
	}
	if strings.TrimSpace(effectivePath) == "" || strings.TrimSpace(modelHint) == "" {
		return nil
	}
	targetName := defaultRegistry.Lookup(runtime).IdentityFilename()
	if targetName == "" {
		return nil
	}

	var b strings.Builder
	b.WriteString("\n## Model Selection\n\n")
	if model != "" {
		fmt.Fprintf(&b, "This session is running with model: **%s**\n\n", model)
	} else {
		b.WriteString("This session is using the provider's default model.\n\n")
	}
	fmt.Fprintf(&b, "Available models for this provider: %s\n", modelHint)
	b.WriteString("\nTo change the model for an agent, set model: in .aom/agents.yaml and respawn.\n")

	targetPath := filepath.Join(effectivePath, targetName)
	f, err := os.OpenFile(targetPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open identity file for model hint (agent %q): %w", agentName, err)
	}
	defer f.Close()

	if _, err := f.WriteString(b.String()); err != nil {
		return fmt.Errorf("append model hint to identity file for agent %q: %w", agentName, err)
	}
	return nil
}

func writeCodexMCPConfig(agentName string, servers []config.ResolvedMCPServer, worktreePath string) error {
	type stdioEntry struct {
		Type    string   `json:"type"`
		Command string   `json:"command"`
		Args    []string `json:"args,omitempty"`
	}
	type httpEntry struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	}

	entries := make(map[string]any, len(servers))
	for _, s := range servers {
		switch s.Type {
		case "stdio":
			entries[s.Name] = stdioEntry{Type: "stdio", Command: s.Command, Args: s.Args}
		case "http":
			entries[s.Name] = httpEntry{Type: "http", URL: s.URL}
		}
	}

	payload := map[string]any{"mcpServers": entries}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal codex MCP config for agent %q: %w", agentName, err)
	}

	dir := filepath.Join(worktreePath, ".codex")
	if err := ensureDir(dir); err != nil {
		return fmt.Errorf("create .codex dir for agent %q: %w", agentName, err)
	}
	dst := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("write .codex/mcp.json for agent %q: %w", agentName, err)
	}
	return nil
}

// SeedTaskArtifacts creates the initial task artifact set and appends seed events.
func (s *Service) SeedTaskArtifacts(params SyncParams) error {
	if err := s.writeArtifacts(params); err != nil {
		return err
	}

	if err := s.appendEvent(params, Event{
		Type:        "task.created",
		Actor:       defaultActor(params.CreatedBy),
		Summary:     fmt.Sprintf("Task created in %s mode", params.Task.Mode),
		StateEffect: fmt.Sprintf("Task %s", params.Task.Status),
	}); err != nil {
		return err
	}

	for _, item := range params.Steps {
		if err := s.appendEvent(params, Event{
			Type:        "step.proposed",
			Actor:       "aom",
			StepID:      item.ID,
			Summary:     fmt.Sprintf("Step seeded: %s", item.Title),
			StateEffect: fmt.Sprintf("Step %s", item.Status),
		}); err != nil {
			return err
		}
	}

	return nil
}

// RefreshTaskArtifacts rewrites the current task state artifacts without changing the timeline.
func (s *Service) RefreshTaskArtifacts(params SyncParams) error {
	return s.writeArtifacts(params)
}

// EnsureReviewNotesTemplate creates or refreshes a structured review-notes.md template.
// If an existing file belongs to a different task (stale content), it is overwritten.
func (s *Service) EnsureReviewNotesTemplate(params SyncParams, reviewer, sessionID string) error {
	dir := s.taskDir(params)
	if err := ensureDir(dir); err != nil {
		return fmt.Errorf("create artifact dir %q: %w", dir, err)
	}

	path := filepath.Join(dir, "review-notes.md")
	if existing, err := os.ReadFile(path); err == nil {
		existingTaskID := reviewNotesTaskID(string(existing))
		if existingTaskID == "" || existingTaskID == params.Task.ID {
			// No task ID (manual/real findings) or already owned by this task — preserve.
			return nil
		}
		// Task ID present but belongs to a different task — stale, overwrite.
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat review-notes.md: %w", err)
	}

	content := s.renderReviewNotesMarkdown(params, reviewer, sessionID)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write review-notes.md: %w", err)
	}
	return nil
}

// reviewNotesTaskID extracts the task ID from a review-notes.md header line "- Task: <id>".
func reviewNotesTaskID(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- Task:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "- Task:"))
		}
	}
	return ""
}

// EnsureHandoffTemplate creates a structured handoff.md template for one task-bound session.
func (s *Service) EnsureHandoffTemplate(params SyncParams, activeSession session.Record) error {
	dir := s.taskDir(params)
	if err := ensureDir(dir); err != nil {
		return fmt.Errorf("create artifact dir %q: %w", dir, err)
	}

	path := filepath.Join(dir, "handoff.md")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat handoff.md: %w", err)
	}

	content := s.renderHandoffTemplateMarkdown(params, activeSession)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write handoff.md: %w", err)
	}
	return nil
}

// AppendEvent records one canonical task log event.
func (s *Service) AppendEvent(params SyncParams, event Event) error {
	return s.appendEvent(params, event)
}

// AppendEvents records multiple canonical task log events in order.
func (s *Service) AppendEvents(params SyncParams, events []Event) error {
	for _, event := range events {
		if err := s.appendEvent(params, event); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) writeArtifacts(params SyncParams) error {
	if strings.TrimSpace(params.Task.ID) == "" {
		return fmt.Errorf("task id is required")
	}
	if strings.TrimSpace(params.Task.Title) == "" {
		return fmt.Errorf("task title is required")
	}

	dir := s.taskDir(params)
	if err := ensureDir(dir); err != nil {
		return fmt.Errorf("create artifact dir %q: %w", dir, err)
	}

	files := map[string]string{
		"task.md":  s.renderTaskMarkdown(params),
		"state.md": s.renderStateMarkdown(params),
		"index.md": s.renderIndexMarkdown(params),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	if err := s.ensureLogFile(params); err != nil {
		return err
	}

	modeArtifacts := s.modeArtifacts(params)
	if err := s.removeUnusedModeArtifacts(dir, modeArtifacts); err != nil {
		return err
	}
	for name, content := range modeArtifacts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	return nil
}

func (s *Service) renderTaskMarkdown(params SyncParams) string {
	artifactRoot := fmt.Sprintf(".aom/%s/%s", s.stateDir, params.Task.ID)
	worktreePath := "not provisioned yet"
	worktreeBranch := "-"
	if params.Worktree != nil {
		worktreePath = params.Worktree.WorktreePath
		worktreeBranch = params.Worktree.BranchName
		if usesWorktreeArtifactRoot(params.Worktree) {
			artifactRoot = filepath.Join(params.Worktree.WorktreePath, ".agent")
		}
	}

	// Build Pipeline Position section.
	var pipelineSection strings.Builder
	pipelineSection.WriteString("## Pipeline Position\n\n")
	if len(params.BlockedBy) == 0 {
		pipelineSection.WriteString("Blocked by: (none — this task can start now)\n")
	} else {
		pipelineSection.WriteString("Blocked by (must finish first):\n")
		for _, b := range params.BlockedBy {
			agent := b.PreferredAgent
			if agent == "" {
				agent = b.PreferredRole
			}
			if agent == "" {
				agent = "unassigned"
			}
			fmt.Fprintf(&pipelineSection, "  - %s — %s (assigned to: %s, status: %s)\n", b.ID, b.Title, agent, b.Status)
		}
	}
	if len(params.Unblocks) == 0 {
		pipelineSection.WriteString("Unblocks when done: (none)\n")
	} else {
		pipelineSection.WriteString("Unblocks when done:\n")
		for _, u := range params.Unblocks {
			agent := u.PreferredAgent
			if agent == "" {
				agent = u.PreferredRole
			}
			if agent == "" {
				agent = "unassigned"
			}
			fmt.Fprintf(&pipelineSection, "  - %s — %s (assigned to: %s)\n", u.ID, u.Title, agent)
		}
	}

	return fmt.Sprintf(`# Task

## Identity
- Task ID: %s
- Title: %s
- Task Mode: %s
- Status: %s
- Created By: %s
- Assigned Role: %s
- Assigned Agent: %s
- Worktree: %s (informational — already your CWD; create files using relative paths only)
- Worktree Branch: %s
- Artifact Root: %s

## Goal
%s

## Scope
- Complete the steps tracked for this task

## Out of Scope
- Worktree isolation and provider-native runtime integration remain outside this slice

%s
## Constraints
- Keep continuity state in AOM artifacts
- Follow the current task and step workflow state machine

## Team Communication & AOM Reference

### Communicate with teammates
- Broadcast to team:              aom channel append "your message"
- Direct message a teammate:      aom message send <agent-name> "your message"
- Check your inbox:               aom message read <your-agent-name>
- Read a file from another worktree: aom worktree read-file <task-id> <relative-path>

NOTE: Sandboxed runtimes (codex) stage messages to .agent/outbox.md — the operator
runs "aom outbox flush" to publish. Seeing "Message staged to outbox" is expected.

### Wait for another session to finish
  aom session wait <session-id> --event task.completed
  aom session wait <session-id> --event handoff.prepared
  aom session wait <session-id> --event checkpoint.created
  (default timeout: 30m — override with --timeout 2h)

### See what tasks are unblocked and ready
  aom next                   # list unblocked tasks by priority
  aom next --format json     # machine-readable, useful for scripting
  aom task list              # all tasks with status and assignments
  aom task list --status ready --format json

### Read shared team state
  cat .aom/team-brief.md     # team snapshot (operator runs: aom team brief)
  cat .aom/project-board.md  # full task board

## Success Criteria
- Planned steps are completed or explicitly resolved
- Task status reflects the final operator decision
- Relevant verification is captured before closure
- All modified files are committed synchronously before signaling completion:
  run git add -A && git commit in the foreground; if it fails for any reason, use aom worktree commit <task-id>
`,
		params.Task.ID,
		params.Task.Title,
		params.Task.Mode,
		params.Task.Status,
		defaultActor(params.CreatedBy),
		emptyFallback(params.Task.PreferredRole),
		emptyFallback(params.Task.PreferredAgent),
		worktreePath,
		worktreeBranch,
		artifactRoot,
		params.Task.Title,
		pipelineSection.String(),
	)
}

func (s *Service) renderStateMarkdown(params SyncParams) string {
	activeStep := selectActiveStep(params.Steps)
	currentSessionID := "-"
	currentRuntime := "-"
	if params.ActiveSession != nil {
		currentSessionID = params.ActiveSession.ID
		currentRuntime = params.ActiveSession.Runtime
	}
	currentStep := "-"
	stepStatus := "-"
	if activeStep != nil {
		currentStep = fmt.Sprintf("%s %s", activeStep.ID, activeStep.Title)
		stepStatus = activeStep.Status
	}

	return fmt.Sprintf(`# Current State

## Status
- Task Status: %s
- Step Status: %s

## Ownership
- Current Owner: %s
- Current Runtime: %s
- Current Session: %s
- Current Step: %s

## Goal
%s

## Completed Work
%s

## Remaining Work
%s

## Touched Files
- None recorded yet

## Constraints
- Stay within the current task scope
- Preserve continuity through markdown artifacts

## Open Questions
- None recorded yet

## Next Action
%s

## Last Updated By
- Actor: %s
- Session: %s
`,
		params.Task.Status,
		stepStatus,
		currentOwner(params.Task, activeStep),
		currentRuntime,
		currentSessionID,
		currentStep,
		params.Task.Title,
		renderCompletedWork(params.Steps),
		renderRemainingWork(params.Steps),
		emptyFallback(params.RecommendedNextAction),
		defaultActor(params.UpdatedBy),
		currentSessionID,
	)
}

func (s *Service) renderIndexMarkdown(params SyncParams) string {
	activeStep := selectActiveStep(params.Steps)
	activeStepLine := "-"
	if activeStep != nil {
		activeStepLine = fmt.Sprintf("%s %s", activeStep.ID, activeStep.Title)
	}

	activeSession := "-"
	runtime := "-"
	recoveryStatus := "No active session"
	worktreeStatus := "NotProvisioned"
	if params.ActiveSession != nil {
		activeSession = params.ActiveSession.ID
		runtime = params.ActiveSession.Runtime
		recoveryStatus = "Live"
	}
	if params.Worktree != nil {
		worktreeStatus = params.Worktree.Status
	}
	checkpointID, checkpointAt := s.latestCheckpointInfo(params)
	reviewCount := CountUnresolvedReviewItems(filepath.Join(s.taskDir(params), "review-notes.md"))
	reviewOwnerHint := s.reviewOwnerHintLine(params)

	return fmt.Sprintf(`# Task Index

## Identity
- Task ID: %s
- Title: %s
- Mode: %s
- Status: %s
- Priority: %s
- Blocked by: %s

## Active Control
- Active Step: %s
- Assigned Role: %s
- Assigned Agent: %s
- Active Session: %s
- Runtime: %s
- Worktree Status: %s
- Continuity Readiness: %s

## Artifacts
%s

## Checkpoint
- Latest Checkpoint: %s
- Last Checkpoint At: %s

## Attention
- Unresolved Review Items: %d
- Review Owner Hint: %s
- Pending Approvals: 0
- Session Recovery Status: %s

## Recommended Next Action
%s
`,
		params.Task.ID,
		params.Task.Title,
		params.Task.Mode,
		params.Task.Status,
		renderPriorityLabel(params.Task.Priority),
		renderBlockedByLine(params.BlockedBy),
		activeStepLine,
		emptyFallback(params.Task.PreferredRole),
		emptyFallback(params.Task.PreferredAgent),
		activeSession,
		runtime,
		worktreeStatus,
		computeContinuityReadiness(params, reviewCount),
		s.renderArtifactInventory(params),
		checkpointID,
		checkpointAt,
		reviewCount,
		reviewOwnerHint,
		recoveryStatus,
		emptyFallback(params.RecommendedNextAction),
	)
}

// computeContinuityReadiness returns "High", "Medium", or "Low" based on how
// ready the task state is for an agent to resume work without operator help.
func computeContinuityReadiness(params SyncParams, reviewCount int) string {
	taskStatus := params.Task.Status
	hasActiveSession := params.ActiveSession != nil
	worktreeOK := params.Worktree == nil || params.Worktree.Status == worktree.StatusReady || params.Worktree.Status == worktree.StatusActive

	// Definitive blockers → Low.
	switch taskStatus {
	case "Blocked", "NeedsAttention", "Failed":
		return "Low"
	}
	if params.Worktree != nil && params.Worktree.Status == worktree.StatusNeedsRepair {
		return "Low"
	}
	if reviewCount > 0 {
		return "Low"
	}

	// Full green-path → High.
	if (taskStatus == "InProgress" || taskStatus == "Ready") && hasActiveSession && worktreeOK {
		return "High"
	}

	return "Medium"
}

func (s *Service) reviewOwnerHintLine(params SyncParams) string {
	if params.ReviewOwnerAmbiguous {
		return "mixed owners - operator must choose"
	}
	if strings.TrimSpace(params.ReviewOwnerHint) != "" {
		return params.ReviewOwnerHint
	}
	return "-"
}

func (s *Service) renderReviewNotesMarkdown(params SyncParams, reviewer, sessionID string) string {
	activeStep := selectActiveStep(params.Steps)
	reviewStepID := "-"
	if activeStep != nil {
		reviewStepID = activeStep.ID
	}

	return fmt.Sprintf(`# Review Notes

## Summary
- Task: %s
- Review Step: %s
- Reviewer: %s
- Session: %s
- Status: Pending review

## Items
- No findings recorded yet
`,
		params.Task.ID,
		reviewStepID,
		emptyFallback(reviewer),
		emptyFallback(sessionID),
	)
}

func (s *Service) renderHandoffTemplateMarkdown(params SyncParams, activeSession session.Record) string {
	activeStep := selectActiveStep(params.Steps)
	stepLine := "-"
	toRole := params.Task.PreferredRole
	if activeStep != nil {
		stepLine = fmt.Sprintf("%s %s", activeStep.ID, activeStep.Title)
		if strings.TrimSpace(activeStep.RoleName) != "" {
			toRole = activeStep.RoleName
		}
	}

	suggestedRuntime := "-"
	if strings.TrimSpace(activeSession.Runtime) != "" {
		suggestedRuntime = activeSession.Runtime
	}

	return fmt.Sprintf(`# Handoff

## Transfer
- From Role: %s
- From Agent: %s
- From Session: %s
- From Runtime: %s
- To Role: %s
- Suggested Runtime: %s
- Task: %s
- Step: %s
- Reason: Fill this in when the work is ready for transfer

## Completed
- Fill in what was completed in this session

## Remaining
- Fill in what still needs to happen next

## Touched Files
- Record touched files before signaling handoff.prepared

## Constraints
- Stay within the current task scope
- Preserve continuity through markdown artifacts

## Warnings
- Record any risks, caveats, or unresolved questions

## Exact Next Action
Read .agent/task.md, .agent/state.md, and .agent/log.md before continuing.

## Do Not Redo
- Record what the next owner should not repeat
`,
		emptyFallback(activeSession.RoleName),
		emptyFallback(activeSession.AgentName),
		emptyFallback(activeSession.ID),
		emptyFallback(activeSession.Runtime),
		emptyFallback(toRole),
		suggestedRuntime,
		params.Task.ID,
		stepLine,
	)
}

func (s *Service) renderArtifactInventory(params SyncParams) string {
	dir := s.taskDir(params)
	lines := []string{
		"- task.md: present",
		"- state.md: present",
		"- log.md: present",
		fmt.Sprintf("- handoff.md: %s", s.artifactPresence(dir, "handoff.md")),
		fmt.Sprintf("- review-notes.md: %s", s.artifactPresence(dir, "review-notes.md")),
	}

	switch params.Task.Mode {
	case "Requirements-first":
		lines = append(lines, "- requirements.md: present", "- design.md: n/a", "- tasks.md: present")
	case "Design-first":
		lines = append(lines, "- requirements.md: n/a", "- design.md: present", "- tasks.md: present")
	default:
		lines = append(lines, "- requirements.md: n/a", "- design.md: n/a", "- tasks.md: n/a")
	}

	return strings.Join(lines, "\n")
}

func (s *Service) artifactPresence(dir, name string) string {
	if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
		return "present"
	}
	return "absent"
}

func renderPriorityLabel(p int) string {
	switch {
	case p >= 10:
		return "high"
	case p <= -10:
		return "low"
	default:
		return "normal"
	}
}

func renderBlockedByLine(blockers []task.Record) string {
	if len(blockers) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(blockers))
	for _, b := range blockers {
		parts = append(parts, b.ID)
	}
	return strings.Join(parts, ", ")
}

func (s *Service) latestCheckpointInfo(params SyncParams) (string, string) {
	data, err := os.ReadFile(filepath.Join(s.taskDir(params), "log.md"))
	if err != nil {
		return "-", "-"
	}

	lines := strings.Split(string(data), "\n")
	latestID := "-"
	latestAt := "-"
	currentTimestamp := ""
	inCheckpointEvent := false
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "### "):
			inCheckpointEvent = strings.Contains(line, "| checkpoint.created")
			if !inCheckpointEvent {
				continue
			}
			parts := strings.SplitN(strings.TrimPrefix(line, "### "), " | ", 2)
			if len(parts) > 0 {
				currentTimestamp = strings.TrimSpace(parts[0])
			}
		case inCheckpointEvent && strings.Contains(line, "Checkpoint CHK-"):
			if idx := strings.Index(line, "CHK-"); idx >= 0 {
				checkpointID := strings.TrimSpace(line[idx:])
				if space := strings.IndexAny(checkpointID, " )"); space >= 0 {
					checkpointID = checkpointID[:space]
				}
				latestID = checkpointID
				if currentTimestamp != "" {
					latestAt = currentTimestamp
				}
			}
			inCheckpointEvent = false
		}
	}

	return latestID, latestAt
}

func (s *Service) modeArtifacts(params SyncParams) map[string]string {
	switch params.Task.Mode {
	case "Requirements-first":
		return map[string]string{
			"requirements.md": "# Requirements\n\n## Summary\n- Capture the accepted requirements for this task.\n",
			"tasks.md":        s.renderTasksMarkdown(params.Steps),
		}
	case "Design-first":
		return map[string]string{
			"design.md": "# Design\n\n## Summary\n- Capture the accepted design constraints for this task.\n",
			"tasks.md":  s.renderTasksMarkdown(params.Steps),
		}
	default:
		return nil
	}
}

func (s *Service) renderTasksMarkdown(steps []step.Record) string {
	var builder strings.Builder
	builder.WriteString("# Planned Tasks\n\n## Steps\n")
	if len(steps) == 0 {
		builder.WriteString("- No planned steps yet\n")
		return builder.String()
	}

	for _, item := range steps {
		builder.WriteString(fmt.Sprintf("- %s | %s | %s\n", item.ID, item.StepType, item.Title))
	}
	return builder.String()
}

func (s *Service) removeUnusedModeArtifacts(dir string, keep map[string]string) error {
	candidates := []string{"requirements.md", "design.md", "tasks.md"}
	for _, name := range candidates {
		if _, ok := keep[name]; ok {
			continue
		}
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	return nil
}

func (s *Service) ensureLogFile(params SyncParams) error {
	dir := s.taskDir(params)
	path := filepath.Join(dir, "log.md")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat log.md: %w", err)
	}

	if err := ensureDir(dir); err != nil {
		return fmt.Errorf("create artifact dir %q: %w", dir, err)
	}
	if err := os.WriteFile(path, []byte("# Task Log\n\n## Events\n"), 0o644); err != nil {
		return fmt.Errorf("write log.md: %w", err)
	}
	return nil
}

func (s *Service) appendEvent(params SyncParams, event Event) error {
	if err := s.ensureLogFile(params); err != nil {
		return err
	}

	taskID := params.Task.ID
	timestamp := s.now().Format(time.RFC3339)
	entry := fmt.Sprintf(
		"\n### %s | %s | %s\n- Actor: %s\n- Task: %s\n%s%s- Summary: %s\n- State Effect: %s\n",
		timestamp,
		s.eventIDGen(),
		event.Type,
		defaultActor(event.Actor),
		taskID,
		optionalLine("Step", event.StepID),
		optionalLine("Session", event.SessionID),
		event.Summary,
		event.StateEffect,
	)

	path := filepath.Join(s.taskDir(params), "log.md")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log.md for append: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(entry); err != nil {
		return fmt.Errorf("append log.md: %w", err)
	}

	return nil
}

func (s *Service) taskDir(params SyncParams) string {
	if usesWorktreeArtifactRoot(params.Worktree) {
		return filepath.Join(params.Worktree.WorktreePath, ".agent")
	}
	return filepath.Join(s.repoPath, ".aom", s.stateDir, params.Task.ID)
}

// TaskLogPath returns the absolute path of log.md for a task.
func (s *Service) TaskLogPath(params SyncParams) string {
	return filepath.Join(s.taskDir(params), "log.md")
}

func usesWorktreeArtifactRoot(mapping *worktree.Record) bool {
	if mapping == nil || strings.TrimSpace(mapping.WorktreePath) == "" {
		return false
	}

	return mapping.Status == worktree.StatusReady || mapping.Status == worktree.StatusActive
}

func renderCompletedWork(steps []step.Record) string {
	completed := make([]string, 0)
	for _, item := range steps {
		if item.Status == "Completed" || item.Status == "Skipped" || item.Status == "Canceled" {
			completed = append(completed, fmt.Sprintf("- %s | %s", item.ID, item.Title))
		}
	}
	if len(completed) == 0 {
		return "- None recorded yet"
	}
	return strings.Join(completed, "\n")
}

func renderRemainingWork(steps []step.Record) string {
	remaining := make([]string, 0)
	for _, item := range steps {
		if item.Status == "Completed" || item.Status == "Skipped" || item.Status == "Canceled" {
			continue
		}
		remaining = append(remaining, fmt.Sprintf("- %s | %s | status=%s", item.ID, item.Title, item.Status))
	}
	if len(remaining) == 0 {
		return "- No remaining planned work"
	}
	return strings.Join(remaining, "\n")
}

func selectActiveStep(steps []step.Record) *step.Record {
	for _, status := range []string{"InProgress", "Ready", "Confirmed", "Proposed", "Blocked", "NeedsAttention"} {
		for _, item := range steps {
			if item.Status == status {
				stepCopy := item
				return &stepCopy
			}
		}
	}
	return nil
}

func currentOwner(taskRecord task.Record, activeStep *step.Record) string {
	if activeStep != nil {
		if strings.TrimSpace(activeStep.AgentName) != "" {
			return activeStep.AgentName
		}
		if strings.TrimSpace(activeStep.RoleName) != "" {
			return activeStep.RoleName
		}
	}
	if strings.TrimSpace(taskRecord.PreferredAgent) != "" {
		return taskRecord.PreferredAgent
	}
	if strings.TrimSpace(taskRecord.PreferredRole) != "" {
		return taskRecord.PreferredRole
	}
	return "-"
}

func optionalLine(label, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return fmt.Sprintf("- %s: %s\n", label, value)
}

func emptyFallback(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func defaultActor(value string) string {
	if strings.TrimSpace(value) == "" {
		return "aom"
	}
	return strings.TrimSpace(value)
}

// CountUnresolvedReviewItems returns the number of review note items whose status is not resolved.
func CountUnresolvedReviewItems(path string) int {
	items := parseReviewItems(path)
	count := 0
	for _, item := range items {
		if !isResolvedReviewStatus(item.Status) {
			count++
		}
	}
	return count
}

// SuggestedReviewOwner returns the shared unresolved owner when all open review items point to one owner.
func SuggestedReviewOwner(path string) string {
	owner, ambiguous := ReviewOwnerHint(path)
	if ambiguous {
		return ""
	}
	return owner
}

// ReviewOwnerHint returns the shared unresolved owner and whether the unresolved owner signals are ambiguous.
func ReviewOwnerHint(path string) (string, bool) {
	items := parseReviewItems(path)
	owner := ""
	for _, item := range items {
		if isResolvedReviewStatus(item.Status) {
			continue
		}
		currentOwner := strings.TrimSpace(item.Owner)
		if currentOwner == "" {
			return "", true
		}
		if owner == "" {
			owner = currentOwner
			continue
		}
		if !strings.EqualFold(owner, currentOwner) {
			return "", true
		}
	}
	return owner, false
}

type reviewItem struct {
	Status string
	Owner  string
}

func parseReviewItems(path string) []reviewItem {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	items := make([]reviewItem, 0)
	inItem := false
	current := reviewItem{}
	flush := func() {
		if !inItem {
			return
		}
		current.Status = strings.ToLower(strings.TrimSpace(current.Status))
		current.Owner = strings.TrimSpace(current.Owner)
		items = append(items, current)
		inItem = false
		current = reviewItem{}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			flush()
			inItem = true
			continue
		}
		if !inItem {
			continue
		}
		if strings.HasPrefix(trimmed, "- Status:") {
			current.Status = strings.TrimSpace(strings.TrimPrefix(trimmed, "- Status:"))
		}
		if strings.HasPrefix(trimmed, "- Owner:") {
			current.Owner = strings.TrimSpace(strings.TrimPrefix(trimmed, "- Owner:"))
		}
	}
	flush()

	return items
}

func isResolvedReviewStatus(status string) bool {
	switch status {
	case "resolved", "closed", "fixed", "done", "accepted":
		return true
	default:
		return false
	}
}

func defaultEventIDGenerator() string {
	return "EVT-" + strconvFormatInt(time.Now().UnixNano())
}

func strconvFormatInt(value int64) string {
	return fmt.Sprintf("%d", value)
}

// MergePlanParams carries the data needed to write a merge-plan.md artifact.
type MergePlanParams struct {
	TaskID        string
	TargetBranch  string
	ConflictScore string // "Green" | "Yellow" | "Red"
	Overlaps      []MergePlanOverlap
}

// MergePlanOverlap describes one overlapping file.
type MergePlanOverlap struct {
	Path        string
	OtherBranch string
}

// WriteMergePlan writes merge-plan.md into the task's canonical artifact root.
func (s *Service) WriteMergePlan(params SyncParams, plan MergePlanParams) error {
	dir := s.taskDir(params)
	if err := ensureDir(dir); err != nil {
		return fmt.Errorf("create artifact dir: %w", err)
	}

	content := s.renderMergePlanMarkdown(plan)
	return os.WriteFile(filepath.Join(dir, "merge-plan.md"), []byte(content), 0o644)
}

func (s *Service) renderMergePlanMarkdown(plan MergePlanParams) string {
	var overlapLines strings.Builder
	if len(plan.Overlaps) == 0 {
		overlapLines.WriteString("No overlapping files detected.\n")
	} else {
		for _, o := range plan.Overlaps {
			fmt.Fprintf(&overlapLines, "- `%s` — also modified in branch `%s`\n", o.Path, o.OtherBranch)
		}
	}

	var actions strings.Builder
	if len(plan.Overlaps) > 0 {
		actions.WriteString("- [ ] Review overlapping files with the other task owner\n")
	}
	actions.WriteString("- [ ] Run tests after merge\n")

	return fmt.Sprintf(`# Merge Plan
- Task: %s
- Target branch: %s
- Prepared at: %s
- Conflict score: %s

## File Overlaps
%s
## Recommended actions
%s`,
		plan.TaskID,
		plan.TargetBranch,
		s.now().Format(time.RFC3339),
		plan.ConflictScore,
		overlapLines.String(),
		actions.String(),
	)
}

// TeamBriefAgent is a summary of one agent for the team brief.
type TeamBriefAgent struct {
	Name          string
	Role          string
	Runtime       string
	SessionStatus string
}

// TeamBriefTask is a summary of one task for the team brief.
type TeamBriefTask struct {
	ID         string
	Title      string
	Status     string
	Priority   string
	Agent      string
	BlockedBy  []string
}

// TeamBriefParams carries all data needed to generate the team brief.
type TeamBriefParams struct {
	ProjectName     string
	Tasks           []TeamBriefTask
	PendingRequests []string // formatted lines
	ChannelTail     []string // last N channel messages
	Agents          []TeamBriefAgent
}

// GenerateTeamBrief writes .aom/team-brief.md and returns its path.
func (s *Service) GenerateTeamBrief(params TeamBriefParams) (string, error) {
	path := filepath.Join(s.repoPath, ".aom", "team-brief.md")
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return "", fmt.Errorf("create team brief dir: %w", err)
	}

	content := s.renderTeamBriefMarkdown(params)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write team brief: %w", err)
	}

	return path, nil
}

func (s *Service) renderTeamBriefMarkdown(params TeamBriefParams) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# AOM Team Brief\n")
	fmt.Fprintf(&b, "- Generated: %s\n", s.now().Format(time.RFC3339))
	fmt.Fprintf(&b, "- Project: %s\n\n", params.ProjectName)

	fmt.Fprintf(&b, "## Active Tasks\n\n")
	if len(params.Tasks) == 0 {
		fmt.Fprintf(&b, "No active tasks.\n\n")
	} else {
		fmt.Fprintf(&b, "| Task | Title | Status | Priority | Agent | Blocked by |\n")
		fmt.Fprintf(&b, "|------|-------|--------|----------|-------|------------|\n")
		for _, t := range params.Tasks {
			blockedBy := "-"
			if len(t.BlockedBy) > 0 {
				blockedBy = strings.Join(t.BlockedBy, ", ")
			}
			agent := t.Agent
			if agent == "" {
				agent = "-"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
				t.ID, t.Title, t.Status, t.Priority, agent, blockedBy)
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "## Pending Requests\n\n")
	if len(params.PendingRequests) == 0 {
		fmt.Fprintf(&b, "No pending requests.\n\n")
	} else {
		for _, line := range params.PendingRequests {
			fmt.Fprintf(&b, "- %s\n", line)
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "## Team Channel (last messages)\n\n")
	if len(params.ChannelTail) == 0 {
		fmt.Fprintf(&b, "No messages.\n\n")
	} else {
		for _, msg := range params.ChannelTail {
			fmt.Fprintf(&b, "%s\n", msg)
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "## Agents\n\n")
	if len(params.Agents) == 0 {
		fmt.Fprintf(&b, "No agents configured.\n\n")
	} else {
		fmt.Fprintf(&b, "| Name | Role | Runtime | Session Status |\n")
		fmt.Fprintf(&b, "|------|------|---------|----------------|\n")
		for _, a := range params.Agents {
			status := a.SessionStatus
			if status == "" {
				status = "no session"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", a.Name, a.Role, a.Runtime, status)
		}
	}

	return b.String()
}

// ensureDir creates the directory and forces 0755 permissions regardless of umask.
// Directories in worktrees need execute bits so agents can cd into them and write files.
func ensureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	return os.Chmod(path, 0o755)
}

// WriteProjectBoard writes .aom/project-board.md with a snapshot of all project tasks.
// Agents can read this file to understand team-wide task state.
func (s *Service) WriteProjectBoard(projectName string, tasks []TeamBriefTask) (string, error) {
	path := filepath.Join(s.repoPath, ".aom", "project-board.md")
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return "", fmt.Errorf("create project board dir: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Project Board\n")
	fmt.Fprintf(&b, "- Project: %s\n", projectName)
	fmt.Fprintf(&b, "- Updated: %s\n\n", s.now().Format(time.RFC3339))

	fmt.Fprintf(&b, "## Tasks\n\n")
	if len(tasks) == 0 {
		fmt.Fprintf(&b, "No tasks yet.\n\n")
	} else {
		fmt.Fprintf(&b, "| Task | Title | Status | Priority | Agent | Blocked By |\n")
		fmt.Fprintf(&b, "|------|-------|--------|----------|-------|------------|\n")
		for _, t := range tasks {
			agent := t.Agent
			if agent == "" {
				agent = "unassigned"
			}
			blockedBy := "-"
			if len(t.BlockedBy) > 0 {
				blockedBy = strings.Join(t.BlockedBy, ", ")
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
				t.ID, t.Title, t.Status, t.Priority, agent, blockedBy)
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "---\n")
	fmt.Fprintf(&b, "_Auto-generated by aom. Read-only — do not edit manually._\n")

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("write project board: %w", err)
	}

	return path, nil
}
