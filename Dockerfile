# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /remnawave-node ./cmd/node

# Final stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

RUN mkdir -p /var/lib/remnawave-node /var/log/remnawave-node

COPY --from=builder /remnawave-node /usr/local/bin/remnawave-node

ENV NODE_PORT=3000

EXPOSE 3000

CMD ["/usr/local/bin/remnawave-node"]
