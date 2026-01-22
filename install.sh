#!/bin/bash
set -e

# ConfigLock Installer
# Usage: curl -sSL https://raw.githubusercontent.com/baggiiiie/configlock/main/configlock/install.sh | bash

# Configuration
REPO="baggiiiie/configlock"
INSTALL_DIR="$HOME/.local/bin"
BINARY_NAME="configlock"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)     echo "linux";;
        Darwin*)    echo "darwin";;
        *)          error "Unsupported OS: $(uname -s)";;
    esac
}

# Detect architecture
detect_arch() {
    local arch=$(uname -m)
    case "$arch" in
        x86_64)     echo "amd64";;
        amd64)      echo "amd64";;
        arm64)      echo "arm64";;
        aarch64)    echo "arm64";;
        *)          error "Unsupported architecture: $arch";;
    esac
}

# Get latest release version
get_latest_version() {
    local version=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$version" ]; then
        error "Failed to fetch latest version from GitHub"
    fi
    echo "$version"
}

# Download and install
install_binary() {
    local os=$(detect_os)
    local arch=$(detect_arch)

    info "Detected platform: ${os}/${arch}"

    # Get latest version
    info "Fetching latest version..."
    local version=$(get_latest_version)
    info "Latest version: ${version}"

    # Construct download URL
    local binary_name="${BINARY_NAME}-${os}-${arch}"
    local download_url="https://github.com/${REPO}/releases/download/${version}/${binary_name}"

    info "Downloading from: ${download_url}"

    # Create temporary directory
    local tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" EXIT

    # Download binary
    if ! curl -sSL -o "${tmp_dir}/${BINARY_NAME}" "$download_url"; then
        error "Failed to download binary. Please check if the release exists at:\n  ${download_url}"
    fi

    # Make it executable
    chmod +x "${tmp_dir}/${BINARY_NAME}"

    # Create install directory if it doesn't exist
    mkdir -p "$INSTALL_DIR"

    info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
    mv "${tmp_dir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    
    info "Installation complete!"
    echo ""
    info "If '$INSTALL_DIR' is not in your PATH, you may need to add it to run '${BINARY_NAME}' directly."
    info "Example: export PATH=\"$PATH:$INSTALL_DIR\""
    echo ""
    info "Run '${BINARY_NAME} --help' to get started"
    info "Run '${BINARY_NAME} init' to set up ConfigLock"
}

# Check for required commands
check_requirements() {
    for cmd in curl sed grep; do
        if ! command -v $cmd &> /dev/null; then
            error "Required command not found: $cmd"
        fi
    done
}

# Main
main() {
    echo ""
    info "ConfigLock Installer"
    echo ""

    check_requirements
    install_binary

    echo ""
    info "🎉 ConfigLock installed successfully!"
    echo ""
}

main
