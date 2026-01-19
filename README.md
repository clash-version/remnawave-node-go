# Remnawave Node Go

Go implementation of [remnawave/node](https://github.com/remnawave/node).

**Features:** Single binary, single port, embedded Xray-core, zero dependencies.

## Installation

### One-line Install (Recommended)

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/clash-version/remnawave-node-go/main/install.sh)
```

After installation:

```bash
remnawave-node-config set-payload <YOUR_SECRET_KEY>
systemctl enable --now remnawave-node
```

### Docker

```bash
docker run -d \
  --name remnawave-node \
  --restart always \
  -e SECRET_KEY="<YOUR_SECRET_KEY>" \
  -p 3000:3000 \
  ghcr.io/clash-version/remnawave-node-go:latest
```

### Manual Install

1. Download binary:

```bash
VERSION=$(curl -s https://api.github.com/repos/clash-version/remnawave-node-go/releases/latest | grep tag_name | cut -d '"' -f 4)
curl -Lo remnawave-node https://github.com/clash-version/remnawave-node-go/releases/download/${VERSION}/remnawave-node_linux_amd64
chmod +x remnawave-node
sudo mv remnawave-node /usr/local/bin/
```

2. Create systemd service:

```bash
sudo tee /etc/systemd/system/remnawave-node.service << EOF
[Unit]
Description=Remnawave Node
After=network.target

[Service]
Type=simple
Environment=SECRET_KEY=<YOUR_SECRET_KEY>
Environment=NODE_PORT=3000
ExecStart=/usr/local/bin/remnawave-node
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
```

3. Start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now remnawave-node
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SECRET_KEY` | Yes | - | Key from panel |
| `NODE_PORT` | No | 3000 | Listen port |

## Commands

```bash
systemctl status remnawave-node     # Status
journalctl -u remnawave-node -f     # Logs
systemctl restart remnawave-node    # Restart
```

## Project Structure

```
cmd/node/           # Entry point
internal/
  config/           # Configuration
  middleware/       # HTTP middleware
  server/           # HTTP server
  services/         # Business logic
pkg/
  crypto/           # Key parsing
  logger/           # Logging
  xraycore/         # Embedded Xray-core
```

## Comparison with Node.js Version

| | Node.js | Go |
|---|---|---|
| Deploy | Node.js + Xray + Supervisord | Single binary |
| Ports | 4 | 1 |
| Memory | ~150MB | ~30MB |

## License

MIT