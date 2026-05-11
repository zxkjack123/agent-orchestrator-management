package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/agent"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/config"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/db"
)

const aomDirName = ".aom"

// Service owns project initialization and registration flows.
type Service struct{}

// InitParams describes project init input.
type InitParams struct {
	Name          string
	RepoPath      string
	DefaultBranch string
	SessionPrefix string
}

// InitResult describes project init output.
type InitResult struct {
	ProjectName string
	RepoPath    string
	AOMPath     string
	DBPath      string
}

// OpenResult describes a reconciled project state for Milestone 1.
type OpenResult struct {
	Project        Record
	Agents         []agent.Record
	DBPath         string
	TerminalDriver string
	SessionPrefix  string
}

// NewService creates a project service.
func NewService() *Service {
	return &Service{}
}

// Init creates the baseline AOM project structure and registers it in SQLite.
func (s *Service) Init(params InitParams) (*InitResult, error) {
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return nil, fmt.Errorf("project name is required")
	}

	repoPath := strings.TrimSpace(params.RepoPath)
	if repoPath == "" {
		return nil, fmt.Errorf("repo path is required")
	}

	repoAbsPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repo path: %w", err)
	}

	if _, err := os.Stat(repoAbsPath); err != nil {
		return nil, fmt.Errorf("stat repo path %q: %w", repoAbsPath, err)
	}

	defaultBranch := strings.TrimSpace(params.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	sessionPrefix := strings.TrimSpace(params.SessionPrefix)
	if sessionPrefix == "" {
		sessionPrefix = sanitizeName(name)
	}

	aomPath := filepath.Join(repoAbsPath, aomDirName)
	if err := os.MkdirAll(aomPath, 0o755); err != nil {
		return nil, fmt.Errorf("create .aom directory: %w", err)
	}

	cfg := baselineConfig(name, repoAbsPath, defaultBranch, sessionPrefix)
	if err := writeConfigFiles(aomPath, cfg); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(aomPath, "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	projectRepo := NewRepository(sqlDB)
	projectID := sanitizeName(name)
	if err := projectRepo.Upsert(Record{
		ID:            projectID,
		Name:          name,
		RepoPath:      repoAbsPath,
		DefaultBranch: defaultBranch,
	}); err != nil {
		return nil, err
	}

	agentRepo := agent.NewRepository(sqlDB)
	if err := agentRepo.Sync(projectID, cfg.Agents); err != nil {
		return nil, err
	}

	return &InitResult{
		ProjectName: name,
		RepoPath:    repoAbsPath,
		AOMPath:     aomPath,
		DBPath:      dbPath,
	}, nil
}

// Open loads config, opens the DB, and syncs config-backed state for an existing project.
func (s *Service) Open(repoPath string) (*OpenResult, error) {
	repoAbsPath, err := filepath.Abs(strings.TrimSpace(repoPath))
	if err != nil {
		return nil, fmt.Errorf("resolve repo path: %w", err)
	}

	cfg, err := config.LoadProjectConfig(repoAbsPath)
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(repoAbsPath, aomDirName, "sessions.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	projectRepo := NewRepository(sqlDB)
	projectID := sanitizeName(cfg.Project.Name)
	record := Record{
		ID:            projectID,
		Name:          cfg.Project.Name,
		RepoPath:      repoAbsPath,
		DefaultBranch: cfg.Project.DefaultBranch,
	}
	if err := projectRepo.Upsert(record); err != nil {
		return nil, err
	}

	agentRepo := agent.NewRepository(sqlDB)
	if err := agentRepo.Sync(projectID, cfg.Agents); err != nil {
		return nil, err
	}

	agents, err := agentRepo.ListByProjectID(projectID)
	if err != nil {
		return nil, err
	}

	return &OpenResult{
		Project:        record,
		Agents:         agents,
		DBPath:         dbPath,
		TerminalDriver: cfg.Project.Runtime.Terminal,
		SessionPrefix:  cfg.Project.Runtime.SessionPrefix,
	}, nil
}

func sanitizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")

	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			builder.WriteRune(r)
		}
	}

	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "aom-project"
	}

	return result
}
