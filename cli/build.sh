#!/usr/bin/env bash
set -euo pipefail

# Build and install the Go CLI for your AI orchestrator

# Directory containing this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Assume project root is one level up
PROJECT_ROOT="${SCRIPT_DIR}/.."
# Package path to main.go
CLI_SRC_DIR="${PROJECT_ROOT}/cli"
# Output binary name
OUTPUT_BIN="mind"
# Install destination (can override via INSTALL_PATH env var)
INSTALL_PATH="${INSTALL_PATH:-/usr/local/bin}"

echo "📦 Building CLI..."
cd "${CLI_SRC_DIR}"
go mod tidy
# Build only the Go source; output binary named by OUTPUT_BIN
go build -o "${OUTPUT_BIN}" main.go

echo "🚚 Installing to ${INSTALL_PATH}..."
# Create install dir if needed
mkdir -p "${INSTALL_PATH}"
sudo install -m 755 "${OUTPUT_BIN}" "${INSTALL_PATH}/${OUTPUT_BIN}"

# Cleanup local binary
rm mind

echo "✅ Installed '${OUTPUT_BIN}' to '${INSTALL_PATH}/${OUTPUT_BIN}'"
