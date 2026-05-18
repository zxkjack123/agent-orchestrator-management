package project

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/config"
	"gopkg.in/yaml.v3"
)

// AgentProfilePath returns the path to an agent's profile.md file.
func AgentProfilePath(aomPath, agentName string) string {
	return filepath.Join(aomPath, "agents", agentName, "profile.md")
}

// ReadAgentProfile returns the content of an agent's profile.md, or empty string if not found.
func ReadAgentProfile(aomPath, agentName string) (string, error) {
	data, err := os.ReadFile(AgentProfilePath(aomPath, agentName))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read agent profile %q: %w", agentName, err)
	}
	return string(data), nil
}

// WriteAgentProfile overwrites an agent's profile.md with new content.
func WriteAgentProfile(aomPath, agentName, content string) error {
	path := AgentProfilePath(aomPath, agentName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create agent profile dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write agent profile %q: %w", agentName, err)
	}
	return nil
}

// AddAgentParams describes the parameters for adding a new agent.
type AddAgentParams struct {
	Name    string
	Role    string
	Class   string // role class for profile template; defaults to "builder" when empty
	Runtime string
}

// AddAgentToConfig adds a new agent entry to agents.yaml and seeds its profile.
// If the role does not exist, it is auto-created. Class controls the profile template
// used; when empty, an existing role's class is preserved, or "builder" is used for new roles.
func AddAgentToConfig(aomPath string, params AddAgentParams) error {
	agentsPath := filepath.Join(aomPath, "agents.yaml")
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		return fmt.Errorf("read agents.yaml: %w", err)
	}

	var cfg config.AgentsFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse agents.yaml: %w", err)
	}
	if cfg.Agents == nil {
		cfg.Agents = make(map[string]config.AgentConfig)
	}
	if cfg.Roles == nil {
		cfg.Roles = make(map[string]config.RoleConfig)
	}

	if _, exists := cfg.Agents[params.Name]; exists {
		return fmt.Errorf("agent %q already exists in agents.yaml", params.Name)
	}

	if existing, exists := cfg.Roles[params.Role]; !exists {
		roleCfg := defaultInlineRoleConfig()
		if params.Class != "" {
			roleCfg.Class = params.Class
		}
		cfg.Roles[params.Role] = roleCfg
	} else if params.Class != "" && existing.Class != params.Class {
		existing.Class = params.Class
		cfg.Roles[params.Role] = existing
	}

	cfg.Agents[params.Name] = config.AgentConfig{
		Runtime: params.Runtime,
		Role:    params.Role,
		Enabled: true,
	}

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal agents.yaml: %w", err)
	}
	if err := os.WriteFile(agentsPath, out, 0o644); err != nil {
		return fmt.Errorf("write agents.yaml: %w", err)
	}

	roleCfg := cfg.Roles[params.Role]
	content, err := renderAgentProfile(params.Name, params.Role, params.Runtime, roleCfg.Class, "", aomPath)
	if err != nil {
		return fmt.Errorf("render agent profile %q: %w", params.Name, err)
	}
	return WriteAgentProfile(aomPath, params.Name, content)
}

// UpdateProfileSection replaces a named markdown section (## Heading) with new content.
// Returns the updated profile string. If the section is not found, it is appended.
func UpdateProfileSection(profile, section, newContent string) string {
	heading := "## " + section
	lines := strings.Split(profile, "\n")
	start := -1
	end := len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i
			continue
		}
		if start >= 0 && i > start && strings.HasPrefix(strings.TrimSpace(line), "## ") {
			end = i
			break
		}
	}

	newLines := strings.Split(strings.TrimRight(newContent, "\n"), "\n")
	replacement := append([]string{heading}, newLines...)
	replacement = append(replacement, "")

	if start < 0 {
		// Section not found — append it
		return strings.Join(append(lines, replacement...), "\n")
	}

	updated := make([]string, 0, len(lines))
	updated = append(updated, lines[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, lines[end:]...)
	return strings.Join(updated, "\n")
}

func seedAgentProfiles(aomPath string, cfg *config.ProjectConfig, templateDir string) error {
	if cfg == nil {
		return fmt.Errorf("project config is required")
	}

	for agentName, agentCfg := range cfg.Agents.Agents {
		roleCfg, ok := cfg.Agents.Roles[agentCfg.Role]
		if !ok {
			return fmt.Errorf("agent %q references unknown role %q", agentName, agentCfg.Role)
		}

		profilePath := filepath.Join(aomPath, "agents", agentName, "profile.md")
		if _, err := os.Stat(profilePath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat agent profile %q: %w", profilePath, err)
		}

		if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
			return fmt.Errorf("create agent profile dir %q: %w", filepath.Dir(profilePath), err)
		}

		content, err := renderAgentProfile(agentName, agentCfg.Role, agentCfg.Runtime, roleCfg.Class, templateDir, aomPath)
		if err != nil {
			return fmt.Errorf("render agent profile %q: %w", agentName, err)
		}
		if err := os.WriteFile(profilePath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write agent profile %q: %w", profilePath, err)
		}
	}

	return nil
}

// profileTemplateData holds the variables injected into base.md.tmpl.
type profileTemplateData struct {
	AgentName   string
	RoleName    string
	RuntimeName string
	RoleSection string
}

// renderAgentProfile renders a profile.md for an agent by composing the role-specific
// section template with the common base template.
//
// Lookup order for each template file:
//  1. templateDir/profiles/<file> — explicit override passed at init time
//  2. {aomPath}/templates/profiles/<file> — project-local override (for aom agent add)
//  3. Embedded default (templates/project-init/profiles/<file>)
func renderAgentProfile(agentName, roleName, runtimeName, roleClass, templateDir, aomPath string) (string, error) {
	roleSection, err := loadProfileSection(roleClass, templateDir, aomPath)
	if err != nil {
		return "", err
	}

	baseSrc, err := loadProfileTemplate("base.md.tmpl", templateDir, aomPath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("profile").Parse(string(baseSrc))
	if err != nil {
		return "", fmt.Errorf("parse base profile template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, profileTemplateData{
		AgentName:   agentName,
		RoleName:    roleName,
		RuntimeName: runtimeName,
		RoleSection: roleSection,
	}); err != nil {
		return "", fmt.Errorf("render profile for agent %q: %w", agentName, err)
	}
	return buf.String(), nil
}

// loadProfileSection loads the role-specific markdown section for a given role class.
// Falls back to default.md.tmpl when no class-specific file exists.
func loadProfileSection(roleClass, templateDir, aomPath string) (string, error) {
	fileName := strings.TrimSpace(strings.ToLower(roleClass)) + ".md.tmpl"
	data, err := loadProfileTemplate(fileName, templateDir, aomPath)
	if err != nil {
		// Class-specific file not found — use default.
		data, err = loadProfileTemplate("default.md.tmpl", templateDir, aomPath)
		if err != nil {
			return "", fmt.Errorf("load default profile section: %w", err)
		}
	}
	return string(data), nil
}

// loadProfileTemplate reads a profile template file, checking custom directories
// before falling back to the embedded defaults.
func loadProfileTemplate(fileName, templateDir, aomPath string) ([]byte, error) {
	// 1. Explicit templateDir (from project init --template-dir)
	if templateDir != "" {
		p := filepath.Join(templateDir, "profiles", fileName)
		if data, err := os.ReadFile(p); err == nil {
			return data, nil
		}
	}
	// 2. Project-local .aom/templates/profiles/ (used by aom agent add after init)
	if aomPath != "" {
		p := filepath.Join(aomPath, "templates", "profiles", fileName)
		if data, err := os.ReadFile(p); err == nil {
			return data, nil
		}
	}
	// 3. Embedded default.
	embeddedPath := "templates/project-init/profiles/" + fileName
	data, err := projectInitTemplates.ReadFile(embeddedPath)
	if err != nil {
		return nil, fmt.Errorf("profile template %q not found: %w", fileName, err)
	}
	return data, nil
}

