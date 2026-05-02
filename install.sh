#!/bin/bash

# CLIProxyAPIPlus - Installer
# Downloads pre-built binaries from GitHub Releases

set -euo pipefail

SCRIPT_NAME="cliproxyapi-installer"
REPO_OWNER="HsnSaboor"
REPO_NAME="CLIProxyAPIPlus"
API_URL="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"

detect_os() {
    case "$(uname -s)" in
        Linux*)
            OS="linux"
            ;;
        Darwin*)
            OS="darwin"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            OS="windows"
            ;;
        *)
            log_error "Unsupported OS: $(uname -s)"
            exit 1
            ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            log_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac
}

set_paths() {
    case "$OS" in
        linux)
            if [[ -d "$HOME/.local/share" ]]; then
                CACHE_DIR="$HOME/.cache/cliproxyapi"
            else
                CACHE_DIR="$HOME/.cliproxyapi-cache"
            fi
            PROD_DIR="$HOME/.cliproxyapi"
            ;;
        darwin)
            CACHE_DIR="$HOME/Library/Caches/cliproxyapi"
            PROD_DIR="$HOME/cliproxyapi"
            ;;
        windows)
            CACHE_DIR="$LOCALAPPDATA/cliproxyapi"
            PROD_DIR="$LOCALAPPDATA/cliproxyapi"
            ;;
    esac
    AUTH_DIR="$HOME/.cli-proxy-api"
}

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step() { echo -e "${CYAN}[STEP]${NC} $1"; }

is_service_running() {
    if [[ "$OS" == "linux" ]]; then
        systemctl --user is-active --quiet cliproxyapi.service 2>/dev/null
    else
        pgrep -f "cli-proxy-api" >/dev/null 2>&1
    fi
}

stop_service() {
    if is_service_running; then
        log_info "Stopping service..."
        if [[ "$OS" == "linux" ]]; then
            systemctl --user stop cliproxyapi.service 2>/dev/null || true
        fi
        pkill -f "cli-proxy-api" 2>/dev/null || true
        sleep 2
    fi
}

generate_api_key() {
    local prefix="sk-"
    local chars="abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    local key=""
    for i in {1..45}; do
        key="${key}${chars:$((RANDOM % ${#chars})):1}"
    done
    echo "${prefix}${key}"
}

check_api_keys() {
    local config_file="${PROD_DIR}/config.yaml"
    [[ ! -f "$config_file" ]] && return 1
    grep -q '"your-api-key-1"' "$config_file" && return 1
    grep -q '"your-api-key-2"' "$config_file" && return 1
    grep -A 10 "^api-keys:" "$config_file" | grep -q '"sk-[^"]*"' && return 0
    return 1
}

get_latest_release() {
    log_step "Fetching latest release..."

    local response
    response=$(curl -fSL "$API_URL" 2>/dev/null) || {
        log_error "Failed to fetch release info. Check network."
        exit 1
    }

    VERSION=$(echo "$response" | grep -o '"tag_name": "[^"]*"' | cut -d'"' -f4)
    if [[ -z "$VERSION" ]]; then
        log_error "Could not determine latest version"
        exit 1
    fi

    log_success "Latest version: $VERSION"
}

download_and_verify() {
    log_step "Downloading release..."

    mkdir -p "$CACHE_DIR"
    cd "$CACHE_DIR"

    local ext="tar.gz"
    [[ "$OS" == "windows" ]] && ext="zip"

    local filename="CLIProxyAPIPlus_${VERSION}_${OS}_${ARCH}.${ext}"
    local url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/${filename}"

    log_info "Downloading $filename..."
    curl -fSL "$url" -o "$filename" || {
        log_error "Failed to download $filename"
        exit 1
    }

    log_info "Downloading checksums.txt..."
    local checksum_url="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/checksums.txt"
    curl -fSL "$checksum_url" -o "checksums.txt" || {
        log_error "Failed to download checksums"
        exit 1
    }

    log_info "Verifying checksum..."
    local expected_hash
    expected_hash=$(grep "$filename" "checksums.txt" | awk '{print $1}')
    local actual_hash
    actual_hash=$(sha256sum "$filename" | awk '{print $1}')

    if [[ "$expected_hash" != "$actual_hash" ]]; then
        log_error "Checksum mismatch!"
        log_error "Expected: $expected_hash"
        log_error "Actual:   $actual_hash"
        exit 1
    fi

    log_success "Checksum verified"
}

extract_and_deploy() {
    log_step "Extracting..."

    cd "$CACHE_DIR"

    local ext="tar.gz"
    [[ "$OS" == "windows" ]] && ext="zip"

    local filename="CLIProxyAPIPlus_${VERSION}_${OS}_${ARCH}.${ext}"

    mkdir -p "$PROD_DIR/config_backup"

    log_info "Backing up config..."
    if [[ -f "$PROD_DIR/config.yaml" ]]; then
        local ts
        ts=$(date +"%Y%m%d_%H%M%S")
        cp "$PROD_DIR/config.yaml" "$PROD_DIR/config_backup/config_${ts}.yaml"
    fi

    log_info "Backing up auth tokens..."
    if [[ -d "$AUTH_DIR" ]]; then
        local token_ts
        token_ts=$(date +"%Y%m%d_%H%M")
        tar -czf "$PROD_DIR/config_backup/tokens_${token_ts}.tar.gz" -C "$AUTH_DIR" . 2>/dev/null || true
    fi

    if [[ "$OS" == "windows" ]]; then
        unzip -o "$filename" -d "$PROD_DIR" >/dev/null
    else
        tar -xzf "$filename" -C "$PROD_DIR"
    fi

    if [[ -f "$PROD_DIR/cli-proxy-api" ]]; then
        mv "$PROD_DIR/cli-proxy-api" "$PROD_DIR/cli-proxy-api.old"
    fi

    if [[ -f "$PROD_DIR/CLIProxyAPIPlus_${VERSION}_${OS}_${ARCH}/cli-proxy-api" ]]; then
        mv "$PROD_DIR/CLIProxyAPIPlus_${VERSION}_${OS}_${ARCH}/cli-proxy-api" "$PROD_DIR/cli-proxy-api"
        rm -rf "$PROD_DIR/CLIProxyAPIPlus_${VERSION}_${OS}_${ARCH}"
    fi

    if [[ -f "$PROD_DIR/cli-proxy-api" ]]; then
        chmod +x "$PROD_DIR/cli-proxy-api"
    fi
    log_success "Binary deployed"

    create_systemd_service

    log_info "Starting service..."
    if [[ "$OS" == "linux" ]]; then
        systemctl --user restart cliproxyapi.service
    fi
    sleep 3

    if is_service_running; then
        log_success "Service is running"
    else
        log_warning "Service not running, starting manually..."
        nohup "$PROD_DIR/cli-proxy-api" > "$PROD_DIR/nohup.out" 2>&1 &
        sleep 3
    fi
}

create_systemd_service() {
    if [[ "$OS" != "linux" ]]; then
        return
    fi

    local systemd_dir="$HOME/.config/systemd/user"
    mkdir -p "$systemd_dir"

    cat > "$systemd_dir/cliproxyapi.service" << EOF
[Unit]
Description=CLIProxyAPI Service
After=network.target

[Service]
Type=simple
WorkingDirectory=$PROD_DIR
ExecStart=$PROD_DIR/cli-proxy-api
Restart=always
RestartSec=10
Environment=HOME=$HOME

[Install]
WantedBy=default.target
EOF

    systemctl --user daemon-reload || true
}

create_config() {
    if [[ -f "$PROD_DIR/config.yaml" ]]; then
        return
    fi

    # Look in extracted folder (before binary is moved out)
    local extracted_dir="$PROD_DIR/CLIProxyAPIPlus_${VERSION}_${OS}_${ARCH}"
    local example_config="$extracted_dir/config.example.yaml"

    # Fallback to cache if already extracted and moved
    [[ ! -f "$example_config" ]] && example_config="$CACHE_DIR/config.example.yaml"

    if [[ -f "$example_config" ]]; then
        cp "$example_config" "$PROD_DIR/config.yaml"
        local key1 key2
        key1=$(generate_api_key)
        key2=$(generate_api_key)
        sed -i "s/\"your-api-key-1\"/\"$key1\"/g" "$PROD_DIR/config.yaml" 2>/dev/null || true
        sed -i "s/\"your-api-key-2\"/\"$key2\"/g" "$PROD_DIR/config.yaml" 2>/dev/null || true
        log_success "Created config.yaml with generated API keys"
    else
        log_warning "config.example.yaml not found, skipping config creation"
    fi
}

show_status() {
    echo
    echo "CLIProxyAPIPlus - Status"
    echo "========================"
    echo "Cache Dir:   $CACHE_DIR"
    echo "Install Dir: $PROD_DIR"
    echo "Auth Dir:    $AUTH_DIR"

    local version_line
    version_line=$(grep "CLIProxyAPIPlus" "$CACHE_DIR/checksums.txt" 2>/dev/null | head -1)
    if [[ -n "$version_line" ]]; then
        echo "Version:    $(echo "$version_line" | sed 's/.*CLIProxyAPIPlus_\([0-9-]*\).*/\1/')"
    fi

    [[ -f "$PROD_DIR/cli-proxy-api" ]] && echo "Binary:      Present" || echo "Binary:      Missing"
    [[ -f "$PROD_DIR/config.yaml" ]] && echo "Config:      Present" || echo "Config:      Missing"
    check_api_keys && echo "API Keys:    Configured" || echo "API Keys:    NOT CONFIGURED"

    echo
    if is_service_running; then
        echo -e "Service:     ${GREEN}RUNNING${NC}"
    else
        echo -e "Service:     ${RED}NOT RUNNING${NC}"
    fi
    echo
}

show_quick_start() {
    echo
    echo -e "${GREEN}Quick Start:${NC}"
    echo -e "${BLUE}1. Configure:${NC}  ${CYAN}nano ${PROD_DIR}/config.yaml${NC}"
    echo -e "${BLUE}2. Auth:${NC}       ${CYAN}${PROD_DIR}/cli-proxy-api --login${NC}"
    echo -e "${BLUE}3. Run:${NC}        ${CYAN}systemctl --user start cliproxyapi.service${NC} (Linux)"
    echo
}

install_or_upgrade() {
    detect_os
    set_paths
    get_latest_release
    download_and_verify

    # Extract just to get config.example.yaml
    log_step "Preparing config..."
    cd "$CACHE_DIR"
    if [[ "$OS" == "windows" ]]; then
        unzip -o "CLIProxyAPIPlus_${VERSION}_${OS}_${ARCH}.zip" -d "$PROD_DIR" >/dev/null
    else
        tar -xzf "CLIProxyAPIPlus_${VERSION}_${OS}_${ARCH}.tar.gz" -C "$PROD_DIR"
    fi

    create_config
    extract_and_deploy

    if [[ ! -f "$PROD_DIR/config.yaml" ]] || ! check_api_keys; then
        echo
        echo -e "${YELLOW}API Keys Required${NC}"
        echo -e "${BLUE}Edit: ${CYAN}nano ${PROD_DIR}/config.yaml${NC}"
    fi

    show_quick_start
}

uninstall() {
    if [[ ! -d "$PROD_DIR" ]]; then
        log_warning "Not installed"
        exit 0
    fi

    log_info "Found at: $PROD_DIR"
    read -p "Remove? (y/N): " -n 1 -r
    echo
    [[ ! $REPLY =~ ^[Yy]$ ]] && exit 0

    stop_service
    rm -rf "$PROD_DIR"
    rm -rf "$CACHE_DIR"
    log_success "Uninstalled"
}

main() {
    detect_os
    set_paths

    case "${1:-install}" in
        install|upgrade)
            install_or_upgrade
            ;;
        status)
            show_status
            ;;
        uninstall)
            uninstall
            ;;
        -h|--help)
            cat << EOF
CLIProxyAPIPlus - Installer

Usage: $SCRIPT_NAME [command]

Commands:
  install, upgrade   Download latest release and install (default)
  status             Show installation status
  uninstall          Remove installation
  -h, --help        This help

Paths:
  Cache:   $CACHE_DIR
  Binary:  $PROD_DIR
  Auth:    $AUTH_DIR

EOF
            ;;
        *)
            log_error "Unknown: $1"
            exit 1
            ;;
    esac
}

main "$@"