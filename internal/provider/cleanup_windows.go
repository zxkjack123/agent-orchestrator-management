//go:build windows

package provider

func findProcessesByPath(_ string) ([]int, error) { return nil, nil }
func terminateProcess(_ int, _ bool)               {}
func isProcessAlive(_ int) bool                    { return false }
