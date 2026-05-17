// Package config loads and validates AOM project configuration files.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	aomDirName = ".aom"
)

var (
	knownWorktreeModes = map[string]struct{}{
		"dedicated-writer": {},
		"read-only":        {},
	}

	knownCheckpointExpectations = map[string]struct{}{
		"required": {},
		"optional": {},
	}

	knownSessionModes = map[string]struct{}{
		"interactive": {},
		"headless":    {},
	}

	knownRuntimes = map[string]struct{}{
		"codex":  {},
		"claude": {},
		"kiro":   {},
		"gemini": {},
	}
)

// ProjectConfig is the fully loaded AOM project configuration set.
type ProjectConfig struct {
	RootPath string
	AOMPath  string

	Project   ProjectFile
	Agents    AgentsFile
	Resources ResourcesFile
	Policy    PolicyFile
}

// ProjectFile models .aom/project.yaml.
type ProjectFile struct {
	Name          string        `yaml:"name"`
	Repo          string        `yaml:"repo"`
	DefaultBranch string        `yaml:"default_branch"`
	Runtime       RuntimeConfig `yaml:"runtime"`
	Context       ContextConfig `yaml:"context"`
}

// RuntimeConfig stores global runtime defaults for AOM.
type RuntimeConfig struct {
	Terminal      string `yaml:"terminal"`
	SessionPrefix string `yaml:"session_prefix"`
}

// ContextConfig stores project artifact defaults.
type ContextConfig struct {
	StateDir           string `yaml:"state_dir"`
	CheckpointRequired bool   `yaml:"checkpoint_required"`
}

// AgentsFile models .aom/agents.yaml.
type AgentsFile struct {
	Roles  map[string]RoleConfig  `yaml:"roles"`
	Agents map[string]AgentConfig `yaml:"agents"`
}

// RoleConfig defines role-level behavior.
type RoleConfig struct {
	Class                 string `yaml:"class"`
	WorktreeMode          string `yaml:"worktree_mode"`
	CheckpointExpectation string `yaml:"checkpoint_expectation"`
	DefaultSessionMode    string `yaml:"default_session_mode"`
}

// AgentConfig defines a concrete project agent.
type AgentConfig struct {
	Runtime string `yaml:"runtime"`
	Role    string `yaml:"role"`
	Enabled bool   `yaml:"enabled"`
}

// ResourcesFile models .aom/resources.yaml.
type ResourcesFile struct {
	Skills       map[string]SkillConfig       `yaml:"skills"`
	MCPServers   map[string]MCPServerConfig   `yaml:"mcp_servers"`
	RoleBindings map[string]RoleBindingConfig `yaml:"role_bindings"`
}

// SkillConfig defines a project-governed skill.
type SkillConfig struct {
	Path     string   `yaml:"path"`
	Shared   bool     `yaml:"shared"`
	Runtimes []string `yaml:"runtimes"`
}

// MCPServerConfig defines a project-governed MCP server.
type MCPServerConfig struct {
	Type     string   `yaml:"type"`
	Command  string   `yaml:"command"`
	Args     []string `yaml:"args"`
	URL      string   `yaml:"url"`
	Shared   bool     `yaml:"shared"`
	Runtimes []string `yaml:"runtimes"`
}

// RoleBindingConfig binds project resources to a role.
type RoleBindingConfig struct {
	Skills     []string `yaml:"skills"`
	MCPServers []string `yaml:"mcp_servers"`
}

// PolicyFile models .aom/policy.yaml.
type PolicyFile struct {
	Policy PolicyConfig `yaml:"policy"`
}

// PolicyConfig defines project policy defaults and controls.
type PolicyConfig struct {
	DenyCommands    []string              `yaml:"deny_commands"`
	RequireApproval []string              `yaml:"require_approval"`
	SessionDefaults SessionDefaultsConfig `yaml:"session_defaults"`
	OwnerExceptions OwnerExceptionsConfig `yaml:"owner_exceptions"`
}

// SessionDefaultsConfig defines session-scoped defaults.
type SessionDefaultsConfig struct {
	ApprovalScope string `yaml:"approval_scope"`
	YoloMode      string `yaml:"yolo_mode"`
}

// OwnerExceptionsConfig defines owner exception policy.
type OwnerExceptionsConfig struct {
	Enabled     bool `yaml:"enabled"`
	LogRequired bool `yaml:"log_required"`
}

// findProjectRoot walks up from startPath until it finds a directory containing
// an .aom/project.yaml file. Returns the directory path or an error if not found.
// This mirrors git's behaviour of finding .git/ from within any subdirectory,
// which allows agents running inside worktrees to call AOM commands without
// needing to know the project root explicitly.
// FindProjectRoot is the exported form of findProjectRoot, for use by CLI
// commands that need the project root without opening the database.
func FindProjectRoot(startPath string) (string, error) {
	return findProjectRoot(startPath)
}

func findProjectRoot(startPath string) (string, error) {
	current, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("resolve start path: %w", err)
	}
	for {
		candidate := filepath.Join(current, aomDirName, "project.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", fmt.Errorf("no AOM project found at or above %q", startPath)
}

// LoadProjectConfig loads and validates the AOM config set rooted at projectRoot.
// If projectRoot does not directly contain .aom/project.yaml, it walks up the
// directory tree to find the nearest ancestor that does. This allows callers
// running inside a git worktree to discover the project without knowing the
// absolute project root.
func LoadProjectConfig(projectRoot string) (*ProjectConfig, error) {
	rootPath, err := findProjectRoot(projectRoot)
	if err != nil {
		return nil, err
	}

	cfg := &ProjectConfig{
		RootPath: rootPath,
		AOMPath:  filepath.Join(rootPath, aomDirName),
	}

	if err := loadYAML(filepath.Join(cfg.AOMPath, "project.yaml"), &cfg.Project); err != nil {
		return nil, fmt.Errorf("load project.yaml: %w", err)
	}
	if err := loadYAML(filepath.Join(cfg.AOMPath, "agents.yaml"), &cfg.Agents); err != nil {
		return nil, fmt.Errorf("load agents.yaml: %w", err)
	}
	if err := loadYAML(filepath.Join(cfg.AOMPath, "resources.yaml"), &cfg.Resources); err != nil {
		return nil, fmt.Errorf("load resources.yaml: %w", err)
	}
	if err := loadYAML(filepath.Join(cfg.AOMPath, "policy.yaml"), &cfg.Policy); err != nil {
		return nil, fmt.Errorf("load policy.yaml: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks the full loaded project configuration set.
func (c *ProjectConfig) Validate() error {
	if err := c.validateProject(); err != nil {
		return err
	}
	if err := c.validateAgents(); err != nil {
		return err
	}
	if err := c.validateResources(); err != nil {
		return err
	}
	if err := c.validatePolicy(); err != nil {
		return err
	}
	return nil
}

func (c *ProjectConfig) validateProject() error {
	if strings.TrimSpace(c.Project.Name) == "" {
		return fmt.Errorf("validate project config: name is required")
	}
	if strings.TrimSpace(c.Project.Repo) == "" {
		return fmt.Errorf("validate project config: repo is required")
	}

	repoPath := c.Project.Repo
	if !filepath.IsAbs(repoPath) {
		repoPath = filepath.Join(c.RootPath, repoPath)
	}
	if _, err := os.Stat(repoPath); err != nil {
		return fmt.Errorf("validate project config: repo path %q: %w", c.Project.Repo, err)
	}
	if strings.TrimSpace(c.Project.DefaultBranch) == "" {
		return fmt.Errorf("validate project config: default_branch is required")
	}
	if c.Project.Runtime.Terminal != "tmux" {
		return fmt.Errorf("validate project config: runtime.terminal must be %q", "tmux")
	}
	if strings.TrimSpace(c.Project.Runtime.SessionPrefix) == "" {
		return fmt.Errorf("validate project config: runtime.session_prefix is required")
	}
	if strings.TrimSpace(c.Project.Context.StateDir) == "" {
		return fmt.Errorf("validate project config: context.state_dir is required")
	}

	return nil
}

func (c *ProjectConfig) validateAgents() error {
	for roleName, role := range c.Agents.Roles {
		if strings.TrimSpace(role.Class) == "" {
			return fmt.Errorf("validate agents config: role %q class is required", roleName)
		}
		if err := requireKnown(role.WorktreeMode, knownWorktreeModes); err != nil {
			return fmt.Errorf("validate agents config: role %q worktree_mode: %w", roleName, err)
		}
		if err := requireKnown(role.CheckpointExpectation, knownCheckpointExpectations); err != nil {
			return fmt.Errorf("validate agents config: role %q checkpoint_expectation: %w", roleName, err)
		}
		if err := requireKnown(role.DefaultSessionMode, knownSessionModes); err != nil {
			return fmt.Errorf("validate agents config: role %q default_session_mode: %w", roleName, err)
		}
	}

	for agentName, agent := range c.Agents.Agents {
		if _, ok := c.Agents.Roles[agent.Role]; !ok {
			return fmt.Errorf("validate agents config: agent %q references unknown role %q", agentName, agent.Role)
		}
		if err := requireKnown(agent.Runtime, knownRuntimes); err != nil {
			return fmt.Errorf("validate agents config: agent %q runtime: %w", agentName, err)
		}
	}

	return nil
}

func (c *ProjectConfig) validateResources() error {
	for skillName, skill := range c.Resources.Skills {
		if strings.TrimSpace(skill.Path) == "" {
			return fmt.Errorf("validate resources config: skill %q path is required", skillName)
		}
		if err := validateProjectRelativePath(skill.Path); err != nil {
			return fmt.Errorf("validate resources config: skill %q path: %w", skillName, err)
		}
		if err := validateRuntimes(skill.Runtimes); err != nil {
			return fmt.Errorf("validate resources config: skill %q runtimes: %w", skillName, err)
		}
	}

	for serverName, server := range c.Resources.MCPServers {
		switch server.Type {
		case "stdio":
			if strings.TrimSpace(server.Command) == "" {
				return fmt.Errorf("validate resources config: mcp server %q command is required for stdio", serverName)
			}
		case "http":
			if strings.TrimSpace(server.URL) == "" {
				return fmt.Errorf("validate resources config: mcp server %q url is required for http", serverName)
			}
		default:
			return fmt.Errorf("validate resources config: mcp server %q type %q is not supported", serverName, server.Type)
		}

		if err := validateRuntimes(server.Runtimes); err != nil {
			return fmt.Errorf("validate resources config: mcp server %q runtimes: %w", serverName, err)
		}
	}

	for roleName, binding := range c.Resources.RoleBindings {
		if _, ok := c.Agents.Roles[roleName]; !ok {
			return fmt.Errorf("validate resources config: role binding references unknown role %q", roleName)
		}
		for _, skillName := range binding.Skills {
			if _, ok := c.Resources.Skills[skillName]; !ok {
				return fmt.Errorf("validate resources config: role %q references unknown skill %q", roleName, skillName)
			}
		}
		for _, serverName := range binding.MCPServers {
			if _, ok := c.Resources.MCPServers[serverName]; !ok {
				return fmt.Errorf("validate resources config: role %q references unknown mcp server %q", roleName, serverName)
			}
		}
	}

	return nil
}

func (c *ProjectConfig) validatePolicy() error {
	if c.Policy.Policy.SessionDefaults.ApprovalScope != "per-session" {
		return fmt.Errorf("validate policy config: session_defaults.approval_scope must be %q", "per-session")
	}

	switch c.Policy.Policy.SessionDefaults.YoloMode {
	case "enabled", "disabled":
	default:
		return fmt.Errorf("validate policy config: session_defaults.yolo_mode must be %q or %q", "enabled", "disabled")
	}

	return nil
}

func loadYAML(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, target); err != nil {
		return err
	}

	return nil
}

func requireKnown(value string, allowed map[string]struct{}) error {
	if _, ok := allowed[value]; !ok {
		return fmt.Errorf("%q is not supported", value)
	}

	return nil
}

func validateRuntimes(runtimes []string) error {
	if len(runtimes) == 0 {
		return fmt.Errorf("at least one runtime is required")
	}

	for _, runtime := range runtimes {
		if err := requireKnown(runtime, knownRuntimes); err != nil {
			return err
		}
	}

	return nil
}

// ResolvedSkill is a skill with its name resolved from the map key.
type ResolvedSkill struct {
	Name string
	SkillConfig
}

// ResolvedMCPServer is an MCP server config with its name resolved from the map key.
type ResolvedMCPServer struct {
	Name string
	MCPServerConfig
}

// RoleResources holds the concrete skills and MCP servers bound to one role+runtime pair.
type RoleResources struct {
	Skills     []ResolvedSkill
	MCPServers []ResolvedMCPServer
}

// ResourcesForRole resolves the skills and MCP servers bound to roleName,
// filtered to those compatible with runtimeName. Returns empty RoleResources
// when there is no binding for the role or when Resources is empty.
func (f *ResourcesFile) ResourcesForRole(roleName, runtimeName string) RoleResources {
	binding, ok := f.RoleBindings[roleName]
	if !ok {
		return RoleResources{}
	}

	var skills []ResolvedSkill
	for _, skillName := range binding.Skills {
		skill, ok := f.Skills[skillName]
		if !ok {
			continue
		}
		if !runtimeInList(skill.Runtimes, runtimeName) {
			continue
		}
		skills = append(skills, ResolvedSkill{Name: skillName, SkillConfig: skill})
	}

	var mcpServers []ResolvedMCPServer
	for _, serverName := range binding.MCPServers {
		server, ok := f.MCPServers[serverName]
		if !ok {
			continue
		}
		if !runtimeInList(server.Runtimes, runtimeName) {
			continue
		}
		mcpServers = append(mcpServers, ResolvedMCPServer{Name: serverName, MCPServerConfig: server})
	}

	return RoleResources{Skills: skills, MCPServers: mcpServers}
}

func runtimeInList(runtimes []string, target string) bool {
	for _, r := range runtimes {
		if r == target {
			return true
		}
	}
	return false
}

func validateProjectRelativePath(path string) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute paths are not allowed")
	}

	cleaned := filepath.Clean(path)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path must stay inside the project")
	}

	return nil
}
