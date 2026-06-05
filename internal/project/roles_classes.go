package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/config"
	"gopkg.in/yaml.v3"
)

// RoleInfo describes a role defined in agents.yaml.
type RoleInfo struct {
	Name                  string   `json:"name"`
	Class                 string   `json:"class"`
	WorktreeMode          string   `json:"worktree_mode"`
	CheckpointExpectation string   `json:"checkpoint_expectation"`
	DefaultSessionMode    string   `json:"default_session_mode"`
	AgentsUsing           []string `json:"agents_using"`
	Description           string   `json:"description"`
}

// ClassSource describes where a class template originates.
type ClassSource string

const (
	ClassSourceBuiltin          ClassSource = "builtin"
	ClassSourceCustom           ClassSource = "custom"
	ClassSourceBuiltinOverridden ClassSource = "builtin-overridden"
)

// ClassInfo describes a class template.
type ClassInfo struct {
	Name        string      `json:"name"`
	Source      ClassSource `json:"source"`
	RolesUsing  []string    `json:"roles_using"`
	Description string      `json:"description"`
}

// parseClassDescription extracts the value of the first
// <!-- description: ... --> comment in a template file.
// Returns an empty string when no such comment is found.
func parseClassDescription(content string) string {
	const prefix = "<!-- description:"
	const suffix = "-->"
	idx := strings.Index(content, prefix)
	if idx < 0 {
		return ""
	}
	rest := content[idx+len(prefix):]
	end := strings.Index(rest, suffix)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

// ClassDetail is ClassInfo plus the editable template content.
type ClassDetail struct {
	ClassInfo
	Content     string `json:"content"`
	IsProtected bool   `json:"is_protected"` // true = builtin with no override
}

// RoleCreateParams holds parameters for creating a new role.
type RoleCreateParams struct {
	Class                 string
	WorktreeMode          string
	CheckpointExpectation string
	DefaultSessionMode    string
	Description           string
}

// RoleUpdateParams holds optional fields for updating a role.
type RoleUpdateParams struct {
	Class                 *string
	WorktreeMode          *string
	CheckpointExpectation *string
	DefaultSessionMode    *string
	Description           *string
}

// ListRoles returns all roles defined in agents.yaml, annotated with which agents use them.
func ListRoles(aomPath string) ([]RoleInfo, error) {
	af, err := readAgentsFile(aomPath)
	if err != nil {
		return nil, err
	}

	agentsByRole := make(map[string][]string)
	for name, ac := range af.Agents {
		agentsByRole[ac.Role] = append(agentsByRole[ac.Role], name)
	}
	for role := range agentsByRole {
		sort.Strings(agentsByRole[role])
	}

	roles := make([]RoleInfo, 0, len(af.Roles))
	for name, rc := range af.Roles {
		roles = append(roles, RoleInfo{
			Name:                  name,
			Class:                 rc.Class,
			WorktreeMode:          rc.WorktreeMode,
			CheckpointExpectation: rc.CheckpointExpectation,
			DefaultSessionMode:    rc.DefaultSessionMode,
			AgentsUsing:           agentsByRole[name],
			Description:           rc.Description,
		})
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].Name < roles[j].Name })
	return roles, nil
}

// GetRole returns a single role by name.
func GetRole(aomPath, name string) (RoleInfo, error) {
	roles, err := ListRoles(aomPath)
	if err != nil {
		return RoleInfo{}, err
	}
	for _, r := range roles {
		if r.Name == name {
			return r, nil
		}
	}
	return RoleInfo{}, fmt.Errorf("role %q not found", name)
}

// CreateRole adds a new role to agents.yaml.
func CreateRole(aomPath, name string, params RoleCreateParams) error {
	af, err := readAgentsFile(aomPath)
	if err != nil {
		return err
	}
	if af.Roles == nil {
		af.Roles = make(map[string]config.RoleConfig)
	}
	if _, exists := af.Roles[name]; exists {
		return fmt.Errorf("role %q already exists", name)
	}
	rc := config.RoleConfig{
		Class:                 params.Class,
		WorktreeMode:          params.WorktreeMode,
		CheckpointExpectation: params.CheckpointExpectation,
		DefaultSessionMode:    params.DefaultSessionMode,
		Description:           params.Description,
	}
	if rc.Class == "" {
		rc.Class = "generic"
	}
	if rc.WorktreeMode == "" {
		rc.WorktreeMode = "dedicated-writer"
	}
	if rc.CheckpointExpectation == "" {
		rc.CheckpointExpectation = "required"
	}
	if rc.DefaultSessionMode == "" {
		rc.DefaultSessionMode = "interactive"
	}
	af.Roles[name] = rc
	return saveAgentsFile(aomPath, af)
}

// UpdateRole updates an existing role in agents.yaml.
func UpdateRole(aomPath, name string, params RoleUpdateParams) error {
	af, err := readAgentsFile(aomPath)
	if err != nil {
		return err
	}
	rc, ok := af.Roles[name]
	if !ok {
		return fmt.Errorf("role %q not found", name)
	}
	if params.Class != nil {
		rc.Class = *params.Class
	}
	if params.WorktreeMode != nil {
		rc.WorktreeMode = *params.WorktreeMode
	}
	if params.CheckpointExpectation != nil {
		rc.CheckpointExpectation = *params.CheckpointExpectation
	}
	if params.DefaultSessionMode != nil {
		rc.DefaultSessionMode = *params.DefaultSessionMode
	}
	if params.Description != nil {
		rc.Description = *params.Description
	}
	af.Roles[name] = rc
	return saveAgentsFile(aomPath, af)
}

// DeleteRole removes a role from agents.yaml. Returns error if any agent uses it.
func DeleteRole(aomPath, name string) error {
	af, err := readAgentsFile(aomPath)
	if err != nil {
		return err
	}
	if _, ok := af.Roles[name]; !ok {
		return fmt.Errorf("role %q not found", name)
	}
	for agentName, ac := range af.Agents {
		if ac.Role == name {
			return fmt.Errorf("cannot delete role %q: agent %q is using it — remove or reassign the agent first", name, agentName)
		}
	}
	delete(af.Roles, name)
	return saveAgentsFile(aomPath, af)
}

// PreviewRoleProfile renders a full profile for a role without writing it to disk.
func PreviewRoleProfile(aomPath, roleName, runtime string) (string, error) {
	af, err := readAgentsFile(aomPath)
	if err != nil {
		return "", err
	}
	rc, ok := af.Roles[roleName]
	if !ok {
		return "", fmt.Errorf("role %q not found", roleName)
	}
	if runtime == "" {
		runtime = "claude"
	}
	return renderAgentProfile("<preview>", roleName, runtime, rc.Class, "", aomPath)
}

// builtinClassNames lists the class names available from the embedded templates
// (i.e. each file in templates/project-init/profiles/ minus base.md.tmpl and default.md.tmpl).
func builtinClassNames() []string {
	entries, err := fs.ReadDir(projectInitTemplates, "templates/project-init/profiles")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := strings.TrimSuffix(e.Name(), ".md.tmpl")
		if n == "base" || n == "default" || !strings.HasSuffix(e.Name(), ".md.tmpl") {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ListClasses returns all known class templates: built-in (embedded) and custom (project-level).
func ListClasses(aomPath string) ([]ClassInfo, error) {
	af, err := readAgentsFile(aomPath)
	if err != nil {
		return nil, err
	}

	rolesByClass := make(map[string][]string)
	for roleName, rc := range af.Roles {
		rolesByClass[rc.Class] = append(rolesByClass[rc.Class], roleName)
	}
	for cls := range rolesByClass {
		sort.Strings(rolesByClass[cls])
	}

	seen := make(map[string]bool)
	var out []ClassInfo

	// Built-in classes from embedded FS.
	for _, name := range builtinClassNames() {
		seen[name] = true
		source := ClassSourceBuiltin
		var desc string
		// Prefer project-level override for description if present.
		overridePath := filepath.Join(aomPath, "templates", "profiles", name+".md.tmpl")
		if overrideData, err2 := os.ReadFile(overridePath); err2 == nil {
			source = ClassSourceBuiltinOverridden
			desc = parseClassDescription(string(overrideData))
		}
		if desc == "" {
			if embData, err2 := projectInitTemplates.ReadFile("templates/project-init/profiles/" + name + ".md.tmpl"); err2 == nil {
				desc = parseClassDescription(string(embData))
			}
		}
		out = append(out, ClassInfo{
			Name:        name,
			Source:      source,
			RolesUsing:  rolesByClass[name],
			Description: desc,
		})
	}

	// Project-level custom class files.
	profilesDir := filepath.Join(aomPath, "templates", "profiles")
	entries, _ := os.ReadDir(profilesDir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md.tmpl") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md.tmpl")
		if seen[name] {
			continue // already listed as builtin-overridden
		}
		var desc string
		if data, err2 := os.ReadFile(filepath.Join(profilesDir, e.Name())); err2 == nil {
			desc = parseClassDescription(string(data))
		}
		out = append(out, ClassInfo{
			Name:        name,
			Source:      ClassSourceCustom,
			RolesUsing:  rolesByClass[name],
			Description: desc,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// GetClassTemplate returns the template content and metadata for a class.
func GetClassTemplate(aomPath, className string) (ClassDetail, error) {
	// Check for project-level override.
	customPath := filepath.Join(aomPath, "templates", "profiles", className+".md.tmpl")
	if data, err := os.ReadFile(customPath); err == nil {
		source := ClassSourceCustom
		if isBuiltinClass(className) {
			source = ClassSourceBuiltinOverridden
		}
		return ClassDetail{
			ClassInfo:   ClassInfo{Name: className, Source: source, Description: parseClassDescription(string(data))},
			Content:     string(data),
			IsProtected: false,
		}, nil
	}

	// Built-in embedded.
	if isBuiltinClass(className) {
		data, err := projectInitTemplates.ReadFile("templates/project-init/profiles/" + className + ".md.tmpl")
		if err != nil {
			return ClassDetail{}, fmt.Errorf("class %q not found", className)
		}
		return ClassDetail{
			ClassInfo:   ClassInfo{Name: className, Source: ClassSourceBuiltin, Description: parseClassDescription(string(data))},
			Content:     string(data),
			IsProtected: true,
		}, nil
	}

	return ClassDetail{}, fmt.Errorf("class %q not found", className)
}

// SetClassTemplate writes a custom class template to .aom/templates/profiles/<class>.md.tmpl.
// For built-in classes, this creates a project-level override.
func SetClassTemplate(aomPath, className, content string) error {
	dir := filepath.Join(aomPath, "templates", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create profiles template dir: %w", err)
	}
	dst := filepath.Join(dir, className+".md.tmpl")
	tmp, err := os.CreateTemp(dir, ".class-*.md.tmpl.tmp")
	if err != nil {
		return fmt.Errorf("create temp class template: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("write class template %q: %w", className, err)
	}
	return nil
}

// DeleteClassTemplate removes a project-level class template override.
// For built-in classes with an override, this reverts to the embedded default.
// For pure custom classes, this deletes the file entirely.
// Returns an error if the class is built-in with no project-level override.
func DeleteClassTemplate(aomPath, className string) error {
	customPath := filepath.Join(aomPath, "templates", "profiles", className+".md.tmpl")
	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		if isBuiltinClass(className) {
			return fmt.Errorf("class %q is a built-in class with no project override — nothing to delete", className)
		}
		return fmt.Errorf("class %q not found", className)
	}
	return os.Remove(customPath)
}

// GetSystemTemplate returns the content of base.md.tmpl (the AOM system protocol section).
// This is the read-only Zone A of every profile.
func GetSystemTemplate() (string, error) {
	data, err := projectInitTemplates.ReadFile("templates/project-init/profiles/base.md.tmpl")
	if err != nil {
		return "", fmt.Errorf("read system template: %w", err)
	}
	return string(data), nil
}

// PreviewClassProfile renders a full profile using the given class template,
// using placeholder values for agent name, role, and runtime.
func PreviewClassProfile(aomPath, className, roleName, agentName, runtime string) (string, error) {
	if agentName == "" {
		agentName = "<preview>"
	}
	if roleName == "" {
		roleName = className
	}
	if runtime == "" {
		runtime = "claude"
	}
	return renderAgentProfile(agentName, roleName, runtime, className, "", aomPath)
}

// isBuiltinClass reports whether the given class name has an embedded template.
func isBuiltinClass(className string) bool {
	for _, n := range builtinClassNames() {
		if n == className {
			return true
		}
	}
	return false
}

// readAgentsFile reads and parses .aom/agents.yaml.
func readAgentsFile(aomPath string) (config.AgentsFile, error) {
	data, err := os.ReadFile(filepath.Join(aomPath, "agents.yaml"))
	if err != nil {
		return config.AgentsFile{}, fmt.Errorf("read agents.yaml: %w", err)
	}
	var af config.AgentsFile
	if err := yaml.Unmarshal(data, &af); err != nil {
		return config.AgentsFile{}, fmt.Errorf("parse agents.yaml: %w", err)
	}
	if af.Roles == nil {
		af.Roles = make(map[string]config.RoleConfig)
	}
	if af.Agents == nil {
		af.Agents = make(map[string]config.AgentConfig)
	}
	return af, nil
}

// saveAgentsFile writes agents.yaml using the canonical marshaler.
func saveAgentsFile(aomPath string, af config.AgentsFile) error {
	out, err := marshalAgentsFile(af)
	if err != nil {
		return fmt.Errorf("marshal agents.yaml: %w", err)
	}
	return os.WriteFile(filepath.Join(aomPath, "agents.yaml"), out, 0o644)
}
