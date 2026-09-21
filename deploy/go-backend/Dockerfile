# syntax=docker/dockerfile:1.7
#
# Sthira backend Go service — owned by deploy/go-backend/.
#
# Pinned to the exact Go toolchain the project uses (see backend/go.mod and
# backend/README.md). The runtime image is distroless / non-root and carries
# only the compiled binary plus the timezone database; source code, datasets,
# model weights and test fixtures are intentionally not copied. The build
# context is the repository root, but only backend/ is copied in.
#
# Coordination note: changes to backend/go.mod or to backend/cmd/sthira/main.go
# are owned by Worker 1 in plan/next-two-worker-1.md. Re-publish this image
# after those changes land; do not modify them here.

ARG GO_VERSION=1.27.1

# ---- build stage ----------------------------------------------------------
FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src

# Cache module downloads before copying the rest of the source.
COPY backend/go.mod backend/go.sum ./backend/
# `go mod download` reads ./... but the module root is backend/.
WORKDIR /src/backend
RUN go mod download

# Copy the backend source tree. The repository's root .dockerignore already
# filters .git, .venv, .env and the frontend/v2 node modules/dist; nothing
# else from the repository is needed to compile the binary.
WORKDIR /src
COPY backend/ ./backend/

WORKDIR /src/backend
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS=-mod=readonly
# `-trimpath` removes absolute paths from the binary so a leaked stacktrace
# never reveals build-host layout. `-ldflags="-s -w"` strips DWARF for a
# smaller image; debugging symbols are not needed inside the runtime image.

# Two binaries are built in this package:
#   - sthira:      the API server, owned by backend/cmd/sthira/main.go
#                  (changes to that file are owned by Worker 1).
#   - sthmigrate:  the explicit one-shot migration runner, owned here
#                  under deploy/go-backend/migrate/. It is intentionally a
#                  separate binary so the API server is never responsible
#                  for racing migrations on startup.
# Both binaries use the pinned 1.27.1 toolchain declared by backend/go.mod.
RUN go build -trimpath -ldflags="-s -w" -o /out/sthira ./cmd/sthira \
 && go build -trimpath -ldflags="-s -w" \
      -o /out/sthmigrate ./cmd/sthmigrate

# ---- runtime stage --------------------------------------------------------
# Distroless base: minimal Debian userland, no shell, no package manager.
# `nonroot` image enforces UID 65532; combined with the explicit `USER`
# directive below this is defence-in-depth — even a container-escape bug that
# got root inside the namespace lands on a non-root host UID.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

# Distroless ships /etc/ssl/certs/ca-certificates.crt and the basic tzdata
# set, but we add an explicit copy of the CA bundle for clarity. The Go
# binary does not bundle TLS assets of its own.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=build /out/sthira /usr/local/bin/sthira

# Run as the distroless non-root UID 65532. The image's USER directive is
# declarative; the base image's `nonroot` tag enforces the same UID.
USER 65532:65532

# The API binds to 127.0.0.1:8080 by default inside the container; compose
# maps the loopback port to the host.
EXPOSE 8080

# Match the binary's documented signal/shutdown semantics: it uses
# signal.NotifyContext on SIGINT/SIGTERM and drains on ctx cancellation.
# A bare entrypoint preserves PID-1 semantics so SIGTERM is delivered.
ENTRYPOINT ["/usr/local/bin/sthira"]
