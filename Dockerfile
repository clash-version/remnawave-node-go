# Build stage# Build stage# Build stage

FROM golang:1.25-alpine AS builder

# Install build dependencies

RUN apk add --no-cache git



# Set working directory# Install build dependencies# Install build dependencies

WORKDIR /app

RUN apk add --no-cache gitRUN apk add --no-cache git make

# Copy go mod files

COPY go.mod go.sum ./



# Download dependencies# Set working directory# Set working directory

RUN go mod download

WORKDIR /appWORKDIR /app

# Copy source code

COPY . .



# Build binary with embedded Xray-core# Copy go mod files# Copy go mod files

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /remnawave-node ./cmd/node

COPY go.mod go.sum ./COPY go.mod go.sum ./

# Final stage - minimal image

FROM alpine:3.19



# Install minimal runtime dependencies# Download dependencies# Download dependencies

RUN apk add --no-cache ca-certificates tzdata

RUN go mod downloadRUN go mod download

# Create directories

RUN mkdir -p /var/lib/remnawave-node /var/log/remnawave-node



# Copy binary from builder (includes embedded Xray-core)# Copy source code# Copy source code

COPY --from=builder /remnawave-node /usr/local/bin/remnawave-node

COPY . .COPY . .

# Environment variables

ENV NODE_PORT=3000



# Expose port# Build binary with embedded Xray-core# Build binary

EXPOSE 3000

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /remnawave-node ./cmd/nodeRUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /remnawave-node ./cmd/node

# Run the single binary (no supervisor needed)

CMD ["/usr/local/bin/remnawave-node"]


# Final stage - minimal image# Final stage

FROM alpine:3.19FROM alpine:3.19



# Install minimal runtime dependencies# Install runtime dependencies

RUN apk add --no-cache \RUN apk add --no-cache \

    ca-certificates \    ca-certificates \

    tzdata    tzdata \

    supervisor \

# Create directories    curl \

RUN mkdir -p /var/lib/remnawave-node /var/log/remnawave-node    bash



# Copy binary from builder (includes embedded Xray-core)# Install Xray-core

COPY --from=builder /remnawave-node /usr/local/bin/remnawave-nodeRUN curl -L -o /tmp/xray.zip https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip && \

    unzip /tmp/xray.zip -d /tmp/xray && \

# Environment variables    mv /tmp/xray/xray /usr/local/bin/xray && \

ENV NODE_PORT=3000    chmod +x /usr/local/bin/xray && \

    rm -rf /tmp/xray /tmp/xray.zip

# Expose port

EXPOSE 3000# Create directories

RUN mkdir -p /var/lib/remnawave-node /var/log/remnawave-node /etc/remnawave-node

# Health check - uses the main API port now

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \# Copy binary from builder

    CMD wget -q --spider http://127.0.0.1:${NODE_PORT}/health || exit 1COPY --from=builder /remnawave-node /usr/local/bin/remnawave-node



# Run the single binary (no supervisor needed)# Copy supervisor config

CMD ["/usr/local/bin/remnawave-node"]COPY deploy/supervisord.conf /etc/supervisor/conf.d/supervisord.conf


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
