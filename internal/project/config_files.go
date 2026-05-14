package project

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

//go:embed templates/project-init/*.tmpl
var projectInitTemplates embed.FS

type projectTemplateData struct {
	Name          string
	RepoPath      string
	DefaultBranch string
	SessionPrefix string
}

func writeConfigFiles(aomPath, name, repoPath, defaultBranch, sessionPrefix, templateDir string) error {
	data := projectTemplateData{
		Name:          name,
		RepoPath:      repoPath,
		DefaultBranch: defaultBranch,
		SessionPrefix: sessionPrefix,
	}

	files := map[string]string{
		"project.yaml":   "templates/project-init/project.yaml.tmpl",
		"agents.yaml":    "templates/project-init/agents.yaml.tmpl",
		"resources.yaml": "templates/project-init/resources.yaml.tmpl",
		"policy.yaml":    "templates/project-init/policy.yaml.tmpl",
	}

	for outputName, templatePath := range files {
		rendered, err := renderTemplate(templatePath, templateDir, data)
		if err != nil {
			return fmt.Errorf("render %s: %w", outputName, err)
		}

		path := filepath.Join(aomPath, outputName)
		if err := os.WriteFile(path, rendered, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outputName, err)
		}
	}

	if err := ensureRootGitignore(repoPath); err != nil {
		return err
	}

	return nil
}

func ensureRootGitignore(repoPath string) error {
	path := filepath.Join(repoPath, ".gitignore")
	const entry = ".agent/"

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read .gitignore: %w", err)
	}
	if strings.Contains(string(data), entry) {
		return nil
	}

	var content string
	if len(data) == 0 {
		content = entry + "\n"
	} else {
		content = string(data)
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += entry + "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}
	return nil
}

func renderTemplate(templatePath, templateDir string, data projectTemplateData) ([]byte, error) {
	source, err := readTemplateSource(templatePath, templateDir)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(filepath.Base(templatePath)).Parse(string(source))
	if err != nil {
		return nil, err
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return nil, err
	}

	return rendered.Bytes(), nil
}

func readTemplateSource(templatePath, templateDir string) ([]byte, error) {
	if templateDir == "" {
		return projectInitTemplates.ReadFile(templatePath)
	}

	customPath := filepath.Join(templateDir, filepath.Base(templatePath))
	data, err := os.ReadFile(customPath)
	if err != nil {
		return nil, fmt.Errorf("read custom template %q: %w", customPath, err)
	}

	return data, nil
}

func resolvePresetTemplateDir(name string) (string, error) {
	name = filepath.Clean(name)
	if name == "." || name == "" {
		return "", fmt.Errorf("template preset is required")
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve preset template path: runtime caller is unavailable")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	templateDir := filepath.Join(repoRoot, "templates", "project-init", name)
	info, err := os.Stat(templateDir)
	if err != nil {
		return "", fmt.Errorf("stat preset template dir %q: %w", templateDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("preset template dir %q is not a directory", templateDir)
	}

	return templateDir, nil
}
