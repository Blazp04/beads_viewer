#!/usr/bin/env bash
set -euo pipefail

REPO_OWNER="Dicklesworthstone"
REPO_NAME="beads_viewer"
BIN_NAME="bv"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

print_info() { printf "\033[1;34m==>\033[0m %s\n" "$1"; }
print_success() { printf "\033[1;32m==>\033[0m %s\n" "$1"; }
print_error() { printf "\033[1;31m==>\033[0m %s\n" "$1"; }
print_warn() { printf "\033[1;33m==>\033[0m %s\n" "$1"; }

detect_platform() {
    local os arch

    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    arch="$(uname -m)"

    case "$os" in
        linux) os="linux" ;;
        darwin) os="darwin" ;;
        mingw*|msys*|cygwin*) os="windows" ;;
        *) print_error "Unsupported OS: $os"; return 1 ;;
    esac

    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *) print_error "Unsupported architecture: $arch"; return 1 ;;
    esac

    echo "${os}_${arch}"
}

get_latest_release() {
    local url="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" 2>/dev/null
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "$url" 2>/dev/null
    else
        return 1
    fi
}

download_file() {
    local url="$1"
    local dest="$2"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$dest"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$dest"
    else
        return 1
    fi
}

try_binary_install() {
    local platform="$1"
    local tmp_dir

    print_info "Checking for pre-built binary..."

    # Get latest release info
    local release_json
    release_json=$(get_latest_release) || return 1

    # Extract version tag
    local version
    version=$(echo "$release_json" | grep -oP '"tag_name":\s*"\K[^"]+' | head -1) || return 1

    if [ -z "$version" ]; then
        return 1
    fi

    print_info "Latest release: $version"

    # Construct asset name based on goreleaser naming convention
    local asset_name="${BIN_NAME}_${platform}"
    local ext=""

    if [[ "$platform" == windows_* ]]; then
        ext=".zip"
        asset_name="${asset_name}.zip"
    else
        ext=".tar.gz"
        asset_name="${asset_name}.tar.gz"
    fi

    # Find the download URL for this asset
    local download_url
    download_url=$(echo "$release_json" | grep -oP '"browser_download_url":\s*"\K[^"]+'"${asset_name}"'[^"]*' | head -1) || true

    # Try alternate naming conventions if first attempt failed
    if [ -z "$download_url" ]; then
        # Try with version in name: bv_v0.9.0_linux_amd64.tar.gz
        local versioned_name="${BIN_NAME}_${version}_${platform}${ext}"
        download_url=$(echo "$release_json" | grep -oP '"browser_download_url":\s*"\K[^"]*'"${versioned_name}"'[^"]*' | head -1) || true
    fi

    if [ -z "$download_url" ]; then
        # Try without leading 'v' in version: bv_0.9.0_linux_amd64.tar.gz
        local version_no_v="${version#v}"
        local versioned_name="${BIN_NAME}_${version_no_v}_${platform}${ext}"
        download_url=$(echo "$release_json" | grep -oP '"browser_download_url":\s*"\K[^"]*'"${versioned_name}"'[^"]*' | head -1) || true
    fi

    if [ -z "$download_url" ]; then
        print_warn "No pre-built binary found for $platform"
        return 1
    fi

    print_info "Downloading $download_url..."

    tmp_dir=$(mktemp -d)
    trap 'rm -rf "$tmp_dir"' EXIT

    local archive_path="$tmp_dir/archive${ext}"

    if ! download_file "$download_url" "$archive_path"; then
        print_warn "Download failed"
        return 1
    fi

    # Extract the binary
    print_info "Extracting..."

    if [[ "$ext" == ".zip" ]]; then
        if command -v unzip >/dev/null 2>&1; then
            unzip -q "$archive_path" -d "$tmp_dir"
        else
            print_warn "unzip not found"
            return 1
        fi
    else
        tar -xzf "$archive_path" -C "$tmp_dir"
    fi

    # Find the binary in extracted contents
    local binary_path
    binary_path=$(find "$tmp_dir" -name "$BIN_NAME" -type f -executable 2>/dev/null | head -1)

    if [ -z "$binary_path" ]; then
        # Try without executable check (for freshly extracted files)
        binary_path=$(find "$tmp_dir" -name "$BIN_NAME" -type f 2>/dev/null | head -1)
    fi

    if [ -z "$binary_path" ] && [[ "$platform" == windows_* ]]; then
        binary_path=$(find "$tmp_dir" -name "${BIN_NAME}.exe" -type f 2>/dev/null | head -1)
    fi

    if [ -z "$binary_path" ]; then
        print_warn "Binary not found in archive"
        return 1
    fi

    chmod +x "$binary_path"

    # Install to destination
    local dest_path="$INSTALL_DIR/$BIN_NAME"

    if [ -w "$INSTALL_DIR" ]; then
        mv "$binary_path" "$dest_path"
    else
        print_info "Installing to $INSTALL_DIR requires sudo..."
        sudo mv "$binary_path" "$dest_path"
    fi

    print_success "Installed $BIN_NAME $version to $dest_path"
    return 0
}

try_go_install() {
    print_info "Attempting to build from source with go install..."

    if ! command -v go >/dev/null 2>&1; then
        print_error "Go is not installed and no pre-built binary is available."
        print_error "Please install Go from https://golang.org or download a release binary manually."
        exit 1
    fi

    # Check Go version
    local go_version major minor
    go_version=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+' | head -1) || go_version="0.0"
    major=$(echo "$go_version" | cut -d. -f1)
    minor=$(echo "$go_version" | cut -d. -f2)

    if [ "$major" -lt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -lt 21 ]; }; then
        print_error "Go 1.21 or later is required for building from source. Found: go$go_version"
        exit 1
    fi

    local repo="github.com/${REPO_OWNER}/${REPO_NAME}"

    if go install "$repo/cmd/$BIN_NAME@latest"; then
        local gobin="${GOBIN:-$(go env GOPATH)/bin}"
        local installed_path="$gobin/$BIN_NAME"

        if [ -f "$installed_path" ]; then
            print_success "Built and installed $BIN_NAME to $installed_path"

            if ! command -v "$BIN_NAME" >/dev/null 2>&1; then
                print_warn "$gobin may not be in your PATH."
                print_info "Add this to your shell profile:"
                print_info "  export PATH=\"\$PATH:$gobin\""
            fi
            return 0
        fi
    fi

    print_error "Build from source failed."
    exit 1
}

main() {
    print_info "Installing $BIN_NAME..."

    local platform
    platform=$(detect_platform) || {
        print_warn "Could not detect platform, will try building from source"
        try_go_install
        exit 0
    }

    print_info "Detected platform: $platform"

    # First, try to download pre-built binary
    if try_binary_install "$platform"; then
        print_info "Run '$BIN_NAME' in any beads project to view issues."
        exit 0
    fi

    # Fall back to building from source
    print_info "Pre-built binary not available, falling back to source build..."
    try_go_install

    print_info "Run '$BIN_NAME' in any beads project to view issues."
}

main "$@"
