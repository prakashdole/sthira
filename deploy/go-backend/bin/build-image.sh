#!/usr/bin/env bash
# bin/build-image.sh — produce an image for the Go backend without using
# the repository root as Docker build context. Stages only what the
# Dockerfile needs (backend/ + go.work files if any) into a temporary build
# directory whose `.dockerignore` is owned here, then invokes docker build
# against that staged context. The root .dockerignore is intentionally not
# modified.
#
# This is the only path the lifecycle scripts use for image construction.

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PACKAGE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${PACKAGE_DIR}/../.." && pwd)"

# Project name overrides conventional docker compose project naming so the
# stack never collides with another compose project on the host. Default
# comes from compose.yml; CI may export STHIRA_DEPLOY_PROJECT.
PROJECT_NAME="${STHIRA_DEPLOY_PROJECT:-sthira-go}"
IMAGE_TAG="${STHIRA_DEPLOY_IMAGE:-${PROJECT_NAME}-backend:local}"
BUILDER="${STHIRA_DEPLOY_BUILDER:-docker}"

STAGE_DIR="$(mktemp -d -t sthira-go-stage.XXXXXXXX)"
trap 'rm -rf "${STAGE_DIR}"' EXIT

# Copy only the backend tree. The repository's frontend, loadmodel, fixtures,
# raw .txt evidence, .venv, .git and plan docs are not needed to compile the
# Go binary and would bloat the build context if included.
mkdir -p "${STAGE_DIR}/backend"
# `cp -R` then prune non-source files for tightness.
rsync -a --delete \
  --exclude='.git' \
  --exclude='__pycache__' \
  --exclude='*.pyc' \
  --exclude='*.txt' \
  --exclude='testdata/**' \
  "${REPO_ROOT}/backend/" "${STAGE_DIR}/backend/"

# Owned per-package ignore: keep it next to this build script. BuildKit
# reads .dockerignore from the build-context root, so this exact file is
# honoured.
cat > "${STAGE_DIR}/.dockerignore" <<'EOF'
# Owned by deploy/go-backend/. Do not edit the repo-root .dockerignore.
**/*.txt
**/*.md
**/*.json
**/*.yml
**/*.yaml
**/*.sh
**/*.git*
.git
.gitignore
Dockerfile*
.dockerignore
testdata/
*_test.go
EOF

echo "Building ${IMAGE_TAG} from staged context ${STAGE_DIR}"
"${BUILDER}" build \
  --build-arg "GO_VERSION=$(grep -m1 '^go ' "${REPO_ROOT}/backend/go.mod" | awk '{print $2}')" \
  -f "${PACKAGE_DIR}/Dockerfile" \
  -t "${IMAGE_TAG}" \
  "${STAGE_DIR}"

echo "${IMAGE_TAG}" > "${STAGE_DIR}/.image-tag"
cat "${STAGE_DIR}/.image-tag"
