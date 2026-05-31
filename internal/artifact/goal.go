package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const goalFilename = "goal.json"

// GoalRecord is the persistent representation of the project goal.
type GoalRecord struct {
	Text   string    `json:"text"`
	Status string    `json:"status"` // "open" | "complete"
	SetAt  time.Time `json:"set_at"`
}

// GoalPath returns the absolute path to the goal file for a project.
func GoalPath(repoPath string) string {
	return filepath.Join(repoPath, ".aom", goalFilename)
}

// WriteGoalFile creates or overwrites the project goal.
func WriteGoalFile(repoPath, text string) (string, error) {
	r := GoalRecord{
		Text:   text,
		Status: "open",
		SetAt:  time.Now().UTC(),
	}
	return writeGoalRecord(repoPath, r)
}

// ReadGoalFile reads the current project goal. Returns an error if none is set.
func ReadGoalFile(repoPath string) (GoalRecord, error) {
	data, err := os.ReadFile(GoalPath(repoPath))
	if err != nil {
		return GoalRecord{}, fmt.Errorf("no goal set (run `aom goal set \"<text>\"` first): %w", err)
	}
	var r GoalRecord
	if err := json.Unmarshal(data, &r); err != nil {
		return GoalRecord{}, fmt.Errorf("parse goal file: %w", err)
	}
	return r, nil
}

// CompleteGoalFile marks the current project goal as complete.
func CompleteGoalFile(repoPath string) error {
	r, err := ReadGoalFile(repoPath)
	if err != nil {
		return err
	}
	r.Status = "complete"
	_, err = writeGoalRecord(repoPath, r)
	return err
}

func writeGoalRecord(repoPath string, r GoalRecord) (string, error) {
	p := GoalPath(repoPath)
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal goal: %w", err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return "", fmt.Errorf("write goal file: %w", err)
	}
	return p, nil
}
