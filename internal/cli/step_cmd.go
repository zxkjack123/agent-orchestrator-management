package cli

import (
	"fmt"
	"strings"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/artifact"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/step"
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

	record, err := stepService.Update(params.id, step.UpdateParams{
		Status:    params.status,
		RoleName:  params.role,
		AgentName: params.agent,
	})
	if err != nil {
		return err
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
