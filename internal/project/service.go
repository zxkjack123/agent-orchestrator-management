package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/agent"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/config"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/db"
	"gopkg.in/yaml.v3"
)

const aomDirName = ".aom"

// Service owns project initialization and registration flows.
type Service struct{}

// InitAgentOption describes one selectable agent from a project-init template.
type InitAgentOption struct {
	Name    string
	Role    string
	Runtime string
}

// InitAgentSelection describes one requested agent selection during project init.
type InitAgentSelection struct {
	Name    string
	Role    string
	Runtime string
	Inline  bool
}

// InitParams describes project init input.
type InitParams struct {
	Name            string
	RepoPath        string
	DefaultBranch   string
	SessionPrefix   string
	TemplateName    string
	TemplateDir     string
	AgentSelections []InitAgentSelection
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
	RoleConfigs    map[string]config.RoleConfig
	Resources      config.ResourcesFile
	Policy         config.PolicyFile
	DBPath         string
	StateDir       string
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
		defaultBranch = detectDefaultBranch(repoAbsPath)
	}

	sessionPrefix := strings.TrimSpace(params.SessionPrefix)
	if sessionPrefix == "" {
		sessionPrefix = sanitizeName(name)
	}

	templateDir, err := resolveInitTemplateDir(params.TemplateName, params.TemplateDir)
	if err != nil {
		return nil, err
	}

	aomPath := filepath.Join(repoAbsPath, aomDirName)
	if err := os.MkdirAll(aomPath, 0o755); err != nil {
		return nil, fmt.Errorf("create .aom directory: %w", err)
	}

	if err := writeConfigFiles(aomPath, name, repoAbsPath, defaultBranch, sessionPrefix, templateDir, params.AgentSelections); err != nil {
		return nil, err
	}

	cfg, err := config.LoadProjectConfig(repoAbsPath)
	if err != nil {
		return nil, err
	}
	if err := seedAgentProfiles(aomPath, cfg); err != nil {
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

// PreviewInitAgents returns the available agents from the chosen init template.
func (s *Service) PreviewInitAgents(params InitParams) ([]InitAgentOption, error) {
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

	defaultBranch := strings.TrimSpace(params.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	sessionPrefix := strings.TrimSpace(params.SessionPrefix)
	if sessionPrefix == "" {
		sessionPrefix = sanitizeName(name)
	}

	templateDir, err := resolveInitTemplateDir(params.TemplateName, params.TemplateDir)
	if err != nil {
		return nil, err
	}

	rendered, err := renderAgentsConfig(projectTemplateData{
		Name:          name,
		RepoPath:      repoAbsPath,
		DefaultBranch: defaultBranch,
		SessionPrefix: sessionPrefix,
	}, templateDir, nil)
	if err != nil {
		return nil, fmt.Errorf("render agents template: %w", err)
	}

	var cfg initAgentsConfig
	if err := yaml.Unmarshal(rendered, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal agents template: %w", err)
	}

	options := make([]InitAgentOption, 0, len(cfg.Agents))
	for agentName, agentCfg := range cfg.Agents {
		options = append(options, InitAgentOption{
			Name:    agentName,
			Role:    agentCfg.Role,
			Runtime: agentCfg.Runtime,
		})
	}
	sort.Slice(options, func(i, j int) bool {
		return options[i].Name < options[j].Name
	})
	return options, nil
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

	// Seed profiles for any agents added to agents.yaml after the initial project init.
	// seedAgentProfiles is idempotent — it skips agents whose profile file already exists.
	aomPath := filepath.Join(repoAbsPath, aomDirName)
	if err := seedAgentProfiles(aomPath, cfg); err != nil {
		return nil, err
	}

	agents, err := agentRepo.ListByProjectID(projectID)
	if err != nil {
		return nil, err
	}

	return &OpenResult{
		Project:        record,
		Agents:         agents,
		RoleConfigs:    cfg.Agents.Roles,
		Resources:      cfg.Resources,
		Policy:         cfg.Policy,
		DBPath:         dbPath,
		StateDir:       cfg.Project.Context.StateDir,
		TerminalDriver: cfg.Project.Runtime.Terminal,
		SessionPrefix:  cfg.Project.Runtime.SessionPrefix,
	}, nil
}

func detectDefaultBranch(repoPath string) string {
	out, err := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "main"
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || branch == "HEAD" {
		return "main"
	}
	return branch
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

func resolveInitTemplateDir(templateName, templateDir string) (string, error) {
	templateDir = strings.TrimSpace(templateDir)
	templateName = strings.TrimSpace(templateName)
	if templateDir != "" && templateName != "" {
		return "", fmt.Errorf("template_dir and template preset cannot both be set")
	}
	if templateName != "" {
		resolved, err := resolvePresetTemplateDir(templateName)
		if err != nil {
			return "", err
		}
		templateDir = resolved
	}
	if templateDir == "" {
		return "", nil
	}

	resolved, err := filepath.Abs(templateDir)
	if err != nil {
		return "", fmt.Errorf("resolve template dir: %w", err)
	}
	if info, err := os.Stat(resolved); err != nil {
		return "", fmt.Errorf("stat template dir %q: %w", resolved, err)
	} else if !info.IsDir() {
		return "", fmt.Errorf("template dir %q is not a directory", resolved)
	}
	return resolved, nil
}
