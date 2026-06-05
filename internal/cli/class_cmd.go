package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/project"
)

func (r Runner) executeClass(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("class subcommand is required: list, show, create, edit, override, delete, preview")
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			r.printHelp()
			return nil
		}
	}
	switch args[0] {
	case "list":
		return r.executeClassList(args[1:])
	case "show":
		return r.executeClassShow(args[1:])
	case "create":
		return r.executeClassCreate(args[1:])
	case "edit":
		return r.executeClassEdit(args[1:])
	case "override":
		return r.executeClassOverride(args[1:])
	case "delete":
		return r.executeClassDelete(args[1:])
	case "preview":
		return r.executeClassPreview(args[1:])
	default:
		return fmt.Errorf("unknown class subcommand %q — valid: list, show, create, edit, override, delete, preview", args[0])
	}
}

func (r Runner) executeClassList(_ []string) error {
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}
	classes, err := project.ListClasses(result.AOMPath)
	if err != nil {
		return err
	}
	fmt.Fprintln(r.stdout, "Classes")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "  %-20s %-22s %s\n", "NAME", "SOURCE", "ROLES USING")
	fmt.Fprintln(r.stdout, "  "+strings.Repeat("-", 70))
	for _, c := range classes {
		roles := strings.Join(c.RolesUsing, ", ")
		if roles == "" {
			roles = "-"
		}
		source := string(c.Source)
		fmt.Fprintf(r.stdout, "  %-20s %-22s %s\n", c.Name, source, roles)
		if c.Description != "" {
			fmt.Fprintf(r.stdout, "  %-20s %-22s %s\n", "", "", "↳ "+c.Description)
		}
	}
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, "  Sources: builtin = embedded default | custom = project-defined | builtin-overridden = builtin with project override")
	fmt.Fprintln(r.stdout, "  Custom templates: .aom/templates/profiles/<class>.md.tmpl")
	return nil
}

func (r Runner) executeClassShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aom class show <name>")
	}
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}
	detail, err := project.GetClassTemplate(result.AOMPath, args[0])
	if err != nil {
		return err
	}
	source := string(detail.Source)
	protected := ""
	if detail.IsProtected {
		protected = " (read-only — use 'aom class override " + args[0] + "' to create an editable copy)"
	}
	fmt.Fprintf(r.stdout, "Class: %s  [%s%s]\n\n", detail.Name, source, protected)
	fmt.Fprintln(r.stdout, detail.Content)
	return nil
}

func (r Runner) executeClassCreate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aom class create <name> [--from <existing-class>]")
	}
	name := args[0]
	fromClass := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--from" && i+1 < len(args) {
			fromClass = args[i+1]
			i++
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// Check name doesn't conflict with built-in.
	existing, err := project.GetClassTemplate(result.AOMPath, name)
	if err == nil && existing.Source == project.ClassSourceBuiltin {
		return fmt.Errorf("class %q is a built-in class — use 'aom class override %s' to create a project-level override", name, name)
	}
	if err == nil {
		return fmt.Errorf("class %q already exists (source: %s) — use 'aom class edit %s' to modify it", name, existing.Source, name)
	}

	// Seed content from --from or use a starter template.
	var content string
	if fromClass != "" {
		src, err2 := project.GetClassTemplate(result.AOMPath, fromClass)
		if err2 != nil {
			return fmt.Errorf("source class %q not found: %w", fromClass, err2)
		}
		content = src.Content
	} else {
		content = defaultClassTemplate(name)
	}

	if err := project.SetClassTemplate(result.AOMPath, name, content); err != nil {
		return err
	}
	customPath := filepath.Join(result.AOMPath, "templates", "profiles", name+".md.tmpl")
	fmt.Fprintf(r.stdout, "Class %q created: %s\n", name, customPath)
	fmt.Fprintf(r.stdout, "Next steps:\n")
	fmt.Fprintf(r.stdout, "  aom class edit %s                # open in editor\n", name)
	fmt.Fprintf(r.stdout, "  aom role create <role> --class %s  # create a role using this class\n", name)
	return nil
}

func (r Runner) executeClassOverride(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aom class override <built-in-name>")
	}
	name := args[0]
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}
	detail, err := project.GetClassTemplate(result.AOMPath, name)
	if err != nil {
		return fmt.Errorf("class %q not found: %w", name, err)
	}
	if detail.Source != project.ClassSourceBuiltin {
		return fmt.Errorf("class %q is already a %s class — edit it directly with 'aom class edit %s'", name, detail.Source, name)
	}
	if err := project.SetClassTemplate(result.AOMPath, name, detail.Content); err != nil {
		return err
	}
	customPath := filepath.Join(result.AOMPath, "templates", "profiles", name+".md.tmpl")
	fmt.Fprintf(r.stdout, "Override created: %s\n", customPath)
	fmt.Fprintf(r.stdout, "Built-in %q has been copied to a project-level override.\n", name)
	fmt.Fprintf(r.stdout, "Edit it with: aom class edit %s\n", name)
	fmt.Fprintf(r.stdout, "Revert to built-in: aom class delete %s\n", name)
	return nil
}

func (r Runner) executeClassEdit(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aom class edit <name>")
	}
	name := args[0]
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	detail, err := project.GetClassTemplate(result.AOMPath, name)
	if err != nil {
		return fmt.Errorf("class %q not found: %w", name, err)
	}
	if detail.IsProtected {
		return fmt.Errorf("class %q is a built-in class and cannot be edited directly.\nRun 'aom class override %s' first to create a project-level copy.", name, name)
	}

	// The file already exists at .aom/templates/profiles/<name>.md.tmpl.
	filePath := filepath.Join(result.AOMPath, "templates", "profiles", name+".md.tmpl")

	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, filePath)
	cmd.Stdin = r.stdin
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}
	fmt.Fprintf(r.stdout, "Class %q saved: %s\n", name, filePath)
	fmt.Fprintf(r.stdout, "Respawn agents using this class to apply changes.\n")
	return nil
}

func (r Runner) executeClassDelete(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aom class delete <name>")
	}
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}
	detail, err := project.GetClassTemplate(result.AOMPath, args[0])
	if err != nil {
		return fmt.Errorf("class %q not found", args[0])
	}
	if err := project.DeleteClassTemplate(result.AOMPath, args[0]); err != nil {
		return err
	}
	if detail.Source == project.ClassSourceBuiltinOverridden {
		fmt.Fprintf(r.stdout, "Project override for built-in class %q removed — reverted to embedded default.\n", args[0])
	} else {
		fmt.Fprintf(r.stdout, "Custom class %q deleted.\n", args[0])
	}
	return nil
}

func (r Runner) executeClassPreview(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aom class preview <name> [--runtime claude|codex] [--role <role>] [--agent <name>]")
	}
	className := args[0]
	runtime := "claude"
	roleName := ""
	agentName := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--runtime":
			if i+1 < len(args) {
				runtime = args[i+1]
				i++
			}
		case "--role":
			if i+1 < len(args) {
				roleName = args[i+1]
				i++
			}
		case "--agent":
			if i+1 < len(args) {
				agentName = args[i+1]
				i++
			}
		}
	}
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}
	rendered, err := project.PreviewClassProfile(result.AOMPath, className, roleName, agentName, runtime)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "--- Profile preview for class %q (runtime: %s) ---\n\n", className, runtime)
	fmt.Fprintln(r.stdout, rendered)
	return nil
}

// executeSystemTemplateShow shows the AOM system protocol template (base.md.tmpl).
func (r Runner) executeSystemTemplateShow(_ []string) error {
	content, err := project.GetSystemTemplate()
	if err != nil {
		return err
	}
	fmt.Fprintln(r.stdout, "--- AOM System Template (base.md.tmpl) — read-only ---")
	fmt.Fprintln(r.stdout, "This template is embedded in the AOM binary and injected into every agent profile.")
	fmt.Fprintln(r.stdout, "It covers: AOM Workflow, Team Communication, Collaboration Routines, Constraints.")
	fmt.Fprintln(r.stdout, "")
	fmt.Fprintln(r.stdout, content)
	return nil
}

// defaultClassTemplate returns a starter template for a new custom class.
func defaultClassTemplate(name string) string {
	title := strings.Title(strings.ReplaceAll(name, "-", " ")) //nolint:staticcheck
	return fmt.Sprintf(`## Responsibilities
- Complete the assigned task according to the task artifacts and current session state
- Deliver clear, well-structured output that the operator and teammates can act on
- Signal progress at each step and announce completion via the team channel

## Work Standards
- Read task.md and state.md before starting — understand scope and prior progress
- Keep output structured: use sections, bullet points, or tables as appropriate
- Flag ambiguities early — update state.md with open questions rather than guessing
- Deliver incremental results via the channel; do not wait until the full task is done

## %s-Specific Instructions
<!-- Add domain-specific responsibilities and standards for the %s class here -->

## Custom Instructions
<!-- Add project-specific or agent-specific instructions here. This section is managed by the operator and will not be overwritten by AOM system updates. -->
`, title, name)
}
