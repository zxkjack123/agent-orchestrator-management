package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runHook executes .aom/hooks/<name>.sh if it exists. Args are passed as
// positional arguments to the script. Non-fatal: errors and non-zero exits
// are printed to stderr but never block the main AOM command.
func runHook(repoPath, hookName string, args ...string) {
	hookPath := filepath.Join(repoPath, ".aom", "hooks", hookName+".sh")
	if _, err := os.Stat(hookPath); err != nil {
		return
	}
	cmd := exec.Command("sh", append([]string{hookPath}, args...)...)
	cmd.Env = append(os.Environ(),
		"AOM_REPO="+repoPath,
		"AOM_HOOK="+hookName,
	)
	cmd.Dir = repoPath
	out, _ := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Fprintf(os.Stderr, "[hook: %s]\n%s\n", hookName, strings.TrimSpace(string(out)))
	}
}
