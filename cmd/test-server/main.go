// Command test-server is a stateful mock of the Avalara API for connector tests.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", ":8080", "listen address")
	username := flag.String("username", "testuser", "accepted Basic auth username")
	password := flag.String("password", "testpass", "accepted Basic auth password")
	fixture := flag.String("fixture", "", "JSON file with roles and users")
	users := flag.Int("users", 0, "generate N users spread across all roles (replaces fixture users)")
	latency := flag.Duration("latency", 0, "delay added to every API response")
	ping401 := flag.Bool("ping-401", false, "return 401 for wrong credentials on ping instead of 200 with authenticated=false")
	adminRemote := flag.Bool("admin-allow-remote", false, "allow non-loopback clients on /_mock/")
	flag.Parse()

	state := defaultState()
	if *fixture != "" {
		data, err := os.ReadFile(*fixture)
		if err != nil {
			return fmt.Errorf("read fixture: %w", err)
		}
		if state, err = decodeState(data); err != nil {
			return fmt.Errorf("parse fixture: %w", err)
		}
	}
	if *users > 0 {
		state.Users = generateUsers(*users, state.Roles)
	}

	cfg := config{Username: *username, Password: *password, Latency: *latency, Ping401: *ping401, AdminRemote: *adminRemote}
	log.Printf("test server listening on %s (%d users, %d roles)", *addr, len(state.Users), len(state.Roles))
	srv := &http.Server{
		Addr:              *addr,
		Handler:           newHandler(cfg, state),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}
