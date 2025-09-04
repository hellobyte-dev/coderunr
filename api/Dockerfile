# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o server ./cmd/server

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    bash \
    curl \
    && rm -rf /var/cache/apk/*

# Create user for security
RUN addgroup -g 1000 coderunr && \
    adduser -D -s /bin/bash -u 1000 -G coderunr coderunr

# Create directories
RUN mkdir -p /opt/coderunr/packages && \
    chown -R coderunr:coderunr /opt/coderunr

# Copy binary from builder stage
COPY --from=builder /app/server /usr/local/bin/server

# Set permissions
RUN chmod +x /usr/local/bin/server

# Switch to non-root user
USER coderunr

# Set working directory
WORKDIR /opt/coderunr

# Set environment variables
ENV CODERUNR_DATA_DIRECTORY=/opt/coderunr
ENV CODERUNR_BIND_ADDRESS=0.0.0.0:2000
ENV CODERUNR_LOG_LEVEL=info

# Expose port
EXPOSE 2000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:2000/health || exit 1

# Run the server
CMD ["server"]
