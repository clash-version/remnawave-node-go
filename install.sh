#!/bin/bash

# Remnawave Node Go - One-click Installation Script
# Usage: curl -fsSL https://raw.githubusercontent.com/clash-version/remnawave-node-go/main/install.sh | bash
# Or: wget -qO- https://raw.githubusercontent.com/clash-version/remnawave-node-go/main/install.sh | bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
GITHUB_REPO="clash-version/remnawave-node-go"
BINARY_NAME="remnawave-node"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/remnawave-node"
DATA_DIR="/var/lib/remnawave-node"
SERVICE_NAME="remnawave-node"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

# Print banner
print_banner() {
    echo -e "${CYAN}"
    echo "╔═══════════════════════════════════════════════════════════╗"
    echo "║                                                           ║"
    echo "║           Remnawave Node Go Installer                     ║"
    echo "║                                                           ║"
    echo "╚═══════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

# Print colored messages
info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "This script must be run as root. Please use 'sudo' or run as root user."
    fi
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        *)
            error "Unsupported operating system: $OS"
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l|armv7)
            ARCH="armv7"
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac

    PLATFORM="${OS}_${ARCH}"
    info "Detected platform: ${PLATFORM}"
}

# Check required commands
check_dependencies() {
    local missing_deps=()

    for cmd in curl jq tar; do
        if ! command -v "$cmd" &> /dev/null; then
            missing_deps+=("$cmd")
        fi
    done

    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        info "Installing missing dependencies: ${missing_deps[*]}"
        
        if command -v apt-get &> /dev/null; then
            apt-get update -qq
            apt-get install -y -qq "${missing_deps[@]}"
        elif command -v yum &> /dev/null; then
            yum install -y -q "${missing_deps[@]}"
        elif command -v dnf &> /dev/null; then
            dnf install -y -q "${missing_deps[@]}"
        elif command -v pacman &> /dev/null; then
            pacman -Sy --noconfirm "${missing_deps[@]}"
        elif command -v apk &> /dev/null; then
            apk add --no-cache "${missing_deps[@]}"
        else
            error "Could not install dependencies. Please install manually: ${missing_deps[*]}"
        fi
    fi
}

# Get latest release version from GitHub
get_latest_version() {
    info "Fetching latest release version..."
    
    LATEST_VERSION=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | jq -r '.tag_name')
    
    if [[ -z "$LATEST_VERSION" || "$LATEST_VERSION" == "null" ]]; then
        error "Failed to get latest version from GitHub"
    fi
    
    info "Latest version: ${LATEST_VERSION}"
}

# Download and install binary
download_and_install() {
    local version="${1:-$LATEST_VERSION}"
    local download_url="https://github.com/${GITHUB_REPO}/releases/download/${version}/${BINARY_NAME}_${version#v}_${PLATFORM}.tar.gz"
    local temp_dir=$(mktemp -d)
    local archive_file="${temp_dir}/${BINARY_NAME}.tar.gz"

    info "Downloading ${BINARY_NAME} ${version} for ${PLATFORM}..."
    info "URL: ${download_url}"

    if ! curl -fsSL -o "$archive_file" "$download_url"; then
        rm -rf "$temp_dir"
        error "Failed to download binary. Please check if the release exists for your platform."
    fi

    info "Extracting archive..."
    tar -xzf "$archive_file" -C "$temp_dir"

    # Find the binary (it might be in a subdirectory)
    local binary_path=$(find "$temp_dir" -name "$BINARY_NAME" -type f -executable 2>/dev/null | head -1)
    
    if [[ -z "$binary_path" ]]; then
        # Try without executable flag (in case permissions aren't set)
        binary_path=$(find "$temp_dir" -name "$BINARY_NAME" -type f 2>/dev/null | head -1)
    fi

    if [[ -z "$binary_path" ]]; then
        # List contents for debugging
        warning "Archive contents:"
        ls -la "$temp_dir"
        rm -rf "$temp_dir"
        error "Binary not found in archive"
    fi

    # Stop service if running
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        info "Stopping existing service..."
        systemctl stop "$SERVICE_NAME"
    fi

    # Install binary
    info "Installing binary to ${INSTALL_DIR}..."
    chmod +x "$binary_path"
    mv "$binary_path" "${INSTALL_DIR}/${BINARY_NAME}"

    # Cleanup
    rm -rf "$temp_dir"

    success "Binary installed successfully"
}

# Create directories
create_directories() {
    info "Creating directories..."
    
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$DATA_DIR"
    
    chmod 755 "$CONFIG_DIR"
    chmod 755 "$DATA_DIR"
}

# Create systemd service
create_service() {
    info "Creating systemd service..."

    cat > "$SERVICE_FILE" << 'EOF'
[Unit]
Description=Remnawave Node Go
Documentation=https://github.com/clash-version/remnawave-node-go
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
ExecStart=/usr/local/bin/remnawave-node
Restart=always
RestartSec=5
LimitNOFILE=65535
StandardOutput=journal
StandardError=journal
SyslogIdentifier=remnawave-node

# Environment file (optional)
EnvironmentFile=-/etc/remnawave-node/env

# Security hardening (optional, can be enabled if needed)
# NoNewPrivileges=true
# ProtectSystem=strict
# ProtectHome=true
# PrivateTmp=true
# ReadWritePaths=/var/lib/remnawave-node

[Install]
WantedBy=multi-user.target
EOF

    # Create default environment file if not exists
    if [[ ! -f "${CONFIG_DIR}/env" ]]; then
        cat > "${CONFIG_DIR}/env" << 'EOF'
# Remnawave Node Go Environment Configuration
# Uncomment and modify as needed

# SSL Certificate paths (required)
# SSL_CERT_PATH=/etc/remnawave-node/cert.pem
# SSL_KEY_PATH=/etc/remnawave-node/key.pem

# Node payload (base64 encoded, from panel)
# REMNAWAVE_NODE_PAYLOAD=

# API Port (default: from payload)
# API_PORT=443

# Xray API Port (default: 61000)
# XRAY_API_PORT=61000

# Log level: debug, info, warn, error
# LOG_LEVEL=info

# Disable hash check for config comparison
# DISABLE_HASHED_SET_CHECK=false
EOF
    fi

    # Reload systemd
    systemctl daemon-reload
    
    success "Systemd service created"
}

# Create helper script for configuration
create_config_helper() {
    local helper_script="${INSTALL_DIR}/remnawave-node-config"
    
    cat > "$helper_script" << 'SCRIPT'
#!/bin/bash

CONFIG_DIR="/etc/remnawave-node"
ENV_FILE="${CONFIG_DIR}/env"

show_help() {
    echo "Remnawave Node Configuration Helper"
    echo ""
    echo "Usage: remnawave-node-config <command> [options]"
    echo ""
    echo "Commands:"
    echo "  set-payload <payload>   Set the node payload (base64 string from panel)"
    echo "  set-cert <cert> <key>   Set SSL certificate paths"
    echo "  show                    Show current configuration"
    echo "  edit                    Edit configuration file"
    echo "  status                  Show service status"
    echo "  logs                    Show service logs"
    echo "  restart                 Restart the service"
    echo ""
}

set_payload() {
    local payload="$1"
    if [[ -z "$payload" ]]; then
        echo "Error: Payload is required"
        exit 1
    fi
    
    # Update or add payload in env file
    if grep -q "^REMNAWAVE_NODE_PAYLOAD=" "$ENV_FILE" 2>/dev/null; then
        sed -i "s|^REMNAWAVE_NODE_PAYLOAD=.*|REMNAWAVE_NODE_PAYLOAD=${payload}|" "$ENV_FILE"
    else
        echo "REMNAWAVE_NODE_PAYLOAD=${payload}" >> "$ENV_FILE"
    fi
    
    echo "Payload updated. Restart service to apply: systemctl restart remnawave-node"
}

set_cert() {
    local cert="$1"
    local key="$2"
    
    if [[ -z "$cert" || -z "$key" ]]; then
        echo "Error: Both certificate and key paths are required"
        exit 1
    fi
    
    # Update cert paths
    if grep -q "^SSL_CERT_PATH=" "$ENV_FILE" 2>/dev/null; then
        sed -i "s|^SSL_CERT_PATH=.*|SSL_CERT_PATH=${cert}|" "$ENV_FILE"
    else
        echo "SSL_CERT_PATH=${cert}" >> "$ENV_FILE"
    fi
    
    if grep -q "^SSL_KEY_PATH=" "$ENV_FILE" 2>/dev/null; then
        sed -i "s|^SSL_KEY_PATH=.*|SSL_KEY_PATH=${key}|" "$ENV_FILE"
    else
        echo "SSL_KEY_PATH=${key}" >> "$ENV_FILE"
    fi
    
    echo "Certificate paths updated. Restart service to apply: systemctl restart remnawave-node"
}

show_config() {
    echo "Current configuration (${ENV_FILE}):"
    echo "----------------------------------------"
    if [[ -f "$ENV_FILE" ]]; then
        grep -v "^#" "$ENV_FILE" | grep -v "^$"
    else
        echo "(No configuration file found)"
    fi
}

case "$1" in
    set-payload)
        set_payload "$2"
        ;;
    set-cert)
        set_cert "$2" "$3"
        ;;
    show)
        show_config
        ;;
    edit)
        ${EDITOR:-nano} "$ENV_FILE"
        ;;
    status)
        systemctl status remnawave-node
        ;;
    logs)
        journalctl -u remnawave-node -f
        ;;
    restart)
        systemctl restart remnawave-node
        ;;
    *)
        show_help
        ;;
esac
SCRIPT

    chmod +x "$helper_script"
    success "Configuration helper created: ${helper_script}"
}

# Print post-installation instructions
print_instructions() {
    echo ""
    echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}    Installation Complete!${NC}"
    echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "${CYAN}Next steps:${NC}"
    echo ""
    echo "1. Configure your node (get payload from Remnawave panel):"
    echo -e "   ${YELLOW}remnawave-node-config set-payload <your-payload>${NC}"
    echo ""
    echo "2. Set SSL certificate paths:"
    echo -e "   ${YELLOW}remnawave-node-config set-cert /path/to/cert.pem /path/to/key.pem${NC}"
    echo ""
    echo "3. Or edit configuration directly:"
    echo -e "   ${YELLOW}nano /etc/remnawave-node/env${NC}"
    echo ""
    echo "4. Start the service:"
    echo -e "   ${YELLOW}systemctl start remnawave-node${NC}"
    echo ""
    echo "5. Enable auto-start on boot:"
    echo -e "   ${YELLOW}systemctl enable remnawave-node${NC}"
    echo ""
    echo -e "${CYAN}Useful commands:${NC}"
    echo "  - Check status:    systemctl status remnawave-node"
    echo "  - View logs:       journalctl -u remnawave-node -f"
    echo "  - Restart:         systemctl restart remnawave-node"
    echo "  - Config helper:   remnawave-node-config --help"
    echo ""
    echo -e "${CYAN}Files:${NC}"
    echo "  - Binary:          ${INSTALL_DIR}/${BINARY_NAME}"
    echo "  - Config:          ${CONFIG_DIR}/env"
    echo "  - Data:            ${DATA_DIR}"
    echo "  - Service:         ${SERVICE_FILE}"
    echo ""
}

# Uninstall function
uninstall() {
    warning "Uninstalling Remnawave Node Go..."
    
    # Stop and disable service
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        systemctl stop "$SERVICE_NAME"
    fi
    if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
        systemctl disable "$SERVICE_NAME"
    fi
    
    # Remove files
    rm -f "${INSTALL_DIR}/${BINARY_NAME}"
    rm -f "${INSTALL_DIR}/remnawave-node-config"
    rm -f "$SERVICE_FILE"
    
    # Reload systemd
    systemctl daemon-reload
    
    success "Uninstallation complete"
    info "Configuration directory ${CONFIG_DIR} was preserved"
    info "To remove all data: rm -rf ${CONFIG_DIR} ${DATA_DIR}"
}

# Update function
update() {
    info "Updating Remnawave Node Go..."
    
    get_latest_version
    
    # Check current version
    if [[ -x "${INSTALL_DIR}/${BINARY_NAME}" ]]; then
        CURRENT_VERSION=$("${INSTALL_DIR}/${BINARY_NAME}" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
        info "Current version: ${CURRENT_VERSION}"
    fi
    
    download_and_install
    
    # Restart service if it was running
    if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
        info "Restarting service..."
        systemctl restart "$SERVICE_NAME"
    fi
    
    success "Update complete!"
}

# Main installation function
install() {
    print_banner
    check_root
    detect_platform
    check_dependencies
    get_latest_version
    download_and_install
    create_directories
    create_service
    create_config_helper
    print_instructions
}

# Parse command line arguments
case "${1:-install}" in
    install)
        install
        ;;
    update|upgrade)
        check_root
        detect_platform
        check_dependencies
        update
        ;;
    uninstall|remove)
        check_root
        uninstall
        ;;
    --help|-h)
        echo "Remnawave Node Go Installer"
        echo ""
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  install     Install Remnawave Node Go (default)"
        echo "  update      Update to latest version"
        echo "  uninstall   Remove Remnawave Node Go"
        echo "  --help      Show this help message"
        echo ""
        echo "One-line install:"
        echo "  curl -fsSL https://raw.githubusercontent.com/${GITHUB_REPO}/main/install.sh | bash"
        ;;
    *)
        error "Unknown command: $1. Use --help for usage."
        ;;
esac
