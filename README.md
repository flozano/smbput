# smbput

`smbput` is a single-binary SMB client written in Go that can list, download, and upload files on an SMB share without requiring kernel mounts or Kerberos.

## Build

### Using Make (recommended)

```bash
make build              # Build optimized binary (3.4MB)
make build-aggressive   # Even smaller binary (3.3MB)
make install-upx        # Install UPX compression tool (optional)
make build-upx          # Compress with UPX (~1.5MB, 55% smaller!)
make size-comparison    # Compare optimization levels
```

**Size comparison:**
- Standard build: 5.1MB (with debug info)
- Optimized build: 3.4MB (33% smaller, `-s -w -trimpath`)
- Aggressive build: 3.3MB (35% smaller)
- UPX compressed: ~1.5MB (70% smaller, requires UPX tool)

Cross-compile for other platforms:
```bash
make build-arm          # 32-bit ARM (Raspberry Pi)
make build-arm64        # 64-bit ARM
make build-windows      # Windows
make build-macos        # macOS
make build-all          # All platforms
```

Run `make help` to see all available targets.

### Manual build

```bash
# Standard build
go build -o smbput

# Optimized build (smaller binary)
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o smbput
```

### Cross-compile for 32-bit ARM

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags="-s -w" -o smbput
```

Adjust `GOARM` (5–7) if your target CPU requires it.

### Optional: UPX Compression

For even smaller binaries (~70% size reduction), you can install UPX:

```bash
# Automatic installation (uses Makefile)
make install-upx

# Or install manually
# On Ubuntu/Debian:
apt-get install upx-ucl

# Or download from:
# https://github.com/upx/upx/releases
```

Then build with UPX compression:
```bash
make build-upx    # Creates ~1.5MB binary
```

## Usage

```bash
export SMB_PASSWORD=secret
smbput -server 10.0.0.10 -share documents -user alice ls /
smbput -server fileserver:445 -share drop -user alice get reports/weekly.pdf ./weekly.pdf
smbput -server fileserver -share drop -user alice put ./notes.txt uploads/notes.txt
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
