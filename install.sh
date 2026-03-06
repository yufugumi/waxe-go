#!/bin/sh
set -e

# Install script for axel (WAXE accessibility scanner)
# Usage: curl -sSfL https://raw.githubusercontent.com/yufugumi/waxe-go/main/install.sh | sh

REPO="github.com/yufugumi/waxe-go"
BINARY_NAME="axel"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"

main() {
    check_dependencies
    detect_platform

    printf "Installing %s for %s/%s...\n" "$BINARY_NAME" "$OS" "$ARCH"

    if has_go; then
        install_with_go
    else
        printf "Error: Go is required to install %s from source.\n" "$BINARY_NAME" >&2
        printf "Install Go from https://go.dev/dl/ and try again.\n" >&2
        exit 1
    fi

    setup_path
    printf "\n%s installed successfully to %s/%s\n" "$BINARY_NAME" "$INSTALL_DIR" "$BINARY_NAME"
    printf "Run '%s scan --help' to get started.\n" "$BINARY_NAME"
}

check_dependencies() {
    if ! command -v git >/dev/null 2>&1; then
        printf "Error: git is required but not found.\n" >&2
        exit 1
    fi
}

has_go() {
    command -v go >/dev/null 2>&1
}

detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"

    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        armv7*) ARCH="arm" ;;
        *)
            printf "Error: unsupported architecture: %s\n" "$ARCH" >&2
            exit 1
            ;;
    esac

    case "$OS" in
        linux|darwin) ;;
        mingw*|msys*|cygwin*) OS="windows" ;;
        *)
            printf "Error: unsupported OS: %s\n" "$OS" >&2
            exit 1
            ;;
    esac
}

install_with_go() {
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    printf "Cloning repository...\n"
    git clone --depth 1 "https://${REPO}.git" "$tmpdir/waxe-go" 2>&1

    printf "Building %s...\n" "$BINARY_NAME"
    cd "$tmpdir/waxe-go"
    CGO_ENABLED=0 go build -o "$tmpdir/${BINARY_NAME}" ./cmd/scanner

    mkdir -p "$INSTALL_DIR"
    mv "$tmpdir/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
}

setup_path() {
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) return ;;
    esac

    printf "\nNote: %s is not in your PATH.\n" "$INSTALL_DIR"
    printf "Add it by appending one of the following to your shell profile:\n\n"

    shell_name="$(basename "${SHELL:-/bin/sh}")"
    case "$shell_name" in
        fish)
            printf "  fish_add_path %s\n" "$INSTALL_DIR"
            ;;
        zsh)
            printf "  echo 'export PATH=\"%s:\$PATH\"' >> ~/.zshrc\n" "$INSTALL_DIR"
            ;;
        *)
            printf "  echo 'export PATH=\"%s:\$PATH\"' >> ~/.bashrc\n" "$INSTALL_DIR"
            ;;
    esac
}

main
