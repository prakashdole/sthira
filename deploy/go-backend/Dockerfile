# syntax=docker/dockerfile:1.7
#
# Sthira backend Go service — owned by deploy/go-backend/.
#
# Pinned to the exact Go toolchain the project uses (see backend/go.mod and
# backend/README.md). The runtime API image is distroless / non-root and carries
# only the compiled binary plus the CA bundle; source code, datasets,
# model weights and test fixtures are intentionally not copied. The build
# context is the repository root or staged directory containing backend/ and
# deploy/go-backend/.
#
# A dedicated migration image target (migrate) is provided based on
# postgis/postgis:18-3.6 (or postgres:18-bookworm) with psql and SQL migration
# files included, keeping the API image minimal and non-root.

ARG GO_VERSION=1.27.1
ARG POSTGIS_IMAGE=postgis/postgis:18-3.6

# ---- build stage ----------------------------------------------------------
FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src

# Cache backend module downloads before copying the rest of the source.
COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /src/backend
RUN go mod download

# Cache migrate module downloads from its own module directory.
WORKDIR /src
COPY deploy/go-backend/migrate/go.mod ./deploy/go-backend/migrate/
WORKDIR /src/deploy/go-backend/migrate
RUN go mod download

# Copy the source trees for both modules.
WORKDIR /src
COPY backend/ ./backend/
COPY deploy/go-backend/migrate/ ./deploy/go-backend/migrate/

# Build API server binary from backend module root.
WORKDIR /src/backend
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/sthira ./cmd/sthira

# Build migration runner and DSN env helper from migrate module root.
WORKDIR /src/deploy/go-backend/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/sthmigrate ./cmd/sthmigrate \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/pgdsn-env ./cmd/pgdsn-env

# ---- migrate stage --------------------------------------------------------
# Dedicated migration runner image containing psql, sthmigrate, and SQL migration files.
# Uses postgis/postgis:18-3.6 with psql available, keeping the API image minimal and distroless.
FROM ${POSTGIS_IMAGE} AS migrate

COPY --from=build /out/sthmigrate /usr/local/bin/sthmigrate
COPY --from=build /out/pgdsn-env /usr/local/bin/pgdsn-env
COPY backend/migrations /migrations

USER postgres

ENTRYPOINT ["/usr/local/bin/sthmigrate"]
CMD ["up"]

# ---- runtime api stage ----------------------------------------------------
# Distroless base: minimal Debian userland, no shell, no package manager.
# nonroot image enforces UID 65532; combined with the explicit USER directive
# below this provides defence-in-depth — even a container-escape bug that
# got root inside the namespace lands on a non-root host UID.
FROM gcr.io/distroless/static-debian12:nonroot AS api

# Distroless ships /etc/ssl/certs/ca-certificates.crt; we copy it explicitly
# so a future rebuild that strips the distroless CA bundle still has the
# standard certificate set inside the image.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=build /out/sthira /usr/local/bin/sthira

# Run as the distroless non-root UID 65532. The image's USER directive is
# declarative; the base image's nonroot tag enforces the same UID.
USER 65532:65532

# The API binds to 0.0.0.0:8080 inside the container; compose maps the
# loopback port on host to this container port.
EXPOSE 8080

# Match the binary's documented signal/shutdown semantics: it uses
# signal.NotifyContext on SIGINT/SIGTERM and drains on ctx cancellation.
# A bare entrypoint preserves PID-1 semantics so SIGTERM is delivered.
ENTRYPOINT ["/usr/local/bin/sthira"]
