package cli

import "github.com/lattapon-aek/agent-orchestrator-management/internal/step"

func autoSkipPlaceholderIntegrationSteps(stepService *step.Service, steps []step.Record) ([]step.Record, error) {
	eligible := map[string]bool{
		"Proposed": true,
		"Ready":    true,
	}

	updated := make([]step.Record, 0)
	for _, item := range steps {
		if item.StepType != "integration" || !eligible[item.Status] {
			continue
		}

		record, err := stepService.Update(item.ID, step.UpdateParams{Status: "Skipped"})
		if err != nil {
			return nil, err
		}
		updated = append(updated, *record)
	}

	return updated, nil
}

func autoCompleteIntegrationSteps(stepService *step.Service, steps []step.Record) ([]step.Record, error) {
	eligible := map[string]bool{
		"Proposed":   true,
		"Confirmed":  true,
		"Ready":      true,
		"InProgress": true,
	}

	updated := make([]step.Record, 0)
	for _, item := range steps {
		if item.StepType != "integration" || !eligible[item.Status] {
			continue
		}

		current := item
		for _, nextStatus := range stepWalkPath(item.Status, "Completed") {
			record, err := stepService.Update(current.ID, step.UpdateParams{Status: nextStatus})
			if err != nil {
				return nil, err
			}
			current = *record
		}
		updated = append(updated, current)
	}

	return updated, nil
}
