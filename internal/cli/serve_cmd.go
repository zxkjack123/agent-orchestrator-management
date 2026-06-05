package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/server"
)

// servePIDFile returns the path to the PID file used by aom serve.
func servePIDFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".aom")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "serve.pid"), nil
}

type serveMeta struct {
	PID  int    `json:"pid"`
	Addr string `json:"addr"`
}

func writeServeMeta(pidFile string, meta serveMeta) {
	data, _ := json.Marshal(meta)
	_ = os.WriteFile(pidFile, data, 0644)
}

func readServeMeta(pidFile string) (serveMeta, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return serveMeta{}, err
	}
	var meta serveMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		// Legacy: plain PID number
		pid, perr := strconv.Atoi(strings.TrimSpace(string(data)))
		if perr != nil {
			return serveMeta{}, fmt.Errorf("invalid PID file")
		}
		return serveMeta{PID: pid}, nil
	}
	return meta, nil
}

func (r Runner) executeServe(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "stop":
			return r.executeServeStop()
		case "restart":
			return r.executeServeRestart(args[1:])
		}
	}
	return r.executeServeStart(args)
}

func (r Runner) executeServeStart(args []string) error {
	host := "localhost"
	port := "7777"

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			i++
			if i >= len(args) {
				return fmt.Errorf("--port requires a value")
			}
			port = strings.TrimSpace(args[i])
		case "--host":
			i++
			if i >= len(args) {
				return fmt.Errorf("--host requires a value")
			}
			host = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	pidFile, err := servePIDFile()
	if err != nil {
		return fmt.Errorf("pid file: %w", err)
	}

	// Write PID + addr so stop/restart can find this process.
	meta := serveMeta{PID: os.Getpid(), Addr: host + ":" + port}
	writeServeMeta(pidFile, meta)
	defer os.Remove(pidFile)

	srv, err := server.New(r.app)
	if err != nil {
		return fmt.Errorf("init server: %w", err)
	}

	addr := host + ":" + port
	fmt.Fprintf(r.stdout, "AOM Web Server listening on http://%s\n", addr)
	fmt.Fprintf(r.stdout, "UI:  http://%s\n", addr)
	fmt.Fprintf(r.stdout, "API: http://%s/api/v1/projects\n", addr)
	fmt.Fprintf(r.stdout, "(build frontend first: cd web && npm install && npm run build)\n")

	return http.ListenAndServe(addr, srv.Handler())
}

func (r Runner) executeServeStop() error {
	pidFile, err := servePIDFile()
	if err != nil {
		return err
	}
	meta, err := readServeMeta(pidFile)
	if err != nil {
		return fmt.Errorf("no server running (pid file not found)")
	}

	proc, err := os.FindProcess(meta.PID)
	if err != nil {
		_ = os.Remove(pidFile)
		return fmt.Errorf("process %d not found", meta.PID)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		_ = os.Remove(pidFile)
		return fmt.Errorf("stop server: %w", err)
	}

	// Wait up to 5s for process to exit.
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			break // process gone
		}
	}
	_ = os.Remove(pidFile)
	fmt.Fprintf(r.stdout, "Server stopped (pid %d)\n", meta.PID)
	return nil
}

func (r Runner) executeServeRestart(args []string) error {
	pidFile, _ := servePIDFile()
	meta, _ := readServeMeta(pidFile)

	// Stop existing server if running.
	if meta.PID > 0 {
		if proc, err := os.FindProcess(meta.PID); err == nil {
			_ = proc.Signal(syscall.SIGTERM)
			for i := 0; i < 10; i++ {
				time.Sleep(500 * time.Millisecond)
				if err := proc.Signal(syscall.Signal(0)); err != nil {
					break
				}
			}
		}
		_ = os.Remove(pidFile)
		fmt.Fprintf(r.stdout, "Stopped pid %d\n", meta.PID)
	}

	// Re-exec self as `aom serve [args...]`.
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	serveArgs := append([]string{"serve"}, args...)
	cmd := exec.Command(exe, serveArgs...)
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	cmd.Stdin = r.stdin
	return cmd.Run()
}
