package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runHook executes .aom/hooks/<name>.sh if it exists. Args are passed as
// positional arguments to the script. Non-fatal: errors and non-zero exits
// are printed to stderr but never block the main AOM command.
// A 15-second timeout prevents a stalled hook from keeping its sh subprocess
// alive after aom exits.
func runHook(repoPath, hookName string, args ...string) {
	hookPath := filepath.Join(repoPath, ".aom", "hooks", hookName+".sh")
	if _, err := os.Stat(hookPath); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", append([]string{hookPath}, args...)...)
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

