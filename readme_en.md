# Cora
[中文版本](readme.md)

Cora, Community Collaboration CLI. A unified command-line interface to interact with community services. Access forums, mailing lists, meetings, issue，CICD, and more — all from a single binary, driven by OpenAPI specs published by each backend service.

![Cora](assets/img/cora.png)

## Overview

`cora` (Community Collaboration) is built for open-source community developers who interact with multiple community services daily. Instead of juggling separate tools and web UIs, you get a consistent `cora <service> <resource> <verb>` command structure across all services.

**Key characteristics:**

- **Zero-code extensibility** — Adding a new backend service requires only a config entry pointing to its OpenAPI spec. No CLI code changes needed.
- **OpenAPI-driven commands** — Commands are generated dynamically at runtime from each service's OpenAPI 3.0 spec.
- **Spec caching** — Specs are cached locally (24h TTL by default) for sub-200ms cold starts with no network overhead.
- **Customisable output** — Declarative view configs let you control which fields are shown and how they're formatted. `--format json/yaml` outputs the full raw response, ideal for scripting and agents.
- **Scriptable** — stdout/stderr separation, semantic exit codes, and `--format json` output suitable for piping to `jq`.

## Supported Services

| Service | Command | Spec source | Auth method |
|---------|---------|-------------|-------------|
| [GitCode](https://gitcode.com) | `gitcode` | Built-in (no `spec_url` needed) | Personal access token (`?access_token=`) |
| [GitHub](https://github.com) | `github` | Built-in (no `spec_url` needed) | PAT / Fine-grained token (`Authorization: Bearer …`) |
| [Etherpad](https://etherpad.org) | `etherpad` | Built-in (no `spec_url` needed) | API key (`?apikey=`) |
| [Jenkins](https://www.jenkins.io) | `jenkins` | Built-in (no `spec_url` needed) | HTTP Basic Auth (`base64(username:api_token)`) |
| [Forum / Discourse](https://www.discourse.org) | `forum` (customisable) | Requires `spec_url` | API key + username (headers) |

> Built-in services (gitcode, github, etherpad, jenkins) have their OpenAPI spec embedded in the binary — no `spec_url` required. However, `base_url` must be explicitly set in the config file; there are no hardcoded default URLs.

## Command Structure

```
cora <service> <resource> <verb> [flags]
```

| Layer | Example | Source |
|-------|---------|--------|
| `cora` | — | Binary entry point |
| `<service>` | `gitcode`, `forum`, `etherpad` | Config file |
| `<resource>` | `issues`, `posts`, `topics` | OpenAPI `tags[0]` |
| `<verb>` | `list`, `get`, `create`, `delete` | OpenAPI `operationId` |

## Usage Examples

### GitCode

```bash
# List repositories
cora gitcode repos list --owner my-org

# Get repository details
cora gitcode repos get --owner my-org --repo my-repo

# List open issues
cora gitcode issues list --owner my-org --repo my-repo --state open

# Get a single issue (formatted table)
cora gitcode issues get --owner my-org --repo my-repo --number 1367

# Output as JSON and pipe to jq
cora gitcode issues get --owner my-org --repo my-repo --number 1367 --format json | jq '.title'

# Output as YAML
cora gitcode issues list --owner my-org --repo my-repo --format yaml

# Preview the HTTP request without sending it
cora gitcode issues create --owner my-org --repo my-repo --title "test" --dry-run
```

### Forum (Discourse)

```bash
# List recent posts
cora forum posts list

# Get a specific post
cora forum posts get --id 42

# Create a post
cora forum posts create --title "Release v1.2.0" --raw "Body text"

# Output as JSON and pipe to jq
cora forum posts list --format json | jq '.[].username'

# Force-refresh the cached OpenAPI spec
cora forum posts list --refresh-spec
```

### Etherpad

```bash
# List all pads
cora etherpad pads list

# Get pad content
cora etherpad pads get-text --pad-id my-pad

# Create a new pad
cora etherpad pads create-pad --pad-id new-pad
```

### GitHub

```bash
# Get repository details
cora github repos get --owner cncf --repo cora

# List open issues
cora github issues list --owner cncf --repo cora --state open

# Get a single issue
cora github issues get --owner cncf --repo cora --issue-number 1

# List pull requests
cora github pulls list --owner cncf --repo cora --state open

# JSON output + jq
cora github issues get --owner cncf --repo cora --issue-number 1 --format json | jq '.title'
```

### Jenkins

```bash
# List all jobs
cora jenkins jobs list

# Get job details
cora jenkins jobs get --name my-job

# Trigger a build
cora jenkins jobs build --name my-job

# Enable / disable a job
cora jenkins jobs enable-job --name my-job
cora jenkins jobs disable-job --name my-job

# Delete a job
cora jenkins jobs delete --name my-job

# Get build details
cora jenkins builds get --name my-job --number 1

# View the queue
cora jenkins queue list

# JSON output
cora jenkins jobs list --format json | jq '.jobs[].name'
```

### Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `table` | Output format: `table`, `json`, or `yaml` |
| `--dry-run` | `false` | Print the HTTP request without sending it |
| `--refresh-spec` | `false` | Bypass cache and re-fetch the service spec |
| `--verbose` | `false` | Enable verbose logging (INFO + DEBUG level) |

## Output Customisation

### Format Modes

`--format` controls the global output format and applies to every sub-command:

| Value | Behaviour |
|-------|-----------|
| `table` (default) | Applies the view config to render a formatted table; generic fallback when no view is defined |
| `json` | Bypasses all views, pretty-prints the full raw response as JSON |
| `yaml` | Bypasses all views, converts the full raw response to YAML |

**`--format json/yaml` always outputs the complete, unfiltered raw response** — suitable for scripting and agents. The view system is only active in `table` mode.

### View System

cora ships with built-in view definitions for common operations (field selection, formatting). You can override them or add new ones via `~/.config/cora/views.yaml`.

**Operations covered by built-in views:**

| Service | Operation | Render mode |
|---------|-----------|-------------|
| gitcode | `issues get` | Vertical KV table (single object) |
| gitcode | `issues list` | Horizontal table (list) |
| gitcode | `repos get` | Vertical KV table |
| gitcode | `repos list` | Horizontal table |
| gitcode | `pulls get` | Vertical KV table |
| gitcode | `pulls list` | Horizontal table |
| forum | `topics list` | Horizontal table |
| forum | `topics get` | Vertical KV table |
| forum | `posts list` | Horizontal table |
| etherpad | `pads list` | Horizontal table |

### Setting up views.yaml

Copy the example file to the default location and edit as needed:

```bash
mkdir -p ~/.config/cora
cp config/views.example.yaml ~/.config/cora/views.yaml
```

Override the path via environment variable:

```bash
export CORA_VIEWS=/path/to/my-views.yaml
```

Or set it in `config.yaml`:

```yaml
views_file: /path/to/my-views.yaml
```

### views.yaml Format

```yaml
<service-name>:
  <resource>/<verb>:
    root_field: ""        # optional: key that contains the list (empty = auto-detect)
    columns:
      - field: <dot.path> # required: JSON field path, supports dot notation (e.g. user.login)
        label: <string>   # optional: column header (auto-derived when empty)
        format: <type>    # optional: text (default) | json | date | multiline
        truncate: <int>   # optional: max rune count before "…" (0 = unlimited)
        width: <int>      # optional: fixed column width (list tables only)
        date_fmt: <string># optional: Go time format for format=date
        indent: <bool>    # optional: pretty-print JSON when format=json
```

**Format types:**

| Value | Best for | Rendering |
|-------|---------|-----------|
| `text` (default) | Strings, numbers, booleans | Converted to string, newlines collapsed, `truncate` applied |
| `json` | Nested objects, arrays | Raw JSON fragment; `indent: true` enables pretty-print |
| `date` | ISO 8601 timestamps | Parsed and reformatted using `date_fmt` (default `2006-01-02`) |
| `multiline` | Long text (body, description) | Newlines preserved, `truncate` applied by character count |

### Example: Override a built-in view to show specific fields

```yaml
# ~/.config/cora/views.yaml
gitcode:
  issues/get:
    columns:
      - field: number
        label: "No."
      - field: title
        label: Title
        truncate: 80
      - field: state
      - field: html_url
        label: URL
      - field: user.login
        label: Author
      - field: created_at
        label: Created
        format: date
```

### Example: Define a view for an operation with no built-in

```yaml
gitcode:
  commits/list:
    columns:
      - field: sha
        label: SHA
        truncate: 8
        width: 10
      - field: commit.message
        label: Message
        truncate: 60
        width: 62
      - field: commit.author.name
        label: Author
        width: 18
      - field: commit.author.date
        label: Date
        format: date
        width: 12
```

User views completely replace the matching built-in view (whole replacement, no column merging).

## Installation

### Build from source

**Requirements:** Go 1.22+, make

```bash
git clone https://github.com/cncf/cora.git
cd cora
make build
mv bin/cora /usr/local/bin/
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
cp config/config.example.yaml ~/.config/cora/config.yaml
# Edit the file and fill in your values
```

### Config File Reference

```yaml
services:
  # ── GitCode (built-in spec — no spec_url needed) ──
  gitcode:
    base_url: https://api.gitcode.com   # required; no default
    auth:
      gitcode:
        access_token: "your-personal-access-token"  # GitCode Settings → Personal Access Tokens

  # ── Etherpad (built-in spec — no spec_url needed) ──
  etherpad:
    base_url: https://your-etherpad-host/api/1.3.0  # required; no default
    auth:
      etherpad:
        api_key: "your-etherpad-api-key"

  # ── GitHub (built-in spec — no spec_url needed) ──
  github:
    base_url: https://api.github.com    # required; use https://<host>/api/v3 for GHE Server
    auth:
      github:
        token: "your-github-pat"          # https://github.com/settings/tokens

  # ── Jenkins (built-in spec — no spec_url needed) ──
  jenkins:
    base_url: https://jenkins.example.com          # required; no default
    auth:
      jenkins:
        username: "your-jenkins-username"
        api_token: "your-jenkins-api-token"          # JENKINS_URL/user/<you>/configure

  # ── Forum / Discourse (spec_url required) ──
  forum:
    # spec_url: URL or local path to the service's OpenAPI spec.
    # Supported schemes: http://, https://, file://, or a bare filesystem path.
    spec_url: assets/openapi/forum/openapi.json

    # base_url: the API root that all spec paths are appended to.
    base_url: https://forum.example.org

    auth:
      discourse:
        api_key: "your-api-key"
        api_username: "your-username"

  # Add more services without modifying CLI code:
  # myservice:
  #   spec_url: https://myservice.example.org/openapi.yaml
  #   base_url: https://myservice.example.org

# Global spec cache settings (optional — defaults shown).
spec_cache:
  ttl: 24h
  dir: ~/.config/cora/cache

# Custom views.yaml path (optional — defaults to ~/.config/cora/views.yaml).
views_file: ~/.config/cora/views.yaml
```

> **Note:** Built-in services (`gitcode`, `github`, `etherpad`, `jenkins`) have no hardcoded default `base_url`. It must be set explicitly in the config file.

### Environment Variables

All config values can be overridden by `CORA_`-prefixed environment variables. Environment variables take precedence over the config file.

| Environment variable | Config key | Description |
|----------------------|-----------|-------------|
| `CORA_CONFIG` | — | Override the config file path |
| `CORA_VIEWS` | — | Override the views.yaml path |
| `CORA_SPEC_CACHE_TTL` | `spec_cache.ttl` | Cache TTL (e.g. `12h`) |
| `CORA_SPEC_CACHE_DIR` | `spec_cache.dir` | Cache directory path |
| `CORA_SERVICES_<NAME>_BASE_URL` | `services.<name>.base_url` | Override a service's API root |
| `CORA_SERVICES_<NAME>_SPEC_URL` | `services.<name>.spec_url` | Override a service's spec URL |
| `CORA_SERVICES_GITCODE_AUTH_GITCODE_ACCESS_TOKEN` | `services.gitcode.auth.gitcode.access_token` | GitCode personal access token |
| `CORA_SERVICES_ETHERPAD_AUTH_ETHERPAD_API_KEY` | `services.etherpad.auth.etherpad.api_key` | Etherpad API key |
| `CORA_SERVICES_GITHUB_AUTH_GITHUB_TOKEN` | `services.github.auth.github.token` | GitHub PAT / fine-grained token |
| `CORA_SERVICES_JENKINS_AUTH_JENKINS_USERNAME` | `services.jenkins.auth.jenkins.username` | Jenkins username |
| `CORA_SERVICES_JENKINS_AUTH_JENKINS_API_TOKEN` | `services.jenkins.auth.jenkins.api_token` | Jenkins API token |
| `CORA_SERVICES_<NAME>_AUTH_DISCOURSE_API_KEY` | `services.<name>.auth.discourse.api_key` | Discourse API key |
| `CORA_SERVICES_<NAME>_AUTH_DISCOURSE_API_USERNAME` | `services.<name>.auth.discourse.api_username` | Discourse username |

> **Note:** Environment variables can only override service entries that **already exist** in the config file. They cannot introduce brand-new services.

#### Local development: .env file

Create a `.env` file in the project root and cora will load it automatically at startup. Existing system environment variables are never overwritten by `.env`. The `.env` file is for local development only — **do not commit it**.

```bash
cp .env.example .env
# Edit .env with your local values
```

`.env` example:

```bash
CORA_SERVICES_FORUM_BASE_URL=http://localhost:3000
CORA_SERVICES_FORUM_AUTH_DISCOURSE_API_KEY=dev-api-key
CORA_SERVICES_FORUM_AUTH_DISCOURSE_API_USERNAME=system
```

### Spec Loading Behaviour

| Priority | Condition | Behaviour |
|----------|-----------|-----------|
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
make build          # Compile binary (output: ./bin/cora)
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
make build && ./bin/cora forum posts list
```

### Testing

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
| `internal/log/mask_test.go` | URL masking, header masking, body formatting |
| `internal/output/formatter_test.go` | JSON/YAML/table output, view rendering, terminal safety |
| `internal/smoke/loader_test.go`     | YAML scenario loading, defaults, empty-file skipping |
| `internal/smoke/assertion_test.go`  | All 10 assertion type validations |
| `internal/smoke/runner_test.go`     | Subprocess invocation, env var injection |
| `internal/smoke/report_test.go`     | HTML report generation |

### Project Layout

```
cora/
├── cmd/cora/main.go                  # Entry point, two-phase command loading
├── internal/
│   ├── auth/resolver.go              # Auth credential injection
│   ├── builder/
│   │   ├── command.go                # OpenAPI spec → Cobra command tree
│   │   └── command_test.go
│   ├── config/config.go              # Config loading and structures
│   ├── executor/executor.go          # HTTP request execution
│   ├── log/
│   │   ├── log.go                    # Levelled logging (Error/Warn/Info/Debug)
│   │   └── mask.go                   # URL/header masking, response body formatting
│   ├── output/
│   │   ├── formatter.go              # Table / JSON / YAML output formatting
│   │   └── formatter_test.go
│   ├── registry/
│   │   ├── registry.go               # Service registry
│   │   └── builtin.go                # Built-in service registration (gitcode, github, etherpad, jenkins)
│   ├── spec/
│   │   ├── loader.go                 # Three-tier spec loading
│   │   ├── cache.go                  # Cache read/write (atomic)
│   │   └── *_test.go
│   └── view/
│       ├── view.go                   # ViewColumn / ViewConfig / Registry types
│       ├── extract.go                # Field path extraction and value formatting
│       ├── builtin.go                # Built-in view definitions
│       └── loader.go                 # views.yaml loader, merges with built-ins
├── pkg/errs/
│   ├── errors.go                     # Error types and exit codes
│   └── errors_test.go
├── assets/
│   ├── openapi/                      # Bundled OpenAPI spec files
│   └── assets.go                     # go:embed declarations
├── config/
│   ├── config.example.yaml           # Config file template
│   └── views.example.yaml            # views.yaml template
├── cmd/
│   ├── cora/main.go                  # cora entry point
│   └── smoke/main.go                 # Smoke Runner entry point
├── scenarios/                        # Smoke test scenario YAML files
├── spec/                             # Architecture design documents
├── Makefile
└── Dockerfile
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

4. (Optional) Define custom views in `~/.config/cora/views.yaml` for frequently used operations.

### Backend Service Requirements

- Use **OpenAPI 3.0** (Swagger 2.0 is not supported)
- Assign `tags` to operations — the first tag becomes the `<resource>` command name
- Use `operationId` values with a known verb prefix: `list`, `get`, `create`, `update`, `delete`, `patch`
- Declare `security` per operation (absent or empty means no auth required)

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

## Smoke Tests

cora ships with an end-to-end smoke testing framework that continuously monitors service sub-command availability, catching broken APIs or unexpected output before they reach users.

### How it works

The Smoke Runner (`cmd/smoke`) reads YAML scenario files from the `scenarios/` directory, invokes the real `cora` binary for each one, and checks exit codes, stdout/stderr content, response time, and JSON fields. It then produces an HTML report. Empty files and comment-only files are silently skipped.

### Scenario file format

```yaml
name: "GitCode · issues list"
service: gitcode
args:
  - issues
  - list
  - --owner
  - openeuler
  - --repo
  - infrastructure
  - --state
  - open
format: table
timeout_ms: 8000
assertions:
  - type: exit_code
    value: 0
  - type: response_time_lt
    value: 5000
  - type: stdout_not_empty
  - type: stderr_not_contains
    value: "ERROR"
  - type: json_has_keys          # only meaningful with format: json
    values: ["title", "state"]
```

**Supported assertion types:**

| Type | Description |
|------|-------------|
| `exit_code` | Exit code equals the specified value |
| `stdout_not_empty` | stdout is not empty |
| `stderr_not_contains` | stderr does not contain the specified string |
| `response_time_lt` | Response time (ms) is below the specified value |
| `json_has_keys` | JSON output contains all specified top-level keys |
| `json_key_not_empty` | The specified JSON key is not empty |
| `table_has_columns` | Table output contains all specified column names |
| `stdout_contains` | stdout contains the specified string |
| `stderr_empty` | stderr is empty |
| `exit_code_not` | Exit code is not equal to the specified value |

### Configuration

Copy the example config and fill in your credentials:

```bash
cp config/smoke-config.example.yaml config/smoke-config.yaml
# Edit smoke-config.yaml with real tokens and URLs
```

Credentials can also be injected via environment variables. Scenario `args` support `${VAR}` substitution:

```bash
export SMOKE_GITCODE_TOKEN=glpat-xxxx
```

### Running

```bash
# Build the smoke runner and run all scenarios
make smoke

# Run only scenarios matching a keyword (filters by name)
make smoke-filter FILTER=gitcode

# Run manually with explicit flags
./bin/smoke-runner \
  --cora-bin ./bin/cora \
  --config ./config/smoke-config.yaml \
  --scenarios-dir ./scenarios \
  --report-dir ./smoke-report
```

Reports are written to `smoke-report/<YYYY-MM-DD>/report.html`, archived by date.

### CI integration

The GitHub Actions workflow (`.github/workflows/smoke.yml`) runs smoke tests nightly at UTC 02:00 and also supports manual dispatch. The HTML report is uploaded as an artifact with a 90-day retention period, named `smoke-report-<YYYY-MM-DD>`.

### Directory layout

```
scenarios/
├── gitcode/
│   ├── repos-list.yaml
│   ├── issues-list.yaml
│   └── issues-get.yaml
├── forum/
│   └── posts-list.yaml      # comment out to skip temporarily
└── etherpad/
    └── pad-list.yaml        # comment out to skip temporarily

internal/smoke/
├── types.go                 # Scenario, Assertion, Result type definitions
├── loader.go                # YAML scenario loading; empty files auto-skipped
├── assertion.go             # 10 assertion types
├── runner.go                # Invokes cora binary with env var injection
└── report.go                # Console output + HTML report generation
```

## Architecture

See the [`spec/`](spec/) directory for the full design documents, including architecture decisions, OpenAPI-driven command generation, auth strategy, spec caching, and the view system design.

## Contributing

Contributions are welcome. Please open an issue before submitting a large change.
