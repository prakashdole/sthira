# syntax=docker/dockerfile:1.7
#
# Sthira backend Go service — owned by deploy/go-backend/.
#
# Pinned to the exact Go toolchain the project uses (see backend/go.mod and
# backend/README.md). The runtime image is distroless / non-root and carries
# only the compiled binary plus the CA bundle; source code, datasets,
# model weights and test fixtures are intentionally not copied. The build
# context is the repository root, but only backend/ and deploy/go-backend/
# are copied in.
#
# Coordination note: changes to backend/go.mod or to backend/cmd/sthira/main.go
# are owned by Worker 1 in plan/next-two-worker-1.md. Re-publish this image
# after those changes land; do not modify them here.

ARG GO_VERSION=1.27.1

# ---- build stage ----------------------------------------------------------
FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src

# Cache backend module downloads before copying the rest of the source.
COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /src/backend
RUN go mod download

# Cache migrate module downloads. The migrate module is stdlib-only so this
# step is essentially free, but it keeps the layer-cache pattern uniform.
# WORKDIR stays at /src/backend during this step but COPY sources are
# always resolved relative to the build context root, so the paths below
# land at /src/backend/migrate/{go.mod,cmd/}.
COPY deploy/go-backend/migrate/go.mod ./migrate/go.mod
COPY deploy/go-backend/migrate/cmd ./migrate/cmd

# Move into the migrate module root for `go mod download`.
WORKDIR /src/backend/migrate
RUN go mod download || true

# Copy the source trees for both modules. Resetting WORKDIR to /src first
# so the COPY destinations are clean.
WORKDIR /src
COPY backend/ ./backend/
COPY deploy/go-backend/migrate/ ./backend/migrate/

# Build both binaries.
#   sthira      — API server, owned by Worker 1
#                 (changes to backend/cmd/sthira/main.go)
#   sthmigrate  — explicit migration runner, owned by this package
#                 (deploy/go-backend/migrate/cmd/sthmigrate)
#   pgdsn-env   — URI->libpq env helper used by go-backup.sh on the host
WORKDIR /src/backend
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS=-mod=readonly
# `-trimpath` removes absolute paths from the binary so a leaked stacktrace
# never reveals build-host layout. `-ldflags="-s -w"` strips DWARF for a
# smaller image; debugging symbols are not needed inside the runtime image.
RUN go build -trimpath -ldflags="-s -w" -o /out/sthira ./cmd/sthira \
 && CGO_ENABLED=0 GOOS=linux GOFLAGS=-mod=mod \
    go build -trimpath -ldflags="-s -w" -o /out/sthmigrate ./migrate/cmd/sthmigrate \
 && CGO_ENABLED=0 GOOS=linux GOFLAGS=-mod=mod \
    go build -trimpath -ldflags="-s -w" -o /out/pgdsn-env ./migrate/cmd/pgdsn-env

# ---- runtime stage --------------------------------------------------------
# Distroless base: minimal Debian userland, no shell, no package manager.
# `nonroot` image enforces UID 65532; combined with the explicit `USER`
# directive below this is defence-in-depth — even a container-escape bug that
# got root inside the namespace lands on a non-root host UID.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

# Distroless ships /etc/ssl/certs/ca-certificates.crt; we copy it explicitly
# so a future rebuild that strips the distroless CA bundle still has the
# standard certificate set inside the image.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=build /out/sthira /usr/local/bin/sthira
COPY --from=build /out/sthmigrate /usr/local/bin/sthmigrate
COPY --from=build /out/pgdsn-env /usr/local/bin/pgdsn-env

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
