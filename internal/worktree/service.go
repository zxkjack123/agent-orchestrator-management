package worktree

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	StatusPlanned     = "Planned"
	StatusReady       = "Ready"
	StatusActive      = "Active"
	StatusNeedsRepair = "NeedsRepair"
)

const maxBranchSegmentLen = 80

const (
	DriftNone                         = ""
	DriftMissingPath                  = "MissingPath"
	DriftUnregisteredArtifactOnlyPath = "UnregisteredArtifactOnlyPath"
	DriftUnregisteredDirtyPath        = "UnregisteredDirtyPath"
)

// CreateParams describes the minimum input needed to create a planned worktree mapping.
type CreateParams struct {
	ProjectID     string
	TaskID        string
	TaskTitle     string
	RepoPath      string
	DefaultBranch string
}

// Service owns worktree mapping behavior for Milestone 5.
type Service struct {
	repo      *Repository
	lookPath  func(string) (string, error)
	runGit    func(repoPath string, args ...string) ([]byte, error)
	stat      func(string) (os.FileInfo, error)
	readDir   func(string) ([]os.DirEntry, error)
	mkdirAll  func(string, os.FileMode) error
	removeAll func(string) error
}

// NewService creates a worktree service backed by the provided database.
func NewService(db *sql.DB) *Service {
	return &Service{
		repo:     NewRepository(db),
		lookPath: exec.LookPath,
		runGit: func(repoPath string, args ...string) ([]byte, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "git", append([]string{
				"-C", repoPath,
				"-c", "credential.helper=",
			}, args...)...)
			cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
			return cmd.CombinedOutput()
		},
		stat:      os.Stat,
		readDir:   os.ReadDir,
		mkdirAll:  os.MkdirAll,
		removeAll: os.RemoveAll,
	}
}

// ProvisionAgentWorkspace creates a permanent git worktree for an agent at
// <repoPath>/.aom/agents/<agentName>/workspace/ on branch agents/<agentName>.
// Idempotent: if the workspace directory already exists and is a registered
// git worktree, returns the path without error.
func (s *Service) ProvisionAgentWorkspace(repoPath, agentName string) (string, error) {
	path := filepath.Join(repoPath, ".aom", "agents", agentName, "workspace")
	branch := "agents/" + agentName

	if _, err := s.stat(path); err == nil {
		return path, nil
	}

	if err := s.mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("provision agent workspace for %q: %w", agentName, err)
	}

	if _, err := s.runGit(repoPath, "worktree", "add", "-b", branch, path); err != nil {
		if _, err2 := s.runGit(repoPath, "worktree", "add", path, branch); err2 != nil {
			return "", fmt.Errorf("provision agent workspace for %q: %w", agentName, err2)
		}
	}

	return path, nil
}

// CreatePlanned inserts or updates the planned worktree mapping for one task.
func (s *Service) CreatePlanned(params CreateParams) (*Record, error) {
	projectID := strings.TrimSpace(params.ProjectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	taskID := strings.TrimSpace(params.TaskID)
	if taskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	repoPath := strings.TrimSpace(params.RepoPath)
	if repoPath == "" {
		return nil, fmt.Errorf("repo path is required")
	}
	defaultBranch := strings.TrimSpace(params.DefaultBranch)
	if defaultBranch == "" {
		return nil, fmt.Errorf("default branch is required")
	}

	record := Record{
		TaskID:       taskID,
		ProjectID:    projectID,
		Status:       StatusPlanned,
		BaseBranch:   defaultBranch,
		BranchName:   plannedBranchName(taskID, params.TaskTitle),
		WorktreePath: plannedWorktreePath(repoPath, taskID, params.TaskTitle),
	}

	if err := s.repo.Upsert(record); err != nil {
		return nil, err
	}

	return s.repo.GetByTaskID(taskID)
}

// GetByTask returns one worktree mapping by task ID.
func (s *Service) GetByTask(taskID string) (*Record, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task id is required")
	}

	return s.repo.GetByTaskID(taskID)
}

// ListByProject returns all worktree mappings for one project.
func (s *Service) ListByProject(projectID string) ([]Record, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}

	return s.repo.ListByProjectID(projectID)
}

// ValidateProvisioningPreconditions checks whether git-backed worktree creation
// can proceed without mutating persisted task state.
func (s *Service) ValidateProvisioningPreconditions(repoPath, defaultBranch string) error {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return fmt.Errorf("repo path is required")
	}
	defaultBranch = strings.TrimSpace(defaultBranch)
	if defaultBranch == "" {
		return fmt.Errorf("default branch is required")
	}

	if _, err := s.lookPath("git"); err != nil {
		return nil
	}
	if _, err := s.runGit(repoPath, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil
	}

	output, err := s.runGit(repoPath, "rev-parse", "--verify", defaultBranch)
	if err == nil {
		return nil
	}

	return fmt.Errorf(
		"project repo %q cannot provision task worktrees from default branch %q yet: %s; create an initial commit first",
		repoPath,
		defaultBranch,
		strings.TrimSpace(string(output)),
	)
}

// EnsureProvisioned upgrades a planned mapping to Ready when the repo supports git worktrees.
func (s *Service) EnsureProvisioned(taskID, repoPath string) (*Record, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return nil, fmt.Errorf("repo path is required")
	}

	record, err := s.repo.GetByTaskID(taskID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("worktree for task %q not found", taskID)
	}

	if record.Status == StatusReady || record.Status == StatusActive {
		return record, nil
	}

	if _, err := s.lookPath("git"); err != nil {
		return record, nil
	}

	if _, err := s.runGit(repoPath, "rev-parse", "--is-inside-work-tree"); err != nil {
		return record, nil
	}

	if err := s.mkdirAll(filepath.Dir(record.WorktreePath), 0o755); err != nil {
		return nil, fmt.Errorf("create worktree parent dir: %w", err)
	}

	if _, err := s.stat(record.WorktreePath); err == nil {
		record.Status = StatusReady
		if err := s.repo.Upsert(*record); err != nil {
			return nil, err
		}
		return s.repo.GetByTaskID(taskID)
	}

	if err := s.addWorktree(repoPath, record); err != nil {
		return nil, err
	}

	record.Status = StatusReady
	if err := s.repo.Upsert(*record); err != nil {
		return nil, err
	}

	return s.repo.GetByTaskID(taskID)
}

// Repair tries to recover a persisted worktree mapping that drifted from git
// registration or the filesystem without changing the task-to-worktree contract.
// The first return value reports whether any actual repair action was taken.
func (s *Service) Repair(taskID, repoPath string) (bool, *Record, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false, nil, fmt.Errorf("task id is required")
	}
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return false, nil, fmt.Errorf("repo path is required")
	}

	record, err := s.repo.GetByTaskID(taskID)
	if err != nil {
		return false, nil, err
	}
	if record == nil {
		return false, nil, fmt.Errorf("worktree for task %q not found", taskID)
	}
	if !s.gitAvailable(repoPath) {
		return false, nil, fmt.Errorf("project repo %q does not support git worktree repair", repoPath)
	}

	if output, err := s.runGit(repoPath, "worktree", "prune"); err != nil {
		return false, nil, fmt.Errorf("prune stale git worktrees: %s", strings.TrimSpace(string(output)))
	}

	pathExists, err := s.pathExists(record.WorktreePath)
	if err != nil {
		return false, nil, fmt.Errorf("stat worktree path for task %q: %w", taskID, err)
	}
	registered, err := s.isRegistered(repoPath, record.WorktreePath)
	if err != nil {
		return false, nil, err
	}

	wasRepaired := false
	switch {
	case pathExists && !registered:
		safeToRecreate, err := s.safeToRecreate(record.WorktreePath)
		if err != nil {
			return false, nil, err
		}
		if safeToRecreate {
			if err := s.removeAll(record.WorktreePath); err != nil {
				return false, nil, fmt.Errorf("remove stale worktree path %q: %w", record.WorktreePath, err)
			}
			if err := s.addWorktree(repoPath, record); err != nil {
				return false, nil, err
			}
			wasRepaired = true
			break
		}
		return false, nil, fmt.Errorf("worktree path %q exists but is not registered; manual cleanup is required before repair", record.WorktreePath)
	case !pathExists && !registered:
		if err := s.mkdirAll(filepath.Dir(record.WorktreePath), 0o755); err != nil {
			return false, nil, fmt.Errorf("create worktree parent dir: %w", err)
		}
		if err := s.addWorktree(repoPath, record); err != nil {
			return false, nil, err
		}
		wasRepaired = true
	}

	record.Status = StatusReady
	if err := s.repo.Upsert(*record); err != nil {
		return false, nil, err
	}

	result, err := s.repo.GetByTaskID(taskID)
	return wasRepaired, result, err
}

// Reconcile refreshes one persisted worktree mapping from filesystem, git registration,
// and whether the task currently has an active session.
func (s *Service) Reconcile(taskID, repoPath string, hasActiveSession bool) (*Record, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return nil, fmt.Errorf("repo path is required")
	}

	record, err := s.repo.GetByTaskID(taskID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}

	if !s.gitAvailable(repoPath) {
		return record, nil
	}

	pathExists := false
	if _, err := s.stat(record.WorktreePath); err == nil {
		pathExists = true
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat worktree path for task %q: %w", taskID, err)
	}

	registered, err := s.isRegistered(repoPath, record.WorktreePath)
	if err != nil {
		return nil, err
	}

	nextStatus := reconciledStatus(record.Status, pathExists, registered, hasActiveSession)
	if nextStatus == record.Status {
		return record, nil
	}

	record.Status = nextStatus
	if err := s.repo.Upsert(*record); err != nil {
		return nil, err
	}

	return s.repo.GetByTaskID(taskID)
}

// DriftKind classifies the current repair condition for one persisted worktree mapping.
func (s *Service) DriftKind(taskID, repoPath string) (string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return DriftNone, fmt.Errorf("task id is required")
	}
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return DriftNone, fmt.Errorf("repo path is required")
	}

	record, err := s.repo.GetByTaskID(taskID)
	if err != nil {
		return DriftNone, err
	}
	if record == nil || record.Status != StatusNeedsRepair || !s.gitAvailable(repoPath) {
		return DriftNone, nil
	}

	pathExists, err := s.pathExists(record.WorktreePath)
	if err != nil {
		return DriftNone, fmt.Errorf("stat worktree path for task %q: %w", taskID, err)
	}
	if !pathExists {
		return DriftMissingPath, nil
	}

	registered, err := s.isRegistered(repoPath, record.WorktreePath)
	if err != nil {
		return DriftNone, err
	}
	if !registered {
		safeToRecreate, err := s.safeToRecreate(record.WorktreePath)
		if err != nil {
			return DriftNone, err
		}
		if safeToRecreate {
			return DriftUnregisteredArtifactOnlyPath, nil
		}
		return DriftUnregisteredDirtyPath, nil
	}

	return DriftNone, nil
}

func (s *Service) gitAvailable(repoPath string) bool {
	if _, err := s.lookPath("git"); err != nil {
		return false
	}
	if _, err := s.runGit(repoPath, "rev-parse", "--is-inside-work-tree"); err != nil {
		return false
	}
	return true
}

func (s *Service) pathExists(path string) (bool, error) {
	if _, err := s.stat(path); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func (s *Service) isRegistered(repoPath, worktreePath string) (bool, error) {
	output, err := s.runGit(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("inspect git worktree registration: %s", strings.TrimSpace(string(output)))
	}

	expected := normalizePath(worktreePath)
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		if normalizePath(path) == expected {
			return true, nil
		}
	}

	return false, nil
}

// normalizePath resolves symlinks and cleans a path for comparison.
// On macOS /tmp is a symlink to /private/tmp; without this git worktree
// list returns /private/... while stored paths use /tmp/..., causing false
// NeedsRepair classifications.
func normalizePath(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return filepath.Clean(p)
}

func (s *Service) addWorktree(repoPath string, record *Record) error {
	branchExists, err := s.branchExists(repoPath, record.BranchName)
	if err != nil {
		return err
	}

	var args []string
	if branchExists {
		args = []string{"worktree", "add", record.WorktreePath, record.BranchName}
	} else {
		args = []string{"worktree", "add", "-b", record.BranchName, record.WorktreePath, record.BaseBranch}
	}

	if output, err := s.runGit(repoPath, args...); err != nil {
		return fmt.Errorf("provision worktree for task %q: %s", record.TaskID, strings.TrimSpace(string(output)))
	}

	return nil
}

func (s *Service) safeToRecreate(worktreePath string) (bool, error) {
	entries, err := s.readDir(worktreePath)
	if err != nil {
		return false, fmt.Errorf("inspect stale worktree path %q: %w", worktreePath, err)
	}
	if len(entries) == 0 {
		return true, nil
	}
	if len(entries) > 1 {
		return false, nil
	}

	entry := entries[0]
	if entry.Name() != ".agent" || !entry.IsDir() {
		return false, nil
	}

	return true, nil
}

func (s *Service) branchExists(repoPath, branchName string) (bool, error) {
	output, err := s.runGit(repoPath, "branch", "--list", branchName)
	if err != nil {
		return false, fmt.Errorf("check existing branch %q: %s", branchName, strings.TrimSpace(string(output)))
	}

	return strings.TrimSpace(string(output)) != "", nil
}

func reconciledStatus(current string, pathExists, registered, hasActiveSession bool) string {
	if !pathExists || !registered {
		switch current {
		case StatusReady, StatusActive, StatusNeedsRepair:
			return StatusNeedsRepair
		default:
			if pathExists || registered {
				return StatusNeedsRepair
			}
			return StatusPlanned
		}
	}

	if hasActiveSession {
		return StatusActive
	}

	return StatusReady
}

func plannedBranchName(taskID, taskTitle string) string {
	full := "aom/" + sanitizeSegment(taskID) + "-" + sanitizeSegment(taskTitle)
	if len(full) > maxBranchSegmentLen {
		full = strings.TrimRight(full[:maxBranchSegmentLen], "-")
	}
	return full
}

func plannedWorktreePath(repoPath, taskID, taskTitle string) string {
	dirName := sanitizeSegment(taskID) + "-" + sanitizeSegment(taskTitle)
	if len(dirName) > maxBranchSegmentLen {
		dirName = strings.TrimRight(dirName[:maxBranchSegmentLen], "-")
	}
	return filepath.Join(repoPath, ".aom", "worktrees", dirName)
}

func sanitizeSegment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")

	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			builder.WriteRune(r)
		}
	}

	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "task"
	}
	return result
}
