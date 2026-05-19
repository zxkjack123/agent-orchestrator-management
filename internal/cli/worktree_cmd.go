package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/artifact"
	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/worktree"
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

// executeWorktreeCommit stages and commits all changes in a task worktree using
// explicit GIT_DIR and GIT_WORK_TREE env vars, bypassing the .git pointer file
// so the commit works correctly regardless of how the worktree was provisioned.
func (r Runner) executeWorktreeCommit(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task identifier is required")
	}

	taskID := strings.TrimSpace(args[0])
	var commitMsg string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-m", "--message":
			i++
			if i >= len(args) {
				return fmt.Errorf("-m requires a commit message")
			}
			commitMsg = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if commitMsg == "" {
		return fmt.Errorf("commit message is required (-m <msg>)")
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
		return fmt.Errorf("task %q has no worktree — work may be in the main repo; use git commit directly, then run: aom checkpoint <session-id>", taskID)
	}
	if mapping.Status != "Ready" && mapping.Status != "Active" {
		return fmt.Errorf("worktree for task %q is not available (status: %s) — use git commit directly, then run: aom checkpoint <session-id>", taskID, mapping.Status)
	}

	wtPath := mapping.WorktreePath
	gitDir, err := resolveWorktreeGitDir(result.Project.RepoPath, wtPath)
	if err != nil {
		return fmt.Errorf("resolve git dir: %w", err)
	}

	env := append(os.Environ(),
		"GIT_DIR="+gitDir,
		"GIT_WORK_TREE="+wtPath,
	)

	const gitTimeout = 60 * time.Second
	addCtx, addCancel := context.WithTimeout(context.Background(), gitTimeout)
	defer addCancel()
	addCmd := exec.CommandContext(addCtx, "git", "add", "-A")
	addCmd.Env = env
	addCmd.Dir = wtPath
	if out, addErr := addCmd.CombinedOutput(); addErr != nil {
		if addCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("git add timed out after %s", gitTimeout)
		}
		return fmt.Errorf("git add: %w\n%s", addErr, strings.TrimSpace(string(out)))
	}

	commitCtx, commitCancel := context.WithTimeout(context.Background(), gitTimeout)
	defer commitCancel()
	commitCmd := exec.CommandContext(commitCtx, "git", "commit", "-m", commitMsg)
	commitCmd.Env = env
	commitCmd.Dir = wtPath
	out, err := commitCmd.CombinedOutput()
	if err != nil {
		if commitCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("git commit timed out after %s", gitTimeout)
		}
		return fmt.Errorf("git commit: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	fmt.Fprint(r.stdout, string(out))
	return nil
}

// resolveWorktreeGitDir finds the GIT_DIR for a git worktree. It first reads
// the .git pointer file in the worktree; if that fails it falls back to
// scanning .git/worktrees/ for a matching gitdir entry.
func resolveWorktreeGitDir(repoPath, worktreePath string) (string, error) {
	gitFile := filepath.Join(worktreePath, ".git")
	data, err := os.ReadFile(gitFile)
	if err == nil {
		line := strings.TrimSpace(string(data))
		if strings.HasPrefix(line, "gitdir:") {
			candidate := strings.TrimSpace(line[len("gitdir:"):])
			if filepath.IsAbs(candidate) {
				return candidate, nil
			}
			return filepath.Clean(filepath.Join(worktreePath, candidate)), nil
		}
	}

	// Fallback: scan .git/worktrees/ for an entry whose gitdir points here.
	worktreesDir := filepath.Join(repoPath, ".git", "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return "", fmt.Errorf("read .git/worktrees: %w", err)
	}
	normTarget := filepath.ToSlash(filepath.Clean(worktreePath))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		linkData, err := os.ReadFile(filepath.Join(worktreesDir, entry.Name(), "gitdir"))
		if err != nil {
			continue
		}
		linkPath := strings.TrimSpace(string(linkData))
		norm := filepath.ToSlash(filepath.Clean(strings.TrimSuffix(linkPath, "/.git")))
		if norm == normTarget {
			return filepath.Join(worktreesDir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("could not find git worktree entry for %s", worktreePath)
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "git", "-C", target, "status", "--short").CombinedOutput()
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
