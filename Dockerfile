# Build the faultline binary
FROM golang:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY . .

# Pick up version metadata when git is available, fall back gracefully
ARG VERSION=dev
ARG COMMIT
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a -ldflags="-s -w \
      -X github.com/CamiloValderruten/faultline/internal/version.Version=${VERSION} \
      -X github.com/CamiloValderruten/faultline/internal/version.Commit=${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)} \
      -X github.com/CamiloValderruten/faultline/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o faultline ./cmd/faultline

# Bundle CA certificates for the final distroless image
RUN mkdir -p /workspace/certs && cp /etc/ssl/certs/ca-certificates.crt /workspace/certs/ca-certificates.crt

# Docker CLI only (no engine). internal/adapters/sandbox/docker runs
# `docker run` / `docker kill` via exec.LookPath("docker"); the daemon lives
# on the host or Kind node. Mount /var/run/docker.sock at deploy time.
#
# Official docker:*-cli image ships a statically linked Linux binary, so it
# runs on gcr.io/distroless/static:nonroot without glibc. BuildKit resolves
# docker:27-cli to the build TARGETPLATFORM (amd64/arm64).
#
# Verify locally (client must print; server needs socket + permission to read
# the socket, e.g. match container user to host docker group or root):
#   docker build -t faultline:dev .
#   docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
#     --entrypoint /usr/local/bin/docker faultline:dev version
FROM docker:27-cli AS docker-cli

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/faultline /faultline
COPY --from=docker-cli /usr/local/bin/docker /usr/local/bin/docker
COPY --from=builder /workspace/certs/ /etc/ssl/certs/
# Same PATH as distroless static; explicit so `docker` resolves via
# exec.LookPath("docker") regardless of base-image defaults.
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
USER nonroot:nonroot
ENTRYPOINT ["/faultline"]
