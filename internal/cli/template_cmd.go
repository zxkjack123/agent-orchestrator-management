package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// taskTemplate defines a reusable task configuration preset.
type taskTemplate struct {
	Name        string
	Description string
	Mode        string
	Steps       []taskTemplateStep
}

// taskTemplateStep is one step entry within a task template.
type taskTemplateStep struct {
	StepType string
	Title    string
}

// builtinTaskTemplates is the set of shipped templates accessible via --template.
var builtinTaskTemplates = map[string]taskTemplate{
	"small-fix": {
		Name:        "small-fix",
		Description: "Targeted bug fix with narrow scope",
		Mode:        "Bugfix",
		Steps: []taskTemplateStep{
			{StepType: "investigate", Title: "Investigate root cause"},
			{StepType: "implement", Title: "Apply fix"},
		},
	},
	"feature-standard": {
		Name:        "feature-standard",
		Description: "Standard feature with design, implementation, and tests",
		Mode:        "Direct",
		Steps: []taskTemplateStep{
			{StepType: "design", Title: "Design approach"},
			{StepType: "implement", Title: "Implement feature"},
			{StepType: "test", Title: "Write and run tests"},
		},
	},
	"risky-change": {
		Name:        "risky-change",
		Description: "High-risk change with requirements, design, implementation, and explicit review",
		Mode:        "Requirements-first",
		Steps: []taskTemplateStep{
			{StepType: "requirements", Title: "Document requirements"},
			{StepType: "design", Title: "Design solution"},
			{StepType: "implement", Title: "Implement"},
			{StepType: "test", Title: "Test and verify"},
			{StepType: "review", Title: "Peer review"},
		},
	},
	"qa-pass": {
		Name:        "qa-pass",
		Description: "Quality assurance check and sign-off pass",
		Mode:        "Direct",
		Steps: []taskTemplateStep{
			{StepType: "qa-check", Title: "Run QA checks"},
			{StepType: "review", Title: "Sign off"},
		},
	},
	"research-spike": {
		Name:        "research-spike",
		Description: "Time-boxed research and exploration",
		Mode:        "Direct",
		Steps: []taskTemplateStep{
			{StepType: "research", Title: "Research and document findings"},
		},
	},
}

// builtinTemplateOrder preserves display ordering for aom task templates.
var builtinTemplateOrder = []string{
	"small-fix",
	"feature-standard",
	"risky-change",
	"qa-pass",
	"research-spike",
}

// customTemplateDir returns the path to the user-defined task template directory.
func customTemplateDir(repoPath string) string {
	return filepath.Join(repoPath, ".aom", "templates", "tasks")
}

// yamlTaskTemplate is the on-disk schema for custom YAML templates.
type yamlTaskTemplate struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Mode        string `yaml:"mode"`
	Steps       []struct {
		StepType string `yaml:"step_type"`
		Title    string `yaml:"title"`
	} `yaml:"steps"`
}

// loadCustomTaskTemplates reads *.yaml files from .aom/templates/tasks/.
func loadCustomTaskTemplates(repoPath string) []taskTemplate {
	dir := customTemplateDir(repoPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []taskTemplate
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var y yamlTaskTemplate
		if err := yaml.Unmarshal(data, &y); err != nil || y.Name == "" {
			continue
		}
		tpl := taskTemplate{
			Name:        y.Name,
			Description: y.Description,
			Mode:        y.Mode,
		}
		for _, s := range y.Steps {
			tpl.Steps = append(tpl.Steps, taskTemplateStep{StepType: s.StepType, Title: s.Title})
		}
		out = append(out, tpl)
	}
	return out
}

// executeTaskTemplates lists available built-in and custom task templates.
func (r Runner) executeTaskTemplates(_ []string) error {
	result, _ := r.app.Projects.Open(".")

	fmt.Fprintln(r.stdout, "Available task templates:")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "  %-20s  %-12s  %s\n", "NAME", "MODE", "DESCRIPTION")
	fmt.Fprintf(r.stdout, "  %-20s  %-12s  %s\n",
		strings.Repeat("-", 20), strings.Repeat("-", 12), strings.Repeat("-", 40))
	for _, name := range builtinTemplateOrder {
		tpl := builtinTaskTemplates[name]
		fmt.Fprintf(r.stdout, "  %-20s  %-12s  %s\n", tpl.Name, tpl.Mode, tpl.Description)
	}

	if result != nil {
		custom := loadCustomTaskTemplates(result.Project.RepoPath)
		for _, tpl := range custom {
			fmt.Fprintf(r.stdout, "  %-20s  %-12s  %s  (custom)\n", tpl.Name, tpl.Mode, tpl.Description)
		}
	}

	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "Usage: aom task create \"<title>\" --template <name>")
	if result != nil {
		fmt.Fprintf(r.stdout, "Custom templates: %s/*.yaml\n", customTemplateDir(result.Project.RepoPath))
	}
	return nil
}

// resolveTaskTemplate looks up a template by name: built-ins first, then custom YAML files.
func resolveTaskTemplate(name string) (*taskTemplate, error) {
	return resolveTaskTemplateWithRepo(name, "")
}

func resolveTaskTemplateWithRepo(name, repoPath string) (*taskTemplate, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if t, ok := builtinTaskTemplates[key]; ok {
		return &t, nil
	}
	if repoPath != "" {
		for _, tpl := range loadCustomTaskTemplates(repoPath) {
			if strings.ToLower(tpl.Name) == key {
				tplCopy := tpl
				return &tplCopy, nil
			}
		}
	}
	return nil, fmt.Errorf("unknown template %q — run 'aom task templates' to list available templates", name)
}
