package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

var (
	llmnrIPv4Addr = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 252), Port: 5355}
	llmnrIPv6Addr = &net.UDPAddr{IP: net.ParseIP("ff02::1:3"), Port: 5355}
)

func resolveHost(ctx context.Context, host string, timeout time.Duration) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}

	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	deadline := time.Now().Add(timeout)
	lookupCtx, lookupCancel := context.WithDeadline(ctx, deadline)
	defer lookupCancel()

	ips, err := lookupHost(lookupCtx, host)
	if len(ips) > 0 {
		return uniqueIPs(ips), nil
	}
	var lastErr error
	if err != nil {
		lastErr = err
	}

	// Try common mDNS suffix before issuing multicast queries; many systems
	// resolve *.local via their standard resolver.
	if !strings.HasSuffix(host, ".local") {
		if mdnsIPs, err := lookupHost(lookupCtx, host+".local"); len(mdnsIPs) > 0 {
			return uniqueIPs(mdnsIPs), nil
		} else if err != nil {
			lastErr = err
		}
	}

	// Fall back to LLMNR multicast queries.
	llmnrTimeout := remaining(deadline)
	if llmnrTimeout <= 0 {
		llmnrTimeout = 500 * time.Millisecond
	}
	if llmnrIPs, err := lookupLLMNR(lookupCtx, host, llmnrTimeout); len(llmnrIPs) > 0 {
		return uniqueIPs(llmnrIPs), nil
	} else if err != nil {
		lastErr = err
	}

	if !strings.HasSuffix(host, ".local") {
		if llmnrIPs, err := lookupLLMNR(lookupCtx, host+".local", llmnrTimeout); len(llmnrIPs) > 0 {
			return uniqueIPs(llmnrIPs), nil
		} else if err != nil {
			lastErr = err
		}
	}

	if len(ips) == 0 {
		if lastErr == nil {
			lastErr = fmt.Errorf("no IP addresses found for %s", host)
		}
		return nil, lastErr
	}
	return uniqueIPs(ips), nil
}

func lookupHost(ctx context.Context, host string) ([]net.IP, error) {
	resolver := net.DefaultResolver
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	ipAddrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	ips := make([]net.IP, 0, len(ipAddrs))
	for _, addr := range ipAddrs {
		if ip := addr.IP; ip != nil {
			ips = append(ips, ip)
		}
	}
	if len(ips) == 0 {
		return nil, errors.New("no IPs returned")
	}
	return ips, nil
}

func lookupLLMNR(ctx context.Context, host string, timeout time.Duration) ([]net.IP, error) {
	name := host
	if !strings.HasSuffix(name, ".") {
		name += "."
	}

	var msg dnsmessage.Message
	qName, err := dnsmessage.NewName(name)
	if err != nil {
		return nil, err
	}

	msg.Header = dnsmessage.Header{
		RecursionDesired: false,
	}
	msg.Questions = []dnsmessage.Question{
		{Name: qName, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET},
		{Name: qName, Type: dnsmessage.TypeAAAA, Class: dnsmessage.ClassINET},
	}
	buf, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	if _, err := conn.WriteToUDP(buf, llmnrIPv4Addr); err != nil {
		return nil, err
	}
	// Best-effort IPv6 query; ignore errors on platforms without IPv6.
	_, _ = conn.WriteToUDP(buf, llmnrIPv6Addr)

	var ips []net.IP
	out := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(out)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				break
			}
			if errors.Is(err, context.DeadlineExceeded) {
				break
			}
			return nil, err
		}

		var parser dnsmessage.Parser
		if _, err := parser.Start(out[:n]); err != nil {
			continue
		}
		if err := parser.SkipQuestion(); err != nil {
			continue
		}
		for {
			answer, err := parser.Answer()
			if errors.Is(err, dnsmessage.ErrSectionDone) {
				break
			}
			if err != nil {
				break
			}
			switch body := answer.Body.(type) {
			case *dnsmessage.AResource:
				ip := net.IP(body.A[:])
				if ip != nil {
					ips = append(ips, ip)
				}
			case *dnsmessage.AAAAResource:
				ip := net.IP(body.AAAA[:])
				if ip != nil {
					ips = append(ips, ip)
				}
			}
		}
	}

	if len(ips) == 0 {
		return nil, errors.New("no LLMNR responses")
	}

	return uniqueIPs(ips), nil
}

func uniqueIPs(ips []net.IP) []net.IP {
	if len(ips) < 2 {
		return ips
	}
	seen := make(map[string]struct{}, len(ips))
	out := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		key := ip.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ip)
	}
	return out
}

func remaining(deadline time.Time) time.Duration {
	return time.Until(deadline)
}
