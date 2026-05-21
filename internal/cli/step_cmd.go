package cli

import (
	"fmt"
	"strings"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/artifact"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/step"
)

func (r Runner) executeStepList(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}

	taskID := strings.TrimSpace(args[0])
	idsOnly := false
	for _, a := range args[1:] {
		if a == "--ids-only" {
			idsOnly = true
		} else {
			return fmt.Errorf("unknown flag %q", a)
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	taskRecord, err := taskService.Get(taskID)
	if err != nil {
		return err
	}
	if taskRecord == nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	stepService, stepDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer stepDB.Close()

	steps, err := stepService.ListByTask(taskRecord.ID)
	if err != nil {
		return err
	}

	if idsOnly {
		for _, item := range steps {
			fmt.Fprintln(r.stdout, item.ID)
		}
		return nil
	}

	fmt.Fprintln(r.stdout, "Steps")
	fmt.Fprintln(r.stdout, "")
	if len(steps) == 0 {
		fmt.Fprintf(r.stdout, "No steps for %s\n", taskRecord.ID)
		return nil
	}

	for _, item := range steps {
		fmt.Fprintf(
			r.stdout,
			"%s | type=%s | title=%s | role=%s | agent=%s | status=%s | dependencies=%s\n",
			item.ID,
			item.StepType,
			item.Title,
			emptyFallback(item.RoleName),
			emptyFallback(item.AgentName),
			item.Status,
			formatDependencies(item.Dependencies),
		)
	}

	return nil
}

func (r Runner) executeStepUpdate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("step identifier is required")
	}

	params := stepUpdateParams{id: strings.TrimSpace(args[0])}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--status":
			i++
			if i >= len(args) {
				return fmt.Errorf("--status requires a value")
			}
			params.status = args[i]
		case "--role":
			i++
			if i >= len(args) {
				return fmt.Errorf("--role requires a value")
			}
			params.role = args[i]
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			params.agent = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	stepService, sqlDB, err := r.app.OpenStepService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	// Normalise the target status so we can build an intermediate path.
	targetStatus, normErr := step.NormaliseStatus(params.status)
	if normErr != nil {
		return normErr
	}

	// Determine the walk path from the current status to the target.
	// If the transition is not direct, auto-advance through intermediates so the
	// operator doesn't have to issue multiple commands (e.g. Confirmed → Completed).
	current, lookupErr := stepService.Get(params.id)
	if lookupErr != nil {
		return lookupErr
	}
	if current == nil {
		return fmt.Errorf("step %q not found", params.id)
	}
	intermediates := stepWalkPath(current.Status, targetStatus)

	var record *step.Record
	for _, s := range intermediates {
		record, err = stepService.Update(params.id, step.UpdateParams{
			Status:    s,
			RoleName:  params.role,
			AgentName: params.agent,
		})
		if err != nil {
			return err
		}
	}
	if record == nil {
		// Target == current; still do the update to apply role/agent changes.
		record, err = stepService.Update(params.id, step.UpdateParams{
			Status:    params.status,
			RoleName:  params.role,
			AgentName: params.agent,
		})
		if err != nil {
			return err
		}
	}

	// Collaboration gate: warn if step is being completed without any channel activity.
	if targetStatus == "Completed" && record != nil && record.AgentName != "" {
		channelData, _ := readChannelFile(result.Project.RepoPath)
		if !strings.Contains(channelData, "| "+record.AgentName+"\n") {
			fmt.Fprintf(r.stdout, "Warning: no channel activity from %q found — consider posting a summary via `aom channel append` before marking complete.\n\n", record.AgentName)
		}
	}

	if err := r.syncTaskArtifacts(result, record.TaskID, artifact.Event{
		Type:        mapStepEventType(record.Status),
		Actor:       "operator",
		StepID:      record.ID,
		Summary:     fmt.Sprintf("Step updated to %s", record.Status),
		StateEffect: fmt.Sprintf("Step %s", record.Status),
	}, false); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Step updated")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Step: %s\n", record.ID)
	fmt.Fprintf(r.stdout, "Status: %s\n", record.Status)
	fmt.Fprintf(r.stdout, "Role: %s\n", emptyFallback(record.RoleName))
	fmt.Fprintf(r.stdout, "Agent: %s\n", emptyFallback(record.AgentName))

	return nil
}

// stepWalkPath returns the ordered list of status values to pass through to get
// from current to target, inclusive of target. Returns nil if already at target
// or if the transition is a single direct hop (no walk needed).
func stepWalkPath(current, target string) []string {
	// Full linear path for the happy-path: Proposed → Confirmed → Ready → InProgress → Completed.
	linearPath := []string{"Proposed", "Confirmed", "Ready", "InProgress", "Completed"}

	ci, ti := -1, -1
	for i, s := range linearPath {
		if s == current {
			ci = i
		}
		if s == target {
			ti = i
		}
	}
	// Only walk forward along the linear path; backward or off-path transitions
	// are left to the service to validate directly.
	if ci < 0 || ti < 0 || ti <= ci {
		return []string{target}
	}
	return linearPath[ci+1 : ti+1]
}

type stepUpdateParams struct {
	id     string
	status string
	role   string
	agent  string
}

func mapStepEventType(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "confirmed":
		return "step.confirmed"
	case "completed":
		return "step.completed"
	default:
		return "step.updated"
	}
}
