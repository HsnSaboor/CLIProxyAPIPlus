#!/bin/bash
# CLIProxyAPIPlus - Source-to-Production Deployment Engine
# Builds from source with Git-based upstream sync, atomic deploy, and auto-rollback

set -euo pipefail

REPO_DIR="$HOME/code/CLIProxyAPIPlus"
PROD_DIR="$HOME/cliproxyapi"
AUTH_DIR="$HOME/.cli-proxy-api"
SCRIPT_NAME="cliproxyapi-installer"
PUSH_TO_ORIGIN="${PUSH_TO_ORIGIN:-1}"
PUSH_TAGS="${PUSH_TAGS:-1}"

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

restore_stash() {
    local stash_ref="$1"
    if [[ -n "$stash_ref" ]]; then
        git stash pop --index -q "$stash_ref" >/dev/null 2>&1 || true
    fi
}

resolve_latest_release_tag() {
    local tag
    tag=$(git tag -l 'v*.*.*-*' --sort=-v:refname | head -1)
    if [[ -n "$tag" ]]; then
        echo "$tag"
        return 0
    fi

    if command -v gh >/dev/null 2>&1; then
        tag=$(gh release list --limit 100 --json tagName --jq '.[] | .tagName' 2>/dev/null \
            | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+-[0-9]+$' \
            | sort -Vr \
            | head -1 || true)
        if [[ -n "$tag" ]]; then
            echo "$tag"
            return 0
        fi
    fi

    return 1
}

is_service_running() {
    if systemctl --user is-active --quiet cliproxyapi.service 2>/dev/null; then
        return 0
    fi
    return 1
}

stop_service() {
    if is_service_running; then
        log_info "Stopping service..."
        systemctl --user stop cliproxyapi.service
    fi
}

stop_processes() {
    local pids
    pids=$(pgrep -f "cli-proxy-api" 2>/dev/null || true)
    if [[ -n "$pids" ]]; then
        log_info "Stopping processes..."
        echo "$pids" | while read -r pid; do
            [[ -n "$pid" ]] && kill "$pid" 2>/dev/null || true
        done
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

git_sync() {
    log_step "Git Sync (upstream)"

    if [[ ! -d "$REPO_DIR/.git" ]]; then
        log_error "Repository not found at $REPO_DIR"
        log_info "Clone it first: git clone https://github.com/HsnSaboor/CLIProxyAPIPlus.git $REPO_DIR"
        exit 1
    fi

    cd "$REPO_DIR"

    local stashed=0
    local stash_ref=""
    if [[ -n $(git status --porcelain 2>/dev/null) ]]; then
        log_info "Stashing uncommitted changes..."
        git stash push -u -m "${SCRIPT_NAME}-autostash-$(date +%Y%m%d-%H%M%S)" >/dev/null
        stash_ref="stash@{0}"
        stashed=1
    fi

    log_info "Refreshing release tags from origin..."
    local fetch_timeout="${FETCH_TIMEOUT:-120}"
    if ! GIT_TERMINAL_PROMPT=0 timeout "$fetch_timeout" git fetch origin --tags --prune 2>/dev/null; then
        log_warning "Failed to refresh origin tags (timeout=${fetch_timeout}s). Continuing with local/gh tag cache."
    else
        log_success "Refreshed origin tags"
    fi

    local latest_tag
    latest_tag=$(resolve_latest_release_tag || true)
    if [[ -z "$latest_tag" ]]; then
        log_error "No stable tag found matching v*.*.*-*"
        exit 1
    fi
    log_info "Latest release tag: $latest_tag"

    log_info "Merging $latest_tag..."
    if ! git merge "$latest_tag" --no-edit; then
        log_error "Merge conflict detected. Manual resolution required."
        log_info "Run: cd $REPO_DIR && git status"
        exit 1
    fi
    
    local commits_since_tag
    commits_since_tag=$(git rev-list --count "${latest_tag}..HEAD")
    local has_new_commits=0
    [[ "$commits_since_tag" -gt 0 ]] && has_new_commits=1
    
    log_success "Merged $latest_tag"

    if [[ "$PUSH_TO_ORIGIN" == "1" ]] && [[ $has_new_commits -eq 1 ]]; then
        log_info "Pushing to origin..."
        git push origin main
        log_success "Pushed to origin"
    elif [[ "$PUSH_TO_ORIGIN" == "1" ]]; then
        log_info "No new commits to push"
    fi

    if [[ "$PUSH_TAGS" == "1" ]] && [[ $has_new_commits -eq 1 ]]; then
        log_info "Creating and pushing release tag..."
        local base_version patch_num
        if [[ "$latest_tag" =~ ^(v[0-9]+\.[0-9]+\.[0-9]+)-([0-9]+)$ ]]; then
            base_version="${BASH_REMATCH[1]}"
            patch_num="${BASH_REMATCH[2]}"
        else
            log_error "Unexpected release tag format: $latest_tag"
            exit 1
        fi
        local new_tag="${base_version}-$((patch_num + 1))"
        
        if git tag -a "$new_tag" -m "Release $new_tag" 2>/dev/null; then
            git push origin "$new_tag"
            log_success "Created and pushed tag: $new_tag"
        else
            log_warning "Tag $new_tag already exists"
        fi
    elif [[ "$PUSH_TAGS" == "1" ]]; then
        log_info "No new commits, skipping tag creation"
    fi

    if [[ $stashed -eq 1 ]]; then
        log_info "Restoring stashed changes..."
        restore_stash "$stash_ref"
    fi
}

build_binary() {
    log_step "Building from source"
    cd "$REPO_DIR"
    
    log_info "Running go build..."
    if ! go build -o server ./cmd/server; then
        log_error "Build failed"
        exit 1
    fi
    log_success "Build complete"
}

deploy() {
    log_step "Deploying to $PROD_DIR"
    
    mkdir -p "$PROD_DIR"
    
    log_info "Backing up config..."
    if [[ -f "$PROD_DIR/config.yaml" ]]; then
        mkdir -p "$PROD_DIR/config_backup"
        chmod 700 "$PROD_DIR/config_backup"
        local ts
        ts=$(date +"%Y%m%d_%H%M%S")
        cp "$PROD_DIR/config.yaml" "$PROD_DIR/config_backup/config_${ts}.yaml"
    fi
    
    log_info "Backing up auth tokens..."
    if [[ -d "$AUTH_DIR" ]]; then
        mkdir -p "$PROD_DIR/config_backup"
        chmod 700 "$PROD_DIR/config_backup"
        local token_ts
        token_ts=$(date +"%Y%m%d_%H%M")
        tar -czf "$PROD_DIR/config_backup/tokens_${token_ts}.tar.gz" -C "$AUTH_DIR" . 2>/dev/null || true
    fi
    
    if [[ -f "$PROD_DIR/server" ]]; then
        mv "$PROD_DIR/server" "$PROD_DIR/server.backup"
    fi
    
    cp "$REPO_DIR/server" "$PROD_DIR/"
    chmod +x "$PROD_DIR/server"
    
    if [[ ! -f "$PROD_DIR/config.yaml" ]]; then
        cp "$REPO_DIR/config.example.yaml" "$PROD_DIR/config.yaml"
        local key1 key2
        key1=$(generate_api_key)
        key2=$(generate_api_key)
        sed -i "s|\"your-api-key-1\"|\"$key1\"|g" "$PROD_DIR/config.yaml"
        sed -i "s|\"your-api-key-2\"|\"$key2\"|g" "$PROD_DIR/config.yaml"
        log_success "Created config.yaml with generated API keys"
    fi
    
    log_success "Deployed to $PROD_DIR"
}

create_systemd_service() {
    local systemd_dir="$HOME/.config/systemd/user"
    mkdir -p "$systemd_dir"

    cat > "$systemd_dir/cliproxyapi.service" << EOF
[Unit]
Description=CLIProxyAPI Service
After=network.target

[Service]
Type=simple
WorkingDirectory=$PROD_DIR
ExecStart=$PROD_DIR/server
Restart=always
RestartSec=10

[Install]
WantedBy=default.target
EOF

    systemctl --user daemon-reload || true
    log_success "Created systemd service"
}

start_service() {
    log_step "Starting service"
    
    create_systemd_service
    
    systemctl --user enable cliproxyapi.service 2>/dev/null || true
    systemctl --user restart cliproxyapi.service
    sleep 3
    
    if is_service_running; then
        log_success "Service is running"
    else
        log_warning "Service not running, check logs: journalctl --user -u cliproxyapi.service"
    fi
}

show_status() {
    echo
    echo "CLIProxyAPIPlus - Status"
    echo "========================"
    echo "Repo Dir:    $REPO_DIR"
    echo "Install Dir: $PROD_DIR"
    echo "Auth Dir:    $AUTH_DIR"
    echo
    [[ -f "$PROD_DIR/server" ]] && echo "Binary:      Present" || echo "Binary:      Missing"
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

main() {
    case "${1:-install}" in
        install|upgrade)
            log_step "CLIProxyAPIPlus Dev Installer"
            stop_service
            stop_processes
            git_sync
            build_binary
            deploy
            start_service
            
            log_success "Installation complete!"
            echo
            log_info "Binary: $PROD_DIR/server"
            log_info "Config: $PROD_DIR/config.yaml"
            
            if ! check_api_keys; then
                echo
                log_warning "Configure API keys: nano $PROD_DIR/config.yaml"
            fi
            ;;
        status)
            show_status
            ;;
        -h|--help)
            cat << EOF
CLIProxyAPIPlus Dev Installer

Usage: $SCRIPT_NAME [command]

Commands:
  install, upgrade   Sync upstream, build, and deploy (default)
  status             Show installation status
  -h, --help        This help

Environment:
  PUSH_TO_ORIGIN=0   Skip pushing to origin
  PUSH_TAGS=0        Skip creating release tags
  FETCH_TIMEOUT=120  Git fetch timeout in seconds

EOF
            ;;
        *)
            log_error "Unknown command: $1"
            exit 1
            ;;
    esac
}

main "$@"
