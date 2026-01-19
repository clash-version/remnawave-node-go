# Build stage - use golang:1.25 (bookworm-based, not alpine)
FROM golang:1.25 AS builder

RUN apt-get update && apt-get install -y git && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /remnawave-node ./cmd/node

# Final stage - minimal runtime
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

RUN mkdir -p /var/lib/remnawave-node /var/log/remnawave-node

COPY --from=builder /remnawave-node /usr/local/bin/remnawave-node

ENV NODE_PORT=3000

EXPOSE 3000

CMD ["/usr/local/bin/remnawave-node"]