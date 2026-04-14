# Cora
中文版本：[readme.md](readme.md)

Cora, Community Collaboration CLI. A unified command-line interface to interact with community services. Access forums, mailing lists, meetings, issue trackers, and more — all from a single binary, driven by OpenAPI specs published by each backend service.
![Cora](assets/img/cora.png)

## Overview

`cora` (Community Collaboration) is built for open-source community developers who interact with multiple community services daily. Instead of juggling separate tools and web UIs, you get a consistent `cora <service> <resource> <verb>` command structure across all services.

**Key characteristics:**

- **Zero-code extensibility** — Adding a new backend service requires only a config entry pointing to its OpenAPI spec. No CLI code changes needed.
- **OpenAPI-driven commands** — Commands are generated dynamically at runtime from each service's OpenAPI 3.0 spec.
- **Spec caching** — Specs are cached locally (24h TTL by default) for sub-200ms cold starts with no network overhead.
- **Scriptable** — stdout/stderr separation, semantic exit codes, and `--format json` output suitable for piping to `jq`.

## Command Structure

```
cora <service> <resource> <verb> [flags]
```

| Layer | Example | Source |
|-------|---------|--------|
| `cora` | — | Binary entry point |
| `<service>` | `forum`, `mail`, `issue` | Config file |
| `<resource>` | `posts`, `topics`, `threads` | OpenAPI `tags[0]` |
| `<verb>` | `list`, `get`, `create`, `delete` | OpenAPI `operationId` |

## Usage Examples

```bash
# List recent posts in the forum
cora forum posts list

# Get a specific post
cora forum posts get --id 42

# Create a new post (preview the HTTP request first)
cora forum posts create --title "Release v1.2.0" --raw "Body text" --dry-run

# Create a post for real
cora forum posts create --title "Release v1.2.0" --raw "Body text"

# Output as JSON and pipe to jq
cora forum posts list --format json | jq '.[].username'

# Force-refresh the cached OpenAPI spec
cora forum posts list --refresh-spec

# Manage the spec cache directly
cora spec refresh forum
```

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `table` | Output format: `table` or `json` |
| `--dry-run` | `false` | Print the HTTP request without sending it |
| `--refresh-spec` | `false` | Bypass cache and re-fetch the service spec |

## Installation

### Build from source

**Requirements:** Go 1.22+, make

```bash
git clone https://github.com/cncf/cora.git
cd cora
make build
mv cora /usr/local/bin/
```

### Docker

```bash
# Build the image
make docker-build

# Run with your local config directory mounted
docker run --rm \
  -v ~/.config/cora:/root/.config/cora:ro \
  cora:latest forum posts list
```

Or use the Makefile shortcut:

```bash
make docker-run ARGS="forum posts list"
```

## Configuration

Cora reads its configuration from `~/.config/cora/config.yaml` by default. Override with the `CORA_CONFIG` environment variable.

### Setup

```bash
mkdir -p ~/.config/cora
cp config.example.yaml ~/.config/cora/config.yaml
# Edit the file and fill in your values
```

### Config File Reference

```yaml
services:
  forum:
    # spec_url: URL or local path to the service's OpenAPI spec.
    # Supported: http://, https://, file://, or a bare filesystem path.
    spec_url: assets/openapi/forum/openapi.json

    # base_url: the API root that all paths from the spec are appended to.
    base_url: https://forum.example.org

    auth:
      discourse:
        api_key: "your-api-key"
        api_username: "your-username"

  # Add more services — no CLI code changes required.
  # mail:
  #   spec_url: https://lists.example.org/openapi.yaml
  #   base_url: https://lists.example.org

# Global spec cache settings (optional — defaults shown).
spec_cache:
  ttl: 24h
  dir: ~/.config/cora/cache
```

### Spec Loading Behavior

| Priority | Condition | Behavior |
|----------|-----------|----------|
| 1 (fastest) | Cache exists and within TTL | Read local file, no network |
| 2 | Cache expired or missing | Fetch from `spec_url`, write cache |
| 3 | Fetch fails, stale cache exists | Use stale cache + stderr warning |
| 4 (error) | Fetch fails, no cache | Exit code 4, check network / config |

Use `--refresh-spec` to force a re-fetch regardless of TTL.

## Local Development

### Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | >= 1.22 | `brew install go` |
| make | any | pre-installed on macOS |
| golangci-lint | >= 1.57 | `brew install golangci-lint` |
| Docker | >= 24.0 | [docker.com](https://www.docker.com) |

### Common Commands

```bash
make build          # Compile binary (output: ./cora)
make build-prod     # Production build (CGO disabled, stripped)
make test           # Run all tests with race detector
make test-unit      # Run short tests only (skip integration)
make test-cover     # Generate HTML coverage report (coverage.html)
make lint           # Run golangci-lint
make fmt            # Format code (gofmt + goimports)
make tidy           # Clean up dependencies (go mod tidy)
make clean          # Remove build artefacts
```

### Running from source

```bash
# Run without building first
go run ./cmd/cora -- forum posts list

# Build then run
make build && ./cora forum posts list
```

### Using a local OpenAPI spec

```yaml
services:
  forum:
    spec_url: assets/openapi/forum/openapi.json   # relative path (recommended)
    # spec_url: file:///path/to/openapi.json      # absolute path also works
    base_url: http://localhost:3000
    auth:
      discourse:
        api_key: "dev-api-key"
        api_username: "system"
```

### Testing

Tests follow the pyramid model — predominantly unit tests covering core logic.

```bash
# Full test suite with race detector
make test

# View coverage summary
make test-cover-text
```

Test files by package:

| Test file | Coverage |
|-----------|----------|
| `pkg/errs/errors_test.go` | Error types, exit codes, constructors |
| `internal/spec/cache_test.go` | Cache read/write, atomic writes, TTL |
| `internal/spec/loader_test.go` | Three-tier loading, HTTP, local files, fallback |
| `internal/builder/command_test.go` | Resource/verb name derivation, flag mapping |
| `internal/output/formatter_test.go` | JSON/table output, terminal safety, data extraction |

### Project Layout

```
cora/
├── cmd/cora/main.go                  # Entry point, two-phase command loading
├── internal/
│   ├── builder/
│   │   ├── command.go                # OpenAPI spec → Cobra command tree
│   │   └── command_test.go
│   ├── config/config.go              # Config loading and structures
│   ├── executor/executor.go          # HTTP request execution
│   ├── output/
│   │   ├── formatter.go              # Table / JSON output formatting
│   │   └── formatter_test.go
│   ├── registry/registry.go          # Service registry
│   ├── spec/
│   │   ├── loader.go                 # Three-tier spec loading
│   │   ├── loader_test.go
│   │   ├── cache.go                  # Cache read/write (atomic)
│   │   └── cache_test.go
│   └── auth/resolver.go              # Auth header injection
├── pkg/errs/
│   ├── errors.go                     # Error types and exit codes
│   └── errors_test.go
├── assets/openapi/                   # Bundled spec files (for local dev)
├── spec/                             # Architecture design documents
├── Makefile
├── Dockerfile
└── config.example.yaml
```

## Adding a New Service

1. Ensure the backend exposes an OpenAPI 3.0 spec at a stable URL.
2. Add an entry to `~/.config/cora/config.yaml`:

   ```yaml
   services:
     myservice:
       spec_url: https://myservice.example.org/openapi.yaml
       base_url: https://myservice.example.org
   ```

3. Run any command — the spec is fetched and cached automatically:

   ```bash
   cora myservice --help
   ```

### Backend Service Requirements

- Use **OpenAPI 3.0** (Swagger 2.0 is not supported)
- Assign `tags` to operations — the first tag becomes the `<resource>` command name
- Use `operationId` values with a known verb prefix: `list`, `get`, `create`, `update`, `delete`, `patch`
- Declare `security` per operation (absent or empty means no auth required)

Optional `x-cli-*` extensions for richer CLI output:

```yaml
x-cli-examples:
  - "cora myservice widgets list --active"
x-cli-flags: [active, limit, cursor]
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | API error (4xx/5xx from the backend) |
| 2 | Auth error (missing or invalid credentials) |
| 3 | Input validation error |
| 4 | Spec load failure |
| 5 | Config error (service not configured, malformed config) |
| 127 | Unclassified error |

## Architecture

See [`spec/architecture-design.md`](spec/architecture-design.md) for the full design document, including ADRs for framework selection, OpenAPI-driven command generation, auth strategy, and spec caching.

## Contributing

Contributions are welcome. Please open an issue before submitting a large change.
