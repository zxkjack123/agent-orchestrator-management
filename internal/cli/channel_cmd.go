package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/config"
)

func (r Runner) executeChannelAppend(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("message is required")
	}

	agentName := "operator"
	var msgParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			agentName = strings.TrimSpace(args[i])
		case "--message":
			i++
			if i >= len(args) {
				return fmt.Errorf("--message requires a value")
			}
			msgParts = append(msgParts, args[i])
		default:
			msgParts = append(msgParts, args[i])
		}
	}

	message := strings.TrimSpace(strings.Join(msgParts, " "))
	if message == "" {
		return fmt.Errorf("message is required")
	}

	// Use lightweight root discovery — no DB open required for a channel write.
	repoPath, err := config.FindProjectRoot(".")
	if err != nil {
		return err
	}

	if err := appendChannelMessage(repoPath, agentName, message, time.Now()); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Message appended to channel")
	fmt.Fprintf(r.stdout, "Agent: %s\n", agentName)
	fmt.Fprintf(r.stdout, "Message: %s\n", message)
	fmt.Fprintf(r.stdout, "Channel: %s\n", channelFilePath(repoPath))
	return nil
}

func (r Runner) executeChannelRead(args []string) error {
	repoPath, err := config.FindProjectRoot(".")
	if err != nil {
		return err
	}

	content, err := readChannelFile(repoPath)
	if err != nil {
		return err
	}

	if content == "" {
		fmt.Fprintln(r.stdout, "Channel is empty")
		fmt.Fprintf(r.stdout, "Channel: %s\n", channelFilePath(repoPath))
		return nil
	}

	fmt.Fprint(r.stdout, content)
	return nil
}
