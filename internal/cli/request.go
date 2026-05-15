package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RequestRecord holds a parsed task request.
type RequestRecord struct {
	ID          string
	Title       string
	RequestedBy string // "session-id / agent-name"
	ParentTask  string
	Priority    string
	Status      string // pending | approved | rejected
	Reason      string
}

func requestsDir(repoPath string) string {
	return filepath.Join(repoPath, ".aom", "requests")
}

func requestFilePath(repoPath, requestID string) string {
	return filepath.Join(requestsDir(repoPath), requestID+".md")
}

func writeRequestArtifact(repoPath string, rec RequestRecord) error {
	if err := os.MkdirAll(requestsDir(repoPath), 0o755); err != nil {
		return fmt.Errorf("create requests dir: %w", err)
	}

	content := fmt.Sprintf(`# Task Request: %s
- ID: %s
- Requested by: %s
- Parent task: %s
- Priority: %s
- Status: %s
- Reason: %s
`,
		rec.Title,
		rec.ID,
		emptyFallback(rec.RequestedBy),
		emptyFallback(rec.ParentTask),
		emptyFallback(rec.Priority),
		rec.Status,
		emptyFallback(rec.Reason),
	)

	return os.WriteFile(requestFilePath(repoPath, rec.ID), []byte(content), 0o644)
}

// readPendingRequests returns all request records in .aom/requests/ with status "pending".
func readPendingRequests(repoPath string) ([]RequestRecord, error) {
	dir := requestsDir(repoPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read requests dir: %w", err)
	}

	var records []RequestRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || strings.HasSuffix(e.Name(), ".archive.md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		rec := parseRequestArtifact(string(data))
		if rec.Status == "pending" {
			records = append(records, rec)
		}
	}

	return records, nil
}

// readAllRequests returns every request regardless of status.
func readAllRequests(repoPath string) ([]RequestRecord, error) {
	dir := requestsDir(repoPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read requests dir: %w", err)
	}

	var records []RequestRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || strings.HasSuffix(e.Name(), ".archive.md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		records = append(records, parseRequestArtifact(string(data)))
	}

	return records, nil
}

func parseRequestArtifact(content string) RequestRecord {
	var rec RequestRecord
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "# Task Request: "):
			rec.Title = strings.TrimPrefix(line, "# Task Request: ")
		case strings.HasPrefix(line, "- ID: "):
			rec.ID = strings.TrimPrefix(line, "- ID: ")
		case strings.HasPrefix(line, "- Requested by: "):
			rec.RequestedBy = strings.TrimPrefix(line, "- Requested by: ")
		case strings.HasPrefix(line, "- Parent task: "):
			rec.ParentTask = strings.TrimPrefix(line, "- Parent task: ")
		case strings.HasPrefix(line, "- Priority: "):
			rec.Priority = strings.TrimPrefix(line, "- Priority: ")
		case strings.HasPrefix(line, "- Status: "):
			rec.Status = strings.TrimPrefix(line, "- Status: ")
		case strings.HasPrefix(line, "- Reason: "):
			rec.Reason = strings.TrimPrefix(line, "- Reason: ")
		}
	}
	return rec
}

func generateRequestID(now time.Time) string {
	return "REQ-" + strconv.FormatInt(now.UnixNano(), 10)
}
