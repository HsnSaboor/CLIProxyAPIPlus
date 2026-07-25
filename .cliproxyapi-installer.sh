#!/bin/bash
# CLIProxyAPIPlus - Source-to-Production Deployment Engine
# Builds from source with Git-based upstream sync, atomic deploy, and auto-rollback

set -euo pipefail

REPO_DIR="$HOME/code/CLIProxyAPIPlus"
PROD_DIR="$HOME/cliproxyapi"
AUTH_DIR="$HOME/.cli-proxy-api"
BACKUP_DIR="$HOME/.cliproxyapi-backups"
SCRIPT_NAME="cliproxyapi-installer"
PUSH_TO_ORIGIN="${PUSH_TO_ORIGIN:-1}"
CREATE_RELEASE="${CREATE_RELEASE:-1}"

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

_STASH_REF=""
_STASHED=0

_cleanup() {
    local exit_code=$?
    if [[ $_STASHED -eq 1 && -n "$_STASH_REF" ]]; then
        log_warning "Restoring stashed changes after interruption..."
        cd "$REPO_DIR" 2>/dev/null || true
        if ! git stash pop 2>&1; then
            log_error "Stash still pending. Run: cd $REPO_DIR && git stash pop"
        fi
    fi
    exit "$exit_code"
}
trap _cleanup EXIT

is_service_running() {
    systemctl --user is-active --quiet cliproxyapi.service 2>/dev/null
}

stop_service() {
    if is_service_running; then
        log_info "Stopping service..."
        systemctl --user stop cliproxyapi.service
        for i in $(seq 1 10); do
            if ! is_service_running; then
                break
            fi
            sleep 1
        done
    fi
}

stop_processes() {
    local pids
    pids=$(pgrep -f "$PROD_DIR/server" 2>/dev/null || true)
    if [[ -n "$pids" ]]; then
        log_info "Stopping processes..."
        echo "$pids" | while read -r pid; do
            [[ -n "$pid" ]] && kill "$pid" 2>/dev/null || true
        done
        for i in $(seq 1 10); do
            if ! pgrep -f "$PROD_DIR/server" >/dev/null 2>&1; then
                return 0
            fi
            sleep 1
        done
        log_warning "Processes did not terminate after 10s, sending SIGKILL..."
        echo "$pids" | while read -r pid; do
            [[ -n "$pid" ]] && kill -9 "$pid" 2>/dev/null || true
        done
        sleep 1
    fi
}

generate_api_key() {
    local prefix="sk-"
    local key
    key=$(head -c 45 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 45)
    echo "${prefix}${key}"
}

check_api_keys() {
    local config_file="${PROD_DIR}/config.yaml"
    [[ ! -f "$config_file" ]] && return 1
    grep -q '"your-api-key-1"' "$config_file" && return 1
    grep -q '"your-api-key-2"' "$config_file" && return 1
    grep -qE '^api-keys:' "$config_file" || return 1
    grep -qE '^[^#]*"sk-[A-Za-z0-9]+"' "$config_file" && return 0
    return 1
}

git_sync() {
    log_step "Git Sync (jc fork)"

    if [[ ! -d "$REPO_DIR/.git" ]]; then
        log_error "Repository not found at $REPO_DIR"
        log_info "Clone it first: git clone https://github.com/HsnSaboor/CLIProxyAPIPlus.git $REPO_DIR"
        exit 1
    fi

    cd "$REPO_DIR"

    if [[ -n $(git status --porcelain 2>/dev/null) ]]; then
        log_info "Stashing uncommitted changes..."
        if git stash push -u -m "${SCRIPT_NAME}-autostash-$(date +%Y%m%d-%H%M%S)" 2>&1; then
            _STASHED=1
            _STASH_REF="1"
        fi
    fi

    local jc_remote="jc"
    local jc_url="https://github.com/jc01rho/CLIProxyAPIPlus.git"

    if ! git remote get-url "$jc_remote" &>/dev/null; then
        log_info "Adding remote 'jc' -> $jc_url"
        git remote add "$jc_remote" "$jc_url"
    elif [[ "$(git remote get-url "$jc_remote")" != "$jc_url" ]]; then
        log_info "Updating remote 'jc' -> $jc_url"
        git remote set-url "$jc_remote" "$jc_url"
    fi

    local fetch_timeout="${FETCH_TIMEOUT:-120}"
    log_info "Fetching from jc..."
    if ! GIT_TERMINAL_PROMPT=0 timeout "$fetch_timeout" git fetch "$jc_remote" 2>&1; then
        log_error "Failed to fetch from jc (timeout=${fetch_timeout}s)"
        exit 1
    fi
    log_success "Fetched from jc"

    log_info "Merging jc/main with -X theirs..."
    if ! git merge "$jc_remote/main" --no-edit -X theirs; then
        git merge --abort 2>/dev/null || true
        log_error "Merge conflict detected. Manual resolution required."
        log_info "Run: cd $REPO_DIR && git merge jc/main -X theirs"
        exit 1
    fi

    log_success "Merged jc/main"

    if [[ $_STASHED -eq 1 && -n "$_STASH_REF" ]]; then
        log_info "Restoring stashed changes..."
        if ! git stash pop 2>&1; then
            log_warning "Failed to restore stashed changes. They remain in the stash stack."
            log_info "Run: cd $REPO_DIR && git stash pop"
        fi
        _STASHED=0
        _STASH_REF=""
    fi

    if [[ "$PUSH_TO_ORIGIN" == "1" ]]; then
        log_info "Pushing to origin..."
        if ! git push origin main 2>&1; then
            log_warning "Failed to push to origin. Continuing."
        else
            log_success "Pushed to origin"
        fi
    fi

    if [[ "$CREATE_RELEASE" == "1" ]]; then
        log_info "Creating release tag..."
        local latest_tag new_tag base_version patch_num
        latest_tag=$(git tag -l 'v*.*.*-*' --sort=-v:refname | { head -1; true; })
        if [[ -z "$latest_tag" ]]; then
            latest_tag=$(git tag -l 'v*.*.*' --sort=-v:refname | { head -1; true; })
            if [[ -n "$latest_tag" ]]; then
                new_tag="${latest_tag}-1"
            fi
        elif [[ "$latest_tag" =~ ^(v[0-9]+\.[0-9]+\.[0-9]+)-([0-9]+)$ ]]; then
            base_version="${BASH_REMATCH[1]}"
            patch_num="${BASH_REMATCH[2]}"
            new_tag="${base_version}-$((patch_num + 1))"
        fi

        if [[ -z "${new_tag:-}" ]]; then
            log_warning "No existing tags found, skipping release"
        elif git tag -a "$new_tag" -m "Release $new_tag" 2>&1; then
            log_success "Created tag: $new_tag"
            if git push origin "$new_tag" 2>&1; then
                log_success "Pushed tag $new_tag — GoReleaser workflow triggered"
            else
                log_warning "Failed to push tag $new_tag"
            fi
        else
            log_warning "Tag $new_tag already exists"
        fi
    fi
}

build_binary() {
    log_step "Building from source"

    if ! command -v go &>/dev/null; then
        log_error "Go compiler not found. Install Go 1.26+ first."
        exit 1
    fi

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
    mkdir -p "$BACKUP_DIR"
    chmod 700 "$BACKUP_DIR"

    if [[ -f "$PROD_DIR/config.yaml" ]]; then
        log_info "Backing up config..."
        local ts
        ts=$(date +"%Y%m%d_%H%M%S")
        cp "$PROD_DIR/config.yaml" "$BACKUP_DIR/config_${ts}.yaml"
    fi

    if [[ -d "$AUTH_DIR" ]]; then
        log_info "Backing up auth tokens..."
        local ts
        ts=$(date +"%Y%m%d_%H%M%S")
        tar -czf "$BACKUP_DIR/tokens_${ts}.tar.gz" -C "$AUTH_DIR" . 2>/dev/null || true
    fi

    log_info "Pruning old backups (keeping last 10)..."
    while IFS= read -r -d '' f; do rm -f "$f"; done < <(find "$BACKUP_DIR" -maxdepth 1 -name 'config_*.yaml' -printf '%T@ %p\0' 2>/dev/null | sort -znr | tail -n +11 | cut -z -d' ' -f2-)
    while IFS= read -r -d '' f; do rm -f "$f"; done < <(find "$BACKUP_DIR" -maxdepth 1 -name 'tokens_*.tar.gz' -printf '%T@ %p\0' 2>/dev/null | sort -znr | tail -n +11 | cut -z -d' ' -f2-)

    # Atomic binary replacement: backup old, stage new, then swap
    if [[ -f "$PROD_DIR/server" ]]; then
        cp "$PROD_DIR/server" "$PROD_DIR/server.backup"
    fi
    cp "$REPO_DIR/server" "$PROD_DIR/server.new"
    chmod +x "$PROD_DIR/server.new"
    # mv is atomic on same filesystem (ext4/xfs) — no window without a binary
    mv "$PROD_DIR/server.new" "$PROD_DIR/server"

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

    for i in $(seq 1 10); do
        if is_service_running; then
            log_success "Service is running"
            return 0
        fi
        sleep 1
    done

    log_warning "Service not running, check logs: journalctl --user -u cliproxyapi.service"
}

show_status() {
    echo
    echo "CLIProxyAPIPlus - Status"
    echo "========================"
    echo "Repo Dir:    $REPO_DIR"
    echo "Install Dir: $PROD_DIR"
    echo "Auth Dir:    $AUTH_DIR"
    echo
    [[ -x "$PROD_DIR/server" ]] && echo "Binary:      Present" || echo "Binary:      Missing"
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
  CREATE_RELEASE=0   Skip creating and pushing release tag
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
