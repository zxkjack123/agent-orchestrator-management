package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const projectMemoryFilename = "project-memory.md"

func projectMemoryPath(repoPath string) string {
	return filepath.Join(repoPath, ".aom", projectMemoryFilename)
}

// executeMemory dispatches aom memory subcommands.
func (r Runner) executeMemory(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("memory subcommand is required: append | show | clear")
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			r.printHelp()
			return nil
		}
	}
	switch args[0] {
	case "append":
		return r.executeMemoryAppend(args[1:])
	case "show":
		return r.executeMemoryShow(args[1:])
	case "clear":
		return r.executeMemoryClear(args[1:])
	default:
		return fmt.Errorf("unknown memory subcommand %q", args[0])
	}
}

// executeMemoryAppend adds a timestamped note to .aom/project-memory.md.
// Usage: aom memory append "<note>"
func (r Runner) executeMemoryAppend(args []string) error {
	note := strings.TrimSpace(strings.Join(args, " "))
	if note == "" {
		return fmt.Errorf("note text is required")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	memPath := projectMemoryPath(result.Project.RepoPath)

	// Ensure file exists with header on first use.
	if _, statErr := os.Stat(memPath); os.IsNotExist(statErr) {
		header := "# Project Memory\n\n" +
			"Append-only log of decisions, conventions, and gotchas.\n" +
			"Injected into every agent session at spawn time.\n\n"
		if err := os.WriteFile(memPath, []byte(header), 0o644); err != nil {
			return fmt.Errorf("create project-memory.md: %w", err)
		}
	}

	actor := resolvedActor()
	ts := time.Now().UTC().Format("2006-01-02")
	line := fmt.Sprintf("[%s] %s: %s\n", ts, actor, note)

	f, err := os.OpenFile(memPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open project-memory.md: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("write project-memory.md: %w", err)
	}

	fmt.Fprintln(r.stdout, "Memory saved.")
	fmt.Fprintf(r.stdout, "File: %s\n", memPath)
	fmt.Fprintf(r.stdout, "Note: %s\n", line)
	return nil
}

// executeMemoryShow prints the current contents of project-memory.md.
func (r Runner) executeMemoryShow(args []string) error {
	_ = args

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	memPath := projectMemoryPath(result.Project.RepoPath)
	data, err := os.ReadFile(memPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(r.stdout, "No project memory yet. Use: aom memory append \"<note>\"")
			return nil
		}
		return fmt.Errorf("read project-memory.md: %w", err)
	}

	fmt.Fprintf(r.stdout, "%s", data)
	return nil
}

// executeMemoryClear truncates project-memory.md after confirmation.
// Usage: aom memory clear [--confirm]
func (r Runner) executeMemoryClear(args []string) error {
	confirmed := false
	for _, a := range args {
		if a == "--confirm" {
			confirmed = true
		}
	}

	if !confirmed {
		return fmt.Errorf("this will erase all project memory. Re-run with --confirm to proceed")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	memPath := projectMemoryPath(result.Project.RepoPath)
	if _, err := os.Stat(memPath); os.IsNotExist(err) {
		fmt.Fprintln(r.stdout, "Nothing to clear — project-memory.md does not exist.")
		return nil
	}

	if err := os.WriteFile(memPath, []byte("# Project Memory\n\n"), 0o644); err != nil {
		return fmt.Errorf("clear project-memory.md: %w", err)
	}

	fmt.Fprintln(r.stdout, "Project memory cleared.")
	return nil
}
