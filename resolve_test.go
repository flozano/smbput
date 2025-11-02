package main

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestResolveHostReturnsIPWhenGivenIP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ips, err := resolveHost(ctx, "10.0.0.5", time.Second)
	if err != nil {
		t.Fatalf("resolveHost returned error: %v", err)
	}
	if len(ips) != 1 {
		t.Fatalf("expected single IP, got %d", len(ips))
	}
	if !ips[0].Equal(net.ParseIP("10.0.0.5")) {
		t.Fatalf("resolveHost returned %v, want 10.0.0.5", ips[0])
	}
}

func TestUniqueIPs(t *testing.T) {
	ips := []net.IP{
		net.ParseIP("192.168.1.1"),
		net.ParseIP("192.168.1.1"),
		net.ParseIP("192.168.1.2"),
		nil,
		net.ParseIP("192.168.1.2"),
	}

	out := uniqueIPs(ips)
	if len(out) != 2 {
		t.Fatalf("expected 2 unique IPs, got %d", len(out))
	}
	if !out[0].Equal(net.ParseIP("192.168.1.1")) {
		t.Fatalf("first IP = %v, want 192.168.1.1", out[0])
	}
	if !out[1].Equal(net.ParseIP("192.168.1.2")) {
		t.Fatalf("second IP = %v, want 192.168.1.2", out[1])
	}
}
