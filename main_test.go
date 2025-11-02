package main

import "testing"

func TestSplitServerAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantIP   string
		wantPort string
		wantErr  bool
	}{
		{
			name:     "hostname default port",
			input:    "fileserver",
			wantIP:   "fileserver",
			wantPort: "445",
		},
		{
			name:     "hostname custom port",
			input:    "fileserver:1445",
			wantIP:   "fileserver",
			wantPort: "1445",
		},
		{
			name:     "ipv4 default port",
			input:    "192.168.1.10",
			wantIP:   "192.168.1.10",
			wantPort: "445",
		},
		{
			name:     "ipv6 default port",
			input:    "[2001:db8::1]",
			wantIP:   "2001:db8::1",
			wantPort: "445",
		},
		{
			name:     "ipv6 custom port",
			input:    "[2001:db8::1]:1445",
			wantIP:   "2001:db8::1",
			wantPort: "1445",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, port, err := splitServerAddress(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if host != tc.wantIP || port != tc.wantPort {
				t.Fatalf("splitServerAddress(%q) = (%q, %q), want (%q, %q)", tc.input, host, port, tc.wantIP, tc.wantPort)
			}
		})
	}
}

func TestNormalizeRemotePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "."},
		{".", "."},
		{"/", "."},
		{"\\\\server\\share", "server/share"},
		{"folder/file.txt", "folder/file.txt"},
		{"/folder/./file.txt", "folder/file.txt"},
		{"folder\\nested\\", "folder/nested"},
	}

	for _, tc := range tests {
		if got := normalizeRemotePath(tc.input); got != tc.want {
			t.Fatalf("normalizeRemotePath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
