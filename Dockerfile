# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go.mod first for caching
COPY go.mod ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o shadowsend .

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary
COPY --from=builder /build/shadowsend /usr/local/bin/shadowsend

# Create non-root user
RUN adduser -D -u 1000 shadowsend
USER shadowsend

WORKDIR /home/shadowsend

ENTRYPOINT ["shadowsend"]
CMD ["--help"]
