package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/config"
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

