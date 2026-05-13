package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/db"
)

func TestServiceCreatePlannedCreatesMapping(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	service := NewService(sqlDB)

	record, err := service.CreatePlanned(CreateParams{
		ProjectID:     "proj-1",
		TaskID:        "TASK-001",
		TaskTitle:     "Fix login validation",
		RepoPath:      "C:/repo",
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("CreatePlanned failed: %v", err)
	}

	if record.Status != "Planned" {
		t.Fatalf("Status = %q, want Planned", record.Status)
	}
	if record.BaseBranch != "main" {
		t.Fatalf("BaseBranch = %q, want main", record.BaseBranch)
	}
	if !strings.HasPrefix(record.BranchName, "aom/task-001-fix-login-validation") {
		t.Fatalf("BranchName = %q, want sanitized branch prefix", record.BranchName)
	}
	if !strings.Contains(record.WorktreePath, filepath.Join(".aom", "worktrees")) {
		t.Fatalf("WorktreePath = %q, want .aom/worktrees path", record.WorktreePath)
	}

	loaded, err := service.GetByTask("TASK-001")
	if err != nil {
		t.Fatalf("GetByTask failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("GetByTask returned nil record")
	}
	if loaded.TaskID != "TASK-001" {
		t.Fatalf("TaskID = %q, want TASK-001", loaded.TaskID)
	}
}

func TestServiceEnsureProvisionedSkipsNonGitRepo(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	service := NewService(sqlDB)
	service.lookPath = func(string) (string, error) { return "git", nil }
	service.runGit = func(string, ...string) ([]byte, error) { return nil, fmt.Errorf("not a git repo") }

	record, err := service.CreatePlanned(CreateParams{
		ProjectID:     "proj-1",
		TaskID:        "TASK-001",
		TaskTitle:     "Fix login validation",
		RepoPath:      "C:/repo",
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("CreatePlanned failed: %v", err)
	}

	provisioned, err := service.EnsureProvisioned(record.TaskID, "C:/repo")
	if err != nil {
		t.Fatalf("EnsureProvisioned failed: %v", err)
	}
	if provisioned.Status != "Planned" {
		t.Fatalf("Status = %q, want Planned", provisioned.Status)
	}
}

func TestServiceValidateProvisioningPreconditionsSkipsNonGitRepo(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	service := NewService(sqlDB)
	service.lookPath = func(string) (string, error) { return "git", nil }
	service.runGit = func(string, ...string) ([]byte, error) { return nil, fmt.Errorf("not a git repo") }

	if err := service.ValidateProvisioningPreconditions("C:/repo", "main"); err != nil {
		t.Fatalf("ValidateProvisioningPreconditions failed: %v", err)
	}
}

func TestServiceValidateProvisioningPreconditionsRejectsUnbornDefaultBranch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	service := NewService(sqlDB)
	service.lookPath = func(string) (string, error) { return "git", nil }
	service.runGit = func(repoPath string, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "rev-parse" && args[1] == "--is-inside-work-tree" {
			return []byte("true\n"), nil
		}
		if len(args) >= 3 && args[0] == "rev-parse" && args[1] == "--verify" && args[2] == "main" {
			return []byte("fatal: invalid reference: main"), fmt.Errorf("exit status 128")
		}
		return nil, nil
	}

	err = service.ValidateProvisioningPreconditions("/repo", "main")
	if err == nil {
		t.Fatal("ValidateProvisioningPreconditions returned nil error, want unborn branch failure")
	}
	if !strings.Contains(err.Error(), "create an initial commit first") {
		t.Fatalf("error = %q, want initial commit hint", err)
	}
}

func TestServiceEnsureProvisionedMarksReadyAfterGitWorktreeAdd(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repoRoot := t.TempDir()
	service := NewService(sqlDB)
	service.lookPath = func(string) (string, error) { return "git", nil }
	service.stat = func(path string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	service.mkdirAll = func(string, os.FileMode) error { return nil }
	var calls [][]string
	service.runGit = func(repoPath string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{repoPath}, args...))
		if len(args) >= 2 && args[0] == "rev-parse" {
			return []byte("true\n"), nil
		}
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			return []byte("prepared\n"), nil
		}
		return nil, nil
	}

	record, err := service.CreatePlanned(CreateParams{
		ProjectID:     "proj-1",
		TaskID:        "TASK-001",
		TaskTitle:     "Fix login validation",
		RepoPath:      repoRoot,
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("CreatePlanned failed: %v", err)
	}

	provisioned, err := service.EnsureProvisioned(record.TaskID, repoRoot)
	if err != nil {
		t.Fatalf("EnsureProvisioned failed: %v", err)
	}
	if provisioned.Status != "Ready" {
		t.Fatalf("Status = %q, want Ready", provisioned.Status)
	}
	if len(calls) < 2 {
		t.Fatalf("git calls = %d, want at least 2", len(calls))
	}
	var addCall []string
	for _, call := range calls {
		if len(call) >= 3 && call[1] == "worktree" && call[2] == "add" {
			addCall = call
			break
		}
	}
	if !strings.Contains(strings.Join(addCall, " "), "worktree add -b") {
		t.Fatalf("calls = %#v, want worktree add -b", calls)
	}
}

func TestServiceReconcileMarksActiveForRegisteredWorktree(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repoRoot := t.TempDir()
	service := NewService(sqlDB)
	service.lookPath = func(string) (string, error) { return "git", nil }
	service.mkdirAll = func(string, os.FileMode) error { return nil }
	service.runGit = func(repoPath string, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "rev-parse" {
			return []byte("true\n"), nil
		}
		if len(args) >= 3 && args[0] == "worktree" && args[1] == "list" && args[2] == "--porcelain" {
			return []byte("worktree " + filepath.Join(repoRoot, ".aom", "worktrees", "task-001-fix-login-validation") + "\n"), nil
		}
		return nil, nil
	}

	record, err := service.CreatePlanned(CreateParams{
		ProjectID:     "proj-1",
		TaskID:        "TASK-001",
		TaskTitle:     "Fix login validation",
		RepoPath:      repoRoot,
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("CreatePlanned failed: %v", err)
	}
	service.stat = func(path string) (os.FileInfo, error) {
		if filepath.Clean(path) == filepath.Clean(record.WorktreePath) {
			return os.Stat(repoRoot)
		}
		return nil, os.ErrNotExist
	}

	reconciled, err := service.Reconcile(record.TaskID, repoRoot, true)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if reconciled.Status != StatusActive {
		t.Fatalf("Status = %q, want Active", reconciled.Status)
	}
}

func TestServiceReconcileMarksNeedsRepairWhenReadyPathIsMissing(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repoRoot := t.TempDir()
	service := NewService(sqlDB)
	service.lookPath = func(string) (string, error) { return "git", nil }
	service.runGit = func(repoPath string, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "rev-parse" {
			return []byte("true\n"), nil
		}
		if len(args) >= 3 && args[0] == "worktree" && args[1] == "list" && args[2] == "--porcelain" {
			return []byte("worktree " + filepath.Join(repoRoot, ".aom", "worktrees", "task-001-fix-login-validation") + "\n"), nil
		}
		return nil, nil
	}
	service.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }

	record, err := service.CreatePlanned(CreateParams{
		ProjectID:     "proj-1",
		TaskID:        "TASK-001",
		TaskTitle:     "Fix login validation",
		RepoPath:      repoRoot,
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("CreatePlanned failed: %v", err)
	}
	record.Status = StatusReady
	if err := service.repo.Upsert(*record); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	reconciled, err := service.Reconcile(record.TaskID, repoRoot, false)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if reconciled.Status != StatusNeedsRepair {
		t.Fatalf("Status = %q, want NeedsRepair", reconciled.Status)
	}
}

func TestServiceRepairRecreatesMissingRegisteredWorktreeUsingExistingBranch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repoRoot := t.TempDir()
	service := NewService(sqlDB)
	service.lookPath = func(string) (string, error) { return "git", nil }
	service.mkdirAll = func(string, os.FileMode) error { return nil }
	service.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }

	record, err := service.CreatePlanned(CreateParams{
		ProjectID:     "proj-1",
		TaskID:        "TASK-001",
		TaskTitle:     "Fix login validation",
		RepoPath:      repoRoot,
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("CreatePlanned failed: %v", err)
	}
	record.Status = StatusNeedsRepair
	if err := service.repo.Upsert(*record); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	var calls [][]string
	service.runGit = func(repoPath string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{repoPath}, args...))
		switch {
		case len(args) >= 2 && args[0] == "rev-parse":
			return []byte("true\n"), nil
		case len(args) >= 2 && args[0] == "worktree" && args[1] == "prune":
			return nil, nil
		case len(args) >= 3 && args[0] == "worktree" && args[1] == "list" && args[2] == "--porcelain":
			return nil, nil
		case len(args) >= 3 && args[0] == "branch" && args[1] == "--list":
			return []byte("* " + record.BranchName + "\n"), nil
		case len(args) >= 2 && args[0] == "worktree" && args[1] == "add":
			return []byte("prepared\n"), nil
		default:
			return nil, nil
		}
	}

	repaired, err := service.Repair(record.TaskID, repoRoot)
	if err != nil {
		t.Fatalf("Repair failed: %v", err)
	}
	if repaired.Status != StatusReady {
		t.Fatalf("Status = %q, want Ready", repaired.Status)
	}
	if len(calls) == 0 {
		t.Fatal("expected git calls")
	}
	var addCall []string
	for _, call := range calls {
		if len(call) >= 3 && call[1] == "worktree" && call[2] == "add" {
			addCall = call
			break
		}
	}
	if len(addCall) == 0 {
		t.Fatalf("calls = %#v, want worktree add", calls)
	}
	if strings.Contains(strings.Join(addCall, " "), " -b ") {
		t.Fatalf("addCall = %#v, want existing branch reuse without -b", addCall)
	}
}

func TestServiceRepairFailsWhenPathExistsButRegistrationIsMissing(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repoRoot := t.TempDir()
	service := NewService(sqlDB)
	service.lookPath = func(string) (string, error) { return "git", nil }
	service.runGit = func(repoPath string, args ...string) ([]byte, error) {
		switch {
		case len(args) >= 2 && args[0] == "rev-parse":
			return []byte("true\n"), nil
		case len(args) >= 2 && args[0] == "worktree" && args[1] == "prune":
			return nil, nil
		case len(args) >= 3 && args[0] == "worktree" && args[1] == "list" && args[2] == "--porcelain":
			return nil, nil
		default:
			return nil, nil
		}
	}

	record, err := service.CreatePlanned(CreateParams{
		ProjectID:     "proj-1",
		TaskID:        "TASK-001",
		TaskTitle:     "Fix login validation",
		RepoPath:      repoRoot,
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("CreatePlanned failed: %v", err)
	}
	record.Status = StatusNeedsRepair
	if err := service.repo.Upsert(*record); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}
	service.stat = func(path string) (os.FileInfo, error) { return os.Stat(repoRoot) }
	service.readDir = func(path string) ([]os.DirEntry, error) {
		return []os.DirEntry{stubDirEntry{name: "README.md"}}, nil
	}

	_, err = service.Repair(record.TaskID, repoRoot)
	if err == nil {
		t.Fatal("Repair unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "manual cleanup is required") {
		t.Fatalf("err = %v, want manual cleanup hint", err)
	}
}

func TestServiceRepairRecreatesUnregisteredArtifactOnlyPath(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repoRoot := t.TempDir()
	service := NewService(sqlDB)
	service.lookPath = func(string) (string, error) { return "git", nil }
	service.mkdirAll = func(string, os.FileMode) error { return nil }

	record, err := service.CreatePlanned(CreateParams{
		ProjectID:     "proj-1",
		TaskID:        "TASK-001",
		TaskTitle:     "Fix login validation",
		RepoPath:      repoRoot,
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("CreatePlanned failed: %v", err)
	}
	record.Status = StatusNeedsRepair
	if err := service.repo.Upsert(*record); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	service.stat = func(path string) (os.FileInfo, error) { return os.Stat(repoRoot) }
	service.readDir = func(path string) ([]os.DirEntry, error) {
		return []os.DirEntry{stubDirEntry{name: ".agent", dir: true}}, nil
	}

	var removedPath string
	service.removeAll = func(path string) error {
		removedPath = path
		return nil
	}

	var calls [][]string
	service.runGit = func(repoPath string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{repoPath}, args...))
		switch {
		case len(args) >= 2 && args[0] == "rev-parse":
			return []byte("true\n"), nil
		case len(args) >= 2 && args[0] == "worktree" && args[1] == "prune":
			return nil, nil
		case len(args) >= 3 && args[0] == "worktree" && args[1] == "list" && args[2] == "--porcelain":
			return nil, nil
		case len(args) >= 3 && args[0] == "branch" && args[1] == "--list":
			return []byte("* " + record.BranchName + "\n"), nil
		case len(args) >= 2 && args[0] == "worktree" && args[1] == "add":
			return []byte("prepared\n"), nil
		default:
			return nil, nil
		}
	}

	repaired, err := service.Repair(record.TaskID, repoRoot)
	if err != nil {
		t.Fatalf("Repair failed: %v", err)
	}
	if repaired.Status != StatusReady {
		t.Fatalf("Status = %q, want Ready", repaired.Status)
	}
	if removedPath != record.WorktreePath {
		t.Fatalf("removedPath = %q, want %q", removedPath, record.WorktreePath)
	}
	var addCall []string
	for _, call := range calls {
		if len(call) >= 3 && call[1] == "worktree" && call[2] == "add" {
			addCall = call
			break
		}
	}
	if len(addCall) == 0 {
		t.Fatalf("calls = %#v, want worktree add", calls)
	}
}

type stubDirEntry struct {
	name string
	dir  bool
}

func (s stubDirEntry) Name() string               { return s.name }
func (s stubDirEntry) IsDir() bool                { return s.dir }
func (s stubDirEntry) Type() os.FileMode          { return 0 }
func (s stubDirEntry) Info() (os.FileInfo, error) { return nil, nil }

func TestServiceDriftKindClassifiesMissingAndUnregisteredPaths(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer sqlDB.Close()

	repoRoot := t.TempDir()
	service := NewService(sqlDB)
	service.lookPath = func(string) (string, error) { return "git", nil }

	record, err := service.CreatePlanned(CreateParams{
		ProjectID:     "proj-1",
		TaskID:        "TASK-001",
		TaskTitle:     "Fix login validation",
		RepoPath:      repoRoot,
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("CreatePlanned failed: %v", err)
	}
	record.Status = StatusNeedsRepair
	if err := service.repo.Upsert(*record); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	service.runGit = func(repoPath string, args ...string) ([]byte, error) {
		switch {
		case len(args) >= 2 && args[0] == "rev-parse":
			return []byte("true\n"), nil
		case len(args) >= 3 && args[0] == "worktree" && args[1] == "list" && args[2] == "--porcelain":
			return nil, nil
		default:
			return nil, nil
		}
	}

	service.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	driftKind, err := service.DriftKind(record.TaskID, repoRoot)
	if err != nil {
		t.Fatalf("DriftKind failed: %v", err)
	}
	if driftKind != DriftMissingPath {
		t.Fatalf("DriftKind = %q, want %q", driftKind, DriftMissingPath)
	}

	service.stat = func(path string) (os.FileInfo, error) { return os.Stat(repoRoot) }
	service.readDir = func(path string) ([]os.DirEntry, error) {
		return []os.DirEntry{stubDirEntry{name: ".agent", dir: true}}, nil
	}
	driftKind, err = service.DriftKind(record.TaskID, repoRoot)
	if err != nil {
		t.Fatalf("DriftKind failed: %v", err)
	}
	if driftKind != DriftUnregisteredArtifactOnlyPath {
		t.Fatalf("DriftKind = %q, want %q", driftKind, DriftUnregisteredArtifactOnlyPath)
	}

	service.readDir = func(path string) ([]os.DirEntry, error) {
		return []os.DirEntry{stubDirEntry{name: "README.md"}}, nil
	}
	driftKind, err = service.DriftKind(record.TaskID, repoRoot)
	if err != nil {
		t.Fatalf("DriftKind failed: %v", err)
	}
	if driftKind != DriftUnregisteredDirtyPath {
		t.Fatalf("DriftKind = %q, want %q", driftKind, DriftUnregisteredDirtyPath)
	}
}
