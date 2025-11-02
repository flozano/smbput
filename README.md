# smbput

`smbput` is a single-binary SMB client written in Go that can list, download, and upload files on an SMB share without requiring kernel mounts or Kerberos.

## Build

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go build ./...
```

### Cross-compile for 32-bit ARM

```bash
GOCACHE=$(pwd)/.gocache \
GOMODCACHE=$(pwd)/.gomodcache \
GOOS=linux GOARCH=arm GOARM=7 \
go build -o smbput
```

Adjust `GOARM` (5–7) if your target CPU requires it.

## Usage

```bash
smbput -server 10.0.0.10 -share documents -user alice -password secret ls /
smbput -server fileserver:445 -share drop -user alice -password secret get reports/weekly.pdf ./weekly.pdf
smbput -server fileserver -share drop -user alice -password secret put ./notes.txt uploads/notes.txt
```

Options:

- `-server`: SMB server address (`HOST` or `HOST:PORT`, default port 445).
- `-share`: Share name to mount.
- `-user`: Username for NTLM authentication.
- `-password`: Password (fallback to `SMB_PASSWORD` environment variable if unset).
- `-domain`: Optional Windows domain.
- `-timeout`: Dial timeout (default 10s).

Commands:

- `ls [REMOTE PATH]`: List directory contents (defaults to root).
- `get REMOTE_PATH LOCAL_PATH`: Download `REMOTE_PATH` to the local file system.
- `put LOCAL_PATH REMOTE_PATH`: Upload local file to `REMOTE_PATH` on the share (creates missing remote directories).
