//go:build !windows

package provider

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// findProcessesByPath returns PIDs of processes whose command-line argument
// list contains path. Uses pgrep(1) which is available on macOS and Linux.
// Returns nil without error when pgrep finds no matching processes (exit 1).
func findProcessesByPath(path string) ([]int, error) {
	out, err := exec.Command("pgrep", "-f", path).Output()
	if err != nil {
		// pgrep exits 1 when no processes matched — treat as empty result.
		return nil, nil
	}
	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil || pid <= 1 {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func terminateProcess(pid int, force bool) {
	sig := syscall.Signal(syscall.SIGTERM)
	if force {
		sig = syscall.SIGKILL
	}
	_ = syscall.Kill(pid, sig)
}

func isProcessAlive(pid int) bool {
	// Signal 0 probes existence without sending a real signal.
	return syscall.Kill(pid, 0) == nil
}
