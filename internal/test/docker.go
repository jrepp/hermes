// Package test provides shared helpers for Hermes test suites.
package test

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// dockerProbeTimeout bounds the daemon reachability check. Container tests that
// find no daemon must fail fast: testcontainers otherwise blocks on its own
// retry loop until the package-level `go test` timeout expires.
const dockerProbeTimeout = 2 * time.Second

var (
	dockerOnce      sync.Once
	dockerReachable bool
)

// dockerSocketCandidates returns the endpoints to probe, most specific first.
//
// DOCKER_HOST wins when set. Otherwise we try the rootful socket and the
// per-user socket that Docker Desktop and Colima create on macOS.
func dockerSocketCandidates() []string {
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		return []string{host}
	}

	candidates := []string{"unix:///var/run/docker.sock"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			"unix://"+filepath.Join(home, ".docker", "run", "docker.sock"),
			"unix://"+filepath.Join(home, ".colima", "default", "docker.sock"),
		)
	}

	return candidates
}

// dialable reports whether endpoint accepts a connection within the probe
// timeout. Unparseable endpoints are treated as unreachable rather than fatal,
// so a malformed DOCKER_HOST skips tests instead of panicking the suite.
func dialable(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}

	network, address := u.Scheme, u.Host
	if network == "unix" {
		address = u.Path
	}
	if network == "" || address == "" {
		return false
	}

	conn, err := net.DialTimeout(network, address, dockerProbeTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()

	return true
}

// DockerAvailable reports whether a Docker daemon is reachable. The result is
// probed once per process.
func DockerAvailable() bool {
	dockerOnce.Do(func() {
		for _, endpoint := range dockerSocketCandidates() {
			if dialable(endpoint) {
				dockerReachable = true
				return
			}
		}
	})

	return dockerReachable
}

// RequireDocker skips the test when no Docker daemon is reachable.
//
// Container-backed tests must call this before touching testcontainers so that
// a developer without Docker running gets an immediate, explanatory skip.
func RequireDocker(tb testing.TB) {
	tb.Helper()

	if !DockerAvailable() {
		tb.Skipf(
			"skipping: no Docker daemon reachable (probed %v); start Docker or run `cd testing && docker compose up -d`",
			dockerSocketCandidates(),
		)
	}
}

// TCPReachable reports whether addr ("host:port") accepts a connection within
// the probe timeout.
func TCPReachable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, dockerProbeTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()

	return true
}

// RequireTCP skips the test when addr is not reachable.
//
// Use this for tests that talk to a service started outside the test process
// (docker compose, a local daemon). Client constructors for Kafka and similar
// protocols are lazy and succeed without a broker, so they cannot themselves
// serve as availability checks.
func RequireTCP(tb testing.TB, addr, service string) {
	tb.Helper()

	if !TCPReachable(addr) {
		tb.Skipf(
			"skipping: %s not reachable at %s; start it with `cd testing && docker compose up -d`",
			service, addr,
		)
	}
}
