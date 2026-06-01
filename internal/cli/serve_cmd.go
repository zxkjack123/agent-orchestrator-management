package cli

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/server"
)

func (r Runner) executeServe(args []string) error {
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
