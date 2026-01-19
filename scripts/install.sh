#!/bin/bash

# Remnawave Node Installation Script
# Usage: ./install.sh -h <API_HOST> -t <TOKEN>

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="/opt/remnawave-node"
BIN_NAME="remnawave-node"
SERVICE_NAME="remnawave-node"
CONFIG_DIR="/etc/remnawave-node"
DATA_DIR="/var/lib/remnawave-node"
LOG_DIR="/var/log/remnawave-node"
GITHUB_REPO="clash-version/remnawave-node-go"

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

show_help() {
    cat << EOF
Remnawave Node Installation Script

Usage: $0 [OPTIONS]

Options:
    -h, --host <URL>        API host URL (required for new installation)
    -t, --token <TOKEN>     Node token/secret key (required for new installation)
    -p, --port <PORT>       Node port (default: 3000)
    -u, --uninstall         Uninstall remnawave-node
    --upgrade               Upgrade to latest version
    --version <VERSION>     Install specific version
    --help                  Show this help message

Examples:
    # New installation
    $0 -h https://api.remnawave.com -t your_secret_key

    # Install with custom port
    $0 -h https://api.remnawave.com -t your_secret_key -p 8080

    # Upgrade to latest version
    $0 --upgrade

    # Uninstall
    $0 -u

EOF
    exit 0
}

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "Please run as root (use sudo)"
        exit 1
    fi
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$ARCH" in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l)
            ARCH="armv7"
            ;;
        *)
            log_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    case "$OS" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        *)
            log_error "Unsupported operating system: $OS"
            exit 1
            ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    log_info "Detected platform: $PLATFORM"
}

# Download binary from GitHub releases
download_binary() {
    local version="$1"
    
    if [ -z "$version" ] || [ "$version" == "latest" ]; then
        log_info "Fetching latest version..."
        version=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
        if [ -z "$version" ]; then
            log_error "Failed to fetch latest version"
            exit 1
        fi
    fi

    log_info "Downloading version: $version"
    
    local download_url="https://github.com/${GITHUB_REPO}/releases/download/${version}/${BIN_NAME}_${PLATFORM}.tar.gz"
    local tmp_file="/tmp/${BIN_NAME}.tar.gz"

    if ! curl -L -o "$tmp_file" "$download_url"; then
        log_error "Failed to download binary from $download_url"
        exit 1
    fi

    # Extract binary
    mkdir -p "$INSTALL_DIR"
    tar -xzf "$tmp_file" -C "$INSTALL_DIR"
    chmod +x "${INSTALL_DIR}/${BIN_NAME}"
    rm -f "$tmp_file"

    log_info "Binary installed to ${INSTALL_DIR}/${BIN_NAME}"
}

# Create systemd service file
create_service() {
    local port="$1"
    local secret_key="$2"

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=Remnawave Node Service
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
Environment=NODE_PORT=${port}
Environment=SECRET_KEY=${secret_key}
Environment=XTLS_IP=127.0.0.1
Environment=XTLS_PORT=61000
ExecStart=${INSTALL_DIR}/${BIN_NAME}
Restart=always
RestartSec=5
LimitNOFILE=65535

# Security
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=${DATA_DIR} ${LOG_DIR}

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    log_info "Systemd service created"
}

# Create Xray service (managed by supervisord)
create_xray_service() {
    mkdir -p /etc/supervisor/conf.d

    cat > /etc/supervisor/conf.d/xray.conf << EOF
[program:xray]
command=/usr/local/bin/xray run -c ${DATA_DIR}/config.json
directory=${DATA_DIR}
autostart=false
autorestart=true
startsecs=5
startretries=3
redirect_stderr=true
stdout_logfile=${LOG_DIR}/xray.log
stdout_logfile_maxbytes=10MB
stdout_logfile_backups=3
EOF

    log_info "Xray supervisor config created"
}

# Install dependencies
install_dependencies() {
    log_info "Installing dependencies..."

    # Detect package manager
    if command -v apt-get &> /dev/null; then
        apt-get update
        apt-get install -y curl supervisor
    elif command -v yum &> /dev/null; then
        yum install -y curl supervisor
    elif command -v dnf &> /dev/null; then
        dnf install -y curl supervisor
    elif command -v apk &> /dev/null; then
        apk add --no-cache curl supervisor
    else
        log_warn "Could not detect package manager, please install curl and supervisor manually"
    fi
}

# Install Xray-core
install_xray() {
    log_info "Installing Xray-core..."
    
    # Use official Xray install script
    bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install

    # Disable default Xray service (we manage it via supervisord)
    systemctl disable xray --now 2>/dev/null || true

    log_info "Xray-core installed"
}

# Create directories
create_directories() {
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    mkdir -p "$LOG_DIR"
    chmod 755 "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"
}

# Save configuration
save_config() {
    local host="$1"
    local token="$2"
    local port="$3"

    cat > "${CONFIG_DIR}/config.env" << EOF
# Remnawave Node Configuration
# Generated at $(date)

NODE_PORT=${port}
SECRET_KEY=${token}
API_HOST=${host}
XTLS_IP=127.0.0.1
XTLS_PORT=61000
EOF

    chmod 600 "${CONFIG_DIR}/config.env"
    log_info "Configuration saved to ${CONFIG_DIR}/config.env"
}

# Start service
start_service() {
    systemctl enable "$SERVICE_NAME"
    systemctl start "$SERVICE_NAME"
    
    # Start supervisor for xray management
    systemctl enable supervisor
    systemctl start supervisor
    
    log_info "Service started"
}

# Stop service
stop_service() {
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    systemctl disable "$SERVICE_NAME" 2>/dev/null || true
}

# Uninstall
uninstall() {
    log_info "Uninstalling Remnawave Node..."
    
    stop_service
    
    rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
    rm -f "/etc/supervisor/conf.d/xray.conf"
    rm -rf "$INSTALL_DIR"
    rm -rf "$CONFIG_DIR"
    
    systemctl daemon-reload
    supervisorctl update 2>/dev/null || true
    
    log_info "Uninstallation complete"
    log_warn "Data directory ${DATA_DIR} and logs ${LOG_DIR} were preserved. Remove manually if needed."
}

# Upgrade
upgrade() {
    log_info "Upgrading Remnawave Node..."
    
    # Stop service
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    
    # Backup current binary
    if [ -f "${INSTALL_DIR}/${BIN_NAME}" ]; then
        mv "${INSTALL_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}.bak"
    fi
    
    # Download new version
    download_binary "$VERSION"
    
    # Start service
    systemctl start "$SERVICE_NAME"
    
    # Remove backup on success
    rm -f "${INSTALL_DIR}/${BIN_NAME}.bak"
    
    log_info "Upgrade complete"
}

# Main installation
install() {
    local host="$1"
    local token="$2"
    local port="$3"

    log_info "Starting Remnawave Node installation..."

    # Check requirements
    if [ -z "$host" ] || [ -z "$token" ]; then
        log_error "API host (-h) and token (-t) are required"
        show_help
    fi

    # Detect platform
    detect_platform

    # Install dependencies
    install_dependencies

    # Create directories
    create_directories

    # Install Xray
    install_xray

    # Download binary
    download_binary "$VERSION"

    # Save config
    save_config "$host" "$token" "$port"

    # Create services
    create_service "$port" "$token"
    create_xray_service

    # Start service
    start_service

    log_info "Installation complete!"
    echo ""
    echo "==================================="
    echo "Remnawave Node is now running"
    echo "==================================="
    echo "Port: $port"
    echo "Config: ${CONFIG_DIR}/config.env"
    echo "Logs: journalctl -u ${SERVICE_NAME} -f"
    echo ""
    echo "Commands:"
    echo "  Status:  systemctl status ${SERVICE_NAME}"
    echo "  Logs:    journalctl -u ${SERVICE_NAME} -f"
    echo "  Restart: systemctl restart ${SERVICE_NAME}"
    echo ""
}

# Parse arguments
API_HOST=""
TOKEN=""
PORT="3000"
VERSION="latest"
UNINSTALL=false
UPGRADE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--host)
            API_HOST="$2"
            shift 2
            ;;
        -t|--token)
            TOKEN="$2"
            shift 2
            ;;
        -p|--port)
            PORT="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        -u|--uninstall)
            UNINSTALL=true
            shift
            ;;
        --upgrade)
            UPGRADE=true
            shift
            ;;
        --help)
            show_help
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            ;;
    esac
done

# Execute
check_root

if [ "$UNINSTALL" = true ]; then
    uninstall
elif [ "$UPGRADE" = true ]; then
    upgrade
else
    install "$API_HOST" "$TOKEN" "$PORT"
fi
