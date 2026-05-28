package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/config"
)

func (r Runner) executeMessageSend(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: aom message send <agent-name> \"<message>\" [--from <sender>]")
	}

	agentName := strings.TrimSpace(args[0])
	message := strings.TrimSpace(args[1])
	fromSender := os.Getenv("AOM_ACTOR")
	if fromSender == "" {
		fromSender = "operator"
	}

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--from":
			i++
			if i >= len(args) {
				return fmt.Errorf("--from requires a value")
			}
			fromSender = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	// Lightweight root discovery — no DB open required for a mailbox write.
	repoPath, err := config.FindProjectRoot(".")
	if err != nil {
		return err
	}

	// Any sandboxed provider sets AOM_RUNTIME at launch. When set, the agent
	// cannot write outside the worktree, so messages are staged locally for flush.
	if os.Getenv("AOM_RUNTIME") != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}
		if wtRoot := worktreeContextOf(repoPath, cwd); wtRoot != "" {
			if err := appendOutboxMailbox(wtRoot, agentName, fromSender, message, time.Now()); err != nil {
				return err
			}
			fmt.Fprintf(r.stdout, "Message staged to outbox for %s (operator must run: aom outbox flush)\n", agentName)
			return nil
		}
	}

	if err := appendMailboxMessage(repoPath, agentName, message, fromSender, time.Now()); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Message sent to %s\n", agentName)
	return nil
}

func (r Runner) executeMessageRead(args []string) error {
	var agentName string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			agentName = strings.TrimSpace(args[i])
		default:
			if agentName == "" {
				agentName = strings.TrimSpace(args[i])
			} else {
				return fmt.Errorf("unknown flag %q", args[i])
			}
		}
	}

	if agentName == "" {
		return fmt.Errorf("agent name is required (--agent <name>)")
	}

	repoPath, err := config.FindProjectRoot(".")
	if err != nil {
		return err
	}

	content, err := readMailbox(repoPath, agentName)
	if err != nil {
		return err
	}

	if content == "" {
		fmt.Fprintf(r.stdout, "Mailbox for %s is empty.\n", agentName)
		return nil
	}

	fmt.Fprint(r.stdout, content)
	return nil
}

func (r Runner) executeMessageClear(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent name is required")
	}

	agentName := strings.TrimSpace(args[0])

	repoPath, err := config.FindProjectRoot(".")
	if err != nil {
		return err
	}

	if err := clearMailbox(repoPath, agentName); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Mailbox for %s cleared (archived).\n", agentName)
	return nil
}

func (r Runner) executeMessageWatch(args []string) error {
	agentName := ""
	timeoutStr := "30m"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			agentName = strings.TrimSpace(args[i])
		case "--timeout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--timeout requires a value")
			}
			timeoutStr = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if agentName == "" {
		return fmt.Errorf("--agent is required")
	}

	watchTimeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return fmt.Errorf("--timeout value %q is not a valid duration (e.g. 5m, 30m, 1h): %w", timeoutStr, err)
	}

	repoPath, err := config.FindProjectRoot(".")
	if err != nil {
		return err
	}

	mailboxPath := mailboxFilePath(repoPath, agentName)

	if _, err := os.Stat(mailboxPath); os.IsNotExist(err) {
		fmt.Fprintf(r.stdout, "No mailbox for %s yet — waiting...\n", agentName)
	}

	// Byte-offset polling: track current file size and print new ### entries.
	startData, _ := os.ReadFile(mailboxPath)
	lastOffset := len(startData)

	deadline := time.Now().Add(watchTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)

		data, err := os.ReadFile(mailboxPath)
		if err != nil || len(data) <= lastOffset {
			continue
		}

		newPart := string(data[lastOffset:])
		lastOffset = len(data)

		// Print each new entry (lines starting with ### are entry boundaries).
		for _, line := range strings.Split(newPart, "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				fmt.Fprintf(r.stdout, "[inbox] %s\n", trimmed)
			}
		}
	}

	fmt.Fprintf(r.stdout, "inbox watch timed out after %s\n", watchTimeout)
	return nil
}

func (r Runner) executeMessageReply(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: aom message reply <msg-id> \"<reply-text>\"")
	}

	msgID := strings.TrimSpace(args[0])
	replyText := strings.TrimSpace(args[1])

	repoPath, err := config.FindProjectRoot(".")
	if err != nil {
		return err
	}

	// Scan all mailbox files to find the entry with the matching MSG-id.
	mailboxDirPath := filepath.Join(repoPath, mailboxDir)
	entries, err := os.ReadDir(mailboxDirPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read mailbox dir: %w", err)
	}

	senderAgent := ""
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || strings.HasSuffix(entry.Name(), ".archive.md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(mailboxDirPath, entry.Name()))
		if err != nil {
			continue
		}
		if !strings.Contains(string(data), msgID) {
			continue
		}
		// Found the mailbox containing this MSG-id. Extract from: field.
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, msgID) && strings.HasPrefix(line, "### ") {
				// Entry header format: ### <time> | <msg-id> | from: <sender>
				parts := strings.SplitN(line, "| from: ", 2)
				if len(parts) == 2 {
					senderAgent = strings.TrimSpace(parts[1])
				}
				break
			}
		}
		if senderAgent != "" {
			break
		}
	}

	if senderAgent == "" {
		return fmt.Errorf("message %q not found in any mailbox", msgID)
	}

	selfName := os.Getenv("AOM_ACTOR")
	if selfName == "" {
		selfName = "operator"
	}

	if err := appendMailboxMessage(repoPath, senderAgent, "[reply to "+msgID+"] "+replyText, selfName, time.Now()); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Reply sent to %s\n", senderAgent)
	return nil
}

