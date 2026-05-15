// Package merge provides git-based conflict detection between task branches.
package merge

import (
	"fmt"
	"os/exec"
	"strings"
)

// OverlapScore categorises the risk level of a planned merge.
type OverlapScore string

const (
	ScoreGreen  OverlapScore = "Green"
	ScoreYellow OverlapScore = "Yellow"
	ScoreRed    OverlapScore = "Red"
)

// FileOverlap describes a single file touched by both the source branch and
// another branch.
type FileOverlap struct {
	Path        string
	OtherBranch string
}

// CheckResult is the full output of a merge overlap check.
type CheckResult struct {
	SourceBranch  string
	TargetBranch  string
	Score         OverlapScore
	Overlaps      []FileOverlap
}

// CheckOverlaps compares the files changed between sourceBranch..base and
// otherBranch..base and returns any files modified in both.
// repoPath is the working directory for git commands.
func CheckOverlaps(repoPath, sourceBranch, otherBranch, base string) (*CheckResult, error) {
	sourceFiles, err := changedFiles(repoPath, base, sourceBranch)
	if err != nil {
		return nil, fmt.Errorf("list changed files for %q: %w", sourceBranch, err)
	}

	otherFiles, err := changedFiles(repoPath, base, otherBranch)
	if err != nil {
		return nil, fmt.Errorf("list changed files for %q: %w", otherBranch, err)
	}

	otherSet := make(map[string]bool, len(otherFiles))
	for _, f := range otherFiles {
		otherSet[f] = true
	}

	var overlaps []FileOverlap
	for _, f := range sourceFiles {
		if otherSet[f] {
			overlaps = append(overlaps, FileOverlap{
				Path:        f,
				OtherBranch: otherBranch,
			})
		}
	}

	result := &CheckResult{
		SourceBranch: sourceBranch,
		TargetBranch: base,
		Overlaps:     overlaps,
		Score:        score(len(overlaps)),
	}

	return result, nil
}

func changedFiles(repoPath, base, branch string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", base+".."+branch)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		// git exits non-zero when branch/ref does not exist; surface a cleaner error.
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git diff failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}

	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

func score(overlaps int) OverlapScore {
	switch {
	case overlaps == 0:
		return ScoreGreen
	case overlaps <= 3:
		return ScoreYellow
	default:
		return ScoreRed
	}
}
