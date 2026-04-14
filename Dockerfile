# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

ARG VERSION=dev

WORKDIR /src

# Cache dependency downloads separately from source compilation.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.Version=${VERSION}" \
    -o /cora \
    ./cmd/cora

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates: needed for HTTPS requests to remote OpenAPI spec endpoints.
RUN apk --no-cache add ca-certificates

# Set working directory so that relative spec_url paths (e.g. "assets/openapi/forum/openapi.json")
# resolve correctly inside the container, matching local development behaviour
# where the CLI is run from the project root.
WORKDIR /app

COPY --from=builder /cora /usr/local/bin/cora

# Bundle the OpenAPI spec files so the CLI works without a network call
# to fetch the spec on first run.
COPY assets/ /app/assets/

# Config is expected to be mounted at runtime.
# Example: docker run -v ~/.config/cora:/root/.config/cora:ro cora:latest forum posts list
# The spec_url in your config should use the relative path: assets/openapi/forum/openapi.json
VOLUME ["/root/.config/cora"]

ENTRYPOINT ["cora"]
