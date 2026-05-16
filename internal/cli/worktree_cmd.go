package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/artifact"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/worktree"
)

func (r Runner) executeWorktreeRepair(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("worktree repair does not accept extra positional arguments in the current milestone")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskRecord, err := r.loadTaskByID(result, strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if taskRecord == nil {
		return fmt.Errorf("task %q not found", strings.TrimSpace(args[0]))
	}

	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer worktreeDB.Close()

	wasRepaired, record, err := worktreeService.Repair(taskRecord.ID, result.Project.RepoPath)
	if err != nil {
		return err
	}

	if wasRepaired {
		if err := r.syncTaskArtifacts(result, taskRecord.ID, artifact.Event{
			Type:        "worktree.repaired",
			Actor:       "operator",
			Summary:     "Worktree continuity was explicitly repaired",
			StateEffect: fmt.Sprintf("Worktree %s", record.Status),
		}, false); err != nil {
			return err
		}
		fmt.Fprintln(r.stdout, "Worktree repaired")
	} else {
		fmt.Fprintln(r.stdout, "Worktree already healthy, no repair needed")
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Task: %s\n", taskRecord.ID)
	fmt.Fprintf(r.stdout, "Status: %s\n", record.Status)
	fmt.Fprintf(r.stdout, "Branch: %s\n", record.BranchName)
	fmt.Fprintf(r.stdout, "Path: %s\n", record.WorktreePath)

	return nil
}

func (r Runner) executeWorktreeReadFile(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: aom worktree read-file <task-id> <relative-path>")
	}

	taskID := strings.TrimSpace(args[0])
	relPath := filepath.Clean(strings.TrimSpace(args[1]))

	// Reject obvious traversal attempts before hitting the filesystem.
	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("path %q escapes the worktree root", args[1])
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	worktreeService, wDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer wDB.Close()

	mapping, err := worktreeService.GetByTask(taskID)
	if err != nil {
		return err
	}
	if mapping == nil {
		return fmt.Errorf("task %q has no worktree", taskID)
	}
	if mapping.Status != "Ready" && mapping.Status != "Active" {
		return fmt.Errorf("worktree for task %q is not available (status: %s)", taskID, mapping.Status)
	}

	// Final path validation: resolved path must stay inside worktree root.
	worktreeRoot := filepath.Clean(mapping.WorktreePath)
	targetPath := filepath.Join(worktreeRoot, relPath)
	targetPath = filepath.Clean(targetPath)

	if !strings.HasPrefix(targetPath, worktreeRoot+string(filepath.Separator)) && targetPath != worktreeRoot {
		return fmt.Errorf("path %q escapes the worktree root", args[1])
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Audit trail.
	_ = r.syncTaskArtifacts(result, taskID, artifact.Event{
		Type:    "worktree.read",
		Actor:   "operator",
		Summary: fmt.Sprintf("Read file %s from worktree of task %s", relPath, taskID),
	}, false)

	fmt.Fprint(r.stdout, string(data))
	return nil
}

func worktreeHint(taskID string, mapping *worktree.Record, driftKind string) string {
	if mapping == nil {
		return ""
	}

	switch mapping.Status {
	case worktree.StatusNeedsRepair:
		switch driftKind {
		case worktree.DriftMissingPath:
			return fmt.Sprintf("run \"aom worktree repair %s\" to recreate the missing git worktree path before continuing", taskID)
		case worktree.DriftUnregisteredArtifactOnlyPath:
			return fmt.Sprintf("run \"aom worktree repair %s\" to recreate the unregistered worktree path; the existing path only contains AOM-owned content", taskID)
		case worktree.DriftUnregisteredDirtyPath:
			return fmt.Sprintf("inspect the existing worktree path and clean up non-artifact content manually before running \"aom worktree repair %s\"", taskID)
		default:
			return fmt.Sprintf("run \"aom worktree repair %s\" or inspect the git worktree path before continuing", taskID)
		}
	case worktree.StatusActive:
		return "task worktree is currently bound to a live session"
	default:
		return ""
	}
}

func changedFilesSummary(worktreePath, repoPath string) string {
	target := strings.TrimSpace(worktreePath)
	if target == "" {
		target = strings.TrimSpace(repoPath)
	}
	if target == "" {
		return "unavailable in current milestone"
	}

	if _, err := exec.LookPath("git"); err != nil {
		return "unavailable in current milestone"
	}

	output, err := exec.Command("git", "-C", target, "status", "--short").CombinedOutput()
	if err != nil {
		return "unavailable in current milestone"
	}
	lines := strings.FieldsFunc(strings.TrimSpace(string(output)), func(r rune) bool { return r == '\n' })
	if len(lines) == 0 || strings.TrimSpace(string(output)) == "" {
		return "no local changes detected"
	}
	if len(lines) == 1 {
		return strings.TrimSpace(lines[0])
	}
	return fmt.Sprintf("%d changed paths", len(lines))
}
