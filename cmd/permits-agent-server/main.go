// permits-agent-server hosts the public website, authenticated API and MCP.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BCSoftware-LLC/permits-agent/internal/platform"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) > 1 && os.Args[1] == "keygen" {
		if len(os.Args) != 3 {
			return fmt.Errorf("usage: permits-agent-server keygen TENANT")
		}
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		token := "pa_" + base64.RawURLEncoding.EncodeToString(b)
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"access_key": token, "credential": platform.Credential{Tenant: os.Args[2], TokenHash: platform.TokenHash(token), Scopes: []string{"research", "cases:read", "cases:write"}, MonthlyRequestLimit: 10000}})
	}
	addr := flag.String("listen", "127.0.0.1:8080", "HTTP listen address; use a TLS reverse proxy for public access")
	origin := flag.String("origin", "http://localhost:8080", "Canonical public HTTPS origin (localhost HTTP for development)")
	db := flag.String("database", "", "Required path to durable private case database")
	keys := flag.String("keys", "", "Required operator-managed JSON credential file (hashes only)")
	flag.Parse()
	if *db == "" || *keys == "" {
		return fmt.Errorf("--database and --keys are required; see docs/hosting.md")
	}
	raw, err := os.ReadFile(*keys)
	if err != nil {
		return fmt.Errorf("cannot read credentials file")
	}
	var credentials []platform.Credential
	if err = json.Unmarshal(raw, &credentials); err != nil {
		return fmt.Errorf("invalid credentials file")
	}
	store, err := platform.Open(*db)
	if err != nil {
		return err
	}
	defer store.Close()
	var agent *platform.AgentConfig
	if os.Getenv("PERMITS_AGENT_MODEL") != "" {
		endpoint := os.Getenv("PERMITS_AGENT_RESPONSES_URL")
		if endpoint == "" {
			endpoint = "https://api.openai.com/v1/responses"
		}
		agent = &platform.AgentConfig{Endpoint: endpoint, APIKey: os.Getenv("OPENAI_API_KEY"), Model: os.Getenv("PERMITS_AGENT_MODEL"), GatewayToken: os.Getenv("CF_AIG_TOKEN")}
	}
	app, err := platform.New(platform.Config{Origin: *origin, Credentials: credentials, Store: store, Agent: agent})
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: *addr, Handler: app, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { slog.Info("Permits Agent listening", "address", *addr); done <- srv.ListenAndServe() }()
	select {
	case err = <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}
