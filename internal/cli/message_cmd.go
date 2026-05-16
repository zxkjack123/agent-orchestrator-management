package cli

import (
	"fmt"
	"os"
	"strings"
	"time"
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

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	if err := appendMailboxMessage(result.Project.RepoPath, agentName, message, fromSender, time.Now()); err != nil {
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

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	content, err := readMailbox(result.Project.RepoPath, agentName)
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

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	if err := clearMailbox(result.Project.RepoPath, agentName); err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Mailbox for %s cleared (archived).\n", agentName)
	return nil
}

