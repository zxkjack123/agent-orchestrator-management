package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const mailboxDir = ".aom/mailbox"

func mailboxFilePath(repoPath, agentName string) string {
	return filepath.Join(repoPath, mailboxDir, agentName+".md")
}

func appendMailboxMessage(repoPath, agentName, message, fromSender string, now time.Time) error {
	path := mailboxFilePath(repoPath, agentName)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create mailbox dir: %w", err)
	}

	// Create file with header if it does not exist.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		header := fmt.Sprintf("# Mailbox: %s\n\n## Messages\n\n", agentName)
		if err := os.WriteFile(path, []byte(header), 0o644); err != nil {
			return fmt.Errorf("create mailbox file: %w", err)
		}
	}

	msgID := "MSG-" + strconv.FormatInt(now.UnixNano(), 10)
	entry := fmt.Sprintf("### %s | %s | from: %s\n%s\n\n",
		now.Format(time.RFC3339),
		msgID,
		fromSender,
		message,
	)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open mailbox file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("write mailbox entry: %w", err)
	}

	return nil
}

func readMailbox(repoPath, agentName string) (string, error) {
	data, err := os.ReadFile(mailboxFilePath(repoPath, agentName))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read mailbox for %q: %w", agentName, err)
	}
	return string(data), nil
}

func clearMailbox(repoPath, agentName string) error {
	path := mailboxFilePath(repoPath, agentName)
	archivePath := filepath.Join(repoPath, mailboxDir, agentName+".archive.md")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read mailbox for archive: %w", err)
	}

	// Append to archive (create if needed).
	f, err := os.OpenFile(archivePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open archive file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}

	// Reset active mailbox to empty header and advance cursor so the next
	// message watch starts from the fresh baseline, not from the stale end.
	header := fmt.Sprintf("# Mailbox: %s\n\n## Messages\n\n", agentName)
	if err := os.WriteFile(path, []byte(header), 0o644); err != nil {
		return err
	}
	writeMailboxCursor(repoPath, agentName, len(header))
	return nil
}

// cursorFilePath returns the path to the per-agent read-cursor file.
func cursorFilePath(repoPath, agentName string) string {
	return filepath.Join(repoPath, mailboxDir, agentName+".cursor")
}

// readMailboxCursor returns the last consumed byte offset for agentName, or -1
// if no cursor has been written yet (first run).
func readMailboxCursor(repoPath, agentName string) int {
	data, err := os.ReadFile(cursorFilePath(repoPath, agentName))
	if err != nil {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return -1
	}
	return n
}

// writeMailboxCursor persists offset as the next watch start position.
func writeMailboxCursor(repoPath, agentName string, offset int) {
	_ = os.WriteFile(cursorFilePath(repoPath, agentName), []byte(strconv.Itoa(offset)), 0o644)
}

// advanceSenderCursor snapshots the sender's current mailbox size so that a
// subsequent message watch starts after any messages that existed at send time.
// This prevents the race where team replies arrive before the watch call begins.
func advanceSenderCursor(repoPath, sender string) {
	data, _ := os.ReadFile(mailboxFilePath(repoPath, sender))
	writeMailboxCursor(repoPath, sender, len(data))
}

// unreadMessageCount returns the number of message entries in the mailbox.
func unreadMessageCount(repoPath, agentName string) int {
	data, err := os.ReadFile(mailboxFilePath(repoPath, agentName))
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "\n### ")
}

// sessionHealth holds health metrics for a session.
type sessionHealth struct {
	SessionID          string
	TaskID             string
	AgentName          string
	TimeSinceCheckpoint string
	CheckpointWarning  bool
	HandoffWarning     bool
}

// computeSessionHealth derives health metrics by reading the task log.
func computeSessionHealth(logPath string, sessionID string, now time.Time) sessionHealth {
	h := sessionHealth{SessionID: sessionID}

	data, err := os.ReadFile(logPath)
	if err != nil {
		h.CheckpointWarning = true
		h.TimeSinceCheckpoint = "no log"
		return h
	}

	content := string(data)
	var lastCheckpointAt time.Time
	var hasHandoff bool

	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "| checkpoint.created") {
			parts := strings.SplitN(strings.TrimPrefix(line, "### "), " | ", 2)
			if len(parts) > 0 {
				if t, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[0])); err == nil {
					lastCheckpointAt = t
				}
			}
		}
		if strings.Contains(line, "handoff.md") || strings.Contains(line, "handoff.prepared") {
			hasHandoff = true
		}
	}

	if lastCheckpointAt.IsZero() {
		h.TimeSinceCheckpoint = "never"
		h.CheckpointWarning = true
	} else {
		since := now.Sub(lastCheckpointAt)
		h.TimeSinceCheckpoint = formatDuration(since)
		if since > 2*time.Hour {
			h.CheckpointWarning = true
		}
	}

	h.HandoffWarning = !hasHandoff
	return h
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "< 1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}
