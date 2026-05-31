package cli

import (
	"fmt"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/artifact"
)

func (r Runner) executeGoal(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("goal subcommand required: set | show | complete")
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Fprintln(r.stdout, "aom goal set \"<text>\"  — set the project goal for the orchestrator agent")
			fmt.Fprintln(r.stdout, "aom goal show          — print current goal and status")
			fmt.Fprintln(r.stdout, "aom goal complete      — mark the current goal as complete")
			return nil
		}
	}
	switch args[0] {
	case "set":
		return r.executeGoalSet(args[1:])
	case "show":
		return r.executeGoalShow(args[1:])
	case "complete":
		return r.executeGoalComplete(args[1:])
	default:
		return fmt.Errorf("unknown goal command %q", args[0])
	}
}

func (r Runner) executeGoalSet(args []string) error {
	if len(args) == 0 || strings.TrimSpace(strings.Join(args, " ")) == "" {
		return fmt.Errorf("goal text is required: aom goal set \"<text>\"")
	}
	text := strings.Join(args, " ")

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return wrapProjectNotFound(err)
	}

	path, err := artifact.WriteGoalFile(result.Project.RepoPath, text)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "Goal set: %s\n", path)
	fmt.Fprintf(r.stdout, "Text:     %s\n", text)
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Spawn the orchestrator with: aom orchestrator start")
	return nil
}

func (r Runner) executeGoalShow(args []string) error {
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return wrapProjectNotFound(err)
	}

	rec, err := artifact.ReadGoalFile(result.Project.RepoPath)
	if err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Status:  %s\n", rec.Status)
	fmt.Fprintf(r.stdout, "Set at:  %s\n", rec.SetAt.UTC().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(r.stdout, "\nGoal:\n  %s\n", rec.Text)
	return nil
}

func (r Runner) executeGoalComplete(args []string) error {
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return wrapProjectNotFound(err)
	}

	if err := artifact.CompleteGoalFile(result.Project.RepoPath); err != nil {
		return err
	}
	fmt.Fprintln(r.stdout, "Goal marked complete.")
	return nil
}
