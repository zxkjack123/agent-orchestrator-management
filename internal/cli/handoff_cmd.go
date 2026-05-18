package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/artifact"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/project"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/session"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/step"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/worktree"
)

type checkpointParams struct {
	sessionID string
}

type handoffParams struct {
	sessionID string
	target    string
	reason    string
}

func (r Runner) executeCheckpoint(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("checkpoint does not accept extra positional arguments in the current milestone")
	}

	sessionRecord, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if strings.TrimSpace(sessionRecord.TaskID) == "" {
		return fmt.Errorf("session %q is not bound to a task", sessionRecord.ID)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, sessionRecord.TaskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", sessionRecord.TaskID)
	}

	checkpointID := newCheckpointID()
	summary := fmt.Sprintf("Checkpoint %s created for session %s", checkpointID, sessionRecord.ID)
	if err := r.syncTaskArtifactsWithSession(result, sessionRecord.TaskID, artifact.Event{
		Type:        "checkpoint.created",
		Actor:       "operator",
		StepID:      activeStepID(view.Steps),
		SessionID:   sessionRecord.ID,
		Summary:     summary,
		StateEffect: "Checkpoint Ready",
	}, false, sessionRecord); err != nil {
		return err
	}
	if err := r.refreshTaskArtifacts(result, sessionRecord.TaskID, sessionRecord); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Checkpoint created")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Checkpoint: %s\n", checkpointID)
	fmt.Fprintf(r.stdout, "Task: %s\n", sessionRecord.TaskID)
	fmt.Fprintf(r.stdout, "Step: %s\n", emptyFallback(activeStepID(view.Steps)))
	fmt.Fprintf(r.stdout, "Owner: %s\n", sessionRecord.AgentName)
	fmt.Fprintf(r.stdout, "Changed files: %s\n", changedFilesSummary(sessionRecord.WorktreePath, sessionRecord.RepoPath))
	fmt.Fprintf(r.stdout, "Next action: %s\n", recommendTaskAction(view.Task.Status, view.Steps, view.Worktree, view.WorktreeDrift, view.UnresolvedReviewItems, view.ReviewOwnerHint, view.ReviewOwnerAmbiguous))

	return nil
}

func (r Runner) executeHandoff(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	params := handoffParams{sessionID: strings.TrimSpace(args[0])}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--to":
			i++
			if i >= len(args) {
				return fmt.Errorf("--to requires a value")
			}
			params.target = strings.TrimSpace(args[i])
		case "--reason":
			i++
			if i >= len(args) {
				return fmt.Errorf("--reason requires a value")
			}
			params.reason = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if params.target == "" {
		return fmt.Errorf("--to is required")
	}

	sessionRecord, err := r.loadSessionByIdentifier(params.sessionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(sessionRecord.TaskID) == "" {
		return fmt.Errorf("session %q is not bound to a task", sessionRecord.ID)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	view, err := r.loadTaskView(result, sessionRecord.TaskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", sessionRecord.TaskID)
	}

	targetRole, targetAgent, suggestedRuntime, err := resolveHandoffTarget(result, params.target)
	if err != nil {
		return err
	}
	reason := params.reason
	if reason == "" {
		reason = "Operator requested ownership transfer"
	}

	if err := r.transferHandoffOwnership(result, view, targetRole, targetAgent); err != nil {
		return err
	}

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	if sessionRecord.Status != "WaitingHandoff" {
		sessionRecord.Status = "WaitingHandoff"
		updated, err := sessionService.Save(*sessionRecord)
		if err != nil {
			return err
		}
		sessionRecord = updated
	}

	if err := r.writeHandoffArtifact(result, view, *sessionRecord, targetRole, targetAgent, suggestedRuntime, reason); err != nil {
		return err
	}

	if err := r.syncTaskArtifactsWithSession(result, sessionRecord.TaskID, artifact.Event{
		Type:        "handoff.prepared",
		Actor:       "operator",
		StepID:      activeStepID(view.Steps),
		SessionID:   sessionRecord.ID,
		Summary:     fmt.Sprintf("Handoff prepared from %s to %s", sessionRecord.AgentName, ownershipSummary(targetRole, targetAgent)),
		StateEffect: "Handoff Ready and ownership transferred",
	}, false, sessionRecord); err != nil {
		return err
	}
	if err := r.refreshTaskArtifacts(result, sessionRecord.TaskID, sessionRecord); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Handoff prepared")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", sessionRecord.TaskID)
	fmt.Fprintf(r.stdout, "From session: %s\n", sessionRecord.ID)
	fmt.Fprintf(r.stdout, "To role: %s\n", targetRole)
	fmt.Fprintf(r.stdout, "To agent: %s\n", emptyFallback(targetAgent))
	fmt.Fprintf(r.stdout, "Suggested runtime: %s\n", emptyFallback(suggestedRuntime))
	fmt.Fprintf(r.stdout, "Readiness: ready\n")
	fmt.Fprintf(r.stdout, "Ownership transferred: %s\n", ownershipSummary(targetRole, targetAgent))
	fmt.Fprintf(r.stdout, "Next action: %s\n", handoffNextAction(sessionRecord.TaskID, targetRole, targetAgent))

	return nil
}


func (r Runner) transferHandoffOwnership(result *project.OpenResult, view *taskView, roleName, agentName string) error {
	if result == nil || view == nil {
		return fmt.Errorf("handoff ownership context is required")
	}

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	if _, err := taskService.AssignOwner(view.Task.ID, roleName, agentName); err != nil {
		return err
	}

	activeStep := selectHandoffStep(view.Steps)
	if activeStep == nil {
		return nil
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	if _, err := stepService.AssignOwner(activeStep.ID, roleName, agentName); err != nil {
		return err
	}
	return nil
}

func selectHandoffStep(steps []step.Record) *step.Record {
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

func ownershipSummary(roleName, agentName string) string {
	if strings.TrimSpace(agentName) != "" {
		return fmt.Sprintf("%s (%s)", agentName, roleName)
	}
	return roleName
}


func taskHandoffPath(repoPath, stateDir, taskID string, mapping *worktree.Record) string {
	return filepath.Join(taskArtifactRoot(repoPath, stateDir, taskID, mapping), "handoff.md")
}

func newCheckpointID() string {
	return "CHK-" + fmt.Sprintf("%d", time.Now().UnixNano())
}

func resolveHandoffTarget(result *project.OpenResult, target string) (role string, agentName string, suggestedRuntime string, err error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", "", "", fmt.Errorf("handoff target is required")
	}

	for _, agentRecord := range result.Agents {
		if agentRecord.Name == target {
			return agentRecord.Role, agentRecord.Name, agentRecord.Runtime, nil
		}
	}
	if _, ok := result.RoleConfigs[target]; ok {
		return target, "", "", nil
	}

	var validAgents, validRoles []string
	for _, a := range result.Agents {
		validAgents = append(validAgents, a.Name)
	}
	for role := range result.RoleConfigs {
		validRoles = append(validRoles, role)
	}
	return "", "", "", fmt.Errorf("handoff target %q not found\n  valid agents: %s\n  valid roles:  %s",
		target, strings.Join(validAgents, ", "), strings.Join(validRoles, ", "))
}

func handoffNextAction(taskID, role, agentName string) string {
	if strings.TrimSpace(agentName) != "" {
		return fmt.Sprintf("run \"aom session spawn %s --task %s\" to continue from the prepared handoff", agentName, taskID)
	}
	return fmt.Sprintf("choose an agent for role %s and spawn it against task %s", role, taskID)
}

func (r Runner) writeHandoffArtifact(result *project.OpenResult, view *taskView, sessionRecord session.Record, targetRole, targetAgent, suggestedRuntime, reason string) error {
	if result == nil || view == nil {
		return fmt.Errorf("handoff artifact context is required")
	}

	path := taskHandoffPath(result.Project.RepoPath, result.StateDir, view.Task.ID, view.Worktree)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create handoff dir: %w", err)
	}

	content := fmt.Sprintf(`# Handoff

## Transfer
- From Role: %s
- From Agent: %s
- From Session: %s
- From Runtime: %s
- To Role: %s
- Suggested Runtime: %s
- Task: %s
- Step: %s
- Reason: %s

## Completed
- Review current state in state.md before continuing
- Continue from the current worktree and latest task log

## Remaining
- Continue the assigned step from the latest task context
- Validate the current diff and task state before making new changes

## Touched Files
- %s

## Constraints
- Preserve the current task scope and worktree continuity
- Start from AOM artifacts rather than transcript memory alone

## Warnings
- Review unresolved notes in log.md and state.md before continuing

## Exact Next Action
%s

## Do Not Redo
- Do not restart the task from scratch
- Do not ignore the existing worktree and artifact context
`,
		sessionRecord.RoleName,
		sessionRecord.AgentName,
		sessionRecord.ID,
		sessionRecord.Runtime,
		targetRole,
		emptyFallback(suggestedRuntime),
		view.Task.ID,
		emptyFallback(activeStepID(view.Steps)),
		reason,
		changedFilesSummary(sessionRecord.WorktreePath, sessionRecord.RepoPath),
		handoffNextAction(view.Task.ID, targetRole, targetAgent),
	)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write handoff.md: %w", err)
	}

	return nil
}

