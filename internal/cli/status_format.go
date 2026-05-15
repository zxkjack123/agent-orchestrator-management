package cli

import (
	"io"
	"os"

	"github.com/mattn/go-isatty"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
)

// isTTYWriter reports whether w is a terminal that supports ANSI codes.
// Respects the NO_COLOR convention.
func isTTYWriter(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// colorize wraps text with an ANSI escape code only when w is a terminal.
func colorize(text, code string, w io.Writer) string {
	if !isTTYWriter(w) {
		return text
	}
	return code + text + ansiReset
}

// colorStatus colors a status value by semantic meaning.
func colorStatus(status string, w io.Writer) string {
	switch status {
	case "Working", "InProgress", "Active", "Ready", "Idle", "Completed":
		return colorize(status, ansiGreen, w)
	case "NeedsAttention", "Blocked", "WaitingApproval", "WaitingHandoff", "NeedsRepair":
		return colorize(status, ansiYellow, w)
	case "Failed":
		return colorize(status, ansiRed, w)
	case "Stopped", "Detached", "Archived", "Done", "Skipped", "Canceled":
		return colorize(status, ansiDim, w)
	default:
		return status
	}
}

// sectionLabel returns a visually distinct label that still contains the
// original name as a substring, preserving test assertions using Contains.
func sectionLabel(name string, w io.Writer) string {
	return colorize("── ", ansiBold, w) + name
}
