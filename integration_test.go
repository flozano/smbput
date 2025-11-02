//go:build integration
// +build integration

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestSambaIntegration_ShowsSharesAndTransfersFiles(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	shareDir := t.TempDir()

	req := testcontainers.ContainerRequest{
		Image:        "dperson/samba",
		ExposedPorts: []string{"445/tcp"},
		WaitingFor:   wait.ForListeningPort("445/tcp").WithStartupTimeout(90 * time.Second),
		Cmd: []string{
			"-p",
			"-u", "testuser;testpass",
			"-s", "public;/srv/public;yes;no;yes;testuser",
		},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(shareDir, testcontainers.ContainerMountTarget("/srv/public")),
		),
	}

	sambaC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		if shouldSkipDocker(err) {
			t.Skipf("skipping integration test, Docker unavailable: %v", err)
		}
		t.Fatalf("failed to start samba container: %v", err)
	}
	defer func() {
		_ = sambaC.Terminate(context.Background())
	}()

	host, err := sambaC.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := sambaC.MappedPort(ctx, "445")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}

	opts := smbOptions{
		address:  fmt.Sprintf("%s:%s", host, port.Port()),
		share:    "public",
		user:     "testuser",
		password: "testpass",
		timeout:  15 * time.Second,
	}

	session, cleanup, err := dialSession(opts)
	if err != nil {
		t.Fatalf("dial session: %v", err)
	}
	defer cleanup()

	shares, err := session.ListSharenames()
	if err != nil {
		t.Fatalf("ListSharenames: %v", err)
	}
	if !containsShare(shares, "public") {
		t.Fatalf("expected share 'public' in %v", shares)
	}

	share, shareCleanup, err := connect(opts)
	if err != nil {
		t.Fatalf("connect share: %v", err)
	}
	defer shareCleanup()

	localTemp := t.TempDir()
	putFilePath := filepath.Join(localTemp, "put.txt")
	const payload = "integration payload"
	if err := os.WriteFile(putFilePath, []byte(payload), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	if err := putFile(share, putFilePath, "integration/put.txt"); err != nil {
		t.Fatalf("putFile failed: %v", err)
	}

	remote, err := share.Open("integration/put.txt")
	if err != nil {
		t.Fatalf("open remote: %v", err)
	}
	data, err := io.ReadAll(remote)
	if err != nil {
		_ = remote.Close()
		t.Fatalf("read remote: %v", err)
	}
	if err := remote.Close(); err != nil {
		t.Fatalf("close remote: %v", err)
	}
	if string(data) != payload {
		t.Fatalf("remote contents = %q, want %q", string(data), payload)
	}

	getPath := filepath.Join(localTemp, "get.txt")
	if err := getFile(share, "integration/put.txt", getPath); err != nil {
		t.Fatalf("getFile failed: %v", err)
	}
	got, err := os.ReadFile(getPath)
	if err != nil {
		t.Fatalf("read get file: %v", err)
	}
	if string(got) != payload {
		t.Fatalf("downloaded payload %q, want %q", string(got), payload)
	}

	// Remote cleanup is optional; the container will be discarded after the test.
}

func containsShare(shares []string, name string) bool {
	for _, s := range shares {
		if s == name {
			return true
		}
	}
	return false
}

func shouldSkipDocker(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, s := range []string{
		"docker daemon is not running",
		"cannot connect to the docker daemon",
		"error during connect",
		"permission denied while trying to connect to the docker daemon",
		"connect: no such file or directory",
		"connection refused",
		"socket not set",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}
