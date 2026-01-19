# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /remnawave-node ./cmd/node

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    supervisor \
    curl \
    bash

# Install Xray-core
RUN curl -L -o /tmp/xray.zip https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip && \
    unzip /tmp/xray.zip -d /tmp/xray && \
    mv /tmp/xray/xray /usr/local/bin/xray && \
    chmod +x /usr/local/bin/xray && \
    rm -rf /tmp/xray /tmp/xray.zip

# Create directories
RUN mkdir -p /var/lib/remnawave-node /var/log/remnawave-node /etc/remnawave-node

# Copy binary from builder
COPY --from=builder /remnawave-node /usr/local/bin/remnawave-node

# Copy supervisor config
COPY deploy/supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# Create xray supervisor config
RUN echo '[program:xray]' > /etc/supervisor/conf.d/xray.conf && \
    echo 'command=/usr/local/bin/xray run -c /var/lib/remnawave-node/config.json' >> /etc/supervisor/conf.d/xray.conf && \
    echo 'autostart=false' >> /etc/supervisor/conf.d/xray.conf && \
    echo 'autorestart=true' >> /etc/supervisor/conf.d/xray.conf && \
    echo 'redirect_stderr=true' >> /etc/supervisor/conf.d/xray.conf && \
    echo 'stdout_logfile=/var/log/remnawave-node/xray.log' >> /etc/supervisor/conf.d/xray.conf

# Environment variables
ENV NODE_PORT=3000 \
    XTLS_IP=127.0.0.1 \
    XTLS_PORT=61000

# Expose port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:61001/internal/get-config || exit 1

# Run
CMD ["/usr/local/bin/remnawave-node"]
