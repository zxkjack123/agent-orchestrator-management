package cli

import (
	"fmt"
	"strings"
	"time"
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
		default:
			msgParts = append(msgParts, args[i])
		}
	}

	message := strings.TrimSpace(strings.Join(msgParts, " "))
	if message == "" {
		return fmt.Errorf("message is required")
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	if err := appendChannelMessage(result.Project.RepoPath, agentName, message, time.Now()); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Message appended to channel")
	fmt.Fprintf(r.stdout, "Agent: %s\n", agentName)
	fmt.Fprintf(r.stdout, "Message: %s\n", message)
	fmt.Fprintf(r.stdout, "Channel: %s\n", channelFilePath(result.Project.RepoPath))
	return nil
}

func (r Runner) executeChannelRead(args []string) error {
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	content, err := readChannelFile(result.Project.RepoPath)
	if err != nil {
		return err
	}

	if content == "" {
		fmt.Fprintln(r.stdout, "Channel is empty")
		fmt.Fprintf(r.stdout, "Channel: %s\n", channelFilePath(result.Project.RepoPath))
		return nil
	}

	fmt.Fprint(r.stdout, content)
	return nil
}
