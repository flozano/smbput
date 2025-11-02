package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"
	"sort"
)

type smbOptions struct {
	address  string
	share    string
	user     string
	password string
	domain   string
	timeout  time.Duration
}

func main() {
	var opts smbOptions

	flag.StringVar(&opts.address, "server", "", "SMB server address (host or host:port)")
	flag.StringVar(&opts.share, "share", "", "SMB share name")
	flag.StringVar(&opts.user, "user", "", "SMB username")
	flag.StringVar(&opts.password, "password", "", "SMB password (or set SMB_PASSWORD env var)")
	flag.StringVar(&opts.domain, "domain", "", "SMB domain (optional)")
	flag.DurationVar(&opts.timeout, "timeout", 10*time.Second, "Dial timeout")
	flag.Parse()

	if opts.password == "" {
		opts.password = os.Getenv("SMB_PASSWORD")
	}

	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		os.Exit(2)
	}

	if opts.address == "" || opts.user == "" || opts.password == "" {
		fmt.Fprintln(os.Stderr, "server, user, and password are required")
		flag.Usage()
		os.Exit(2)
	}

	command := args[0]
	if command != "shares" && opts.share == "" {
		fmt.Fprintln(os.Stderr, "share is required for this command")
		flag.Usage()
		os.Exit(2)
	}

	switch command {
	case "shares":
		if err := listShares(opts); err != nil {
			log.Fatalf("shares failed: %v", err)
		}
	case "ls":
		share, cleanup, err := connect(opts)
		if err != nil {
			log.Fatalf("failed to connect: %v", err)
		}
		defer cleanup()
		remote := "."
		if len(args) > 1 {
			remote = args[1]
		}
		if err := listRemote(share, remote); err != nil {
			log.Fatalf("ls failed: %v", err)
		}
	case "get":
		share, cleanup, err := connect(opts)
		if err != nil {
			log.Fatalf("failed to connect: %v", err)
		}
		defer cleanup()
		if len(args) != 3 {
			printUsage()
			os.Exit(2)
		}
		if err := getFile(share, args[1], args[2]); err != nil {
			log.Fatalf("get failed: %v", err)
		}
	case "put":
		share, cleanup, err := connect(opts)
		if err != nil {
			log.Fatalf("failed to connect: %v", err)
		}
		defer cleanup()
		if len(args) != 3 {
			printUsage()
			os.Exit(2)
		}
		if err := putFile(share, args[1], args[2]); err != nil {
			log.Fatalf("put failed: %v", err)
		}
	default:
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
  smbput -server HOST[:PORT] -share NAME -user USER -password PASS <command> [args...]

Commands:
  shares
  ls [REMOTE PATH]
  get REMOTE_PATH LOCAL_PATH
  put LOCAL_PATH REMOTE_PATH`)
}

func connect(opts smbOptions) (*smb2.Share, func(), error) {
	session, cleanup, err := dialSession(opts)
	if err != nil {
		return nil, nil, err
	}

	share, err := session.Mount(opts.share)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("mount share %s: %w", opts.share, err)
	}

	return share, func() {
		share.Umount()
		cleanup()
	}, nil
}

func dialSession(opts smbOptions) (*smb2.Session, func(), error) {
	host, port, err := splitServerAddress(opts.address)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()

	ips, err := resolveHost(ctx, host, opts.timeout)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve host %s: %w", host, err)
	}

	tcpDialer := &net.Dialer{Timeout: opts.timeout}
	var conn net.Conn
	var dialErr error
	for _, ip := range ips {
		address := net.JoinHostPort(ip.String(), port)
		conn, dialErr = tcpDialer.DialContext(ctx, "tcp", address)
		if dialErr == nil {
			break
		}
	}
	if dialErr != nil {
		return nil, nil, fmt.Errorf("dial %s:%s: %w", host, port, dialErr)
	}

	dialer := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     opts.user,
			Password: opts.password,
			Domain:   opts.domain,
		},
	}

	session, err := dialer.Dial(conn)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("smb negotiate: %w", err)
	}

	cleanup := func() {
		session.Logoff()
		conn.Close()
	}

	return session, cleanup, nil
}

func listRemote(share *smb2.Share, remote string) error {
	remote = normalizeRemotePath(remote)
	files, err := share.ReadDir(remote)
	if err != nil {
		return fmt.Errorf("readdir %s: %w", remote, err)
	}

	for _, fi := range files {
		mod := fi.ModTime().UTC().Format(time.RFC3339)
		kind := "-"
		if fi.IsDir() {
			kind = "d"
		}
		fmt.Printf("%s %s %12d %s\n", kind, mod, fi.Size(), fi.Name())
	}
	return nil
}

func getFile(share *smb2.Share, remote, local string) error {
	remote = normalizeRemotePath(remote)
	dir := filepath.Dir(local)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	src, err := share.Open(remote)
	if err != nil {
		return fmt.Errorf("open remote %s: %w", remote, err)
	}
	defer src.Close()

	dst, err := os.Create(local)
	if err != nil {
		return fmt.Errorf("create local %s: %w", local, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", remote, local, err)
	}
	return nil
}

func putFile(share *smb2.Share, local, remote string) error {
	info, err := os.Stat(local)
	if err != nil {
		return fmt.Errorf("stat local %s: %w", local, err)
	}
	if info.IsDir() {
		return fmt.Errorf("local path %s is a directory", local)
	}

	remote = normalizeRemotePath(remote)
	dir := path.Dir(remote)
	if dir != "." && dir != "/" {
		if err := share.MkdirAll(dir, 0o755); err != nil {
			// Ignore errors - directory may already exist, or we'll fail at Create
			// MkdirAll typically succeeds if path already exists
		}
	}

	src, err := os.Open(local)
	if err != nil {
		return fmt.Errorf("open local %s: %w", local, err)
	}
	defer src.Close()

	dst, err := share.Create(remote)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remote, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", local, remote, err)
	}
	return nil
}

func normalizeRemotePath(p string) string {
	if p == "" {
		return "."
	}
	p = strings.ReplaceAll(p, "\\", "/")
	if strings.HasPrefix(p, "/") {
		p = strings.TrimPrefix(p, "/")
	}
	clean := path.Clean("/" + p)
	if clean == "/" || clean == "." {
		return "."
	}
	return strings.TrimPrefix(clean, "/")
}

func splitServerAddress(address string) (string, string, error) {
	if address == "" {
		return "", "", fmt.Errorf("server address is required")
	}

	if strings.HasPrefix(address, "[") && strings.HasSuffix(address, "]") {
		return strings.Trim(address, "[]"), "445", nil
	}

	host, port, err := net.SplitHostPort(address)
	if err == nil {
		if port == "" {
			port = "445"
		}
		return host, port, nil
	}

	var addrErr *net.AddrError
	if errors.As(err, &addrErr) && strings.Contains(addrErr.Err, "missing port in address") {
		if strings.HasPrefix(address, "[") {
			return strings.Trim(address, "[]"), "445", nil
		}
		return address, "445", nil
	}

	if ip := net.ParseIP(address); ip != nil {
		return address, "445", nil
	}

	return "", "", fmt.Errorf("parse server address %q: %w", address, err)
}

func listShares(opts smbOptions) error {
	session, cleanup, err := dialSession(opts)
	if err != nil {
		return err
	}
	defer cleanup()

	names, err := session.ListSharenames()
	if err != nil {
		return fmt.Errorf("list shares: %w", err)
	}

	if len(names) == 0 {
		fmt.Println("No shares found.")
		return nil
	}

	sort.Strings(names)
	for _, name := range names {
		fmt.Println(name)
	}
	return nil
}
