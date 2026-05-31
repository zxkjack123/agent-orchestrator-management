package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// worktreeContextOf returns the worktree root path when cwd is inside a git
// worktree managed by AOM (i.e. under <project>/.aom/worktrees/<task>/), or
// an empty string when cwd is outside any worktree.
func worktreeContextOf(repoPath, cwd string) string {
	worktreesDir := filepath.Join(repoPath, ".aom", "worktrees")
	rel, err := filepath.Rel(worktreesDir, cwd)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	// rel is like "task-XXX/some/sub/dir" — the first segment is the worktree root.
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	return filepath.Join(worktreesDir, parts[0])
}

func outboxFilePath(worktreeRoot string) string {
	return filepath.Join(worktreeRoot, ".agent", "outbox.md")
}

// appendOutboxChannel stages a channel message in the worktree outbox.
func appendOutboxChannel(worktreeRoot, agentName, message string, now time.Time) error {
	header := fmt.Sprintf("### %s | OUTBOX | channel | %s", now.Format(time.RFC3339), agentName)
	return writeOutboxEntry(worktreeRoot, header, message)
}

// appendOutboxMailbox stages a mailbox message in the worktree outbox.
func appendOutboxMailbox(worktreeRoot, toAgent, fromAgent, message string, now time.Time) error {
	header := fmt.Sprintf("### %s | OUTBOX | mailbox | %s | %s", now.Format(time.RFC3339), toAgent, fromAgent)
	return writeOutboxEntry(worktreeRoot, header, message)
}

func writeOutboxEntry(worktreeRoot, header, message string) error {
	path := outboxFilePath(worktreeRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create outbox dir: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open outbox: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%s\n%s\n\n", header, message); err != nil {
		return fmt.Errorf("write outbox entry: %w", err)
	}
	return nil
}

type outboxEntry struct {
	timestamp time.Time
	kind      string // "channel" or "mailbox"
	target    string // mailbox destination agent (empty for channel)
	agent     string // sender
	message   string
}

func parseOutboxFile(path string) ([]outboxEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read outbox %q: %w", path, err)
	}

	var entries []outboxEntry
	lines := strings.Split(string(data), "\n")
	i := 0
	for i < len(lines) {
		line := lines[i]
		if !strings.HasPrefix(line, "### ") {
			i++
			continue
		}
		parts := strings.Split(line[4:], " | ")
		if len(parts) < 4 || strings.TrimSpace(parts[1]) != "OUTBOX" {
			i++
			continue
		}
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[0]))
		if err != nil {
			i++
			continue
		}
		kind := strings.TrimSpace(parts[2])
		var target, agent string
		switch kind {
		case "channel":
			if len(parts) < 4 {
				i++
				continue
			}
			agent = strings.TrimSpace(parts[3])
		case "mailbox":
			if len(parts) < 5 {
				i++
				continue
			}
			target = strings.TrimSpace(parts[3])
			agent = strings.TrimSpace(parts[4])
		default:
			i++
			continue
		}

		// Collect body until next entry or EOF.
		i++
		var bodyLines []string
		for i < len(lines) && !strings.HasPrefix(lines[i], "### ") {
			bodyLines = append(bodyLines, lines[i])
			i++
		}
		body := strings.TrimRight(strings.Join(bodyLines, "\n"), "\n ")

		entries = append(entries, outboxEntry{
			timestamp: ts,
			kind:      kind,
			target:    target,
			agent:     agent,
			message:   body,
		})
	}
	return entries, nil
}

// outboxRoots collects all directories that may contain an .agent/outbox.md:
// task worktrees (.aom/worktrees/<task>/) and agent workspaces (.aom/agents/<name>/workspace/).
func outboxRoots(repoPath string) []string {
	var roots []string
	for _, subdir := range []string{
		filepath.Join(repoPath, ".aom", "worktrees"),
		filepath.Join(repoPath, ".aom", "agents"),
	} {
		entries, err := os.ReadDir(subdir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			candidate := filepath.Join(subdir, e.Name())
			// Agent workspace dirs have a nested "workspace/" subdirectory.
			ws := filepath.Join(candidate, "workspace")
			if info, err := os.Stat(ws); err == nil && info.IsDir() {
				roots = append(roots, ws)
			} else {
				// Task worktree — root is the directory itself.
				roots = append(roots, candidate)
			}
		}
	}
	return roots
}

// flushAllOutboxes sweeps every worktree and agent workspace, routing all
// pending messages into the shared channel/mailbox. Returns the total flushed.
func flushAllOutboxes(repoPath string) (int, error) {
	total := 0
	for _, root := range outboxRoots(repoPath) {
		n, err := flushWorktreeOutbox(repoPath, root)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// countPendingOutboxMessages returns the total staged outbox entries across all
// worktrees and agent workspaces without modifying any files.
func countPendingOutboxMessages(repoPath string) int {
	total := 0
	for _, root := range outboxRoots(repoPath) {
		msgs, err := parseOutboxFile(outboxFilePath(root))
		if err == nil {
			total += len(msgs)
		}
	}
	return total
}

// flushWorktreeOutbox routes all pending outbox entries from one worktree into
// the shared channel/mailbox and clears the outbox. Returns the number flushed.
func flushWorktreeOutbox(repoPath, worktreeRoot string) (int, error) {
	path := outboxFilePath(worktreeRoot)
	entries, err := parseOutboxFile(path)
	if err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return 0, nil
	}

	for _, e := range entries {
		switch e.kind {
		case "channel":
			if err := appendChannelMessage(repoPath, e.agent, e.message, e.timestamp); err != nil {
				return 0, fmt.Errorf("flush channel entry from %q: %w", e.agent, err)
			}
		case "mailbox":
			if err := appendMailboxMessage(repoPath, e.target, e.message, e.agent, e.timestamp); err != nil {
				return 0, fmt.Errorf("flush mailbox entry to %q: %w", e.target, err)
			}
		}
	}

	// Clear outbox after all entries are flushed successfully.
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		return 0, fmt.Errorf("clear outbox: %w", err)
	}
	return len(entries), nil
}
