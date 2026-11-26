#!/usr/bin/env bash
set -euo pipefail

BIN_DIR="${HOME}/.local/bin"
mkdir -p "${BIN_DIR}"

cd "$(dirname "$0")/.."
go build -ldflags "-X github.com/farishan-labs/processor/internal/version.Version=${VERSION:-dev}" -o "${BIN_DIR}/processor" ./cmd/processor

echo "installed processor to ${BIN_DIR}/processor"